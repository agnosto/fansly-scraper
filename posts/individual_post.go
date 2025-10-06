package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"golang.org/x/time/rate"
)

var (
	limiter = rate.NewLimiter(rate.Every(5*time.Second), 1)
)

type AccountMediaBundle struct {
	ID              string   `json:"id"`
	Access          bool     `json:"access"`
	AccountMediaIDs []string `json:"accountMediaIds"`
	BundleContent   []struct {
		AccountMediaID string `json:"accountMediaId"`
		Pos            int    `json:"pos"`
	} `json:"bundleContent"`
}

type AccountMediaResponse struct {
	Success  bool           `json:"success"`
	Response []AccountMedia `json:"response"`
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
	Metadata string `json:"metadata,omitempty"`
	Variants []struct {
		ID        string     `json:"id"`
		Type      int        `json:"type"`
		Height    int        `json:"height"`
		Mimetype  string     `json:"mimetype"`
		Metadata  string     `json:"metadata,omitempty"`
		Locations []Location `json:"locations"`
	} `json:"variants"`
	Locations []Location `json:"locations"`
}

type AccountMedia struct {
	ID        string     `json:"id"`
	AccountId string     `json:"accountId"`
	Access    bool       `json:"access"`
	Media     MediaItem  `json:"media"`
	Preview   *MediaItem `json:"preview,omitempty"`
}

// PostInfo struct to capture details from a single post API response
type PostInfo struct {
	ID          string `json:"id"`
	AccountId   string `json:"accountId"`
	Content     string `json:"content"`
	CreatedAt   int64  `json:"createdAt"`
	Attachments []struct {
		ContentType int    `json:"contentType"`
		ContentID   string `json:"contentId"`
	} `json:"attachments"`
}

type PostResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Posts               []PostInfo           `json:"posts"`
		AccountMediaBundles []AccountMediaBundle `json:"accountMediaBundles"`
		AccountMedia        []AccountMedia       `json:"accountMedia"`
	} `json:"response"`
}

// GetFullPostDetails fetches post metadata and all associated media items.
func GetFullPostDetails(postId string, fanslyHeaders *headers.FanslyHeaders) (*PostInfo, []AccountMedia, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	if err := limiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/post?ids=%s&ngsw-bypass=true", postId)
	logger.Logger.Printf("[INFO] Fetching details for Post: %s", postId)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to fetch post with status code %d", resp.StatusCode)
	}

	var postResp PostResponse
	if err := json.NewDecoder(resp.Body).Decode(&postResp); err != nil {
		return nil, nil, err
	}
	if !postResp.Success || len(postResp.Response.Posts) == 0 {
		return nil, nil, fmt.Errorf("API request for post %s failed or returned no data", postId)
	}

	postInfo := postResp.Response.Posts[0]

	mediaMap := make(map[string]AccountMedia)
	for _, media := range postResp.Response.AccountMedia {
		mediaMap[media.ID] = media
	}

	bundleMap := make(map[string]AccountMediaBundle)
	for _, bundle := range postResp.Response.AccountMediaBundles {
		bundleMap[bundle.ID] = bundle
	}

	allMediaIDs := make(map[string]struct{})
	for _, attachment := range postInfo.Attachments {
		if attachment.ContentType == 1 { // AccountMedia
			allMediaIDs[attachment.ContentID] = struct{}{}
		} else if attachment.ContentType == 2 { // AccountMediaBundle
			if bundle, ok := bundleMap[attachment.ContentID]; ok {
				for _, mediaID := range bundle.AccountMediaIDs {
					allMediaIDs[mediaID] = struct{}{}
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
			logger.Logger.Printf("[WARN] Failed to fetch some bundled media for post %s: %v", postId, err)
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
	logger.Logger.Printf("[INFO] Retrieved %d media items for post %s", len(filteredMedia), postId)
	return &postInfo, filteredMedia, nil
}

func GetMediaByIDs(ctx context.Context, mediaIDs []string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}

	idSet := make(map[string]struct{})
	uniqueIDs := []string{}
	for _, id := range mediaIDs {
		if _, ok := idSet[id]; !ok {
			idSet[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	if len(uniqueIDs) == 0 {
		return nil, nil
	}

	err := limiter.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("rate limiter error: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account/media?ids=%s&ngsw-bypass=true", strings.Join(uniqueIDs, ","))
	logger.Logger.Printf("[INFO] Fetching details for %d bundled media items", len(uniqueIDs))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch media by IDs with status code %d", resp.StatusCode)
	}

	var mediaResp AccountMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mediaResp); err != nil {
		return nil, err
	}
	if !mediaResp.Success {
		return nil, fmt.Errorf("API reported failure when fetching media by IDs")
	}
	return mediaResp.Response, nil
}

func GetPostMedia(postId string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, error) {
	_, media, err := GetFullPostDetails(postId, fanslyHeaders)
	return media, err
}

func filterMediaWithLocations(mediaItems []AccountMedia) []AccountMedia {
	var filteredMedia []AccountMedia

	for _, accountMedia := range mediaItems {
		hasContent := false

		// 1. Check for explicit download URLs on main media object or its variants
		if len(accountMedia.Media.Locations) > 0 {
			hasContent = true
		}
		if !hasContent {
			for _, variant := range accountMedia.Media.Variants {
				if len(variant.Locations) > 0 {
					hasContent = true
					break
				}
			}
		}

		// 2. As a fallback, check for URLs on the preview object or its variants.
		// This is useful for content where only the preview is available (e.g., access: false).
		if !hasContent && accountMedia.Preview != nil {
			if len(accountMedia.Preview.Locations) > 0 {
				hasContent = true
			}
			if !hasContent {
				for _, variant := range accountMedia.Preview.Variants {
					if len(variant.Locations) > 0 {
						hasContent = true
						break
					}
				}
			}
		}

		// 3. Specifically check for streamable videos (HLS/DASH), as they may not have explicit `locations`.
		// The download logic knows how to handle these manifest types.
		if !hasContent && accountMedia.Media.Type == 2 { // Type 2 is video
			for _, variant := range accountMedia.Media.Variants {
				// Type 302 is HLS (m3u8), 303 is DASH (mpd)
				if variant.Type == 302 || variant.Type == 303 {
					hasContent = true
					break
				}
			}
		}

		if hasContent {
			filteredMedia = append(filteredMedia, accountMedia)
		} else {
			logger.Logger.Printf("[WARN] Skipping AccountMedia %s: No downloadable/streamable content found", accountMedia.ID)
		}
	}

	return filteredMedia
}
