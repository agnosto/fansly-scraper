package config

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Account         AccountConfig         `toml:"account"`
	Options         OptionsConfig         `toml:"options"`
	LiveSettings    LiveSettingsConfig    `toml:"live_settings"`
	Notifications   NotificationsConfig   `toml:"notifications"`
	SecurityHeaders SecurityHeadersConfig `toml:"security_headers"`
}

type LiveSettingsConfig struct {
	SaveLocation         string `toml:"save_location"` // Optional, defaults to main save location if empty
	VODsFileExtension    string `toml:"vods_file_extension"`
	FFmpegConvert        bool   `toml:"ffmpeg_convert"`
	GenerateContactSheet bool   `toml:"generate_contact_sheet"`
	UseMTForContactSheet bool   `toml:"use_mt_for_contact_sheet"`
	FilenameTemplate     string `toml:"filename_template"` // e.g. "{model_username}_{date}_{streamId}_{streamVersion}"
	DateFormat           string `toml:"date_format"`
	RecordChat           bool   `toml:"record_chat"`
}

type AccountConfig struct {
	AuthToken string `toml:"auth_token"`
	UserAgent string `toml:"user_agent"`
}

type OptionsConfig struct {
	SaveLocation string `toml:"save_location"`
	M3U8Download bool   `toml:"m3u8_dl"`
	CheckUpdates bool   `toml:"check_updates"`
}

type NotificationsConfig struct {
	Enabled           bool   `toml:"enabled"`
	SystemNotify      bool   `toml:"system_notify"`
	DiscordWebhook    string `toml:"discord_webhook"`
	DiscordMentionID  string `toml:"discord_mention_id"`
	TelegramBotToken  string `toml:"telegram_bot_token"`
	TelegramChatID    string `toml:"telegram_chat_id"`
	NotifyOnLiveStart bool   `toml:"notify_on_live_start"`
	NotifyOnLiveEnd   bool   `toml:"notify_on_live_end"`
}

type SecurityHeadersConfig struct {
	DeviceID    string    `toml:"device_id"`
	SessionID   string    `toml:"session_id"`
	CheckKey    string    `toml:"check_key"`
	LastUpdated time.Time `toml:"last_updated"`
}

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func GetConfigPath() string {
	currentDirConfig := "config.toml"
	if _, err := os.Stat(currentDirConfig); err == nil {
		return currentDirConfig
	}

	var configDir string
	var err error

	if runtime.GOOS == "darwin" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		configDir = filepath.Join(homeDir, ".config")
	} else {
		configDir, err = os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
	}

	return filepath.Join(configDir, "fansly-scraper", "config.toml")
}

func GetConfigDir() string {
	var configDir string
	var err error

	if runtime.GOOS == "darwin" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		configDir = filepath.Join(homeDir, ".config")
	} else {
		configDir, err = os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
	}

	return filepath.Join(configDir, "fansly-scraper")
}

func SaveConfig(cfg *Config) error {
	configPath := GetConfigPath()

	// Try to load existing config
	existingConfig, err := LoadConfig(configPath)
	if err == nil {
		// Merge the new config with the existing one
		mergedConfig := MergeConfigs(existingConfig, cfg)
		cfg = mergedConfig
	}

	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(cfg)
}

