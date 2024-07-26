package download

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
    //"strconv"

	"golang.org/x/time/rate"
    "github.com/schollz/progressbar/v3"
    //"github.com/k0kubun/go-ansi"

	"go-fansly-scraper/config"
	"go-fansly-scraper/posts"

	_ "github.com/mattn/go-sqlite3"
	//"go-fansly-scraper/headers"
)

type logWriter struct {
    d *Downloader
}

type Downloader struct {
    db           *sql.DB
    saveLocation string
    authToken    string
    userAgent    string
    //headers      *headers.FanslyHeaders
    limiter      *rate.Limiter
    progressBar  *progressbar.ProgressBar
    logMu        sync.Mutex
}

func (w logWriter) Write(p []byte) (n int, err error) {
    w.d.logMu.Lock()
    defer w.d.logMu.Unlock()
    w.d.progressBar.Clear()
    fmt.Print(string(p))
    w.d.progressBar.RenderBlank()
    return len(p), nil
}

 

func NewDownloader(cfg *config.Config) (*Downloader, error) {
    db, err := sql.Open("sqlite3", filepath.Join(cfg.SaveLocation, "downloads.db"))
    if err != nil {
        return nil, err
    }

    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (
        model TEXT NOT NULL,
        hash TEXT PRIMARY KEY,
        path TEXT NOT NULL
    )`)
    if err != nil {
        return nil, err
    }

    /*
    deviceID, err := headers.GetDeviceID()
    if err != nil {
        return nil, err
    } 

    fanslyHeaders := &headers.FanslyHeaders{
        AuthToken: cfg.Authorization,
        UserAgent: cfg.UserAgent,
        DeviceID:  deviceID,
    }

    err = fanslyHeaders.SetCheckKey()
    if err != nil {
        return nil, err
    }

    // Set the session ID
    err = fanslyHeaders.SetSessionID()
    if err != nil {
        return nil, fmt.Errorf("failed to set session ID: %v", err)
    }
    */

    limiter := rate.NewLimiter(rate.Every(2*time.Second), 1)

    bar := progressbar.NewOptions(-1,
        progressbar.OptionSetWriter(os.Stderr),
        progressbar.OptionEnableColorCodes(true),
        progressbar.OptionShowBytes(true),
        progressbar.OptionSetWidth(15),
        progressbar.OptionThrottle(65 * time.Millisecond),
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
        db:           db,
        authToken:    cfg.Authorization,
        userAgent:    cfg.UserAgent,
        saveLocation: cfg.SaveLocation,
        //headers:      fanslyHeaders,
        limiter:      limiter,
        progressBar: bar,
    }, nil
}

func (d *Downloader) DownloadTimeline(ctx context.Context, modelId, modelName string) error { 
    timelinePosts, err := posts.GetAllTimelinePosts(modelId, d.authToken, d.userAgent)
    //log.Printf("Got all timeline posts for %v", modelName)
    //log.Printf("[TimelinePosts] Info: %v", timelinePosts)
    if err != nil {
        return err
    }
    //log.Printf("Retrieved %d posts for %s", len(timelinePosts), modelName)

    baseDir := filepath.Join(d.saveLocation, modelName, "timeline")
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
                log.Printf("Error fetching media for post %s: %v", post.ID, err)
                //if err.Error() == "rate limiter error: context canceled" {
                    // This error indicates that the context was canceled, possibly due to the user stopping the process
                //    return
                //}
                
                return
            }

            for _, accountMedia := range accountMediaItems { 
                //log.Printf("[ACCOUNT MEDIA]: %v", accountMedia)
                //log.Printf("Downloading media item %d/%d for post %d/%d: %v", j+1, len(accountMediaItems), i+1, totalItems, accountMedia.ID)
                err = d.downloadMediaItem(accountMedia, baseDir, modelId, modelName)
                //log.Printf("Downloading Media Item: %v", accountMedia.ID)
                if err != nil {
                    log.Printf("Error downloading media item %s: %v", accountMedia.ID, err)
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

func (d *Downloader) downloadMediaItem(accountMedia posts.AccountMedia, baseDir string, modelId string, modelName string) error {
    // Download main media
    //log.Printf("[INFO] [DLMediaItem] PREVIEW ITEM: %v", accountMedia.Preview)
    //err := d.downloadSingleItem(accountMedia.Media, baseDir, modelId, false, currentItem, totalItems, progressChan)
    //if err != nil {
    //    return fmt.Errorf("error downloading main media: %v", err)
    //}

    // Check if there's a preview and download it
    /*
    if accountMedia.Preview != nil {
        previewItem := *accountMedia.Preview
        //previewItem.ID += "_preview"
        //log.Printf("[INFO] PREVIEW ITEM: %v", previewItem)
        err := d.downloadSingleItem(previewItem, baseDir, modelId, true)
        if err != nil {
            return fmt.Errorf("error downloading preview: %v", err)
        }
    } else {
        //log.Printf("[INFO] [DLMediaItem] MAIN ITEM: %v", accountMedia.Media)
        err := d.downloadSingleItem(accountMedia.Media, baseDir, modelId, false)
        if err != nil { 
            return fmt.Errorf("error downloading main media: %v", err)
        }
    }
    */ 

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
        err := d.downloadSingleItem(accountMedia.Media, baseDir, modelId, modelName, false)
        if err != nil {
            return fmt.Errorf("error downloading main media: %v", err)
        }
    }

    // Check if there's a preview with valid locations and download it
    if accountMedia.Preview != nil && hasValidLocations(*accountMedia.Preview) {
        err := d.downloadSingleItem(*accountMedia.Preview, baseDir, modelId, modelName, true)
        if err != nil {
            return fmt.Errorf("error downloading preview: %v", err)
        }
    }

    // If neither main media nor preview has valid locations, log a warning
    if !hasValidLocations(accountMedia.Media) && (accountMedia.Preview == nil || !hasValidLocations(*accountMedia.Preview)) {
        d.progressBar.Describe(fmt.Sprintf("[yellow]No valid media or preview locations[reset] for item %s", accountMedia.ID))
    }


    return nil
}

func (d *Downloader) downloadSingleItem(item posts.MediaItem, baseDir string, modelId string, modelName string, isPreview bool) error {
    var subDir string
    switch {
    case strings.HasPrefix(item.Mimetype, "image/"):
        subDir = "images"
    case strings.HasPrefix(item.Mimetype, "video/") || item.Mimetype == "application/vnd.apple.mpegurl":
        subDir = "videos"
        if item.Mimetype == "application/vnd.apple.mpegurl" {
            item.Mimetype = "video/mp4"
        }
    case strings.HasPrefix(item.Mimetype, "audio/"):
        subDir = "audios"
        if item.Mimetype == "audio/mp4" {
            item.Mimetype = "audio/mp3"
        }
    default:
        return fmt.Errorf("unknown media type: %s", item.Mimetype)
    }

    mediaUrl := ""
    variantId := item.ID 
    maxHeight := 0
    /*
    if len(item.Locations) > 0 && item.Locations[0].Location != "" {
        mediaUrl = item.Locations[0].Location 
        //log.Printf("[INFO] [DLSingleItem] ITEM: %v", item)
        //log.Printf("[INFO] [DLSingleItem] ITEM.LOCATIONS: %v", item.Locations[0].Location)
    } else if len(item.Variants) > 0 {
        // If main item doesn't have a location, use the first variant with a valid location
        for _, variant := range item.Variants {
            if len(variant.Locations) > 0 && variant.Locations[0].Location != "" {
                mediaUrl = variant.Locations[0].Location
                variantId = variant.ID 
                break
            }
        }
    }
    */ 

    processLocations := func(locations []struct{ Location string `json:"location"`}) bool {
        if len(locations) > 0 && locations[0].Location != "" {
            mediaUrl = locations[0].Location
            return true
        }
        return false
    }

    // Check main item locations
    if processLocations(item.Locations) {
        maxHeight = item.Height
    } else {
        // If main item doesn't have a location, check variants
        for _, variant := range item.Variants {
            if processLocations(variant.Locations) {
                if variant.Height > maxHeight {
                    maxHeight = variant.Height
                    mediaUrl = variant.Locations[0].Location
                    //variantId = variant.ID
                }
            }
        }
    }

    if mediaUrl == "" {
        d.progressBar.Describe(fmt.Sprintf("[red]No Media URL[reset] for item %s", item.ID))
        //log.Printf("Warning: No valid media URL found for item %s, skipping", item.ID)
        return nil
    }

    parsedURL, err := url.Parse(mediaUrl)
    if err != nil {
        return fmt.Errorf("error parsing URL: %v", err)
    }
    ext := filepath.Ext(parsedURL.Path)

    previewSuffix := ""
    if isPreview {
        previewSuffix = "_preview"
    }
    fileName := fmt.Sprintf("%s_%s%s%s", modelId, variantId, previewSuffix, ext)
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
        d.progressBar.Describe(fmt.Sprintf("[yellow]No DB Record[reset], Adding %s", filePath))
        //log.Printf("File exists on filesystem but not in DB, adding to DB: %s\n", filePath)
        hashString, err := d.hashExistingFile(filePath)
        if err != nil {
            return fmt.Errorf("error hashing existing file: %v", err)
        }
        err = d.saveFileHash(modelName, hashString, filePath)
        if err != nil {
            return fmt.Errorf("error saving hash for existing file: %v", err)
        }
        return nil
    }
    
    d.progressBar.Describe(fmt.Sprintf("[green]Downloading[reset] %s", fileName))

    if err := d.limiter.Wait(context.Background()); err != nil {
        return fmt.Errorf("rate limiter wait error: %v", err)
    }

    // Create a new request with the necessary headers
    //req, err := http.NewRequest("GET", mediaUrl, nil)
    //if err != nil {
    //    return fmt.Errorf("error creating request: %v", err)
    //}

    resp, err := d.downloadWithRetry(mediaUrl)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Per File Download Progress - I prefer the overall, even though this 
    //actually smoothly progresses.

    //contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

    //d.progressBar = progressbar.DefaultBytes(
    //    contentLength,
    //    fmt.Sprintf("Downloading %s", fileName),
    //)

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
    err = d.saveFileHash(modelName, hashString, filePath)
    if err != nil {
        return err
    }

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

        req.Header.Add("Authorization", d.authToken)
        req.Header.Add("User-Agent", d.userAgent)

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


func (d *Downloader) fileExists(filePath string) bool {
    var count int
    err := d.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", filePath).Scan(&count)
    if err != nil {
        log.Printf("Error checking if file exists in DB: %v", err)
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

func (d *Downloader) saveFileHash(modelName string, hash, path string) error {
    _, err := d.db.Exec("INSERT OR REPLACE INTO files (model, hash, path) VALUES (?, ?, ?)", modelName, hash, path)
    return err
}

func (d *Downloader) Close() error {
    return d.db.Close()
}
