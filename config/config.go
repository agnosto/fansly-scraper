package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tidwall/gjson"
)

type Config struct {
	Client        http.Client
	UserAgent     string
	Authorization string
	SaveLocation  string
    VODFileExt    string
}

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
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
		exampleConfig := filepath.Join("example-config.json")
		if _, err := os.Stat(exampleConfig); os.IsNotExist(err) {
			// Example config doesn't exist, download default
			err = DownloadConfig("https://github.com/agnosto/fansly-scraper/blob/master/example-config.json", configPath)
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
	return OpenConfigInEditor(configPath)
}

func LoadConfig(configPath string) (*Config, error) {
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	Authorization := gjson.Get(string(cfg), "auth-token")
	useragent := gjson.Get(string(cfg), "user-agent")
	savelocation := gjson.Get(string(cfg), "save-location")
    vodfileext := gjson.Get(string(cfg), "vods-file-extension")

	if useragent.Type == gjson.Null {
		return nil, fmt.Errorf("user-agent is empty")
	}
	if Authorization.Type == gjson.Null {
		return nil, fmt.Errorf("authoriztion is empty")
	}

	return &Config{
		UserAgent:     useragent.Str,
		Authorization: Authorization.Str,
		SaveLocation:  savelocation.Str,
        VODFileExt:    vodfileext.Str,
	}, nil
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
