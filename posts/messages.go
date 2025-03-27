package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agnosto/fansly-scraper/logger"
	"golang.org/x/time/rate"
	"net/http"
	"os"
	"time"
	//"github.com/agnosto/fansly-scraper/headers"
	"github.com/schollz/progressbar/v3"
)

var (
	messageLimiter = rate.NewLimiter(rate.Every(3*time.Second), 2)
)

type Message struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
}

type MessageResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Messages     []Message      `json:"messages"`
		AccountMedia []AccountMedia `json:"accountMedia"`
	} `json:"response"`
}

// Updated Group structure to match new API response
type Group struct {
	AccountID           string `json:"account_id"`
	GroupID             string `json:"groupId"`
	PartnerAccountID    string `json:"partnerAccountId"`
	PartnerUsername     string `json:"partnerUsername"`
	Flags               int    `json:"flags"`
	UnreadCount         int    `json:"unreadCount"`
	SubscriptionTierID  any    `json:"subscriptionTierId"`
	LastMessageID       string `json:"lastMessageId"`
	LastUnreadMessageID string `json:"lastUnreadMessageId"`
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

func GetMessageGroupID(modelID, authToken, userAgent string) (string, error) {
	ctx := context.Background()
	err := messageLimiter.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("rate limiter error: %v", err)
	}

	// Updated URL to the new endpoint
	url := "https://apiv3.fansly.com/api/v1/messaging/groups?limit=1000&ngsw-bypass=true"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", authToken)
	req.Header.Set("User-Agent", userAgent)
	//headers.AddHeadersToRequest(req, true)

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

	// Look for the group with the matching partner account ID
	for _, group := range groupResp.Response.Data {
		if group.PartnerAccountID == modelID {
			return group.GroupID, nil
		}
	}

	return "", fmt.Errorf("no group found for model ID: %s", modelID)
}

func GetAllMessageMedia(modelID, authToken, userAgent string) ([]AccountMedia, error) {
	groupID, err := GetMessageGroupID(modelID, authToken, userAgent)
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
		media, nextCursor, err := getMessageMediaBatch(groupID, msgCursor, authToken, userAgent)
		if err != nil {
			return nil, err
		}

		allMedia = append(allMedia, media...)

		if nextCursor == "" {
			break
		}

		msgCursor = nextCursor
		bar.Add(len(allMedia))
	}

	bar.Finish()
	return allMedia, nil
}

func getMessageMediaBatch(groupID, cursor, authToken, userAgent string) ([]AccountMedia, string, error) {
	ctx := context.Background()
	err := messageLimiter.Wait(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/message?groupId=%s&limit=25&ngsw-bypass=true", groupID)
	if cursor != "0" {
		url += fmt.Sprintf("&before=%s", cursor)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Add("Authorization", authToken)
	req.Header.Add("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch messages with status code %d", resp.StatusCode)
	}

	var msgResp MessageResponse
	err = json.NewDecoder(resp.Body).Decode(&msgResp)
	if err != nil {
		return nil, "", err
	}

	var mediaItems []AccountMedia
	for _, accountMedia := range msgResp.Response.AccountMedia {
		hasValidLocations := false
		// Check main media variants for locations
		for _, variant := range accountMedia.Media.Variants {
			if len(variant.Locations) > 0 {
				hasValidLocations = true
				break
			}
		}
		// Check preview media and its variants for locations
		if accountMedia.Preview != nil {
			if len(accountMedia.Preview.Locations) > 0 {
				hasValidLocations = true
			} else {
				for _, variant := range accountMedia.Preview.Variants {
					if len(variant.Locations) > 0 {
						hasValidLocations = true
						break
					}
				}
			}
		}

		if hasValidLocations {
			mediaItems = append(mediaItems, accountMedia)
		} else {
			logger.Logger.Printf("[WARN] Skipping AccountMedia %s: No valid locations found", accountMedia.ID)
		}
	}

	var nextCursor string
	if len(msgResp.Response.Messages) > 0 {
		nextCursor = msgResp.Response.Messages[len(msgResp.Response.Messages)-1].ID
	}

	logger.Logger.Printf("[INFO] Retrieved %d media items for message batch", len(mediaItems))
	return mediaItems, nextCursor, nil
}
