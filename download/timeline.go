package download

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	//"github.com/dustin/go-humanize"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/db"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/db/service"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"

	"github.com/agnosto/fansly-scraper/headers"
)

type logWriter struct {
	d *Downloader
}

type Downloader struct {
	db                   *sql.DB
	saveLocation         string
	authToken            string
	userAgent            string
	M3U8Download         bool
	headers              *headers.FanslyHeaders
	limiter              *rate.Limiter
	progressBar          *progressbar.ProgressBar
	logMu                sync.Mutex
	ffmpegAvailable      bool
	fileService          *service.FileService
	ProcessedPostService *service.ProcessedPostService
	cfg                  *config.Config
	m3u8Semaphore        *semaphore.Weighted
}

func (w logWriter) Write(p []byte) (n int, err error) {
	w.d.logMu.Lock()
	defer w.d.logMu.Unlock()
	w.d.progressBar.Clear()
	fmt.Print(string(p))
	w.d.progressBar.RenderBlank()
	return len(p), nil
}

func (d *Downloader) fileExists(filePath string) bool {
	return d.fileService.FileExists(filePath)
}

func (d *Downloader) saveFileHash(modelName string, hash, path, fileType, postID string) error {
	return d.fileService.SaveFile(modelName, hash, path, fileType, postID)
}

func NewDownloader(cfg *config.Config, ffmpegAvailable bool) (*Downloader, error) {
	// Initialize database
	database, err := db.NewDatabase(cfg.Options.SaveLocation)
	if err != nil {
		logger.Logger.Printf("Error initializing database: %v", err)
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// New processed post service setup
	postRepo := repository.NewProcessedPostRepository(database.DB)
	processedPostService := service.NewProcessedPostService(postRepo)

	fileRepo := repository.NewFileRepository(database.DB)
	fileService := service.NewFileService(fileRepo)

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
		fileService:          fileService,
		ProcessedPostService: processedPostService,
		authToken:            cfg.Account.AuthToken,
		userAgent:            cfg.Account.UserAgent,
		saveLocation:         cfg.Options.SaveLocation,
		M3U8Download:         cfg.Options.M3U8Download,
		headers:              fanslyHeaders,
		limiter:              limiter,
		progressBar:          bar,
		ffmpegAvailable:      ffmpegAvailable,
		cfg:                  cfg,
		m3u8Semaphore:        semaphore.NewWeighted(2),
	}, nil
}

func (d *Downloader) DownloadTimeline(ctx context.Context, modelId, modelName string, wallID string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	timelinePosts, err := posts.GetAllTimelinePosts(modelId, wallID, d.headers, d.cfg.Options.PostLimit)
	if err != nil {
		logger.Logger.Printf("[ERROR] [%s] Failed to get timeline posts: %v", modelName, err)
		return err
	}

	folderName := "timeline"
	if wallID != "" {
		folderName = fmt.Sprintf("wall_%s", wallID)
	}

	baseDir := filepath.Join(d.saveLocation, strings.ToLower(modelName), folderName)
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
			return err
		}
	}

	d.progressBar = progressbar.NewOptions(len(timelinePosts),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]Processing %s Timeline[reset]", modelName)),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(30*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	tickerDone := make(chan bool)
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-tickerDone:
				return
			case <-ticker.C:
				if d.progressBar != nil {
					d.progressBar.Add(0)
				}
			}
		}
	}()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, post := range timelinePosts {
		wg.Add(1)
		go func(post posts.Post) {
			defer wg.Done()

			shouldSkipFiles := d.cfg.Options.SkipDownloadedPosts && d.ProcessedPostService.PostExists(post.ID)

			if !shouldSkipFiles {
				semaphore <- struct{}{}
				// We only fetch media and download if we aren't skipping
				accountMediaItems, err := posts.GetPostMedia(post.ID, d.headers)
				if err != nil {
					logger.Logger.Printf("[ERROR] [%s] Failed to fetch media for post %s: %v", modelName, post.ID, err)
					d.progressBar.Add(1)
					<-semaphore
					return
				}

				for i, accountMedia := range accountMediaItems {
					err = d.DownloadMediaItem(ctx, accountMedia, baseDir, modelName, post, i)
					if err != nil {
						logger.Logger.Printf("[ERROR] [%s] Failed to download media item %s: %v", modelName, accountMedia.ID, err)
						continue
					}
				}
				<-semaphore
			} else {
				logger.Logger.Printf("[INFO] [%s] Skipping files for post %s, but updating metadata", modelName, post.ID)
			}

			// always save/update metadata
			err := d.ProcessedPostService.MarkPostAsProcessed(post.ID, modelName, post.Content, post.CreatedAt)
			if err != nil {
				logger.Logger.Printf("[ERROR] [%s] Failed to save post metadata %s: %v", modelName, post.ID, err)
			}

			d.progressBar.Add(1)
		}(post)
	}
	wg.Wait()

	tickerDone <- true
	close(tickerDone)

	d.progressBar.Finish()
	//d.progressBar.Clear()
	//fmt.Print("\033[2K\r")
	fmt.Println()
	return nil
}

