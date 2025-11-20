package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/download"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/posts"
)

type DiagnosisSuite struct {
	flags         DiagnosisFlags
	cfg           *config.Config
	report        *strings.Builder
	fanslyHeaders *headers.FanslyHeaders
}

func NewDiagnosisSuite(flags DiagnosisFlags, cfg *config.Config) *DiagnosisSuite {
	return &DiagnosisSuite{
		flags:  flags,
		cfg:    cfg,
		report: &strings.Builder{},
	}
}

func (ds *DiagnosisSuite) Run() {
	ds.log("Starting diagnosis suite...")
	ds.log(fmt.Sprintf("Verbosity Level: %d", ds.flags.Level))
	ds.log("----------------------------------")

	// --- Primary Logic Change Here ---

	// Always run the base tests
	ds.testConfig()
	ds.testAuthentication()

	// Only proceed with API tests if authentication was successful
	if ds.fanslyHeaders != nil {
		ds.testSubscriptionsAndFollows()

		// Test Creator-specifics (Timeline, Messages) if a creator is provided
		if ds.flags.Creator != "" {
			ds.testCreator()
		}

		// Test Post-specifics independently if a post ID is provided
		if ds.flags.PostID != "" {
			ds.testPost()
		}
	}

	ds.log("----------------------------------")
	ds.log("Diagnosis suite finished.")

	ds.saveReport()
}

func (ds *DiagnosisSuite) log(message string) {
	fmt.Println(message)
	ds.report.WriteString(message + "\n")
}

func (ds *DiagnosisSuite) sanitizePath(path string) string {
	re := regexp.MustCompile(`(?i)(C:\\Users\\[^\\]+|/home/[^/]+|/Users/[^/]+)`)
	return re.ReplaceAllString(path, "[REDACTED_USER_PATH]")
}

func (ds *DiagnosisSuite) testConfig() {
	ds.log("\n[1] Testing Configuration")
	if ds.cfg == nil {
		ds.log(" - FAIL: Configuration file could not be loaded. Please run the app once without flags to generate one.")
		return
	}

	configPath := config.GetConfigPath()
	ds.log(fmt.Sprintf(" - Config path: %s", ds.sanitizePath(configPath)))
	ds.log(" - PASS: Config loaded successfully.")

	redactedCfg := *ds.cfg
	redactedCfg.Account.AuthToken = "[REDACTED]"
	redactedCfg.Options.SaveLocation = ds.sanitizePath(redactedCfg.Options.SaveLocation)
	redactedCfg.SecurityHeaders.CheckKey = "[REDACTED]"
	redactedCfg.SecurityHeaders.SessionID = "[REDACTED]"
	redactedCfg.Notifications.DiscordWebhook = "[REDACTED]"
	redactedCfg.Notifications.TelegramBotToken = "[REDACTED]"
	redactedCfg.Notifications.TelegramChatID = "[REDACTED]"

	if ds.flags.Level > 1 {
		ds.log(fmt.Sprintf(" - Loaded config (redacted): %+v", redactedCfg))
	}
}

func (ds *DiagnosisSuite) testAuthentication() {
	ds.log("\n[2] Testing Authentication")
	if ds.cfg == nil {
		ds.log(" - SKIP: Cannot test authentication without a valid config.")
		return
	}

	var errHeaders error
	ds.fanslyHeaders, errHeaders = headers.NewFanslyHeaders(ds.cfg)
	if errHeaders != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not create Fansly headers: %v", errHeaders))
		return
	}
	ds.log(" - INFO: Security headers initialized successfully.")

	account, err := auth.Login(ds.fanslyHeaders)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Login failed. This may be due to an invalid auth_token or User-Agent. Error: %v", err))
		return
	}

	// --- Bonus Fix Here ---
	// Fallback to username if display name is not set
	loggedInAs := account.DisplayName
	if loggedInAs == "" {
		loggedInAs = account.Username
	}
	ds.log(fmt.Sprintf(" - PASS: Logged in as: %s (Username: [REDACTED])", loggedInAs))
}

