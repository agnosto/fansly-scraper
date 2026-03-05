package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
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

	// Build albumContent lookup: mediaId -> AlbumContent (for createdAt timestamps)
	albumContentMap := make(map[string]posts.AlbumContent)
	for _, ac := range content.Response.AlbumContent {
		albumContentMap[ac.MediaId] = ac
	}

	// Group media by accountId, preserving order
	var accountOrder []string
	accountInfoMap := make(map[string]*AccountInfo)
	for _, media := range accountMediaItems {
		if _, exists := accountInfoMap[media.AccountId]; !exists {
			accountOrder = append(accountOrder, media.AccountId)
			accountInfoMap[media.AccountId] = &AccountInfo{
				ID: media.AccountId,
			}
		}
		accountInfoMap[media.AccountId].Media = append(accountInfoMap[media.AccountId].Media, media)
	}

	// Resolve all account IDs → usernames in a single batched call (mirrors auth.GetAccountDetails)
	// This avoids the per-goroutine rate-limit hammering that caused all folders to fall back to IDs.
	uniqueIDs := accountOrder
	logger.Logger.Printf("[INFO] Resolving usernames for %d unique accounts...", len(uniqueIDs))

	resolvedModels, err := auth.GetAccountDetails(uniqueIDs, d.headers)
	if err != nil {
		logger.Logger.Printf("[WARN] Batch username resolution failed (%v); falling back to account IDs for folder names.", err)
	} else {
		for _, model := range resolvedModels {
			if info, ok := accountInfoMap[model.ID]; ok {
				if model.Username != "" {
					info.Username = model.Username
				}
			}
		}
	}

	// Any account whose username is still empty (deleted/private) gets its ID as fallback
	for _, info := range accountInfoMap {
		if info.Username == "" {
			info.Username = info.ID
			logger.Logger.Printf("[INFO] Could not resolve username for account %s; using ID as folder name.", info.ID)
		}
	}

	// Count total media for the progress bar
	totalMedia := 0
	for _, info := range accountInfoMap {
		totalMedia += len(info.Media)
	}

	d.progressBar = progressbar.NewOptions(totalMedia,
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

	// Download in stable order
	var mu sync.Mutex
	_ = mu // retained in case callers extend this with concurrency later

	for _, accountID := range accountOrder {
		accountInfo := accountInfoMap[accountID]

		folderName := strings.ToLower(accountInfo.Username)
		baseDir := filepath.Join(d.saveLocation, folderName, "purchases")

		for _, subDir := range []string{"images", "videos", "audios"} {
			if err := os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
				logger.Logger.Printf("Error creating directory %s: %v", filepath.Join(baseDir, subDir), err)
			}
		}

		logger.Logger.Printf("[INFO] Downloading %d purchased item(s) for %s → %s",
			len(accountInfo.Media), accountID, folderName)

		for i, media := range accountInfo.Media {
			postData := posts.Post{
				ID:      albumID,
				Content: purchasedAlbum.Title,
			}
			if ac, ok := albumContentMap[media.ID]; ok {
				postData.CreatedAt = ac.CreatedAt
			}

			if err := d.DownloadMediaItem(ctx, media, baseDir, accountInfo.Username, postData, i); err != nil {
				logger.Logger.Printf("Error downloading media item %s for account %s: %v", media.ID, accountID, err)
			}
			d.progressBar.Add(1)
		}
	}

	d.progressBar.Finish()
	return nil
}
