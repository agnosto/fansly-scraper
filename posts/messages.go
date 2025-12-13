package posts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

var (
	messageLimiter = rate.NewLimiter(rate.Every(5*time.Second), 1)
)

type Message struct {
	ID             string `json:"id"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"createdAt"`
	SenderId       string `json:"senderId"`
	SenderUsername string `json:"senderUsername,omitempty"`
	Attachments    []struct {
		ContentType int    `json:"contentType"`
		ContentID   string `json:"contentId"`
	} `json:"attachments"`
}

// New struct to hold a message and its media
type MessageWithMedia struct {
	Message Message
	Media   []AccountMedia
}

type MessageResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Messages            []Message            `json:"messages"`
		AccountMedia        []AccountMedia       `json:"accountMedia"`
		AccountMediaBundles []AccountMediaBundle `json:"accountMediaBundles"`
	} `json:"response"`
}

// Structs for creating/retrieving a message group
type UserPayload struct {
	UserID          string `json:"userId"`
	PermissionFlags int    `json:"permissionFlags"`
}

type GroupRequestPayload struct {
	Users        []UserPayload `json:"users"`
	Recipients   []any         `json:"recipients"`
	LastMessage  any           `json:"lastMessage"`
	UserSettings any           `json:"userSettings"`
	Type         int           `json:"type"`
}

type GroupCreateResponse struct {
	Success  bool `json:"success"`
	Response struct {
		ID string `json:"id"`
	} `json:"response"`
}

func GetMessageGroupID(modelID string, fanslyHeaders *headers.FanslyHeaders) (string, error) {
	myUserID, err := auth.GetMyUserID()
	if err != nil {
		return "", fmt.Errorf("could not get own user ID: %v", err)
	}

	payload := GroupRequestPayload{
		Users: []UserPayload{
			{UserID: myUserID, PermissionFlags: 0},
			{UserID: modelID, PermissionFlags: 0},
		},
		Recipients:   []any{},
		LastMessage:  nil,
		UserSettings: nil,
		Type:         1,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling group request payload: %v", err)
	}

	ctx := context.Background()
	if err := messageLimiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limiter error: %v", err)
	}

	url := "https://apiv3.fansly.com/api/v1/group?ngsw-bypass=true"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("error creating group request: %v", err)
	}

	fanslyHeaders.AddHeadersToRequest(req, true)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending group request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code for group request: %d", resp.StatusCode)
	}

	var groupResp GroupCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&groupResp); err != nil {
		return "", fmt.Errorf("error decoding group response: %v", err)
	}

	if !groupResp.Success || groupResp.Response.ID == "" {
		return "", fmt.Errorf("failed to get group ID for model %s", modelID)
	}

	logger.Logger.Printf("[INFO] Successfully found message group ID for model %s", modelID)
	return groupResp.Response.ID, nil
}

func GetAllMessagesWithMedia(modelID string, fanslyHeaders *headers.FanslyHeaders, limit int) ([]MessageWithMedia, error) {
	groupID, err := GetMessageGroupID(modelID, fanslyHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to get group ID: %v", err)
	}

	var allMessagesWithMedia []MessageWithMedia
	msgCursor := "0"
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Fetching Messages"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)
	defer bar.Finish()

	for {
		batch, nextCursor, err := getMessageBatchWithMedia(groupID, msgCursor, fanslyHeaders)
		if err != nil {
			return nil, err
		}
		allMessagesWithMedia = append(allMessagesWithMedia, batch...)
		bar.Add(len(batch))

		if limit > 0 && len(allMessagesWithMedia) >= limit {
			allMessagesWithMedia = allMessagesWithMedia[:limit]
			break
		}

		if nextCursor == "" {
			break
		}
		msgCursor = nextCursor
	}

	return allMessagesWithMedia, nil
}

func getMessageBatchWithMedia(groupID, cursor string, fanslyHeaders *headers.FanslyHeaders) ([]MessageWithMedia, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := messageLimiter.Wait(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/message?groupId=%s&limit=25&ngsw-bypass=true", groupID)
	if cursor != "0" {
		url += fmt.Sprintf("&before=%s", cursor)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch messages with status code %d", resp.StatusCode)
	}

	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, "", err
	}

	mediaMap := make(map[string]AccountMedia)
	for _, media := range msgResp.Response.AccountMedia {
		mediaMap[media.ID] = media
	}

	bundleMap := make(map[string]AccountMediaBundle)
	for _, bundle := range msgResp.Response.AccountMediaBundles {
		bundleMap[bundle.ID] = bundle
	}

	var messagesWithMedia []MessageWithMedia

	for _, msg := range msgResp.Response.Messages {
		mediaIDsForThisMessage := make(map[string]struct{})
		for _, attachment := range msg.Attachments {
			if attachment.ContentType == 1 { // Single media
				mediaIDsForThisMessage[attachment.ContentID] = struct{}{}
			} else if attachment.ContentType == 2 { // Bundle
				if bundle, ok := bundleMap[attachment.ContentID]; ok {
					for _, mediaID := range bundle.AccountMediaIDs {
						mediaIDsForThisMessage[mediaID] = struct{}{}
					}
				}
			}
		}

		if len(mediaIDsForThisMessage) == 0 {
			continue // Skip messages with no media
		}

		var finalMediaItems []AccountMedia
		var mediaToFetch []string
		processedIDs := make(map[string]bool)

		for id := range mediaIDsForThisMessage {
			if media, ok := mediaMap[id]; ok {
				if !processedIDs[id] {
					finalMediaItems = append(finalMediaItems, media)
					processedIDs[id] = true
				}
			} else {
				mediaToFetch = append(mediaToFetch, id)
			}
		}

		if len(mediaToFetch) > 0 {
			fetchedMedia, err := GetMediaByIDs(ctx, mediaToFetch, fanslyHeaders)
			if err != nil {
				logger.Logger.Printf("[WARN] Failed to fetch some bundled media items for message %s: %v", msg.ID, err)
			} else {
				for _, media := range fetchedMedia {
					if !processedIDs[media.ID] {
						finalMediaItems = append(finalMediaItems, media)
						processedIDs[media.ID] = true
					}
				}
			}
		}

		filteredMedia := filterMediaWithLocations(finalMediaItems)
		if len(filteredMedia) > 0 {
			messagesWithMedia = append(messagesWithMedia, MessageWithMedia{
				Message: msg,
				Media:   filteredMedia,
			})
		}
	}

	var nextCursor string
	if len(msgResp.Response.Messages) > 0 {
		nextCursor = msgResp.Response.Messages[len(msgResp.Response.Messages)-1].ID
	}

	logger.Logger.Printf("[INFO] Retrieved %d messages with media in batch", len(messagesWithMedia))
	return messagesWithMedia, nextCursor, nil
}

func FetchMessages(groupID, cursor string, fanslyHeaders *headers.FanslyHeaders) ([]Message, string, error) {
	ctx := context.Background()
	if err := messageLimiter.Wait(ctx); err != nil {
		return nil, "", fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/message?groupId=%s&limit=25&ngsw-bypass=true", groupID)
	if cursor != "0" {
		url += fmt.Sprintf("&before=%s", cursor)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch messages with status code %d", resp.StatusCode)
	}

	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(msgResp.Response.Messages) > 0 {
		nextCursor = msgResp.Response.Messages[len(msgResp.Response.Messages)-1].ID
	}

	return msgResp.Response.Messages, nextCursor, nil
}
