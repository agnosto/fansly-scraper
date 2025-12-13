package config

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
)

// VerifyConfigOnStartup runs all checks to ensure a valid config file exists and is updated.
// This should be called when the application starts.
func VerifyConfigOnStartup() {
	configPath := GetConfigPath()
	if err := EnsureConfigExists(configPath); err != nil {
		log.Printf("Error ensuring config exists: %v", err)
	}
	if _, err := LoadConfig(configPath); err == nil {
		if err := EnsureConfigUpdated(configPath); err != nil {
			log.Printf("Error updating config: %v", err)
		}
	}
}

// EnsureConfigExists checks if a config file is present. If not, it attempts to create one
// by copying an example, creating a default, or downloading it from the repo.
func EnsureConfigExists(configPath string) error {
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config doesn't exist, check for local example-config.toml
		if _, err := os.Stat("example-config.toml"); err == nil {
			err = copyFile("example-config.toml", configPath)
			if err == nil {
				return nil // Successfully copied example config
			}
			log.Printf("Failed to copy example config: %v. Trying next method.", err)
		}

		// Try to create a default config
		defaultConfig := CreateDefaultConfig()
		err = SaveConfig(defaultConfig)
		if err == nil {
			return nil // Successfully created default config
		}
		log.Printf("Failed to create default config: %v. Trying next method.", err)

		// Last resort: try to download config from GitHub
		err = downloadFile("https://raw.githubusercontent.com/agnosto/fansly-scraper/main/example-config.toml", configPath)
		if err != nil {
			return fmt.Errorf("all methods to create a config failed. Last error: %v", err)
		}
	}
	return nil
}