// MergeConfigs merges two configurations, preserving existing values
// while updating with new values and ensuring defaults for missing fields
func MergeConfigs(existing *Config, new *Config) *Config {
	result := &Config{}

	// Create a default config to use for missing fields
	defaultConfig := CreateDefaultConfig()

	// Merge Account section
	result.Account = existing.Account
	// Always update auth token and user agent if provided in new config
	if new.Account.AuthToken != "" {
		result.Account.AuthToken = new.Account.AuthToken
	}
	if new.Account.UserAgent != "" {
		result.Account.UserAgent = new.Account.UserAgent
	}

	// Merge Options section
	result.Options = existing.Options
	// Update save location if provided in new config
	if new.Options.SaveLocation != "" {
		result.Options.SaveLocation = new.Options.SaveLocation
	}
	// If CheckUpdates is explicitly set in new config, use it
	if reflect.ValueOf(new.Options).FieldByName("CheckUpdates").IsValid() {
		result.Options.CheckUpdates = new.Options.CheckUpdates
	}
	// Ensure the CheckUpdates field exists (for older configs)
	// using zero value check since it's a boolean
	if result.Options.SaveLocation == "" {
		result.Options.SaveLocation = defaultConfig.Options.SaveLocation
	}

	// Merge LiveSettings section
	result.LiveSettings = existing.LiveSettings
	// Ensure all LiveSettings fields have values
	if result.LiveSettings.VODsFileExtension == "" {
		result.LiveSettings.VODsFileExtension = defaultConfig.LiveSettings.VODsFileExtension
	}
	if result.LiveSettings.FilenameTemplate == "" {
		result.LiveSettings.FilenameTemplate = defaultConfig.LiveSettings.FilenameTemplate
	}
	if result.LiveSettings.DateFormat == "" {
		result.LiveSettings.DateFormat = defaultConfig.LiveSettings.DateFormat
	}

	// Update specific fields from new config if they're set
	if new.LiveSettings.SaveLocation != "" {
		result.LiveSettings.SaveLocation = new.LiveSettings.SaveLocation
	}
	if new.LiveSettings.VODsFileExtension != "" {
		result.LiveSettings.VODsFileExtension = new.LiveSettings.VODsFileExtension
	}
	if new.LiveSettings.FilenameTemplate != "" {
		result.LiveSettings.FilenameTemplate = new.LiveSettings.FilenameTemplate
	}
	if new.LiveSettings.DateFormat != "" {
		result.LiveSettings.DateFormat = new.LiveSettings.DateFormat
	}
	if reflect.ValueOf(new.LiveSettings).FieldByName("RecordChat").IsValid() {
		result.LiveSettings.RecordChat = new.LiveSettings.RecordChat
	}

	// For boolean fields, we check if they're explicitly set in the new config
	if reflect.ValueOf(new.LiveSettings).FieldByName("FFmpegConvert").IsValid() {
		result.LiveSettings.FFmpegConvert = new.LiveSettings.FFmpegConvert
	}
	if reflect.ValueOf(new.LiveSettings).FieldByName("GenerateContactSheet").IsValid() {
		result.LiveSettings.GenerateContactSheet = new.LiveSettings.GenerateContactSheet
	}
	if reflect.ValueOf(new.LiveSettings).FieldByName("UseMTForContactSheet").IsValid() {
		result.LiveSettings.UseMTForContactSheet = new.LiveSettings.UseMTForContactSheet
	}

	// Merge Notifications section
	// Start with existing notifications config
	result.Notifications = existing.Notifications

	// Check if the Notifications section exists in the existing config
	// If not, use the default values
	existingNotificationsValue := reflect.ValueOf(existing.Notifications)
	existingNotificationsType := existingNotificationsValue.Type()
	isNotificationsZero := true

	// Check if all fields in Notifications are zero values - modernized with range
	for i := range make([]struct{}, existingNotificationsValue.NumField()) {
		if !existingNotificationsValue.Field(i).IsZero() {
			isNotificationsZero = false
			break
		}
	}

	// If Notifications section is empty, use defaults
	if isNotificationsZero {
		result.Notifications = defaultConfig.Notifications
	} else {
		// Check for missing fields in the existing config
		// This ensures that new fields added to the struct get default values

		// Check for NotifyOnLiveStart field
		_, hasNotifyOnLiveStart := existingNotificationsType.FieldByName("NotifyOnLiveStart")
		if !hasNotifyOnLiveStart {
			result.Notifications.NotifyOnLiveStart = defaultConfig.Notifications.NotifyOnLiveStart
		}

		// Check for NotifyOnLiveEnd field
		_, hasNotifyOnLiveEnd := existingNotificationsType.FieldByName("NotifyOnLiveEnd")
		if !hasNotifyOnLiveEnd {
			result.Notifications.NotifyOnLiveEnd = defaultConfig.Notifications.NotifyOnLiveEnd
		}

		// Check for DiscordMentionID field
		_, hasDiscordMentionID := existingNotificationsType.FieldByName("DiscordMentionID")
		if !hasDiscordMentionID {
			result.Notifications.DiscordMentionID = defaultConfig.Notifications.DiscordMentionID
		}
	}

	// Update notification fields from new config if they're set
	if reflect.ValueOf(new.Notifications).FieldByName("Enabled").IsValid() {
		result.Notifications.Enabled = new.Notifications.Enabled
	}
	if reflect.ValueOf(new.Notifications).FieldByName("SystemNotify").IsValid() {
		result.Notifications.SystemNotify = new.Notifications.SystemNotify
	}
	if reflect.ValueOf(new.Notifications).FieldByName("NotifyOnLiveStart").IsValid() {
		result.Notifications.NotifyOnLiveStart = new.Notifications.NotifyOnLiveStart
	}
	if reflect.ValueOf(new.Notifications).FieldByName("NotifyOnLiveEnd").IsValid() {
		result.Notifications.NotifyOnLiveEnd = new.Notifications.NotifyOnLiveEnd
	}

	// Update string fields if they're not empty
	if new.Notifications.DiscordWebhook != "" {
		result.Notifications.DiscordWebhook = new.Notifications.DiscordWebhook
	}
	if new.Notifications.DiscordMentionID != "" {
		result.Notifications.DiscordMentionID = new.Notifications.DiscordMentionID
	}
	if new.Notifications.TelegramBotToken != "" {
		result.Notifications.TelegramBotToken = new.Notifications.TelegramBotToken
	}
	if new.Notifications.TelegramChatID != "" {
		result.Notifications.TelegramChatID = new.Notifications.TelegramChatID
	}

	// Always update security headers
	result.SecurityHeaders = new.SecurityHeaders

	return result
}