func (ds *DiagnosisSuite) testSubscriptionsAndFollows() {
	ds.log("\n[3] Testing Subscriptions and Follows")
	myUserID, err := auth.GetMyUserID()
	if err != nil {
		ds.log(" - FAIL: Could not get user ID.")
		return
	}

	followedUsers, err := auth.GetFollowedUsers(myUserID, ds.fanslyHeaders)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not get followed users: %v", err))
		return
	}
	ds.log(fmt.Sprintf(" - INFO: Found %d followed/subscribed creators.", len(followedUsers)))

	if ds.flags.Level > 2 && len(followedUsers) > 0 {
		for _, user := range followedUsers {
			ds.log(fmt.Sprintf("   - %s", user.Username))
		}
	}
}

// This function now only tests creator-level details
func (ds *DiagnosisSuite) testCreator() {
	ds.log(fmt.Sprintf("\n[4] Testing Creator: %s", ds.flags.Creator))
	modelID, err := core.GetModelIDFromUsername(ds.flags.Creator)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not get model ID for %s: %v", ds.flags.Creator, err))
		return
	}
	ds.log(fmt.Sprintf(" - INFO: Model ID for %s is %s", ds.flags.Creator, modelID))

	timelinePosts, err := posts.GetAllTimelinePosts(modelID, "", ds.fanslyHeaders)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not get timeline posts: %v", err))
	} else {
		ds.log(fmt.Sprintf(" - INFO: Found %d timeline posts.", len(timelinePosts)))
	}

	messages, err := posts.GetAllMessagesWithMedia(modelID, ds.fanslyHeaders)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not get messages: %v", err))
	} else {
		totalMedia := 0
		for _, msg := range messages {
			totalMedia += len(msg.Media)
		}
		ds.log(fmt.Sprintf(" - INFO: Found %d messages with a total of %d media items.", len(messages), totalMedia))
	}
}

// This function is now self-contained for testing a post
func isMediaDownloadable(media posts.AccountMedia) bool {
	// Check the main media object for locations
	if len(media.Media.Locations) > 0 {
		return true
	}
	for _, variant := range media.Media.Variants {
		if len(variant.Locations) > 0 {
			return true
		}
	}
	// If no main media, check the preview object
	if media.Preview != nil {
		if len(media.Preview.Locations) > 0 {
			return true
		}
		for _, variant := range media.Preview.Variants {
			if len(variant.Locations) > 0 {
				return true
			}
		}
	}
	return false
}

// Replace the old testPost function with this one.
func (ds *DiagnosisSuite) testPost() {
	ds.log(fmt.Sprintf("\n[5] Testing Post: %s", ds.flags.PostID))
	postInfo, allMedia, err := posts.GetFullPostDetails(ds.flags.PostID, ds.fanslyHeaders)
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not get post details: %v", err))
		return
	}

	// Get creator username from the post data itself
	creatorDetails, err := auth.GetAccountDetails([]string{postInfo.AccountId}, ds.fanslyHeaders)
	if err != nil || len(creatorDetails) == 0 {
		ds.log(fmt.Sprintf(" - WARN: Could not get creator details for account ID %s: %v", postInfo.AccountId, err))
	}
	creatorUsername := creatorDetails[0].Username
	ds.log(fmt.Sprintf(" - INFO: Post belongs to creator: %s (Account ID: %s)", creatorUsername, postInfo.AccountId))

	// This will now correctly report 6
	ds.log(fmt.Sprintf(" - INFO: Post has %d total media items.", len(allMedia)))

	// --- New, more detailed accessibility logic ---
	directAccessCount := 0
	previewAccessCount := 0
	downloadableCount := 0

	for _, m := range allMedia {
		// Checks the 'access' flag, which is usually true for subscribers
		if m.Access {
			directAccessCount++
		}
		// Checks if a preview exists and has content
		if m.Preview != nil && isMediaDownloadable(posts.AccountMedia{Media: *m.Preview}) {
			previewAccessCount++
		}
		// Checks if anything at all can be downloaded (full content or preview)
		if isMediaDownloadable(m) {
			downloadableCount++
		}
	}
	ds.log(fmt.Sprintf(" - INFO: %d/%d media items are directly accessible (requires subscription).", directAccessCount, len(allMedia)))
	ds.log(fmt.Sprintf(" - INFO: %d/%d media items have a downloadable preview.", previewAccessCount, len(allMedia)))

	if ds.flags.Level > 1 {
		ds.log(" - INFO: Testing a sample download...")
		if creatorUsername == "" {
			ds.log(" - SKIP: Cannot test download without a creator username.")
			return
		}
		// Pass the full list to the download test
		ds.testDownload(postInfo, allMedia, creatorUsername)
	}
}