// EnsureConfigUpdated checks if the config file has all the latest fields and updates it with defaults if needed.
func EnsureConfigUpdated(configPath string) error {
	// Read the raw TOML file to check which fields are actually present
	var rawConfig map[string]any
	_, err := toml.DecodeFile(configPath, &rawConfig)
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		// If loading fails, it might be an invalid format. We can't safely update it.
		return err
	}

	// Create a pristine default config to source missing values from.
	defaultConfig := CreateDefaultConfig()
	isUpdated := false

	if reflect.DeepEqual(cfg.Options, OptionsConfig{}) {
		cfg.Options = defaultConfig.Options
		isUpdated = true
	} else {
		// Check for specific fields added in later versions
		if cfg.Options.SaveLocation == "" {
			cfg.Options.SaveLocation = defaultConfig.Options.SaveLocation
			isUpdated = true
		}

		// Check if new fields exist in the raw TOML
		if optionsMap, ok := rawConfig["options"].(map[string]any); ok {
			if _, exists := optionsMap["skip_previews"]; !exists {
				cfg.Options.SkipPreviews = defaultConfig.Options.SkipPreviews
				isUpdated = true
			}
			if _, exists := optionsMap["use_content_as_filename"]; !exists {
				cfg.Options.UseContentAsFilename = defaultConfig.Options.UseContentAsFilename
				isUpdated = true
			}
			if _, exists := optionsMap["content_filename_template"]; !exists {
				cfg.Options.ContentFilenameTemplate = defaultConfig.Options.ContentFilenameTemplate
				isUpdated = true
			}
			if _, exists := optionsMap["download_media_type"]; !exists {
				cfg.Options.DownloadMediaType = defaultConfig.Options.DownloadMediaType
				isUpdated = true
			}
			if _, exists := optionsMap["skip_downloaded_posts"]; !exists {
				cfg.Options.SkipDownloadedPosts = defaultConfig.Options.SkipDownloadedPosts
				isUpdated = true
			}
			if _, exists := optionsMap["content_filename_length"]; !exists {
				cfg.Options.ContentFilenameLength = defaultConfig.Options.ContentFilenameLength
				isUpdated = true
			}
			if _, exists := optionsMap["date_format"]; !exists {
				cfg.Options.DateFormat = defaultConfig.Options.DateFormat
				isUpdated = true
			}
			if _, exists := optionsMap["download_profile_pic"]; !exists {
				cfg.Options.DownloadProfilePic = defaultConfig.Options.DownloadProfilePic
				isUpdated = true
			}
			if _, exist := optionsMap["post_limit"]; !exist {
				cfg.Options.PostLimit = defaultConfig.Options.PostLimit
				isUpdated = true
			}
		} else {
			// If options section doesn't exist at all, add the fields
			cfg.Options.SkipPreviews = defaultConfig.Options.SkipPreviews
			cfg.Options.UseContentAsFilename = defaultConfig.Options.UseContentAsFilename
			cfg.Options.ContentFilenameTemplate = defaultConfig.Options.ContentFilenameTemplate
			cfg.Options.DownloadMediaType = defaultConfig.Options.DownloadMediaType
			cfg.Options.SkipDownloadedPosts = defaultConfig.Options.SkipDownloadedPosts
			cfg.Options.ContentFilenameLength = defaultConfig.Options.ContentFilenameLength
			cfg.Options.DateFormat = defaultConfig.Options.DateFormat
			cfg.Options.DownloadProfilePic = defaultConfig.Options.DownloadProfilePic
			cfg.Options.PostLimit = defaultConfig.Options.PostLimit
			isUpdated = true
		}
	}

	// Use reflection to generically check for missing fields.
	// This makes it easier to add new fields in the future.
	if reflect.DeepEqual(cfg.Notifications, NotificationsConfig{}) {
		cfg.Notifications = defaultConfig.Notifications
		isUpdated = true
	} else {
		// Check for specific fields added in later versions using raw TOML
		if notificationsMap, ok := rawConfig["notifications"].(map[string]any); ok {
			if _, exists := notificationsMap["notify_on_live_start"]; !exists {
				cfg.Notifications.NotifyOnLiveStart = defaultConfig.Notifications.NotifyOnLiveStart
				isUpdated = true
			}
			if _, exists := notificationsMap["notify_on_live_end"]; !exists {
				cfg.Notifications.NotifyOnLiveEnd = defaultConfig.Notifications.NotifyOnLiveEnd
				isUpdated = true
			}

			if _, exists := notificationsMap["send_contact_sheet_on_live_end"]; !exists {
				cfg.Notifications.SendContactSheetOnLiveEnd = defaultConfig.Notifications.SendContactSheetOnLiveEnd
				isUpdated = true
			}
		}
	}

	if reflect.DeepEqual(cfg.LiveSettings, LiveSettingsConfig{}) {
		cfg.LiveSettings = defaultConfig.LiveSettings
		isUpdated = true
	} else {
		// Check for specific fields added in later versions
		if cfg.LiveSettings.VODsFileExtension == "" {
			cfg.LiveSettings.VODsFileExtension = defaultConfig.LiveSettings.VODsFileExtension
			isUpdated = true
		}
		if cfg.LiveSettings.FilenameTemplate == "" {
			cfg.LiveSettings.FilenameTemplate = defaultConfig.LiveSettings.FilenameTemplate
			isUpdated = true
		}
		if cfg.LiveSettings.DateFormat == "" {
			cfg.LiveSettings.DateFormat = defaultConfig.LiveSettings.DateFormat
			isUpdated = true
		}

		// Check for RecordChat field in raw TOML
		if liveSettingsMap, ok := rawConfig["live_settings"].(map[string]any); ok {
			if _, exists := liveSettingsMap["record_chat"]; !exists {
				cfg.LiveSettings.RecordChat = defaultConfig.LiveSettings.RecordChat
				isUpdated = true
			}

			if _, exists := liveSettingsMap["ffmpeg_recording_options"]; !exists {
				cfg.LiveSettings.FFmpegRecordingOptions = defaultConfig.LiveSettings.FFmpegRecordingOptions
				isUpdated = true
			}
			if _, exists := liveSettingsMap["ffmpeg_conversion_options"]; !exists {
				cfg.LiveSettings.FFmpegConversionOptions = defaultConfig.LiveSettings.FFmpegConversionOptions
				isUpdated = true
			}
		}
	}

	if isUpdated {
		//log.Println("Config file has been updated with new default values.")
		// Save the updated config. Use the simple SaveConfig which now just encodes.
		file, err := os.Create(configPath)
		if err != nil {
			return err
		}
		defer file.Close()
		return toml.NewEncoder(file).Encode(cfg)
	}

	return nil
}

