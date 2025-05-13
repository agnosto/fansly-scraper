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
	limiter = rate.NewLimiter(rate.Every(3*time.Second), 2)
)

type AccountMediaBundles struct {
	ID              string   `json:"id"`
	Access          bool     `json:"access"`
	AccountMediaIDs []string `json:"accountMediaIds"`
	BundleContent   []struct {
		AccountMediaID string `json:"AccountMediaId"`
		Pos            int    `json:"pos"`
	} `json:"bundleContent"`
}

type Location struct {
	Location string            `json:"location"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type MediaItem struct {
	ID       string `json:"id"`
	Type     int    `json:"type"`
	Height   int    `json:"height"`
	Mimetype string `json:"mimetype"`
	Variants []struct {
		ID        string     `json:"id"`
		Type      int        `json:"type"`
		Height    int        `json:"height"`
		Mimetype  string     `json:"mimetype"`
		Locations []Location `json:"locations"`
	} `json:"variants"`
	Locations []Location `json:"locations"`
}

type AccountMedia struct {
	ID        string     `json:"id"`
	AccountId string     `json:"accountId"`
	Media     MediaItem  `json:"media"`
	Preview   *MediaItem `json:"preview,omitempty"` // Added to handle optional preview
}

type PostResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Posts []struct {
			ID          string `json:"id"`
			Attachments []struct {
				ContentType int    `json:"contentType"`
				ContentID   string `json:"contentId"`
			} `json:"attachments"`
		} `json:"posts"`
		AccountMediaBundles []AccountMediaBundles `json:"accountMediaBundles"`
		AccountMedia        []AccountMedia        `json:"accountMedia"`
	} `json:"response"`
}

func GetPostMedia(postId string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, error) {
	ctx := context.Background()
	err := limiter.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/post?ids=%s&ngsw-bypass=true", postId)
	logger.Logger.Printf("[INFO] Starting media parsing for Post: %s with URL: %v", postId, url)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch post with status code %d", resp.StatusCode)
	}

	var postResp PostResponse
	err = json.NewDecoder(resp.Body).Decode(&postResp)
	if err != nil {
		return nil, err
	}

	var accountMediaItems []AccountMedia
	for _, post := range postResp.Response.Posts {
		for _, attachment := range post.Attachments {
			for _, accountMedia := range postResp.Response.AccountMedia {
				if accountMedia.ID == attachment.ContentID {
					hasLocations := false
					// Check main media variants for locations
					for _, variant := range accountMedia.Media.Variants {
						if len(variant.Locations) > 0 {
							hasLocations = true
							break
						}
					}
					// Check preview media and its variants for locations
					if accountMedia.Preview != nil {
						if len(accountMedia.Preview.Locations) > 0 {
							hasLocations = true
						} else {
							for _, variant := range accountMedia.Preview.Variants {
								if len(variant.Locations) > 0 {
									hasLocations = true
									break
								}
							}
						}
					}
					if hasLocations {
						accountMediaItems = append(accountMediaItems, accountMedia)
					} else {
						logger.Logger.Printf("[WARN] POST: %s, Skipping AccountMedia %s: No locations found", postId, accountMedia.ID)
					}
					break
				}
			}
		}
		for _, bundle := range postResp.Response.AccountMediaBundles {
			if bundle.Access {
				for _, bundleContent := range bundle.BundleContent {
					// Find and add the corresponding AccountMedia
					for _, accountMedia := range postResp.Response.AccountMedia {
						if accountMedia.ID == bundleContent.AccountMediaID {
							accountMediaItems = append(accountMediaItems, accountMedia)
							break
						}
					}
				}
			}
		}
	}

	for _, accountMedia := range postResp.Response.AccountMedia {
		// Check if this accountMedia is already added
		alreadyAdded := false
		for _, addedMedia := range accountMediaItems {
			if addedMedia.ID == accountMedia.ID {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			// Add if it has locations or its preview has locations
			if hasLocations(accountMedia.Media) || (accountMedia.Preview != nil && hasLocations(*accountMedia.Preview)) {
				accountMediaItems = append(accountMediaItems, accountMedia)
			}
		}
	}

	logger.Logger.Printf("[INFO] Retrieved %d media items for post %s", len(accountMediaItems), postId)
	return accountMediaItems, nil
}

func hasLocations(media MediaItem) bool {
	if len(media.Locations) > 0 {
		return true
	}
	for _, variant := range media.Variants {
		if len(variant.Locations) > 0 {
			return true
		}
	}
	return false
}
