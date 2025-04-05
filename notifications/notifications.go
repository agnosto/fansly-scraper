package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
)

type NotificationService struct {
	config *config.Config
}

func NewNotificationService(cfg *config.Config) *NotificationService {
	return &NotificationService{
		config: cfg,
	}
}

// NotifyLiveStart sends notifications when a model goes live
func (ns *NotificationService) NotifyLiveStart(username, modelID string) {
	if !ns.config.Notifications.Enabled || !ns.config.Notifications.NotifyOnLiveStart {
		return
	}

	message := fmt.Sprintf("%s is now live!", username)

	// Send system notification if enabled
	if ns.config.Notifications.SystemNotify {
		ns.sendSystemNotification(message, "Fansly Live Alert")
	}

	// Send Discord notification if configured
	if ns.config.Notifications.DiscordWebhook != "" {
		ns.sendDiscordNotification(message, username, modelID)
	}

	// Send Telegram notification if configured
	if ns.config.Notifications.TelegramBotToken != "" && ns.config.Notifications.TelegramChatID != "" {
		ns.sendTelegramNotification(message)
	}
}

// NotifyLiveEnd sends notifications when a model's stream ends
func (ns *NotificationService) NotifyLiveEnd(username, modelID, recordedFilename string) {
	if !ns.config.Notifications.Enabled || !ns.config.Notifications.NotifyOnLiveEnd {
		return
	}

	message := fmt.Sprintf("%s's stream has ended. Recording saved.", username)

	// Send system notification if enabled
	if ns.config.Notifications.SystemNotify {
		ns.sendSystemNotification(message, "Fansly Stream Ended")
	}

	// Send Discord notification if configured
	if ns.config.Notifications.DiscordWebhook != "" {
		ns.sendDiscordNotification(message, username, modelID)
	}

	// Send Telegram notification if configured
	if ns.config.Notifications.TelegramBotToken != "" && ns.config.Notifications.TelegramChatID != "" {
		ns.sendTelegramNotification(message)
	}
}

// sendSystemNotification sends a system notification based on the OS
func (ns *NotificationService) sendSystemNotification(message, title string) {
	switch runtime.GOOS {
	case "darwin":
		ns.sendMacNotification(message, title)
	case "linux":
		ns.sendLinuxNotification(message, title)
	case "windows":
		ns.sendWindowsNotification(message, title)
	default:
		logger.Logger.Printf("System notifications not supported on %s", runtime.GOOS)
	}
}

// sendMacNotification sends a notification on macOS
func (ns *NotificationService) sendMacNotification(message, title string) {
	cmd := exec.Command("osascript", "-e", fmt.Sprintf(`display notification "%s" with title "%s"`, message, title))
	if err := cmd.Run(); err != nil {
		logger.Logger.Printf("Failed to send macOS notification: %v", err)
	}
}

// sendLinuxNotification sends a notification on Linux
func (ns *NotificationService) sendLinuxNotification(message, title string) {
	cmd := exec.Command("notify-send", title, message)
	if err := cmd.Run(); err != nil {
		logger.Logger.Printf("Failed to send Linux notification: %v", err)
	}
}

// sendWindowsNotification sends a notification on Windows
func (ns *NotificationService) sendWindowsNotification(message, title string) {
	// Using PowerShell for Windows notifications
	script := fmt.Sprintf(`
		[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
		$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
		$toastXml = [xml]$template
		$toastXml.GetElementsByTagName("text")[0].AppendChild($toastXml.CreateTextNode("%s")) > $null
		$toastXml.GetElementsByTagName("text")[1].AppendChild($toastXml.CreateTextNode("%s")) > $null
		$toast = [Windows.UI.Notifications.ToastNotification]::new($toastXml)
		[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Fansly Scraper").Show($toast)
	`, title, message)

	cmd := exec.Command("powershell", "-Command", script)
	if err := cmd.Run(); err != nil {
		logger.Logger.Printf("Failed to send Windows notification: %v", err)
	}
}

// sendDiscordNotification sends a notification to Discord via webhook
func (ns *NotificationService) sendDiscordNotification(message, username, modelID string) error {
	type DiscordEmbed struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       int    `json:"color"`
		Timestamp   string `json:"timestamp"`
		URL         string `json:"url,omitempty"` // Add URL field for clickable title
		Footer      struct {
			Text string `json:"text"`
		} `json:"footer"`
	}

	type DiscordWebhookPayload struct {
		Content string         `json:"content"`
		Embeds  []DiscordEmbed `json:"embeds"`
	}

	// Create a clickable link to the live stream
	liveURL := fmt.Sprintf("https://fansly.com/live/%s", username)

	embed := DiscordEmbed{
		Title:       "Fansly Live Alert",
		Description: message,
		Color:       3447003, // Blue color
		Timestamp:   time.Now().Format(time.RFC3339),
		URL:         liveURL, // Add the URL to make the title clickable
	}

	embed.Footer.Text = fmt.Sprintf("Model ID: %s", modelID)

	// Add role/user mention in the content field
	// This will ping the role/user when the notification is sent
	// You can configure this in the config file
	content := ""
	if ns.config.Notifications.DiscordMentionID != "" {
		// Check if it's a role or user mention based on the prefix
		if strings.HasPrefix(ns.config.Notifications.DiscordMentionID, "role:") {
			// It's a role mention
			roleID := strings.TrimPrefix(ns.config.Notifications.DiscordMentionID, "role:")
			content = fmt.Sprintf("<@&%s> %s is now live! %s", roleID, username, liveURL)
		} else {
			// It's a user mention
			content = fmt.Sprintf("<@%s> %s is now live! %s", ns.config.Notifications.DiscordMentionID, username, liveURL)
		}
	} else {
		// No mention, just include the message and URL
		content = fmt.Sprintf("%s is now live! %s", username, liveURL)
	}

	payload := DiscordWebhookPayload{
		Content: content,
		Embeds:  []DiscordEmbed{embed},
	}

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Logger.Printf("Discord webhook returned status: %d", resp.StatusCode)
		return fmt.Errorf("discord webhook returned status: %d", resp.StatusCode)
	}

	return nil
}

// sendTelegramNotification sends a notification to Telegram
func (ns *NotificationService) sendTelegramNotification(message string) {
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

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		logger.Logger.Printf("Failed to marshal Telegram payload: %v", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		logger.Logger.Printf("Failed to send Telegram notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Logger.Printf("Telegram API returned status: %d", resp.StatusCode)
	}
}