// MergeConfigs merges a new configuration into an existing one.
// It prioritizes values from the new config but fills in missing/default values from the existing and default configs.
func MergeConfigs(existing, new *Config) *Config {
	result := &Config{}
	defaultConfig := CreateDefaultConfig()

	// Merge Account: Always take new values if they are not empty.
	result.Account = existing.Account
	if new.Account.AuthToken != "" {
		result.Account.AuthToken = new.Account.AuthToken
	}
	if new.Account.UserAgent != "" {
		result.Account.UserAgent = new.Account.UserAgent
	}

	// Merge Options
	result.Options = existing.Options
	if new.Options.SaveLocation != "" {
		result.Options.SaveLocation = new.Options.SaveLocation
	}
	result.Options.CheckUpdates = new.Options.CheckUpdates
	result.Options.M3U8Download = new.Options.M3U8Download
	result.Options.SkipPreviews = new.Options.SkipPreviews
	result.Options.UseContentAsFilename = new.Options.UseContentAsFilename
	if new.Options.ContentFilenameTemplate != "" {
		result.Options.ContentFilenameTemplate = new.Options.ContentFilenameTemplate
	} else if result.Options.ContentFilenameTemplate == "" {
		result.Options.ContentFilenameTemplate = defaultConfig.Options.ContentFilenameTemplate
	}
	if new.Options.DownloadMediaType != "" {
		result.Options.DownloadMediaType = new.Options.DownloadMediaType
	} else if result.Options.DownloadMediaType == "" {
		result.Options.DownloadMediaType = defaultConfig.Options.DownloadMediaType
	}
	result.Options.SkipDownloadedPosts = new.Options.SkipDownloadedPosts
	result.Options.DownloadProfilePic = new.Options.DownloadProfilePic

	if new.Options.PostLimit != 0 {
		result.Options.PostLimit = new.Options.PostLimit
	} else {
		result.Options.PostLimit = existing.Options.PostLimit
	}

	// Merge LiveSettings
	result.LiveSettings = existing.LiveSettings
	if new.LiveSettings.SaveLocation != "" {
		result.LiveSettings.SaveLocation = new.LiveSettings.SaveLocation
	}
	if new.LiveSettings.VODsFileExtension != "" {
		result.LiveSettings.VODsFileExtension = new.LiveSettings.VODsFileExtension
	} else if result.LiveSettings.VODsFileExtension == "" {
		result.LiveSettings.VODsFileExtension = defaultConfig.LiveSettings.VODsFileExtension
	}
	if new.LiveSettings.FilenameTemplate != "" {
		result.LiveSettings.FilenameTemplate = new.LiveSettings.FilenameTemplate
	} else if result.LiveSettings.FilenameTemplate == "" {
		result.LiveSettings.FilenameTemplate = defaultConfig.LiveSettings.FilenameTemplate
	}
	if new.LiveSettings.DateFormat != "" {
		result.LiveSettings.DateFormat = new.LiveSettings.DateFormat
	} else if result.LiveSettings.DateFormat == "" {
		result.LiveSettings.DateFormat = defaultConfig.LiveSettings.DateFormat
	}

	// For booleans, we can just assign the value from the new config,
	// as it represents the most recent state.
	result.LiveSettings.FFmpegConvert = new.LiveSettings.FFmpegConvert
	result.LiveSettings.GenerateContactSheet = new.LiveSettings.GenerateContactSheet
	result.LiveSettings.UseMTForContactSheet = new.LiveSettings.UseMTForContactSheet
	result.LiveSettings.RecordChat = new.LiveSettings.RecordChat

	if new.LiveSettings.FFmpegRecordingOptions != "" {
		result.LiveSettings.FFmpegRecordingOptions = new.LiveSettings.FFmpegRecordingOptions
	}
	if new.LiveSettings.FFmpegConversionOptions != "" {
		result.LiveSettings.FFmpegConversionOptions = new.LiveSettings.FFmpegConversionOptions
	}

	// Merge Notifications
	result.Notifications = existing.Notifications
	result.Notifications.Enabled = new.Notifications.Enabled
	result.Notifications.SystemNotify = new.Notifications.SystemNotify
	result.Notifications.NotifyOnLiveStart = new.Notifications.NotifyOnLiveStart
	result.Notifications.NotifyOnLiveEnd = new.Notifications.NotifyOnLiveEnd
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

	// Security Headers are always taken from the new config.
	result.SecurityHeaders = new.SecurityHeaders

	return result
}

// copyFile is a simple utility to copy a file from a source to a destination.
func copyFile(srcPath, dstPath string) error {
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

// downloadFile is a simple utility to download a file from a URL.
func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// ResetConfig removes the current config file (if present) and writes a fresh
// default config. This does not preserve any previous values.
func ResetConfig() error {
	configPath := GetConfigPath()
	// Best-effort remove existing file
	_ = os.Remove(configPath)
	// Ensure directory exists without creating a config file
	if err := os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
		return err
	}
	// Overwrite with a pristine default config
	defaultConfig := CreateDefaultConfig()
	// SaveConfig will not merge since we removed the file
	return SaveConfig(defaultConfig)
}
