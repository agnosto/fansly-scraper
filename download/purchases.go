package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
)

type AccountInfo struct {
	Username string
	Media    []posts.AccountMedia
}

func (d *Downloader) DownloadPurchasedContent(ctx context.Context) error {
	purchasedAlbum, err := posts.FetchPurchasedAlbums(d.headers)
	if err != nil {
		return fmt.Errorf("failed to fetch purchased albums: %v", err)
	}
	albumID := purchasedAlbum.ID
	logger.Logger.Printf("Purchases Album ID: %s, Title: %s", albumID, purchasedAlbum.Title)

	content, err := posts.FetchAlbumContent(albumID, d.headers)
	if err != nil {
		return fmt.Errorf("error fetching content for album %s: %v", albumID, err)
	}

	accountMediaItems := content.Response.AggregationData.AccountMedia
	if len(accountMediaItems) == 0 {
		logger.Logger.Printf("[INFO] No purchased media found to download.")
		return nil
	}

	// Create a map from media ID to album content for easy lookup of CreatedAt
	albumContentMap := make(map[string]posts.AlbumContent)
	for _, ac := range content.Response.AlbumContent {
		albumContentMap[ac.MediaId] = ac
	}

	// Create a map to store account info
	accountInfoMap := make(map[string]*AccountInfo)
	var accountInfoMutex sync.Mutex

	// Group media by account ID
	for _, media := range accountMediaItems {
		accountInfoMutex.Lock()
		if _, exists := accountInfoMap[media.AccountId]; !exists {
			accountInfoMap[media.AccountId] = &AccountInfo{Media: []posts.AccountMedia{}}
		}
		accountInfoMap[media.AccountId].Media = append(accountInfoMap[media.AccountId].Media, media)
		accountInfoMutex.Unlock()
	}

	// Fetch usernames for each unique account ID
	var wg sync.WaitGroup
	for accountID := range accountInfoMap {
		wg.Add(1)
		go func(accID string) {
			defer wg.Done()
			username, err := posts.FetchAccountInfo(accID, d.headers)
			if err != nil || username == "" {
				logger.Logger.Printf("Error fetching account info for %s or username is empty: %v", accID, err)
				username = accID // Use AccountId as fallback
			}
			accountInfoMutex.Lock()
			accountInfoMap[accID].Username = username
			accountInfoMutex.Unlock()
		}(accountID)
	}
	wg.Wait()

	mediaBar := progressbar.NewOptions(len(accountMediaItems),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetDescription("[cyan]Downloading Purchased Content[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionThrottle(15*time.Millisecond),
		progressbar.OptionShowIts(),
		progressbar.OptionShowCount(),
	)

	// This is the corrected loop
	for accountID, accountInfo := range accountInfoMap {
		var baseDir string
		if accountInfo.Username == accountID {
			baseDir = filepath.Join(d.saveLocation, accountID, "purchases")
		} else {
			baseDir = filepath.Join(d.saveLocation, strings.ToLower(accountInfo.Username), "purchases")
		}

		for _, subDir := range []string{"images", "videos", "audios"} {
			if err := os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
				logger.Logger.Printf("Error creating directory %s: %v", filepath.Join(baseDir, subDir), err)
				continue
			}
		}

		for i, media := range accountInfo.Media {
			postData := posts.Post{
				ID:      media.ID,
				Content: purchasedAlbum.Title,
			}
			if ac, ok := albumContentMap[media.ID]; ok {
				postData.CreatedAt = ac.CreatedAt
			}

			// Corrected call to the restored downloadMediaItem function
			err = d.downloadMediaItem(ctx, media, baseDir, accountInfo.Username, postData, i)
			if err != nil {
				logger.Logger.Printf("Error downloading media item %s: %v", media.ID, err)
			}
			mediaBar.Add(1)
		}
	}

	mediaBar.Finish()
	return nil
}
