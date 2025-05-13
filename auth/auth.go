package auth

// Auth handles initial login to display user's name and following list when selecting an option.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
)

type Config struct {
	Client        http.Client
	UserAgent     string `json:"user-agent"`
	Authorization string `json:"auth_token"`
}

// UserInfo on the user's account
type AccountInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type ApiResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Account AccountInfo `json:"account"`
	} `json:"response"`
}

type InitialResponse struct {
	Success  bool        `json:"success"`
	Response []AccountID `json:"response"`
}

type AccountID struct {
	AccountId string `json:"accountId"`
}

type TimelineStats struct {
	AccountId      string `json:"accountId"`
	ImageCount     int    `json:"imageCount"`
	VideoCount     int    `json:"videoCount"`
	BundleCount    int    `json:"bundleCount"`
	BundleImgCount int    `json:"bundleImageCount"`
	BundleVidCount int    `json:"bundleVideoCount"`
}

type FollowedModel struct {
	ID            string        `json:"id"`
	Username      string        `json:"username"`
	DisplayName   string        `json:"displayName"`
	TimelineStats TimelineStats `json:"timelineStats"`
}

// Login retrieves the user's account information using the provided headers.
func Login(fanslyHeaders *headers.FanslyHeaders) (*AccountInfo, error) {
	// Create a new HTTP client
	client := &http.Client{}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/account/me?ngsw-bypass=true", nil)
	if err != nil {
		return nil, err
	}

	// Set the headers using the FanslyHeaders struct
	fanslyHeaders.AddHeadersToRequest(req, true)

	// Send the HTTP request and get the response
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("login failed with status code %d", resp.StatusCode)
	}

	// Decode the JSON response
	var apiResponse ApiResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}

	accountInfo := &apiResponse.Response.Account
	return accountInfo, nil
}

func GetFollowedUsers(userId string, fanslyHeaders *headers.FanslyHeaders) ([]FollowedModel, error) {
	// Get List of followed account IDs with pagination
	client := &http.Client{}

	// First, get the total count of followed accounts
	var allAccountIDs []AccountID
	offset := 0
	batchSize := 200 // Smaller batch size to avoid rate limiting

	for {
		// Add delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account/%s/following?before=0&after=0&limit=%d&offset=%d",
			userId, batchSize, offset)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		// Set the headers using the FanslyHeaders struct
		fanslyHeaders.AddHeadersToRequest(req, true)

		// Use exponential backoff for retries
		var resp *http.Response
		maxRetries := 5
		retryDelay := 1 * time.Second

		for retry := 0; retry < maxRetries; retry++ {
			resp, err = client.Do(req)
			if err != nil {
				return nil, err
			}

			// If we get rate limited, wait and retry
			if resp.StatusCode == 429 {
				resp.Body.Close()
				waitTime := retryDelay * time.Duration(1<<uint(retry))
				time.Sleep(waitTime)
				continue
			}

			// If we get any other error, return it
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("failed to fetch following list with status code %d", resp.StatusCode)
			}

			// If we get here, we have a successful response
			break
		}

		// If we've exhausted our retries and still have an error
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch following list after multiple retries with status code %d", resp.StatusCode)
		}

		var followingResponse struct {
			Success  bool        `json:"success"`
			Response []AccountID `json:"response"`
		}

		err = json.NewDecoder(resp.Body).Decode(&followingResponse)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if len(followingResponse.Response) == 0 {
			break // No more accounts to fetch
		}

		allAccountIDs = append(allAccountIDs, followingResponse.Response...)

		if len(followingResponse.Response) < batchSize {
			break // Last batch
		}

		offset += batchSize
	}

	// Process account IDs in smaller batches to avoid 413 errors
	var allModels []FollowedModel
	batchSize = 100 // Even smaller batch size for fetching account details

	for i := 0; i < len(allAccountIDs); i += batchSize {
		// Add delay between batches to avoid rate limiting
		time.Sleep(1 * time.Second)

		end := min(i+batchSize, len(allAccountIDs))
		batch := allAccountIDs[i:end]

		// Concatenate batch IDs with commas
		batchIDsSlice := make([]string, len(batch))
		for j, accountId := range batch {
			batchIDsSlice[j] = accountId.AccountId
		}
		idsParam := strings.Join(batchIDsSlice, ",")

		// Make request for this batch
		modelsURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?ids=%s&ngsw-bypass=true", idsParam)
		req, err := http.NewRequest("GET", modelsURL, nil)
		if err != nil {
			return nil, err
		}

		// Set the headers using the FanslyHeaders struct
		fanslyHeaders.AddHeadersToRequest(req, true)

		// Use exponential backoff for retries
		var resp *http.Response
		maxRetries := 5
		retryDelay := 1 * time.Second

		for retry := 0; retry < maxRetries; retry++ {
			resp, err = client.Do(req)
			if err != nil {
				return nil, err
			}

			// If we get rate limited, wait and retry
			if resp.StatusCode == 429 {
				resp.Body.Close()
				waitTime := retryDelay * time.Duration(1<<uint(retry))
				time.Sleep(waitTime)
				continue
			}

			// If we get any other error, return it
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("failed to fetch models information with status code %d", resp.StatusCode)
			}

			// If we get here, we have a successful response
			break
		}

		// If we've exhausted our retries and still have an error
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch models information after multiple retries with status code %d", resp.StatusCode)
		}

		var modelsResponse struct {
			Success  bool            `json:"success"`
			Response []FollowedModel `json:"response"`
		}

		err = json.NewDecoder(resp.Body).Decode(&modelsResponse)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		allModels = append(allModels, modelsResponse.Response...)
	}

	return allModels, nil
}
