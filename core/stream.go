package core

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/agnosto/fansly-scraper/config"
)

type StreamData struct {
    Username    string
    StreamID    string
    PlaybackURL string
}

func GetStreamData(modelID string) (StreamData, error) {
    url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/streaming/channel/%s?ngsw-bypass=true", modelID)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return StreamData{}, fmt.Errorf("error creating request: %v", err)
    }

    // Add headers from config
    cfg, _ := config.LoadConfig(config.GetConfigPath())
    req.Header.Add("Authorization", cfg.Account.AuthToken)
    req.Header.Add("User-Agent", cfg.Account.UserAgent)

    resp, err := client.Do(req)
    if err != nil {
        return StreamData{}, fmt.Errorf("error making request: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return StreamData{}, fmt.Errorf("error reading response body: %v", err)
    }

    var result map[string]interface{}
    if err := json.Unmarshal(body, &result); err != nil {
        return StreamData{}, fmt.Errorf("error unmarshaling JSON: %v", err)
    }

    if !result["success"].(bool) {
        return StreamData{}, fmt.Errorf("API request was not successful")
    }

    response := result["response"].(map[string]interface{})
    stream := response["stream"].(map[string]interface{})

    return StreamData{
        Username:    stream["title"].(string), // Assuming the title is the username
        StreamID:    stream["id"].(string),
        PlaybackURL: response["playbackUrl"].(string),
    }, nil
}
