package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

var (
	storyLimiter = rate.NewLimiter(rate.Every(3*time.Second), 2)
)

type Story struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	ContentType int    `json:"contentType"`
	ContentID   string `json:"contentId"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type StoriesResponse struct {
	Success  bool `json:"success"`
	Response struct {
		MediaStories    []Story `json:"mediaStories"`
		AggregationData struct {
			AccountMedia []AccountMedia `json:"accountMedia"`
		} `json:"aggregationData"`
	} `json:"response"`
}

func GetModelStories(accountID string, fanslyHeaders *headers.FanslyHeaders) ([]Story, []AccountMedia, error) {
	ctx := context.Background()
	err := storyLimiter.Wait(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/mediastoriesnew?accountId=%s&ngsw-bypass=true", accountID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}

	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var storiesResp StoriesResponse
	err = json.NewDecoder(resp.Body).Decode(&storiesResp)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding response: %v", err)
	}

	validAccountMedia := filterValidAccountMedia(storiesResp.Response.AggregationData.AccountMedia)
	logger.Logger.Printf("[INFO] Retrieved %d stories and %d valid media items",
		len(storiesResp.Response.MediaStories), len(validAccountMedia))

	return storiesResp.Response.MediaStories, validAccountMedia, nil
}

func filterValidAccountMedia(accountMedia []AccountMedia) []AccountMedia {
	var validMedia []AccountMedia
	for _, media := range accountMedia {
		if hasValidLocations(media) {
			validMedia = append(validMedia, media)
		} else {
			logger.Logger.Printf("[WARN] Skipping AccountMedia %s: No valid locations found", media.ID)
		}
	}
	return validMedia
}

func hasValidLocations(media AccountMedia) bool {
	// Check main media variants for locations
	for _, variant := range media.Media.Variants {
		if len(variant.Locations) > 0 {
			return true
		}
	}

	// Check preview media and its variants for locations, if it exists
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
