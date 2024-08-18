package download

import (
	"context"
	"fmt"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type AccountInfo struct {
	Username string
	Media    []posts.AccountMedia
}

func (d *Downloader) DownloadPurchasedContent(ctx context.Context) error {
	albumID, err := posts.FetchPurchasedAlbums(d.authToken, d.userAgent)
	if err != nil {
		return fmt.Errorf("failed to fetch purchased albums: %v", err)
	}
	logger.Logger.Printf("Purchases Album ID: %s", albumID)

	content, err := posts.FetchAlbumContent(albumID, d.authToken, d.userAgent)
	if err != nil {
		return fmt.Errorf("error fetching content for album %s: %v", albumID, err)
	}
	logger.Logger.Printf("Fetched content for album %s", albumID)
	logger.Logger.Printf("Number of AlbumContent items: %d", len(content.Response.AlbumContent))
	logger.Logger.Printf("Number of AccountMedia items: %d", len(content.Response.AggregationData.AccountMedia))

	accountMediaItems := content.Response.AggregationData.AccountMedia
	if len(accountMediaItems) == 0 {
		return fmt.Errorf("no AccountMedia found for album %s", albumID)
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
	for accountID := range accountInfoMap {
		username, err := posts.FetchAccountInfo(accountID, d.authToken, d.userAgent)
		if err != nil || username == "" {
			logger.Logger.Printf("Error fetching account info for %s or username is empty: %v", accountID, err)
			username = accountID // Use AccountId as fallback
		}
		accountInfoMap[accountID].Username = username
	}

	mediaBar := progressbar.NewOptions(len(accountMediaItems),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		//progressbar.OptionShowBytes(false),
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

	for accountID, accountInfo := range accountInfoMap {
		var baseDir string
		if accountInfo.Username == accountID {
			baseDir = filepath.Join(d.saveLocation, accountID, "purchases")
		} else {
			baseDir = filepath.Join(d.saveLocation, strings.ToLower(accountInfo.Username), "purchases")
		}

		// Create subdirectories for images and videos
		for _, subDir := range []string{"images", "videos", "audios"} {
			err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm)
			if err != nil {
				logger.Logger.Printf("Error creating directory %s: %v", filepath.Join(baseDir, subDir), err)
				continue
			}
		}

		for _, media := range accountInfo.Media {
			err = d.downloadMediaItem(ctx, media, baseDir, posts.Post{ID: albumID}, accountInfo.Username)
			if err != nil {
				logger.Logger.Printf("Error downloading media item %s: %v", media.ID, err)
			} else {
				logger.Logger.Printf("Successfully downloaded media item %s to %s", media.ID, baseDir)
			}
			mediaBar.Add(1)
		}
	}

	mediaBar.Finish()
	return nil
}