// Updated to accept the creator username
func (ds *DiagnosisSuite) testDownload(post *posts.PostInfo, media []posts.AccountMedia, creatorUsername string) {
	// We'll create a special config for diagnosis that bypasses the DB check
	diagConfig := *ds.cfg
	diagConfig.Options.SkipDownloadedPosts = false // Ensure we don't skip the whole post

	// We'll pass a flag to the downloader to tell it to ignore the DB hash check
	downloader, _ := download.NewDownloader(&diagConfig, IsFFmpegAvailable())

	/*tempDir := filepath.Join(ds.cfg.Options.SaveLocation, fmt.Sprintf("temp-diagnosis-%s", time.Now().Format("20060102-150405")))
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not create temp dir for download test: %v", err))
		return
	}*/

	tempDir, err := os.MkdirTemp("", "diagnosis-download-")
	if err != nil {
		ds.log(fmt.Sprintf(" - FAIL: Could not create temp dir for download test: %v", err))
		return
	}
	if !ds.flags.KeepTmp {
		defer os.RemoveAll(tempDir)
	}

	ds.log(fmt.Sprintf(" - INFO: Using temp directory for downloads: %s", ds.sanitizePath(tempDir)))
	if ds.flags.KeepTmp {
		// If we're keeping it, print the full, unsanitized path so the user can find it.
		ds.log(fmt.Sprintf(" - INFO: Temporary directory will be kept at: %s", tempDir))
	}

	// --- FIX #1: Create the necessary subdirectories ---
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err := os.MkdirAll(filepath.Join(tempDir, subDir), os.ModePerm); err != nil {
			ds.log(fmt.Sprintf(" - FAIL: Could not create temp subdirectory %s: %v", subDir, err))
			return
		}
	}
	// ---------------------------------------------------

	downloadedCount := 0
	ctx := context.Background()

	for i, mediaItem := range media {
		if isMediaDownloadable(mediaItem) {
			postForDownloader := posts.Post{
				ID:        post.ID,
				Content:   post.Content,
				CreatedAt: post.CreatedAt,
			}
			// We will add a new 'isDiagnosis' parameter to bypass DB checks
			err := downloader.DownloadMediaItem(ctx, mediaItem, tempDir, creatorUsername, postForDownloader, i, true) // <-- Note the new 'true' parameter
			if err != nil {
				ds.log(fmt.Sprintf(" - FAIL: Could not download media item %s: %v", mediaItem.ID, err))
			} else {
				ds.log(fmt.Sprintf(" - PASS: Successfully downloaded media item %s.", mediaItem.ID))
				downloadedCount++
			}
		}
	}
	ds.log(fmt.Sprintf(" - INFO: Download test complete. %d/%d downloadable items attempted.", downloadedCount, ds.countDownloadable(media)))
}

// Add this new helper function to get the correct count for the final log message.
func (ds *DiagnosisSuite) countDownloadable(media []posts.AccountMedia) int {
	count := 0
	for _, m := range media {
		if isMediaDownloadable(m) {
			count++
		}
	}
	return count
}

func (ds *DiagnosisSuite) saveReport() {
	outputFile := ds.flags.OutputFile
	if outputFile == "" {
		outputFile = fmt.Sprintf("diagnosis-report-%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	}

	err := os.WriteFile(outputFile, []byte(ds.report.String()), 0644)
	if err != nil {
		fmt.Printf("\nCould not save report to %s: %v\n", outputFile, err)
	} else {
		fmt.Printf("\nDiagnosis report saved to %s\n", outputFile)
	}
}
