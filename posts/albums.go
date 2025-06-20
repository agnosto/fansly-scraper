package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"golang.org/x/time/rate"
)

type Album struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Title     string `json:"title"`
	Type      int    `json:"type"`
	ItemCount int    `json:"itemCount"`
}

type AlbumsResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Albums []Album `json:"albums"`
	} `json:"response"`
}

type AlbumContent struct {
	ID                 string `json:"id"`
	MediaOfferId       string `json:"mediaOfferId"`
	MediaOfferType     int    `json:"mediaOfferType"`
	MediaOfferBundleId any    `json:"mediaOfferBundleId"`
	MediaId            string `json:"mediaId"`
	MediaType          int    `json:"mediaType"`
	PreviewId          any    `json:"previewId"`
	AlbumId            string `json:"albumId"`
	AccountId          string `json:"accountId"`
	CreatedAt          int64  `json:"createdAt"`
}

type AlbumContentResponse struct {
	Success  bool `json:"success"`
	Response struct {
		AlbumContent    []AlbumContent `json:"albumContent"`
		AggregationData struct {
			AccountMedia []AccountMedia `json:"accountMedia"`
		} `json:"aggregationData"`
	} `json:"response"`
}

// FetchPurchasedAlbums now returns the full Album object.
func FetchPurchasedAlbums(fanslyHeaders *headers.FanslyHeaders) (*Album, error) {
	url := "https://apiv3.fansly.com/api/v1/uservault/albumsnew?accountId&ngsw-bypass=true"
	req, err := http.NewRequest("GET", url, nil)
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

	var albumsResp AlbumsResponse
	err = json.NewDecoder(resp.Body).Decode(&albumsResp)
	if err != nil {
		return nil, err
	}

	for _, album := range albumsResp.Response.Albums {
		if album.Type == 2007 { // 2007 is the type for purchases
			return &album, nil
		}
	}

	return nil, fmt.Errorf("no purchases album found")
}

func FetchAlbumContent(albumID string, fanslyHeaders *headers.FanslyHeaders) (*AlbumContentResponse, error) {
	var allContent AlbumContentResponse
	before := "0"
	hasMore := true
	limiter := rate.NewLimiter(rate.Every(3*time.Second), 2)

	for hasMore {
		if err := limiter.Wait(context.Background()); err != nil {
			return nil, fmt.Errorf("rate limiter error: %v", err)
		}

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/uservault/album/content?albumId=%s&before=%s&after=0&limit=25&ngsw-bypass=true", albumID, before)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		fanslyHeaders.AddHeadersToRequest(req, true)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		// No defer resp.Body.Close() inside the loop

		var contentResp AlbumContentResponse
		err = json.NewDecoder(resp.Body).Decode(&contentResp)
		resp.Body.Close() // Close body after decoding
		if err != nil {
			return nil, err
		}

		allContent.Response.AlbumContent = append(allContent.Response.AlbumContent, contentResp.Response.AlbumContent...)
		allContent.Response.AggregationData.AccountMedia = append(allContent.Response.AggregationData.AccountMedia, contentResp.Response.AggregationData.AccountMedia...)

		if len(contentResp.Response.AlbumContent) == 0 {
			hasMore = false
		} else {
			before = contentResp.Response.AlbumContent[len(contentResp.Response.AlbumContent)-1].ID
		}
	}

	logger.Logger.Printf("Fetched %d purchased media items.", len(allContent.Response.AggregationData.AccountMedia))

	return &allContent, nil
}

func FetchAccountInfo(accountID string, fanslyHeaders *headers.FanslyHeaders) (string, error) {
	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?ids=%s&ngsw-bypass=true", accountID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var accountResp struct {
		Success  bool `json:"success"`
		Response []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"response"`
	}

	err = json.NewDecoder(resp.Body).Decode(&accountResp)
	if err != nil {
		return "", err
	}

	if len(accountResp.Response) > 0 {
		return accountResp.Response[0].Username, nil
	}

	return "", fmt.Errorf("no account found for ID %s", accountID)
}
