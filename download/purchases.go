package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

type AccountInfo struct {
	ID       string
	Username string
	Media    []posts.AccountMedia
}

// firstImageURL extracts the first usable location URL from a profile image
func firstImageURL(img *auth.ProfileImage) string {
	if img == nil {
		return ""
	}
	for _, variant := range img.Variants {
		if len(variant.Locations) > 0 && variant.Locations[0].Location != "" {
			return variant.Locations[0].Location
		}
	}
	return ""
}

// resolveUsernames fills in the Username field of every AccountInfo in
// accountInfoMap for the given account IDs. It consults the database cache
// first, then the Fansly API in small batches (tolerating per-batch failures),
// then falls back to individual lookups for anything still unresolved, and
// finally persists everything it learned back to the cache.
func resolveUsernames(d *Downloader, accountInfoMap map[string]*AccountInfo, uniqueIDs []string) {
	// 1. Database cache
	cached := d.accountService.GetCachedUsernames(uniqueIDs)
	var missing []string
	for _, id := range uniqueIDs {
		if username, ok := cached[id]; ok {
			accountInfoMap[id].Username = username
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		logger.Logger.Printf("[INFO] All %d account usernames resolved from local cache.", len(uniqueIDs))
		return
	}
	logger.Logger.Printf("[INFO] %d account(s) cached locally, resolving %d via API...", len(uniqueIDs)-len(missing), len(missing))

	// 2. Batched API resolution in small chunks; one failing chunk must not
	//    cause every account to fall back to its raw ID.
	const chunkSize = 25
	for i := 0; i < len(missing); i += chunkSize {
		end := min(i+chunkSize, len(missing))
		chunk := missing[i:end]

		models, err := auth.GetAccountDetails(chunk, d.headers)
		if err != nil {
			logger.Logger.Printf("[WARN] Batched resolution failed for %d account(s) (%v); those will be retried individually.", len(chunk), err)
			continue
		}
		for _, model := range models {
			if info, ok := accountInfoMap[model.ID]; ok && model.Username != "" {
				info.Username = model.Username
				_ = d.accountService.SaveAccount(model.ID, model.Username, model.DisplayName,
					firstImageURL(model.Avatar), firstImageURL(model.Banner))
			}
		}
	}

	// 3. Individual lookups (rate limited) for anything still unresolved
	stillMissing := make([]string, 0, len(missing))
	for _, id := range missing {
		if accountInfoMap[id].Username == "" {
			stillMissing = append(stillMissing, id)
		}
	}
	if len(stillMissing) > 0 {
		logger.Logger.Printf("[INFO] Resolving %d remaining account(s) individually...", len(stillMissing))
		limiter := rate.NewLimiter(rate.Every(1500*time.Millisecond), 1)
		for _, id := range stillMissing {
			if err := limiter.Wait(context.Background()); err != nil {
				break
			}
			username, err := posts.FetchAccountInfo(id, d.headers)
			if err != nil || username == "" {
				logger.Logger.Printf("[WARN] Could not resolve username for account %s: %v", id, err)
				continue
			}
			accountInfoMap[id].Username = username
			_ = d.accountService.SaveAccount(id, username, "", "", "")
		}
	}
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

	// Resolve all account IDs → usernames.
	// Strategy: DB cache first (keeps folder names stable across runs even if the
	// API is rate limiting), then batched API resolution in small chunks where one
	// failing chunk doesn't sink the rest, then per-account lookups for stragglers.
	// Only genuinely unresolvable (deleted) accounts fall back to their ID.
	uniqueIDs := accountOrder
	logger.Logger.Printf("[INFO] Resolving usernames for %d unique accounts...", len(uniqueIDs))

	resolveUsernames(d, accountInfoMap, uniqueIDs)

	// Any account whose username is still empty (deleted/private) gets its ID as fallback
	for _, info := range accountInfoMap {
		if info.Username == "" {
			info.Username = info.ID
			logger.Logger.Printf("[INFO] Could not resolve username for account %s; using ID as folder name.", info.ID)
		}
	}

	// If an earlier run fell back to raw IDs, those folders still exist on
	// disk and would cause duplicate downloads; point the user at them.
	for _, info := range accountInfoMap {
		if info.Username != info.ID {
			legacyDir := filepath.Join(d.saveLocation, info.ID)
			if _, err := os.Stat(legacyDir); err == nil {
				logger.Logger.Printf("[INFO] Found legacy ID-named folder %s for @%s; merge it into %s to avoid duplicates.",
					legacyDir, info.Username, strings.ToLower(info.Username))
			}
		}
	}

	// Count total media for the progress bar
	totalMedia := 0
	for _, info := range accountInfoMap {
		totalMedia += len(info.Media)
	}

	d.progressBar = progressbar.NewOptions(totalMedia,
		progressbar.OptionSetWriter(d.progressWriter()),
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
			// Record the purchased media item itself so the library keeps a
			// per-creator record of purchases independent of the album ID.
			if err := d.ProcessedPostService.MarkPostAsProcessedFull(media.ID, accountID, accountInfo.Username, "", "purchases", postData.CreatedAt); err != nil {
				logger.Logger.Printf("[WARN] Failed to record purchased media %s in database: %v", media.ID, err)
			}
			d.progressBar.Add(1)
		}
	}

	d.progressBar.Finish()
	return nil
}
