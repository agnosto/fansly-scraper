package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/headers"
)

type StreamData struct {
	Username      string
	ChatRoomID    string
	StreamID      string
	StreamVersion string
	PlaybackURL   string
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

	// Create FanslyHeaders instance
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		return StreamData{}, fmt.Errorf("error loading config: %v", err)
	}

	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return StreamData{}, fmt.Errorf("error creating headers: %v", err)
	}

	// Use the headers package to add headers
	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := client.Do(req)
	if err != nil {
		return StreamData{}, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StreamData{}, fmt.Errorf("error reading response body: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return StreamData{}, fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	if !result["success"].(bool) {
		return StreamData{}, fmt.Errorf("API request was not successful")
	}

	response := result["response"].(map[string]any)
	stream := response["stream"].(map[string]any)
	version := fmt.Sprintf("%.0f", stream["version"].(float64))

	return StreamData{
		Username:      stream["title"].(string), // Assuming the title is the username
		ChatRoomID:    response["chatRoomId"].(string),
		StreamID:      stream["id"].(string),
		StreamVersion: version,
		PlaybackURL:   response["playbackUrl"].(string),
	}, nil
}
