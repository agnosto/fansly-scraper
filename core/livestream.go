package core

import (
	"encoding/json"
	"fmt"
	"github.com/agnosto/fansly-scraper/config"
	"net/http"
	//"time"
)

type StreamResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Stream struct {
			Status        int    `json:"status"`
			ViewerCount   int    `json:"viewerCount"`
			LastFetchedAt int64  `json:"lastFetchedAt"`
			PlaybackUrl   string `json:"playbackUrl"`
			Access        bool   `json:"access"`
		} `json:"stream"`
	} `json:"response"`
}

func CheckIfModelIsLive(modelID string) (bool, string, error) {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		return false, "", fmt.Errorf("failed to load config: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/streaming/channel/%s", modelID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", cfg.Account.AuthToken)
	req.Header.Set("User-Agent", cfg.Account.UserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var streamResp StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		return false, "", fmt.Errorf("failed to decode response: %v", err)
	}

	isLive := streamResp.Success &&
		streamResp.Response.Stream.Status == 2 &&
		streamResp.Response.Stream.Access /*&&
		time.Now().UnixMilli()-streamResp.Response.Stream.LastFetchedAt < 5*60*1000*/ // Last fetched within 5 minutes

	return isLive, streamResp.Response.Stream.PlaybackUrl, nil
}
