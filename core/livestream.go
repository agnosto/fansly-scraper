package core

import (
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/agnosto/fansly-scraper/config"
)

type StreamResponse struct {
    Success bool `json:"success"`
    Response struct {
        Stream struct {
            Status string `json:"status"`
        } `json:"stream"`
    } `json:"response"`
}

func CheckIfModelIsLive(modelID string) (bool, error) {
    cfg, err := config.LoadConfig(config.GetConfigPath())
    if err != nil {
        return false, fmt.Errorf("failed to load config: %v", err)
    }

    url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/streaming/channel/%s", modelID)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return false, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Authorization", cfg.Account.AuthToken)
    req.Header.Set("User-Agent", cfg.Account.UserAgent)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return false, fmt.Errorf("failed to send request: %v", err)
    }
    defer resp.Body.Close()

    var streamResp StreamResponse
    if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
        return false, fmt.Errorf("failed to decode response: %v", err)
    }

    return streamResp.Success && streamResp.Response.Stream.Status == "live", nil
}
