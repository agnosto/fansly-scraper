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
	ID       string
	Username string
	Media    []posts.AccountMedia
}

func (d *Downloader) DownloadPurchasedContent(ctx context.Context) error {
	purchasedAlbum, err := posts.FetchPurchasedAlbums(d.headers)
	if err != nil {
		// If there's no purchased album, it's not a fatal error, just means nothing to do.
		logger.Logger.Printf("[INFO] Could not find a 'Purchases' album or an error occurred: %v", err)
		return nil
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

	albumContentMap := make(map[string]posts.AlbumContent)
	for _, ac := range content.Response.AlbumContent {
		albumContentMap[ac.MediaId] = ac
	}

	accountInfoMap := make(map[string]*AccountInfo)
	var accountInfoMutex sync.Mutex

	for _, media := range accountMediaItems {
		accountInfoMutex.Lock()
		if _, exists := accountInfoMap[media.AccountId]; !exists {
			accountInfoMap[media.AccountId] = &AccountInfo{
				ID:    media.AccountId,
				Media: []posts.AccountMedia{},
			}
		}
		accountInfoMap[media.AccountId].Media = append(accountInfoMap[media.AccountId].Media, media)
		accountInfoMutex.Unlock()
	}

	var wg sync.WaitGroup
	for accountID := range accountInfoMap {
		wg.Add(1)
		go func(accID string) {
			defer wg.Done()
			username, err := posts.FetchAccountInfo(accID, d.headers)
			if err != nil || username == "" {
				logger.Logger.Printf("Error fetching account info for %s or username is empty: %v. Using Account ID as fallback.", accID, err)
				username = accID // Fallback
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

	for _, accountInfo := range accountInfoMap {
		var baseDir, modelNameForFile string
		if accountInfo.Username == accountInfo.ID {
			baseDir = filepath.Join(d.saveLocation, accountInfo.ID, "purchases")
			modelNameForFile = accountInfo.ID
		} else {
			baseDir = filepath.Join(d.saveLocation, strings.ToLower(accountInfo.Username), "purchases")
			modelNameForFile = accountInfo.Username
		}

		for _, subDir := range []string{"images", "videos", "audios"} {
			if err := os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
				logger.Logger.Printf("Error creating directory %s: %v", filepath.Join(baseDir, subDir), err)
				continue
			}
		}

		for i, media := range accountInfo.Media {
			postData := posts.Post{
				ID:      albumID,
				Content: purchasedAlbum.Title,
			}
			if ac, ok := albumContentMap[media.ID]; ok {
				postData.CreatedAt = ac.CreatedAt
			}

			err = d.downloadMediaItem(ctx, media, baseDir, modelNameForFile, postData, i)
			if err != nil {
				logger.Logger.Printf("Error downloading media item %s: %v", media.ID, err)
			}
			mediaBar.Add(1)
		}
	}

	mediaBar.Finish()
	return nil
}
