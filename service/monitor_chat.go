package service

import (
	"fmt"
	//"path/filepath"
	//"strings"

	"github.com/agnosto/fansly-scraper/core"
)

// InitChatRecorder initializes the chat recorder service
func (ms *MonitoringService) InitChatRecorder() {
	ms.chatRecorder = NewChatRecorder(ms.logger)
}

// StartChatRecording begins recording chat for a livestream
func (ms *MonitoringService) StartChatRecording(modelID, username, chatRoomID, vodFilename string) error {
	if ms.chatRecorder == nil {
		ms.InitChatRecorder()
	}

	// Generate chat filename based on VOD filename
	chatFilename := ms.chatRecorder.GetChatFilename(vodFilename)

	return ms.chatRecorder.StartRecording(modelID, username, chatRoomID, chatFilename)
}

// StopChatRecording stops recording chat for a specific model
func (ms *MonitoringService) StopChatRecording(modelID string) error {
	if ms.chatRecorder == nil {
		return fmt.Errorf("chat recorder not initialized")
	}

	return ms.chatRecorder.StopRecording(modelID)
}

// GetChatRoomID fetches the chat room ID for a model's livestream
func (ms *MonitoringService) GetChatRoomID(modelID string) (string, error) {
	streamData, err := core.GetStreamData(modelID)
	if err != nil {
		return "", fmt.Errorf("error fetching stream data: %v", err)
	}

	// Extract chat room ID from stream data
	chatRoomID := streamData.ChatRoomID
	if chatRoomID == "" {
		return "", fmt.Errorf("no chat room ID found for model %s", modelID)
	}

	return chatRoomID, nil
}
