package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agnosto/fansly-scraper/logger"
	"golang.org/x/time/rate"
	"net/http"
	"time"
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
	ID                 string      `json:"id"`
	MediaOfferId       string      `json:"mediaOfferId"`
	MediaOfferType     int         `json:"mediaOfferType"`
	MediaOfferBundleId interface{} `json:"mediaOfferBundleId"`
	MediaId            string      `json:"mediaId"`
	MediaType          int         `json:"mediaType"`
	PreviewId          interface{} `json:"previewId"`
	AlbumId            string      `json:"albumId"`
	AccountId          string      `json:"accountId"`
	CreatedAt          int64       `json:"createdAt"`
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

func FetchPurchasedAlbums(authToken, userAgent string) (string, error) {
	url := "https://apiv3.fansly.com/api/v1/uservault/albumsnew?accountId&ngsw-bypass=true"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", authToken)
	req.Header.Add("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var albumsResp AlbumsResponse
	err = json.NewDecoder(resp.Body).Decode(&albumsResp)
	if err != nil {
		return "", err
	}

	for _, album := range albumsResp.Response.Albums {
		if album.Type == 2007 { // 2007 is the type for purchases
			return album.ID, nil
		}
	}

	return "", fmt.Errorf("no purchases album found")
}

func FetchAlbumContent(albumID, authToken, userAgent string) (*AlbumContentResponse, error) {
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
		req.Header.Add("Authorization", authToken)
		req.Header.Add("User-Agent", userAgent)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var contentResp AlbumContentResponse
		err = json.NewDecoder(resp.Body).Decode(&contentResp)
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

	logger.Logger.Printf("Fetched allcontent for purchased album. AlbumContent count: %d, AccountMedia count: %d",
		len(allContent.Response.AlbumContent),
		len(allContent.Response.AggregationData.AccountMedia))

	if len(allContent.Response.AlbumContent) > 0 {
		logger.Logger.Printf("Sample AlbumContent: %+v", allContent.Response.AlbumContent[0])
	}
	if len(allContent.Response.AggregationData.AccountMedia) > 0 {
		logger.Logger.Printf("Sample AccountMedia: %+v", allContent.Response.AggregationData.AccountMedia[0])
	}

	logger.Logger.Printf("Fetched allcontent for purchased album %v", allContent)
	return &allContent, nil
}

func FetchAccountInfo(accountID, authToken, userAgent string) (string, error) {
	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?ids=%s&ngsw-bypass=true", accountID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", authToken)
	req.Header.Add("User-Agent", userAgent)

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
		logger.Logger.Printf("Fetched username %s with accountId %s", accountResp.Response[0].Username, accountID)
		return accountResp.Response[0].Username, nil
	}
	return "", nil
}
