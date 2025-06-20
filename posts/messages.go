package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	//"strings"
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
	err := messageLimiter.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("rate limiter error: %v", err)
	}

	url := "https://apiv3.fansly.com/api/v1/messaging/groups?limit=1000&ngsw-bypass=true"
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

	for _, group := range groupResp.Response.Data {
		if group.PartnerAccountID == modelID {
			return group.GroupID, nil
		}
	}

	return "", fmt.Errorf("no group found for model ID: %s", modelID)
}

func GetAllMessageMedia(modelID string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, error) {
	groupID, err := GetMessageGroupID(modelID, fanslyHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to get group ID: %v", err)
	}

	var allMedia []AccountMedia
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

	for {
		media, nextCursor, err := getMessageMediaBatch(groupID, msgCursor, fanslyHeaders)
		if err != nil {
			return nil, err
		}
		allMedia = append(allMedia, media...)
		if nextCursor == "" {
			break
		}
		msgCursor = nextCursor
		bar.Add(len(media))
	}

	bar.Finish()
	return allMedia, nil
}

func getMessageMediaBatch(groupID, cursor string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, string, error) {
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

	allMediaIDs := make(map[string]struct{})
	for _, msg := range msgResp.Response.Messages {
		for _, attachment := range msg.Attachments {
			if attachment.ContentType == 1 {
				allMediaIDs[attachment.ContentID] = struct{}{}
			} else if attachment.ContentType == 2 {
				if bundle, ok := bundleMap[attachment.ContentID]; ok {
					for _, mediaID := range bundle.AccountMediaIDs {
						allMediaIDs[mediaID] = struct{}{}
					}
				}
			}
		}
	}

	var finalMediaItems []AccountMedia
	var mediaToFetch []string
	processedIDs := make(map[string]bool)

	for id := range allMediaIDs {
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
			logger.Logger.Printf("[WARN] Failed to fetch some bundled media items: %v", err)
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

	var nextCursor string
	if len(msgResp.Response.Messages) > 0 {
		nextCursor = msgResp.Response.Messages[len(msgResp.Response.Messages)-1].ID
	}

	logger.Logger.Printf("[INFO] Retrieved %d media items for message batch", len(filteredMedia))
	return filteredMedia, nextCursor, nil
}
