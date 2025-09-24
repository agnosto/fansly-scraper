package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/gen2brain/beeep"
)

type NotificationService struct {
	config *config.Config
}

func NewNotificationService(cfg *config.Config) *NotificationService {
	return &NotificationService{
		config: cfg,
	}
}

// NotifyLiveStart sends notifications when a model goes live (text only)
func (ns *NotificationService) NotifyLiveStart(username, modelID string) {
	if !ns.config.Notifications.Enabled || !ns.config.Notifications.NotifyOnLiveStart {
		return
	}

	message := fmt.Sprintf("%s is now live!", username)

	// Send system notification (no icon for live start)
	if ns.config.Notifications.SystemNotify {
		ns.sendSystemNotification(message, "Fansly Live Alert", "")
	}

	// Send Discord notification (no thumbnail for live start)
	if ns.config.Notifications.DiscordWebhook != "" {
		ns.sendDiscordNotification(message, username, modelID, true, "")
	}

	// Send Telegram notification (no thumbnail for live start)
	if ns.config.Notifications.TelegramBotToken != "" && ns.config.Notifications.TelegramChatID != "" {
		ns.sendTelegramNotification(message, "")
	}
}

// NotifyLiveEnd sends notifications when a model's stream ends, with an optional thumbnail
func (ns *NotificationService) NotifyLiveEnd(username, modelID, recordedFilename, contactSheetPath string) {
	if !ns.config.Notifications.Enabled || !ns.config.Notifications.NotifyOnLiveEnd {
		return
	}

	message := fmt.Sprintf("%s's stream has ended. Recording saved.", username)

	// Check if the thumbnail should be sent based on config and if the file exists
	var thumbnailPathToSend string
	if ns.config.Notifications.SendContactSheetOnLiveEnd && contactSheetPath != "" {
		if _, err := os.Stat(contactSheetPath); err == nil {
			thumbnailPathToSend = contactSheetPath
		} else {
			logger.Logger.Printf("Contact sheet thumbnail not found at path: %s", contactSheetPath)
		}
	}

	// Send system notification (with icon if available)
	if ns.config.Notifications.SystemNotify {
		ns.sendSystemNotification(message, "Fansly Stream Ended", thumbnailPathToSend)
	}

	// Send Discord notification (with thumbnail if available)
	if ns.config.Notifications.DiscordWebhook != "" {
		ns.sendDiscordNotification(message, username, modelID, false, thumbnailPathToSend)
	}

	// Send Telegram notification (with thumbnail if available)
	if ns.config.Notifications.TelegramBotToken != "" && ns.config.Notifications.TelegramChatID != "" {
		ns.sendTelegramNotification(message, thumbnailPathToSend)
	}
}

// sendSystemNotification sends a system notification, optionally with an icon
func (ns *NotificationService) sendSystemNotification(message, title, iconPath string) {
	err := beeep.Notify(title, message, iconPath)
	if err != nil {
		logger.Logger.Printf("Failed to send system notification: %v", err)
	}
}