func (d *Downloader) DownloadMediaItem(ctx context.Context, accountMedia posts.AccountMedia, baseDir, modelName string, contentSource any, index int, isDiagnosis ...bool) error {
	diagMode := len(isDiagnosis) > 0 && isDiagnosis[0]

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

	if hasValidLocations(accountMedia.Media) {
		// Use `contentSource` which can be a post, message, or anything else
		err := d.downloadSingleItem(ctx, accountMedia.Media, baseDir, modelName, false, contentSource, index, diagMode)
		if err != nil {
			logger.Logger.Printf("[ERROR] [%s] Failed to download main media item %s: %v", modelName, accountMedia.ID, err)
			return fmt.Errorf("error downloading main media: %v", err)
		}
	}

	if !d.cfg.Options.SkipPreviews && accountMedia.Preview != nil && hasValidLocations(*accountMedia.Preview) {
		// Use `contentSource` here as well
		err := d.downloadSingleItem(ctx, *accountMedia.Preview, baseDir, modelName, true, contentSource, index, diagMode)
		if err != nil {
			logger.Logger.Printf("[ERROR] [%s] Failed to download preview item for media item %s : %v", modelName, accountMedia.ID, err)
			return fmt.Errorf("error downloading preview: %v", err)
		}
	}

	if !hasValidLocations(accountMedia.Media) && (accountMedia.Preview == nil || !hasValidLocations(*accountMedia.Preview)) {
		d.progressBar.Describe(fmt.Sprintf("[yellow]No valid media or preview locations[reset] for item %s", accountMedia.ID))
	}

	return nil
}

func (d *Downloader) generateFilename(bestMedia posts.MediaItem, modelName string, contentSource any, index int, isPreview bool, ext string) string {
	suffix := ""
	if isPreview {
		suffix = "_preview"
	}

	var sourceID string
	var date int64
	var textContent string
	var shouldUseIndex bool

	// Extract data based on the type of content (Post, Message, Story, etc.)
	if contentSource != nil {
		switch v := contentSource.(type) {
		case posts.Post:
			textContent = v.Content
			date = v.CreatedAt

			// Check if this is the "Purchases" album
			if textContent == "Purchases" {
				// Purchases: Do NOT use index (unstable), use MediaID as source
				shouldUseIndex = false
				sourceID = bestMedia.ID
			} else {
				// Timeline: Use PostID and Index
				shouldUseIndex = true
				sourceID = v.ID
			}
		case posts.Message:
			sourceID = v.ID
			textContent = v.Content
			date = v.CreatedAt
			shouldUseIndex = true // Messages often contain bundles in specific order
		case posts.Story:
			sourceID = v.ID
			date = v.CreatedAt
			shouldUseIndex = false // Stories are usually 1:1, index not required
		case posts.PostInfo: // Handle cases where PostInfo is passed directly
			sourceID = v.ID
			textContent = v.Content
			date = v.CreatedAt
			shouldUseIndex = true
		default:
			// Fallback for Purchases, Profile, or unknown types
			// We use the MediaID as the SourceID to ensure uniqueness without an unstable index
			sourceID = bestMedia.ID
			shouldUseIndex = false
		}
	} else {
		// Fallback if no source is provided
		sourceID = bestMedia.ID
		shouldUseIndex = false
	}

	// 2. Mode A: Default ID-Based Naming (Config: use_content_as_filename = false)
	if !d.cfg.Options.UseContentAsFilename {
		if shouldUseIndex {
			// Format: PostID_Index_MediaID (e.g., 84458..._01_84458... .mp4)
			// We use %02d to pad the index (01, 02, 10) for correct filesystem sorting
			return fmt.Sprintf("%s_%02d_%s%s%s", sourceID, index+1, bestMedia.ID, suffix, ext)
		}
		// Format: SourceID_MediaID (e.g. StoryID_MediaID)
		// If sourceID was set to MediaID in fallback, this effectively becomes MediaID_MediaID,
		// so we check to avoid redundancy
		if sourceID == bestMedia.ID {
			return fmt.Sprintf("%s%s%s", bestMedia.ID, suffix, ext)
		}
		return fmt.Sprintf("%s_%s%s%s", sourceID, bestMedia.ID, suffix, ext)
	}

	// 3. Mode B: Content-Based Naming (Config: use_content_as_filename = true)

	// Check if meaningful text content exists
	hasTextContent := textContent != "" && textContent != "Purchases"

	if hasTextContent {
		// Define Date Format
		dateFormat := d.cfg.Options.DateFormat
		if dateFormat == "" {
			dateFormat = "20060102"
		}
		dateStr := time.Unix(date, 0).Format(dateFormat)

		// Sanitize and Limit Content Text
		cleanContent := strings.ReplaceAll(textContent, "\n", " ")
		runes := []rune(cleanContent)
		charLimit := d.cfg.Options.ContentFilenameLength
		if charLimit <= 0 {
			charLimit = 50
		}
		if len(runes) > charLimit {
			runes = runes[:charLimit]
		}
		cleanContent = string(runes)

		// Get Template
		template := d.cfg.Options.ContentFilenameTemplate
		if template == "" {
			template = "{date}-{content}_{index}"
		}

		// Replace Variables
		// We allow {postId} and {mediaId} here so you can achieve custom ID formats via config if desired
		r := strings.NewReplacer(
			"{date}", dateStr,
			"{content}", cleanContent,
			"{index}", fmt.Sprintf("%d", index+1),
			"{postId}", sourceID,
			"{mediaId}", bestMedia.ID,
			"{model_name}", modelName,
		)

		baseName := r.Replace(template)
		return config.SanitizeFilename(baseName) + suffix + ext
	}

	// Fallback for Content Mode when NO text is present
	// Defaults to Date_Model_MediaID_Index to ensure uniqueness
	var dateStr string
	if date > 0 {
		dateStr = time.Unix(date, 0).Format("20060102")
	} else {
		dateStr = time.Now().Format("20060102")
	}

	baseName := fmt.Sprintf("%s_%s_%s_%d", dateStr, modelName, bestMedia.ID, index+1)
	return config.SanitizeFilename(baseName) + suffix + ext
}

