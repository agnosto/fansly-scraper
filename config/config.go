package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
    "log"
    "runtime"
    "time"

    "github.com/BurntSushi/toml"
)

type Config struct {
    Account         AccountConfig
    Options         OptionsConfig
    SecurityHeaders SecurityHeadersConfig
}

type AccountConfig struct {
    AuthToken string `toml:"auth_token"`
    UserAgent string `toml:"user_agent"`
}

type OptionsConfig struct {
    SaveLocation        string `toml:"save_location"`
    M3U8Download        bool   `toml:"m3u8_dl"`
    VODsFileExtension   string `toml:"vods_file_extension"`
}


type SecurityHeadersConfig struct {
    DeviceID  string `toml:"device_id"`
    SessionID string `toml:"session_id"`
    CheckKey  string `toml:"check_key"`
    LastUpdated  time.Time `toml:"last_updated"`
}


type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func GetConfigPath() string {
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

func SaveConfig(cfg *Config) error {
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
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default to vim if no EDITOR environment variable is set
	}

	cmd := exec.Command(editor, configPath)
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
	// Get the current working directory
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Construct the path to the example-config.ini file in the root directory
	exampleConfigPath := filepath.Join(rootDir, filePath)

	// Check if the file exists in the current directory
	if _, err := os.Stat(exampleConfigPath); os.IsNotExist(err) {
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
	}

	return nil
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
		if _, err := os.Stat(exampleConfig); os.IsNotExist(err) {
			// Example config doesn't exist, download default
			err = DownloadConfig("https://github.com/agnosto/fansly-scraper/blob/master/example-config.toml", configPath)
			if err != nil {
				return err
			}
		} else {
			// Copy example config
			err = CopyFile(exampleConfig, configPath)
			if err != nil {
				return err
			}
		}
	}

	// Open config file in editor
	//return OpenConfigInEditor(configPath)
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
        return nil, fmt.Errorf("save_location is empty in $v", configPath)
    }

    if config.SecurityHeaders.LastUpdated.IsZero() {
        config.SecurityHeaders.LastUpdated = time.Now()
    }

    return &config, nil
}
 
// TODO: get rid of the below functions

func createHeaders(authToken string, userAgent string) map[string]string {
	headers := map[string]string{
		"authority":          "apiv3.fansly.com",
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "en;q=0.8,en-US;q=0.7",
		"authorization":      authToken,
		"origin":             "https://fansly.com",
		"referer":            "https://fansly.com/",
		"sec-ch-ua":          "\"Not.A/Brand\";v=\"8\", \"Chromium\";v=\"114\", \"Google Chrome\";v=\"114\"",
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "\"Windows\"",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         userAgent,
	}

	return headers
}

func getAccountInfo(authToken string, userAgent string) (*Account, error) {
	// Create the headers
	headers := createHeaders(authToken, userAgent)

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/account/me?ngsw-bypass=true", nil)
	if err != nil {
		return nil, err
	}

	// Set the headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Send the HTTP request and get the response
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the JSON response
	var account Account
	err = json.NewDecoder(resp.Body).Decode(&account)
	if err != nil {
		return nil, err
	}

	return &account, nil
}