// EnsureConfigUpdated checks if the config file has all required fields
// and updates it with defaults if needed
func EnsureConfigUpdated(configPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	// Create a default config to compare against
	defaultConfig := CreateDefaultConfig()

	// Check for missing fields and update as needed
	updated := false

	// Check LiveSettings fields
	if cfg.LiveSettings.VODsFileExtension == "" {
		cfg.LiveSettings.VODsFileExtension = defaultConfig.LiveSettings.VODsFileExtension
		updated = true
	}
	if cfg.LiveSettings.FilenameTemplate == "" {
		cfg.LiveSettings.FilenameTemplate = defaultConfig.LiveSettings.FilenameTemplate
		updated = true
	}
	if cfg.LiveSettings.DateFormat == "" {
		cfg.LiveSettings.DateFormat = defaultConfig.LiveSettings.DateFormat
		updated = true
	}
	// Check if RecordChat is missing (this is a new field)
	if reflect.ValueOf(cfg.LiveSettings).FieldByName("RecordChat").IsZero() {
		cfg.LiveSettings.RecordChat = defaultConfig.LiveSettings.RecordChat
		updated = true
	}

	// Check Notifications fields using reflection to detect missing fields
	notificationsType := reflect.TypeOf(cfg.Notifications)
	notificationsValue := reflect.ValueOf(&cfg.Notifications).Elem()
	defaultNotificationsValue := reflect.ValueOf(defaultConfig.Notifications)

	// Check for specific notification fields
	fieldsToCheck := []string{
		"NotifyOnLiveStart",
		"NotifyOnLiveEnd",
		"DiscordMentionID",
	}

	for _, fieldName := range fieldsToCheck {
		_, exists := notificationsType.FieldByName(fieldName)
		if !exists {
			// If the field doesn't exist in the struct definition, we can't do anything
			// This should never happen unless the struct definition changes
			continue
		}

		// Get the field value
		fieldValue := notificationsValue.FieldByName(fieldName)
		defaultFieldValue := defaultNotificationsValue.FieldByName(fieldName)

		// Check if the field is a zero value
		if fieldValue.IsZero() {
			// Set the field to the default value
			fieldValue.Set(defaultFieldValue)
			updated = true
		}
	}

	// If fields were updated, save the config
	if updated {
		file, err := os.Create(configPath)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := toml.NewEncoder(file)
		return encoder.Encode(cfg)
	}

	return nil
}

// Your existing functions remain unchanged

// Add this function to your main.go or similar:
func VerifyConfigOnStartup() {
	configPath := GetConfigPath()
	err := EnsureConfigExists(configPath)
	if err != nil {
		log.Printf("Error ensuring config exists: %v", err)
	}

	err = EnsureConfigUpdated(configPath)
	if err != nil {
		log.Printf("Error updating config: %v", err)
	}
}

func OpenConfigInEditor(configPath string) error {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// On Windows, use the default program associated with .txt files
		cmd = exec.Command("cmd", "/C", "start", "", configPath)
	} else {
		// For UNIX-like systems, use the EDITOR environment variable
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default to vim if no EDITOR environment variable is set
		}
		cmd = exec.Command(editor, configPath)
	}

	//cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func CopyFile(srcPath string, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func DownloadConfig(url string, filePath string) error {
	//log.Printf("Downloading config from: %v to path: %v", url, filePath)
	// Get the current working directory
	//rootDir, err := os.Getwd()
	//if err != nil {
	//	return err
	//}

	// Construct the path to the example-config.ini file in the root directory
	//exampleConfigPath := filepath.Join(rootDir, filePath)

	// Check if the file exists in the current directory
	//if _, err := os.Stat(exampleConfigPath); os.IsNotExist(err) {
	// Send a GET request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
	//}

	//return nil
}

