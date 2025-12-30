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
	ChatRoomID    string
	StreamID      string
	StreamVersion string
	HistoryID     string
	Title         string
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

	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		return StreamData{}, fmt.Errorf("error loading config: %v", err)
	}

	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return StreamData{}, fmt.Errorf("error creating headers: %v", err)
	}

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

	// Check success
	if success, ok := result["success"].(bool); !ok || !success {
		return StreamData{}, fmt.Errorf("API request was not successful")
	}

	response, ok := result["response"].(map[string]any)
	if !ok {
		return StreamData{}, fmt.Errorf("response field not found or invalid")
	}

	stream, ok := response["stream"].(map[string]any)
	if !ok {
		return StreamData{}, fmt.Errorf("stream field not found or invalid")
	}

	// Extract only the fields we need
	chatRoomID, _ := response["chatRoomId"].(string)
	streamID, _ := stream["id"].(string)
	historyID, _ := stream["historyId"].(string)
	title, _ := stream["title"].(string)

	version := ""
	if v, ok := stream["version"].(float64); ok {
		version = fmt.Sprintf("%.0f", v)
	}

	return StreamData{
		ChatRoomID:    chatRoomID,
		StreamID:      streamID,
		StreamVersion: version,
		HistoryID:     historyID,
		Title:         title,
	}, nil
}