func (d *Downloader) downloadSingleItem(ctx context.Context, item posts.MediaItem, baseDir, modelName string, isPreview bool, contentSource any, index int, isDiagnosis bool) error {
	var mediaItems = []posts.MediaItem{}

	getMediaType := func(mimetype string) string {
		switch {
		case strings.HasPrefix(mimetype, "image/"):
			return "image"
		case strings.HasPrefix(mimetype, "video/") || mimetype == "application/vnd.apple.mpegurl":
			return "video"
		case strings.HasPrefix(mimetype, "audio/"):
			return "audio"
		default:
			return "unknown"
		}
	}

	var sourceID string
	if contentSource != nil {
		switch v := contentSource.(type) {
		case posts.Post:
			sourceID = v.ID
		case posts.Message:
			sourceID = v.ID
		case posts.Story:
			sourceID = v.ID
		case posts.PostInfo:
			sourceID = v.ID
		}
	}

	mainType := getMediaType(item.Mimetype)

	processMediaItem := func(mediaItem posts.MediaItem) {
		if len(mediaItem.Locations) > 0 {
			mediaItems = append(mediaItems, mediaItem)
		}

		for _, variant := range mediaItem.Variants {
			variantType := getMediaType(variant.Mimetype)

			if variant.Mimetype == "application/vnd.apple.mpegurl" && !d.M3U8Download {
				continue
			}

			if variantType == mainType && len(variant.Locations) > 0 {
				mediaItems = append(mediaItems, posts.MediaItem{
					ID:        variant.ID,
					Type:      variant.Type,
					Height:    variant.Height,
					Mimetype:  variant.Mimetype,
					Locations: variant.Locations,
				})
			}
		}
	}

	processMediaItem(item)

	if len(mediaItems) == 0 {
		d.progressBar.Describe(fmt.Sprintf("[red]No suitable media found[reset] for item %s", item.ID))
		return nil
	}

	sort.Slice(mediaItems, func(i, j int) bool {
		return mediaItems[j].Height < mediaItems[i].Height
	})

	bestMedia := mediaItems[0]

	var fileType, subDir string
	switch {
	case strings.HasPrefix(item.Mimetype, "image/"):
		subDir, fileType = "images", "image"
	case strings.HasPrefix(item.Mimetype, "video/") || item.Mimetype == "application/vnd.apple.mpegurl":
		subDir, fileType = "videos", "video"
		if item.Mimetype == "application/vnd.apple.mpegurl" {
			item.Mimetype = "video/mp4"
		}
	case strings.HasPrefix(item.Mimetype, "audio/"):
		subDir, fileType = "audios", "audio"
		if item.Mimetype == "audio/mp4" {
			item.Mimetype = "audio/mp3"
		}
	default:
		return fmt.Errorf("unknown media type: %s", item.Mimetype)
	}

	mediaTypeFilter := d.cfg.Options.DownloadMediaType
	normalizedFilter := strings.TrimSuffix(mediaTypeFilter, "s")
	if normalizedFilter != "all" && normalizedFilter != fileType {
		d.progressBar.Describe(fmt.Sprintf("[yellow]Skipping[reset] %s due to media type filter (%s)", item.ID, mediaTypeFilter))
		return nil // Not an error, just skipping
	}

	mediaUrl := bestMedia.Locations[0].Location
	parsedURL, err := url.Parse(mediaUrl)
	if err != nil {
		return fmt.Errorf("error parsing URL: %v", err)
	}

	logger.Logger.Printf("Trying to download (%s) %s", bestMedia.Mimetype, mediaUrl)

	ext := filepath.Ext(parsedURL.Path)
	if strings.HasSuffix(mediaUrl, ".m3u8") {
		ext = ".mp4"
	}

	// Generate filename using the new centralized function
	fileName := d.generateFilename(bestMedia, modelName, contentSource, index, isPreview, ext)
	filePath := filepath.Join(baseDir, subDir, fileName)

	// ... (Rest of downloadSingleItem logic remains the same)
	if !isDiagnosis {
		if d.fileExists(filePath) {
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				d.progressBar.Describe(fmt.Sprintf("[yellow]File missing[reset], Redownloading %s", fileName))
			} else {
				d.progressBar.Describe(fmt.Sprintf("[red]File Exists[reset], Skipping %s", fileName))
				d.progressBar.Add(1) // Make sure to increment progress bar even on skip
				return nil
			}
		}

		if _, err := os.Stat(filePath); err == nil {
			d.progressBar.Describe(fmt.Sprintf("[yellow]No DB Record[reset], Adding %s", fileName))
			hashString, err := d.hashExistingFile(filePath)
			if err != nil {
				logger.Logger.Printf("[ERROR] failed to hash existing file %s: %v", filePath, err)
				return fmt.Errorf("error hashing existing file: %v", err)
			}
			err = d.saveFileHash(modelName, hashString, filePath, fileType, sourceID)
			if err != nil {
				logger.Logger.Printf("[ERROR] failed to save hash for existing file %s: %v", filePath, err)
				return fmt.Errorf("error saving hash for existing file: %v", err)
			}
			d.progressBar.Add(1)
			return nil
		}
	}

	d.progressBar.Describe(fmt.Sprintf("[green]Downloading[reset] %s", fileName))

	if bestMedia.Mimetype == "application/vnd.apple.mpegurl" && d.ffmpegAvailable {
		fullUrl := mediaUrl
		metadata := bestMedia.Locations[0].Metadata
		var frameRate float64

		if bestMedia.Metadata != "" {
			var meta struct {
				FrameRate float64 `json:"frameRate"`
			}
			if err := json.Unmarshal([]byte(bestMedia.Metadata), &meta); err == nil {
				frameRate = meta.FrameRate
				logger.Logger.Printf("Found frame rate %f for M3U8 stream", frameRate)
			}
		}

		var sourceID string
		if p, ok := contentSource.(posts.Post); ok {
			sourceID = p.ID
		} else if m, ok := contentSource.(posts.Message); ok {
			sourceID = m.ID
		}
		if metadata != nil {
			fullUrl += fmt.Sprintf("?ngsw-bypass=true&Policy=%s&Key-Pair-Id=%s&Signature=%s",
				url.QueryEscape(metadata["Policy"]),
				url.QueryEscape(metadata["Key-Pair-Id"]),
				url.QueryEscape(metadata["Signature"]))
		}
		return d.DownloadM3U8(ctx, modelName, fullUrl, filePath, sourceID, frameRate, isDiagnosis)
	}

	return d.downloadRegularFile(mediaUrl, filePath, modelName, fileType, sourceID, isDiagnosis)
}

