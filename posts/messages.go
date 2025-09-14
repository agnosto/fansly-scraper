package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

var (
	messageLimiter = rate.NewLimiter(rate.Every(5*time.Second), 1)
)

type Message struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	CreatedAt   int64  `json:"createdAt"`
	Attachments []struct {
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

type Group struct {
	AccountID           string `json:"account_id"`
	GroupID             string `json:"groupId"`
	PartnerAccountID    string `json:"partnerAccountId"`
	PartnerUsername     string `json:"partnerUsername"`
	Flags               int    `json:"flags"`
	UnreadCount         int    `json:"unreadCount"`
	SubscriptionTierID  any    `json:"subscriptionTierId"`
	LastMessageID       string `json:"lastMessageId"`
	LastUnreadMessageID string `json:"lastUnreadMessageID"`
}

type GroupResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Data            []Group `json:"data"`
		AggregationData struct {
			Accounts []struct{} `json:"accounts"`
			Groups   []struct{} `json:"groups"`
		} `json:"aggregationData"`
	} `json:"response"`
}

func GetMessageGroupID(modelID string, fanslyHeaders *headers.FanslyHeaders) (string, error) {
	ctx := context.Background()
	limit := 50
	offset := 0

	for {
		err := messageLimiter.Wait(ctx)
		if err != nil {
			return "", fmt.Errorf("rate limiter error: %v", err)
		}

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/messaging/groups?limit=%d&offset=%d&ngsw-bypass=true", limit, offset)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("error creating request: %v", err)
		}
		fanslyHeaders.AddHeadersToRequest(req, true)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("error sending request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var groupResp GroupResponse
		if err := json.NewDecoder(resp.Body).Decode(&groupResp); err != nil {
			return "", fmt.Errorf("error decoding response: %v", err)
		}

		if len(groupResp.Response.Data) == 0 {
			break // No more groups to fetch
		}

		for _, group := range groupResp.Response.Data {
			if group.PartnerAccountID == modelID {
				return group.GroupID, nil
			}
		}

		offset += limit
	}

	return "", fmt.Errorf("no group found for model ID: %s", modelID)
}

func GetAllMessagesWithMedia(modelID string, fanslyHeaders *headers.FanslyHeaders) ([]MessageWithMedia, error) {
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
