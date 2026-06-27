package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
)

// DownloadPostByID downloads a single post by its ID or URL.
func (d *Downloader) DownloadPostByID(ctx context.Context, postIdentifier string) error {
	// 1. Extract Post ID from URL or direct ID
	postID := postIdentifier
	if strings.Contains(postIdentifier, "/") {
		parts := strings.Split(postIdentifier, "/")
		// Take the last part, and remove any query parameters
		postID = strings.Split(parts[len(parts)-1], "?")[0]
	}
	// Basic validation
	if _, err := strconv.ParseUint(postID, 10, 64); err != nil {
		return fmt.Errorf("invalid Post ID: %s", postID)
	}

	logger.Logger.Printf("Starting download for post: %s", postID)

	// Perform login to ensure session is valid and currentAccountID is populated
	if _, err := auth.Login(d.headers); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if d.cfg.Options.SkipDownloadedPosts && d.ProcessedPostService.PostExists(postID) {
		logger.Logger.Printf("Post %s has already been processed. Skipping.", postID)
		return nil
	}

	// 2. Get Post Details (info and media)
	postInfo, accountMediaItems, err := posts.GetFullPostDetails(postID, d.headers)
	if err != nil {
		return fmt.Errorf("error getting post details for %s: %w", postID, err)
	}

	// 3. Get Account Details (Username) from the accountId found in the post
	accountDetails, err := auth.GetAccountDetails([]string{postInfo.AccountId}, d.headers)
	if err != nil || len(accountDetails) == 0 {
		return fmt.Errorf("error getting model info for account ID %s: %v", postInfo.AccountId, err)
	}
	modelName := accountDetails[0].Username
	logger.Logger.Printf("Post by model: %s (ID: %s)", modelName, postInfo.AccountId)

	// 4. Create directories for the model
	baseDir := filepath.Join(d.cfg.Options.SaveLocation, strings.ToLower(modelName), "timeline")
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}
	}

	// 5. Download all media items for the post
	postForDownloader := posts.Post{
		ID:        postInfo.ID,
		Content:   postInfo.Content,
		CreatedAt: postInfo.CreatedAt,
	}

	logger.Logger.Printf("Found %d media items to download for post %s.", len(accountMediaItems), postID)
	for i, media := range accountMediaItems {
		err := d.DownloadMediaItem(ctx, media, baseDir, modelName, postForDownloader, i)
		if err != nil {
			logger.Logger.Printf("[ERROR] Failed to download media item %s: %v", media.ID, err)
		}
	}

	d.ProcessedPostService.MarkPostAsProcessed(postInfo.ID, modelName, postInfo.Content, postInfo.CreatedAt)
	return nil
}
