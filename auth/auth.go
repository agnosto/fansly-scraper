package auth

// Auth handles initial login to display user's name and following list when selecting an option.
import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
)

var (
	// Store the current user's ID after login to be used by other packages
	currentAccountID string
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

type Wall struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Metadata    string `json:"metadata"`
	Pos         int    `json:"pos"`
}

type FollowedModel struct {
	ID            string        `json:"id"`
	Username      string        `json:"username"`
	DisplayName   string        `json:"displayName"`
	TimelineStats TimelineStats `json:"timelineStats"`
	Walls         []Wall        `json:"walls"`
}

// Subscription structures
type Subscription struct {
	ID                    string `json:"id"`
	AccountId             string `json:"accountId"`
	SubscriptionTierId    string `json:"subscriptionTierId"`
	SubscriptionTierName  string `json:"subscriptionTierName"`
	SubscriptionTierColor string `json:"subscriptionTierColor"`
	Status                int    `json:"status"`
	Price                 int    `json:"price"`
	EndsAt                int64  `json:"endsAt"`
}

type SubscriptionsResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Stats struct {
			TotalActive  int `json:"totalActive"`
			TotalExpired int `json:"totalExpired"`
			Total        int `json:"total"`
		} `json:"stats"`
		Subscriptions []Subscription `json:"subscriptions"`
	} `json:"response"`
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

	// Store the user's ID for later use
	if accountInfo.ID != "" {
		currentAccountID = accountInfo.ID
	}

	return accountInfo, nil
}

// GetMyUserID returns the logged-in user's ID. Login must be called first.
func GetMyUserID() (string, error) {
	if currentAccountID == "" {
		return "", fmt.Errorf("user not logged in or user ID not found")
	}
	return currentAccountID, nil
}

func GetSubscriptions(fanslyHeaders *headers.FanslyHeaders) ([]string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/subscriptions?ngsw-bypass=true", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating subscriptions request: %v", err)
	}

	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscriptions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscriptions request failed with status code %d", resp.StatusCode)
	}

	var subscriptionsResponse SubscriptionsResponse
	err = json.NewDecoder(resp.Body).Decode(&subscriptionsResponse)
	if err != nil {
		return nil, fmt.Errorf("error decoding subscriptions response: %v", err)
	}

	logger.Logger.Printf("Subscriptions API response - Success: %v, Total: %d, Active: %d, Expired: %d",
		subscriptionsResponse.Success,
		subscriptionsResponse.Response.Stats.Total,
		subscriptionsResponse.Response.Stats.TotalActive,
		subscriptionsResponse.Response.Stats.TotalExpired)

	var accountIDs []string
	for _, subscription := range subscriptionsResponse.Response.Subscriptions {
		accountIDs = append(accountIDs, subscription.AccountId)
		logger.Logger.Printf("Found subscription to account %s (Status: %d)", subscription.AccountId, subscription.Status)
	}

	return accountIDs, nil
}

func GetFollowedUsers(userId string, fanslyHeaders *headers.FanslyHeaders) ([]FollowedModel, error) {
	// Get both followed accounts and subscriptions
	var allAccountIDs []string
	accountIDSet := make(map[string]bool) // To avoid duplicates

	// 1. Get followed accounts (free follows)
	followedIDs, err := getFollowedAccountIDs(userId, fanslyHeaders)
	if err != nil {
		logger.Logger.Printf("Error getting followed accounts: %v", err)
		// Don't return error, continue with subscriptions
	} else {
		for _, id := range followedIDs {
			if !accountIDSet[id] {
				allAccountIDs = append(allAccountIDs, id)
				accountIDSet[id] = true
			}
		}
		logger.Logger.Printf("Found %d followed accounts", len(followedIDs))
	}

	// 2. Get subscribed accounts
	subscribedIDs, err := GetSubscriptions(fanslyHeaders)
	if err != nil {
		logger.Logger.Printf("Error getting subscriptions: %v", err)
		// Don't return error, continue with what we have
	} else {
		for _, id := range subscribedIDs {
			if !accountIDSet[id] {
				allAccountIDs = append(allAccountIDs, id)
				accountIDSet[id] = true
			}
		}
		logger.Logger.Printf("Found %d subscribed accounts", len(subscribedIDs))
	}

	logger.Logger.Printf("Total unique account IDs: %d", len(allAccountIDs))

	if len(allAccountIDs) == 0 {
		logger.Logger.Printf("No followed or subscribed accounts found")
		return []FollowedModel{}, nil
	}

	// 3. Get account details for all IDs
	return GetAccountDetails(allAccountIDs, fanslyHeaders)
}

func getFollowedAccountIDs(userId string, fanslyHeaders *headers.FanslyHeaders) ([]string, error) {
	client := &http.Client{}
	var allAccountIDs []string
	offset := 0
	batchSize := 100

	logger.Logger.Printf("Starting to fetch followed accounts for userId: %s", userId)

	for {
		time.Sleep(500 * time.Millisecond)

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account/%s/following?before=0&after=0&limit=%d&offset=%d",
			userId, batchSize, offset)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

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
				logger.Logger.Printf("Rate limited (429) when fetching following list, retrying in %v", waitTime)
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

		logger.Logger.Printf("Following API response - Success: %v, Count: %d", followingResponse.Success, len(followingResponse.Response))

		if len(followingResponse.Response) == 0 {
			break
		}

		for _, accountID := range followingResponse.Response {
			allAccountIDs = append(allAccountIDs, accountID.AccountId)
		}

		//if len(followingResponse.Response) < batchSize {
		//	break
		//}

		offset += len(followingResponse.Response)
	}

	return allAccountIDs, nil
}

// GetAccountDetails retrieves account information for a list of account IDs.
func GetAccountDetails(accountIDs []string, fanslyHeaders *headers.FanslyHeaders) ([]FollowedModel, error) {
	client := &http.Client{}
	var allModels []FollowedModel
	batchSize := 50 // Even smaller batch size for fetching account details

	for i := 0; i < len(accountIDs); i += batchSize {
		// Add delay between batches to avoid rate limiting
		time.Sleep(2 * time.Second)

		end := min(i+batchSize, len(accountIDs))
		batch := accountIDs[i:end]

		logger.Logger.Printf("Processing batch %d-%d of account details", i, end-1)

		idsParam := strings.Join(batch, ",")
		modelsURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?ids=%s&ngsw-bypass=true", idsParam)

		req, err := http.NewRequest("GET", modelsURL, nil)
		if err != nil {
			return nil, err
		}

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
				logger.Logger.Printf("Rate limited (429) when fetching models information, retrying in %v", waitTime)
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

		logger.Logger.Printf("Models API response - Success: %v, Count: %d", modelsResponse.Success, len(modelsResponse.Response))

		// Note: If some accounts are deleted/invalid, the API will just return fewer results
		// This is normal behavior and we should continue with what we got
		allModels = append(allModels, modelsResponse.Response...)
	}

	logger.Logger.Printf("Final result - returning %d models", len(allModels))
	return allModels, nil
}