func (d *Downloader) downloadWithRetry(url string) (*http.Response, error) {
	backoff := time.Second
	maxRetries := 3

	for range maxRetries {
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

func (d *Downloader) downloadRegularFile(url, filePath string, modelName string, fileType, postID string, isDiagnosis bool) error {
	if err := d.limiter.Wait(context.Background()); err != nil {
		return fmt.Errorf("rate limiter wait error: %v", err)
	}

	resp, err := d.downloadWithRetry(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	/*d.progressBar = progressbar.NewOptions(
		-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription(fmt.Sprintf("[green]Downloading[reset] %s (%s)",
			filepath.Base(filePath),
			humanize.Bytes(uint64(resp.ContentLength)))),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(15*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)*/

	// Update the description of the main bar
	d.progressBar.Describe(fmt.Sprintf("[cyan]Downloading[reset] %s", filepath.Base(filePath)))

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), resp.Body)
	if err != nil {
		return err
	}

	hashString := hex.EncodeToString(hash.Sum(nil))
	//return d.saveFileHash(modelName, hashString, filePath, fileType)
	if !isDiagnosis {
		return d.saveFileHash(modelName, hashString, filePath, fileType, postID)
	}

	return nil
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

func (d *Downloader) Close() error {
	return d.db.Close()
}
