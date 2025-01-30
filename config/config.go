package config

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Account         AccountConfig         `toml:"account"`
	Options         OptionsConfig         `toml:"options"`
	LiveSettings    LiveSettingsConfig    `toml:"live_settings"`
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
	// Read existing config
	existingConfig, err := LoadConfig(GetConfigPath())
	if err == nil {
		// Preserve existing settings while updating security headers
		cfg.LiveSettings = existingConfig.LiveSettings
		cfg.Options = existingConfig.Options
	}

	configPath := GetConfigPath()
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(cfg)
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
