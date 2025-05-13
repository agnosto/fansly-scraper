package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
)

type AccountInfo struct {
	Welcome        string
	FollowedModels []auth.FollowedModel
}

func FetchAccountInfo(configPath string) (AccountInfo, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("error loading config: %v", err)
	}

	// Create FanslyHeaders instance
	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("error creating headers: %v", err)
	}

	accountInfo, err := auth.Login(fanslyHeaders)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("error logging in: %v", err)
	}

	welcome := fmt.Sprintf("Welcome %s | %s", accountInfo.DisplayName, accountInfo.Username)

	// Log that we're starting to fetch followed accounts
	logger.Logger.Printf("Fetching followed accounts for %s. This may take a while if you follow many accounts...", accountInfo.Username)

	followedModels, err := auth.GetFollowedUsers(accountInfo.ID, fanslyHeaders)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("error getting followed models: %v", err)
	}

	logger.Logger.Printf("Successfully fetched %d followed accounts", len(followedModels))

	return AccountInfo{
		Welcome:        welcome,
		FollowedModels: followedModels,
	}, nil
}

//func EditConfig(configPath string) () {}

type AccountResponse struct {
	Success  bool `json:"success"`
	Response []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"response"`
}

func GetModelIDFromUsername(username string) (string, error) {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		return "", fmt.Errorf("failed to load config: %v", err)
	}

	// Create FanslyHeaders instance
	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return "", fmt.Errorf("error creating headers: %v", err)
	}

	AccountURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account?usernames=%s&ngsw-bypass=true", username)
	client := &http.Client{}
	req, err := http.NewRequest("GET", AccountURL, nil)
	if err != nil {
		return "", err
	}

	// Use the headers package to add headers
	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	var accountResponse AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if len(accountResponse.Response) == 0 {
		return "", fmt.Errorf("no account found for username %s", username)
	}

	return accountResponse.Response[0].ID, nil
}