func EnsureConfigExists(configPath string) error {
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		if err != nil {
			return err
		}
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config doesn't exist, check for example config
		exampleConfig := filepath.Join("example-config.toml")
		if _, err := os.Stat(exampleConfig); err == nil {
			// Example config exists, copy it
			err = CopyFile(exampleConfig, configPath)
			if err != nil {
				log.Printf("Failed to copy example config: %v", err)
				// Fall through to try creating default config
			} else {
				return nil // Successfully copied example config
			}
		}

		// If we're here, either example config doesn't exist or copying failed
		// Try to create default config
		defaultConfig := CreateDefaultConfig()
		err = SaveConfig(defaultConfig)
		if err != nil {
			log.Printf("Failed to create default config: %v", err)
			// Fall through to try downloading config
		} else {
			return nil // Successfully created default config
		}

		// If we're here, creating default config failed
		// Try to download config
		err = DownloadConfig("https://raw.githubusercontent.com/agnosto/fansly-scraper/main/example-config.toml", filepath.ToSlash(configPath))
		if err != nil {
			return fmt.Errorf("failed to ensure config exists: %v", err)
		}
	}

	return nil
}

func LoadConfig(configPath string) (*Config, error) {
	var config Config
	_, err := toml.DecodeFile(configPath, &config)
	if err != nil {
		return nil, err
	}

	// Validate config values
	if config.Account.UserAgent == "" {
		return nil, fmt.Errorf("user_agent is empty in %v", configPath)
	}
	if config.Account.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is empty in %v", configPath)
	}
	if config.Options.SaveLocation == "" {
		return nil, fmt.Errorf("save_location is empty in %v", configPath)
	}

	config.Options.SaveLocation = filepath.ToSlash(config.Options.SaveLocation)

	if config.SecurityHeaders.LastUpdated.IsZero() {
		config.SecurityHeaders.LastUpdated = time.Now()
	}

	return &config, nil
}

func CreateDefaultConfig() *Config {
	return &Config{
		Account: AccountConfig{
			AuthToken: "",
			UserAgent: "",
		},
		Options: OptionsConfig{
			SaveLocation: "/path/to/save/content/to",
			M3U8Download: false,
			CheckUpdates: false,
		},
		LiveSettings: LiveSettingsConfig{
			SaveLocation:         "", // Empty means use default path
			VODsFileExtension:    ".ts",
			FFmpegConvert:        true,
			GenerateContactSheet: true,
			UseMTForContactSheet: false,
			FilenameTemplate:     "{model_username}_{date}_{streamId}_{streamVersion}",
			DateFormat:           "20060102_150405",
			RecordChat:           true,
		},
		Notifications: NotificationsConfig{
			Enabled:           false,
			SystemNotify:      true,
			DiscordWebhook:    "",
			DiscordMentionID:  "",
			TelegramBotToken:  "",
			TelegramChatID:    "",
			NotifyOnLiveStart: true,
			NotifyOnLiveEnd:   false,
		},
		SecurityHeaders: SecurityHeadersConfig{
			DeviceID:    "",
			SessionID:   "",
			CheckKey:    "",
			LastUpdated: time.Now(),
		},
	}
}

func FormatVODFilename(template string, data map[string]string) string {
	result := template

	// Add 'v' prefix to streamVersion if it exists
	if version, exists := data["streamVersion"]; exists {
		data["streamVersion"] = "v" + version
	}

	// Replace all placeholders with their values
	for key, value := range data {
		placeholder := fmt.Sprintf("{%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func ResolveLiveSavePath(cfg *Config, username string) string {
	// If LiveSettings.SaveLocation is set, use it directly
	if cfg.LiveSettings.SaveLocation != "" {
		return filepath.ToSlash(cfg.LiveSettings.SaveLocation)
	}

	// Otherwise use default path: SaveLocation/username/lives
	return filepath.ToSlash(filepath.Join(cfg.Options.SaveLocation, strings.ToLower(username), "lives"))
}

func GetVODFilename(cfg *Config, data map[string]string) string {
	// Use configured date format, fallback to default if empty
	dateFormat := cfg.LiveSettings.DateFormat
	if dateFormat == "" {
		dateFormat = "20060102_150405"
	}

	data["date"] = time.Now().Format(dateFormat)

	template := cfg.LiveSettings.FilenameTemplate
	if template == "" {
		template = "{model_username}_{date}_{streamId}"
	}

	filename := FormatVODFilename(template, data)
	return filename + cfg.LiveSettings.VODsFileExtension
}

// Example usage in recording logic:
func BuildVODPath(cfg *Config, username string, streamData map[string]string) string {
	savePath := ResolveLiveSavePath(cfg, username)
	filename := GetVODFilename(cfg, streamData)
	return filepath.Join(savePath, filename)
}
