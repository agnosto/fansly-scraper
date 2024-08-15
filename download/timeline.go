package download

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	//"strconv"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"

	"github.com/agnosto/fansly-scraper/headers"
	_ "modernc.org/sqlite"
)

type logWriter struct {
	d *Downloader
}

type Downloader struct {
	db              *sql.DB
	saveLocation    string
	authToken       string
	userAgent       string
	M3U8Download    bool
	headers         *headers.FanslyHeaders
	limiter         *rate.Limiter
	progressBar     *progressbar.ProgressBar
	logMu           sync.Mutex
	ffmpegAvailable bool
}

func (w logWriter) Write(p []byte) (n int, err error) {
	w.d.logMu.Lock()
	defer w.d.logMu.Unlock()
	w.d.progressBar.Clear()
	fmt.Print(string(p))
	w.d.progressBar.RenderBlank()
	return len(p), nil
}

func NewDownloader(cfg *config.Config, ffmpegAvailable bool) (*Downloader, error) {
	db, err := sql.Open("sqlite", filepath.Join(cfg.Options.SaveLocation, "downloads.db"))
	if err != nil {
		logger.Logger.Printf("Error: %v", err)
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (
        model TEXT NOT NULL,
        hash TEXT PRIMARY KEY,
        path TEXT NOT NULL,
        file_type TEXT NOT NULL
    )`)
	if err != nil {
		logger.Logger.Printf("Error: %v", err)
		return nil, err
	}

	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Fansly headers: %v", err)
	}

	limiter := rate.NewLimiter(rate.Every(2*time.Second), 3)

	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription("[cyan]Downloading[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	return &Downloader{
		db:              db,
		authToken:       cfg.Account.AuthToken,
		userAgent:       cfg.Account.UserAgent,
		saveLocation:    cfg.Options.SaveLocation,
		M3U8Download:    cfg.Options.M3U8Download,
		headers:         fanslyHeaders,
		limiter:         limiter,
		progressBar:     bar,
		ffmpegAvailable: ffmpegAvailable,
	}, nil
}

func (d *Downloader) DownloadTimeline(ctx context.Context, modelId, modelName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	timelinePosts, err := posts.GetAllTimelinePosts(modelId, d.authToken, d.userAgent)
	//log.Printf("Got all timeline posts for %v", modelName)
	//log.Printf("[TimelinePosts] Info: %v", timelinePosts)
	if err != nil {
		logger.Logger.Printf("[ERROR] [%s] Failed to get timeline posts: %v", modelName, err)
		return err
	}
	//log.Printf("Retrieved %d posts for %s", len(timelinePosts), modelName)

	baseDir := filepath.Join(d.saveLocation, strings.ToLower(modelName), "timeline")
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
			return err
		}
	}

	/*
	   parsingBar := progressbar.NewOptions(len(timelinePosts),
	       progressbar.OptionSetWriter(os.Stderr),
	       progressbar.OptionEnableColorCodes(true),
	       progressbar.OptionSetDescription("[cyan]Parsing Posts for Media[reset]"),
	       progressbar.OptionSetWidth(40),
	       progressbar.OptionThrottle(15*time.Millisecond),
	       progressbar.OptionShowCount(),
	       progressbar.OptionShowIts(),
	       progressbar.OptionSpinnerType(14),
	       progressbar.OptionFullWidth(),
	   )


	   var totalItems int
	   var mu sync.Mutex
	   semaphore := make(chan struct{}, 10)

	   var wg sync.WaitGroup
	   for _, post := range timelinePosts {
	       wg.Add(1)
	       go func(post posts.Post) {
	           defer wg.Done()
	           semaphore <- struct{}{} // Acquire semaphore
	           defer func() { <-semaphore }() // Release semaphore

	           accountMediaItems, err := posts.GetPostMedia(post.ID, d.authToken, d.userAgent)
	           if err != nil {
	               log.Printf("Error fetching media for post %s: %v", post.ID, err)
	               return
	           }

	           mu.Lock()
	           totalItems += len(accountMediaItems)
	           mu.Unlock()

	           parsingBar.Add(1)
	       }(post)
	   }
	   wg.Wait()
	   parsingBar.Finish()
	   parsingBar.Clear()
	*/
	// ^ This can be used to get a total amount of media items to use for the progress bar.
	// It does make the entire process take longer. (was too stupid and lazy to redo
	// the function below, to just use the items returned from it.)

	d.progressBar = progressbar.NewOptions(-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		//progressbar.OptionShowBytes(true),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetDescription("[green]Downloading[reset]"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(15*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	//customLogWriter := logWriter{d: d}
	//log.SetOutput(io.MultiWriter(os.Stderr, customLogWriter))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, post := range timelinePosts {
		wg.Add(1)
		go func(post posts.Post) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			accountMediaItems, err := posts.GetPostMedia(post.ID, d.authToken, d.userAgent)
			//log.Printf("Getting Media Items for Post: %v", post.ID)
			if err != nil {
				logger.Logger.Printf("[ERROR] [%s] Failed to fetch media for post %s: %v", modelName, post.ID, err)
				//log.Printf("Error fetching media for post %s: %v", post.ID, err)
				//if err.Error() == "rate limiter error: context canceled" {
				// This error indicates that the context was canceled, possibly due to the user stopping the process
				//    return
				//}

				return
			}

			for _, accountMedia := range accountMediaItems {
				//log.Printf("[ACCOUNT MEDIA]: %v", accountMedia)
				//log.Printf("Downloading media item %d/%d for post %d/%d: %v", j+1, len(accountMediaItems), i+1, totalItems, accountMedia.ID)
				err = d.downloadMediaItem(ctx, accountMedia, baseDir, post, modelName)
				//log.Printf("Downloading Media Item: %v", accountMedia.ID)
				if err != nil {
					logger.Logger.Printf("[ERROR] [%s] Failed to download media item %s: %v", modelName, accountMedia.ID, err)
					//log.Printf("Error downloading media item %s: %v", accountMedia.ID, err)
					continue
				}
				d.progressBar.Add(1)
			}
		}(post)
	}
	wg.Wait()
	//d.progressBar.Finish()
	d.progressBar.Clear()
	//wg.Wait()
	return nil
}

func (d *Downloader) downloadMediaItem(ctx context.Context, accountMedia posts.AccountMedia, baseDir string, post posts.Post, modelName string) error {
	hasValidLocations := func(item posts.MediaItem) bool {
		if len(item.Locations) > 0 {
			return true
		}
		for _, variant := range item.Variants {
			if len(variant.Locations) > 0 {
				return true
			}
		}
		return false
	}

	// Download main media if it has valid locations
	if hasValidLocations(accountMedia.Media) {
		err := d.downloadSingleItem(ctx, accountMedia.Media, baseDir, post.ID, modelName, false)
		if err != nil {
			logger.Logger.Printf("[ERROR] [%s] Failed to download main media item %s: %v", modelName, accountMedia.ID, err)
			return fmt.Errorf("error downloading main media: %v", err)
		}
	}

	// Check if there's a preview with valid locations and download it
	if accountMedia.Preview != nil && hasValidLocations(*accountMedia.Preview) {
		err := d.downloadSingleItem(ctx, *accountMedia.Preview, baseDir, post.ID, modelName, true)
		if err != nil {
			logger.Logger.Printf("[ERROR] [%s] Failed to download preview item for media item %s : %v", modelName, accountMedia.ID, err)
			return fmt.Errorf("error downloading preview: %v", err)
		}
	}

	// If neither main media nor preview has valid locations, log a warning
	if !hasValidLocations(accountMedia.Media) && (accountMedia.Preview == nil || !hasValidLocations(*accountMedia.Preview)) {
		d.progressBar.Describe(fmt.Sprintf("[yellow]No valid media or preview locations[reset] for item %s", accountMedia.ID))
	}

	return nil
}

func (d *Downloader) downloadSingleItem(ctx context.Context, item posts.MediaItem, baseDir string, identifier string, modelName string, isPreview bool) error {
	var bestMedia *posts.MediaItem
	var bestHeight int
	var bestMetadata map[string]string
	var mediaUrl string
	var fileType string

	processMediaItem := func(mediaItem posts.MediaItem) {
		if len(mediaItem.Locations) > 0 && mediaItem.Height > bestHeight {
			bestMedia = &mediaItem
			bestHeight = mediaItem.Height
			bestMetadata = mediaItem.Locations[0].Metadata
			mediaUrl = mediaItem.Locations[0].Location
			//log.Printf("\n[Main Media Height] H: %v", bestHeight)
			//log.Printf("\n[MAIN Media URL] %v\n", mediaUrl)
		}
		if d.M3U8Download && d.ffmpegAvailable {
			//log.Printf("[FFMPEG] Status: %v", d.ffmpegAvailable)
			for _, variant := range mediaItem.Variants {
				if variant.Mimetype == "application/vnd.apple.mpegurl" && variant.Height > bestHeight {
					if len(variant.Locations) > 0 {
						bestMedia = &posts.MediaItem{
							ID:        variant.ID,
							Type:      variant.Type,
							Height:    variant.Height,
							Mimetype:  variant.Mimetype,
							Locations: variant.Locations,
						}
						bestHeight = variant.Height
						bestMetadata = variant.Locations[0].Metadata
						mediaUrl = variant.Locations[0].Location
						//log.Printf("\n[Variant Media Height] %v", bestHeight)
						//log.Printf("\n[Variant Media URL] %v\n", mediaUrl)
					}
				}
			}
		}
	}

	processMediaItem(item)
	//logger.Logger.Printf("[INFO] [%s] BestMedia URL: %v", modelName, mediaUrl)
	//log.Printf("[BEST MEDIA] URL IS: %v", mediaUrl)

	if bestMedia == nil || mediaUrl == "" {
		d.progressBar.Describe(fmt.Sprintf("[red]No suitable media found[reset] for item %s", item.ID))
		return nil
	}

	var subDir string
	switch {
	case strings.HasPrefix(item.Mimetype, "image/"):
		subDir = "images"
		fileType = "image"
	case strings.HasPrefix(item.Mimetype, "video/") || item.Mimetype == "application/vnd.apple.mpegurl":
		subDir = "videos"
		fileType = "video"
		if item.Mimetype == "application/vnd.apple.mpegurl" {
			item.Mimetype = "video/mp4"
		}
	case strings.HasPrefix(item.Mimetype, "audio/"):
		subDir = "audios"
		fileType = "audio"
		if item.Mimetype == "audio/mp4" {
			item.Mimetype = "audio/mp3"
		}
	default:
		return fmt.Errorf("unknown media type: %s", item.Mimetype)
	}

	parsedURL, err := url.Parse(mediaUrl)
	if err != nil {
		return fmt.Errorf("error parsing URL: %v", err)
	}
	ext := filepath.Ext(parsedURL.Path)

	if strings.HasSuffix(mediaUrl, ".m3u8") {
		ext = ".mp4" // We'll still save as .mp4 even though it's originally m3u8
	}

	previewSuffix := ""
	if isPreview {
		previewSuffix = "_preview"
	}
	fileName := fmt.Sprintf("%s_%s%s%s", identifier, bestMedia.ID, previewSuffix, ext)
	filePath := filepath.Join(baseDir, subDir, fileName)
	//log.Printf("[INFO] [DLSingleItem] FILENAME: %v", fileName)

	if d.fileExists(filePath) {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			d.progressBar.Describe(fmt.Sprintf("[yellow]File missing[reset], Redownloading %s", fileName))
		} else {
			d.progressBar.Describe(fmt.Sprintf("[red]File Exists[reset], Skipping %s", fileName))
			//log.Printf("File already exists, skipping: %s\n", filePath)
			return nil
		}
	}

	// Check if the file actually exists on the filesystem
	if _, err := os.Stat(filePath); err == nil {
		d.progressBar.Describe(fmt.Sprintf("[yellow]No DB Record[reset], Adding %s", fileName))
		//log.Printf("File exists on filesystem but not in DB, adding to DB: %s\n", filePath)
		hashString, err := d.hashExistingFile(filePath)
		if err != nil {
			logger.Logger.Printf("[ERROR] failed to hash existing file %s: %v", filePath, err)
			return fmt.Errorf("error hashing existing file: %v", err)
		}
		err = d.saveFileHash(modelName, hashString, filePath, fileType)
		if err != nil {
			logger.Logger.Printf("[ERROR] failed to save hash for existing file %s: %v", filePath, err)
			return fmt.Errorf("error saving hash for existing file: %v", err)
		}
		return nil
	}

	d.progressBar.Describe(fmt.Sprintf("[green]Downloading[reset] %s", fileName))

	if strings.HasSuffix(mediaUrl, ".m3u8") && d.ffmpegAvailable {
		fullUrl := mediaUrl
		if bestMetadata != nil {
			fullUrl += fmt.Sprintf("?ngsw-bypass=true&Policy=%s&Key-Pair-Id=%s&Signature=%s",
				url.QueryEscape(bestMetadata["Policy"]),
				url.QueryEscape(bestMetadata["Key-Pair-Id"]),
				url.QueryEscape(bestMetadata["Signature"]))
		}
		return d.DownloadM3U8(ctx, modelName, fullUrl, filePath)
	}

	d.downloadRegularFile(mediaUrl, filePath, modelName, fileType)

	//log.Printf("Downloaded: %s\n", filePath)

	return nil
}

func (d *Downloader) downloadWithRetry(url string) (*http.Response, error) {
	backoff := time.Second
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		//log.Printf("[dlWithRetry] Download attempt: %v", i)
		if err := d.limiter.Wait(context.Background()); err != nil {
			return nil, fmt.Errorf("rate limiter wait error: %v", err)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		//req.Header.Add("Authorization", d.authToken)
		//req.Header.Add("User-Agent", d.userAgent)
		d.headers.AddHeadersToRequest(req, true)

		if strings.HasSuffix(url, ".m3u8") {
			req.Header.Add("Accept", "*/*")
			req.Header.Add("Origin", "https://fansly.com")
			req.Header.Add("Referer", "https://fansly.com/")
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	return nil, fmt.Errorf("failed to download %s after %d retries", url, maxRetries)
}

func (d *Downloader) downloadRegularFile(url, filePath string, modelName string, fileType string) error {
	//log.Printf("[Download URL] URL: %v", url)
	if err := d.limiter.Wait(context.Background()); err != nil {
		return fmt.Errorf("rate limiter wait error: %v", err)
	}

	resp, err := d.downloadWithRetry(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	hash := sha256.New()
	tee := io.TeeReader(resp.Body, hash)

	//_, err = io.Copy(io.MultiWriter(out, d.progressBar), tee)
	_, err = io.Copy(out, tee)
	if err != nil {
		return err
	}

	hashString := hex.EncodeToString(hash.Sum(nil))
	err = d.saveFileHash(modelName, hashString, filePath, fileType)
	if err != nil {
		return err
	}

	//_, err = io.Copy(out, resp.Body)
	//if err != nil {
	//    return err
	//}

	return nil
}

func (d *Downloader) fileExists(filePath string) bool {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", filePath).Scan(&count)
	if err != nil {
		logger.Logger.Printf("[ERROR] Failed checking if file exists in DB: %v", err)
		//log.Printf("Error checking if file exists in DB: %v", err)
		return false
	}
	return count > 0
}

func (d *Downloader) hashExistingFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (d *Downloader) saveFileHash(modelName string, hash, path, fileType string) error {
	_, err := d.db.Exec("INSERT OR REPLACE INTO files (model, hash, path, file_type) VALUES (?, ?, ?, ?)", modelName, hash, path, fileType)
	return err
}

func (d *Downloader) Close() error {
	return d.db.Close()
}
