package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
)

func (d *Downloader) DumpChatLogs(ctx context.Context, modelId, modelName string) error {
	logger.Logger.Printf("[INFO] Starting chat log dump for %s...", modelName)

	// 1. Prepare Directory
	baseDir := filepath.Join(d.saveLocation, strings.ToLower(modelName), "messages")
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	filePath := filepath.Join(baseDir, "chat_log.json")

	// 2. Load Existing Logs
	var existingMessages []posts.Message
	existingIDs := make(map[string]bool)

	if _, err := os.Stat(filePath); err == nil {
		fileData, err := os.ReadFile(filePath)
		if err == nil {
			if err := json.Unmarshal(fileData, &existingMessages); err == nil {
				for _, msg := range existingMessages {
					existingIDs[msg.ID] = true
				}
				logger.Logger.Printf("[INFO] Loaded %d existing messages.", len(existingMessages))
			}
		}
	}

	// 3. Get Group ID
	groupID, err := posts.GetMessageGroupID(modelId, d.headers)
	if err != nil {
		return err
	}

	// 4. Fetch New Messages
	var newMessages []posts.Message
	cursor := "0"

	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Fetching Chat Logs"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	caughtUp := false

	for !caughtUp {
		batch, nextCursor, err := posts.FetchMessages(groupID, cursor, d.headers)
		if err != nil {
			logger.Logger.Printf("[ERROR] Failed to fetch message batch: %v", err)
			break
		}

		if len(batch) == 0 {
			break
		}

		for _, msg := range batch {
			if existingIDs[msg.ID] {
				caughtUp = true
				continue
			}
			newMessages = append(newMessages, msg)
			bar.Add(1)
		}

		if nextCursor == "" || caughtUp {
			break
		}
		cursor = nextCursor
		time.Sleep(200 * time.Millisecond)
	}
	bar.Finish()
	fmt.Println()

	if len(newMessages) == 0 {
		logger.Logger.Printf("[INFO] Chat log is already up to date.")
		return nil
	}

	// 5. Merge and Sort
	allMessages := append(newMessages, existingMessages...)
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].CreatedAt < allMessages[j].CreatedAt
	})

	// 6. Resolve Usernames
	logger.Logger.Printf("[INFO] Resolving usernames...")

	// Collect all unique sender IDs that need resolving
	uniqueSenderIDs := make(map[string]bool)
	for _, msg := range allMessages {
		if msg.SenderId != "" {
			uniqueSenderIDs[msg.SenderId] = true
		}
	}

	var idsToFetch []string
	for id := range uniqueSenderIDs {
		idsToFetch = append(idsToFetch, id)
	}

	// Fetch account details for all participants (You + Model)
	usernameMap := make(map[string]string)
	if len(idsToFetch) > 0 {
		accounts, err := auth.GetAccountDetails(idsToFetch, d.headers)
		if err != nil {
			logger.Logger.Printf("[WARN] Failed to resolve usernames: %v", err)
		} else {
			for _, acc := range accounts {
				usernameMap[acc.ID] = acc.Username
			}
		}
	}

	// Apply usernames to messages
	for i := range allMessages {
		if name, ok := usernameMap[allMessages[i].SenderId]; ok {
			allMessages[i].SenderUsername = name
		} else {
			// Fallback if lookup failed
			if allMessages[i].SenderId == modelId {
				allMessages[i].SenderUsername = modelName
			} else {
				allMessages[i].SenderUsername = "Unknown_" + allMessages[i].SenderId
			}
		}
	}

	// 7. Save
	logger.Logger.Printf("[INFO] Saving %d total messages to %s", len(allMessages), filePath)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Make it readable
	if err := encoder.Encode(allMessages); err != nil {
		return fmt.Errorf("failed to write log file: %v", err)
	}

	fmt.Printf("Successfully exported chat log with usernames to %s\n", filePath)
	return nil
}