// sendDiscordNotification sends a notification, handling both live start (text) and live end (optional image)
func (ns *NotificationService) sendDiscordNotification(message, username, modelID string, isLiveStart bool, thumbnailPath string) error {
	type DiscordEmbed struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       int    `json:"color"`
		Timestamp   string `json:"timestamp"`
		URL         string `json:"url,omitempty"`
		Footer      struct {
			Text string `json:"text"`
		} `json:"footer"`
		Image struct {
			URL string `json:"url"`
		} `json:"image"`
	}

	type DiscordWebhookPayload struct {
		Content string         `json:"content"`
		Embeds  []DiscordEmbed `json:"embeds"`
	}

	liveURL := fmt.Sprintf("https://fansly.com/live/%s", username)

	var embedTitle string
	var embedColor int
	var content string

	if ns.config.Notifications.DiscordMentionID != "" {
		if roleID, found := strings.CutPrefix(ns.config.Notifications.DiscordMentionID, "role:"); found {
			content = fmt.Sprintf("<@&%s>", roleID)
		} else {
			content = fmt.Sprintf("<@%s>", ns.config.Notifications.DiscordMentionID)
		}
	}

	if isLiveStart {
		embedTitle = "Fansly Live Alert"
		embedColor = 3447003 // Blue
	} else {
		embedTitle = "Fansly Stream Ended"
		embedColor = 15158332 // Red
	}

	embed := DiscordEmbed{
		Title:       embedTitle,
		Description: message,
		Color:       embedColor,
		Timestamp:   time.Now().Format(time.RFC3339),
		URL:         liveURL,
	}
	embed.Footer.Text = fmt.Sprintf("Model ID: %s", modelID)

	payload := DiscordWebhookPayload{
		Content: content, // The 'content' variable now holds the mention for both cases
		Embeds:  []DiscordEmbed{embed},
	}

	// If no thumbnail, send a simple JSON request
	if thumbnailPath == "" {
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			logger.Logger.Printf("Failed to marshal Discord payload: %v", err)
			return err
		}
		resp, err := http.Post(ns.config.Notifications.DiscordWebhook, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			logger.Logger.Printf("Failed to send Discord notification: %v", err)
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			logger.Logger.Printf("Discord webhook returned status: %d", resp.StatusCode)
		}
		return nil
	}

	// If a thumbnail exists, build a multipart request to upload the file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Attach the image file
	file, err := os.Open(thumbnailPath)
	if err != nil {
		logger.Logger.Printf("Error opening thumbnail for Discord, sending text-only: %v", err)
		return ns.sendDiscordNotification(message, username, modelID, isLiveStart, "") // Fallback
	}
	defer file.Close()

	// Discord expects the filename in the attachment URL, so we set it here
	payload.Embeds[0].Image.URL = "attachment://" + filepath.Base(thumbnailPath)

	// Add the JSON part
	jsonPayload, _ := json.Marshal(payload)
	_ = writer.WriteField("payload_json", string(jsonPayload))

	// Add the file part
	part, err := writer.CreateFormFile("file", filepath.Base(thumbnailPath))
	if err != nil {
		return err
	}
	_, _ = io.Copy(part, file)
	writer.Close()

	// Send the multipart request
	req, _ := http.NewRequest("POST", ns.config.Notifications.DiscordWebhook, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Logger.Printf("Discord webhook returned status: %d", resp.StatusCode)
	}
	return nil
}

// sendTelegramNotification sends a notification, using sendPhoto if a thumbnail is provided
func (ns *NotificationService) sendTelegramNotification(message string, thumbnailPath string) {
	// If a thumbnail path is provided, use the sendPhoto endpoint
	if thumbnailPath != "" {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", ns.config.Notifications.TelegramBotToken)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("chat_id", ns.config.Notifications.TelegramChatID)
		_ = writer.WriteField("caption", message)
		_ = writer.WriteField("parse_mode", "HTML")

		file, err := os.Open(thumbnailPath)
		if err != nil {
			logger.Logger.Printf("Error opening thumbnail for Telegram, sending text-only: %v", err)
			ns.sendTelegramNotification(message, "") // Fallback to text-only
			return
		}
		defer file.Close()

		part, _ := writer.CreateFormFile("photo", filepath.Base(thumbnailPath))
		_, _ = io.Copy(part, file)
		writer.Close()

		req, _ := http.NewRequest("POST", url, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode >= 300 {
			logger.Logger.Printf("Failed to send Telegram photo notification. Status: %v, Err: %v", resp.Status, err)
		} else {
			resp.Body.Close()
		}
		return
	}

	// Fallback to text-only sendMessage endpoint
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ns.config.Notifications.TelegramBotToken)
	type TelegramPayload struct {
		ChatID    string `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}
	payload := TelegramPayload{
		ChatID:    ns.config.Notifications.TelegramChatID,
		Text:      message,
		ParseMode: "HTML",
	}
	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Logger.Printf("Failed to send Telegram notification: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		logger.Logger.Printf("Telegram API returned status: %d", resp.StatusCode)
	}
}
