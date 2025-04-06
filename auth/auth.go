package auth

// Auth handles initial login to display user's name and following list when selecting an option.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	//"github.com/agnosto/fansly-scraper/headers"
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

// Login retrieves the user's account information using the provided auth token and user agent.
func Login(authToken string, userAgent string) (*AccountInfo, error) {
	// Create the headers
	// headers := NewHeaders(authToken, userAgent)
	headerMap := map[string]string{
		"Authorization": authToken,
		"User-Agent":    userAgent,
	}
	// Create a new HTTP client
	client := &http.Client{}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/account/me?ngsw-bypass=true", nil)
	if err != nil {
		return nil, err
	}

	// Set the headers
	for key, value := range headerMap {
		req.Header.Add(key, value)
	}

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
	// var accountInfo AccountInfo
	var apiResponse ApiResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}

	accountInfo := &apiResponse.Response.Account

	return accountInfo, nil
}

func GetFollowedUsers(userId string, authToken string, userAgent string) ([]FollowedModel, error) {
	headerMap := map[string]string{
		"Authorization": authToken,
		"User-Agent":    userAgent,
	}
	// Get List of followed account IDs
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/account/"+userId+"/following?before=0&after=0&limit=999&offset=0", nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headerMap {
		req.Header.Add(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch following list with status code %d", resp.StatusCode)
	}

	var followingResponse struct {
		Success  bool        `json:"success"`
		Response []AccountID `json:"response"`
	}
	err = json.NewDecoder(resp.Body).Decode(&followingResponse)
	if err != nil {
		return nil, err
	}

	// Concatenate all account IDs with commas
	accountIDs := make([]string, len(followingResponse.Response))
	for i, accountId := range followingResponse.Response {
		accountIDs[i] = accountId.AccountId
	}
	idsParam := strings.Join(accountIDs, ",")

	// Make a single request to get all followed models' information
	modelsURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?ids=%s&ngsw-bypass=true", idsParam)
	req, err = http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headerMap {
		req.Header.Add(key, value)
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models information with status code %d", resp.StatusCode)
	}

	var modelsResponse struct {
		Success  bool            `json:"success"`
		Response []FollowedModel `json:"response"`
	}
	err = json.NewDecoder(resp.Body).Decode(&modelsResponse)
	if err != nil {
		return nil, err
	}

	return modelsResponse.Response, nil

}
