package headers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	//"strings"
	"time"

	"github.com/agnosto/fansly-scraper/config"
)

type FanslyHeaders struct {
	AuthToken string
	UserAgent string
	DeviceID  string
	SessionID string
	CheckKey  string
	Config    *config.Config
}

const fallbackCheckKey = "necvac-govry3-tybkYz"

func NewFanslyHeaders(cfg *config.Config) (*FanslyHeaders, error) {
	headers := &FanslyHeaders{
		AuthToken: cfg.Account.AuthToken,
		UserAgent: cfg.Account.UserAgent,
		DeviceID:  cfg.SecurityHeaders.DeviceID,
		SessionID: cfg.SecurityHeaders.SessionID,
		CheckKey:  cfg.SecurityHeaders.CheckKey,
		Config:    cfg,
	}

	var updated bool

	if headers.DeviceID == "" {
		deviceID, err := GetDeviceID()
		if err != nil {
			return nil, err
		}
		headers.DeviceID = deviceID
		cfg.SecurityHeaders.DeviceID = deviceID
		updated = true
	}

	if headers.SessionID == "" {
		sessionID, err := GetSessionID(headers.AuthToken)
		if err != nil {
			return nil, err
		}
		headers.SessionID = sessionID
		cfg.SecurityHeaders.SessionID = sessionID
		updated = true
	}

	if headers.CheckKey == "" {
		err := headers.SetCheckKey()
		if err != nil {
			return nil, err
		}
		cfg.SecurityHeaders.CheckKey = headers.CheckKey
		updated = true
	}

	// Save the updated config
	if updated {
		err := config.SaveConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	if time.Since(cfg.SecurityHeaders.LastUpdated) > 7*24*time.Hour {
		if err := headers.RefreshSecurityHeaders(); err != nil {
			return nil, err
		}
	}

	return headers, nil
}

func (f *FanslyHeaders) GetBasicHeaders() map[string]string {
	return map[string]string{
		"Accept-Language": "en-US,en;q=0.9",
		"Authorization":   f.AuthToken,
		"Origin":          "https://fansly.com",
		"Referer":         "https://fansly.com/",
		"User-Agent":      f.UserAgent,
	}
}

func (f *FanslyHeaders) GetFullHeaders(reqURL string) map[string]string {
	headers := f.GetBasicHeaders()
	headers["fansly-client-id"] = f.DeviceID
	headers["fansly-client-ts"] = fmt.Sprintf("%d", getClientTimestamp())
	headers["fansly-session-id"] = f.SessionID
	headers["fansly-client-check"] = f.getFanslyClientCheck(reqURL)
	return headers
}

func (f *FanslyHeaders) AddHeadersToRequest(req *http.Request, fullHeaders bool) {
	var headers map[string]string
	if fullHeaders {
		headers = f.GetFullHeaders(req.URL.String())
	} else {
		headers = f.GetBasicHeaders()
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}
}

func getClientTimestamp() int64 {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	randomValue, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return now + (5000 - randomValue.Int64())
}

func (f *FanslyHeaders) SetCheckKey() error {
	mainJSPattern := `\ssrc\s*=\s*"(main\..*?\.js)"`
	//checkKeyPattern := `this\.checkKey_\s*=\s*\["([^"]+)","([^"]+)"\]\.reverse\(\)\.join\("-"\)\+"([^"]+)"`
	checkKeyPattern := `let\s+i\s*=\s*\[\s*\]\s*;\s*i\.push\s*\(\s*"([^"]+)"\s*\)\s*,\s*i\.push\s*\(\s*"([^"]+)"\s*\)\s*,\s*i\.push\s*\(\s*"([^"]+)"\s*\)\s*,\s*this\.checkKey_\s*=\s*i\.join\s*\(\s*"-"\s*\)`

	checkKey, err := GuessCheckKey(mainJSPattern, checkKeyPattern, f.UserAgent)
	if err != nil {
		fmt.Printf("Warning: %v", err)
		f.CheckKey = fallbackCheckKey
	} else {
		f.CheckKey = checkKey
	}

	return nil
}

func (f *FanslyHeaders) getFanslyClientCheck(reqURL string) string {
	parsedURL, _ := url.Parse(reqURL)
	urlPath := parsedURL.Path
	uniqueIdentifier := fmt.Sprintf("%s_%s_%s", f.CheckKey, urlPath, f.DeviceID)
	digest := cyrb53(uniqueIdentifier)
	return fmt.Sprintf("%x", digest)
}

func cyrb53(str string) uint64 {
	h1 := uint64(0xdeadbeef)
	h2 := uint64(0x41c6ce57)

	for i := 0; i < len(str); i++ {
		ch := uint64(str[i])
		h1 = (h1 ^ ch) * 2654435761
		h2 = (h2 ^ ch) * 1597334677
	}

	h1 = ((h1 ^ (h1 >> 16)) * 2246822507) ^ ((h2 ^ (h2 >> 13)) * 3266489909)
	h2 = ((h2 ^ (h2 >> 16)) * 2246822507) ^ ((h1 ^ (h1 >> 13)) * 3266489909)

	return 4294967296*(h2&0xFFFFFFFF) + (h1 & 0xFFFFFFFF)
}

func GetDeviceID() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://apiv3.fansly.com/api/v1/device/id", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Success  bool   `json:"success"`
		Response string `json:"response"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("failed to get device ID")
	}

	return result.Response, nil
}

func (f *FanslyHeaders) SetSessionID() error {
	sessionID, err := GetSessionID(f.AuthToken)
	if err != nil {
		return err
	}
	f.SessionID = sessionID
	return nil
}

func GetSessionID(authToken string) (string, error) {
	c, _, err := websocket.DefaultDialer.Dial("wss://wsv3.fansly.com/", nil)
	if err != nil {
		return "", err
	}
	defer c.Close()

	message := map[string]any{
		"t": 1,
		"d": fmt.Sprintf("{\"token\":\"%s\"}", authToken),
	}

	err = c.WriteJSON(message)
	if err != nil {
		return "", err
	}

	_, msg, err := c.ReadMessage()
	if err != nil {
		return "", err
	}

	var response struct {
		T int    `json:"t"`
		D string `json:"d"`
	}

	err = json.Unmarshal(msg, &response)
	if err != nil {
		return "", err
	}

	var sessionData struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}

	err = json.Unmarshal([]byte(response.D), &sessionData)
	if err != nil {
		return "", err
	}

	return sessionData.Session.ID, nil
}

func GuessCheckKey(mainJSPattern, checkKeyPattern, userAgent string) (string, error) {
	fanslyURL := "https://fansly.com/"
	client := &http.Client{}

	// Make request to fansly.com
	req, err := http.NewRequest("GET", fanslyURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Find main.*.js file
	mainJSRegex := regexp.MustCompile(mainJSPattern)
	mainJSMatch := mainJSRegex.FindStringSubmatch(string(body))
	if len(mainJSMatch) < 2 {
		return "", fmt.Errorf("main.js file not found")
	}

	mainJS := mainJSMatch[1]
	mainJSURL := fmt.Sprintf("%s%s", fanslyURL, mainJS)

	// Request main.js file
	req, err = http.NewRequest("GET", mainJSURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code for main.js: %d", resp.StatusCode)
	}

	jsBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Find check key
	checkKeyRegex := regexp.MustCompile(checkKeyPattern)
	checkKeyMatch := checkKeyRegex.FindStringSubmatch(string(jsBody))
	if len(checkKeyMatch) < 4 {
		return "", fmt.Errorf("check key not found")
	}

	//checkKey := strings.Join([]string{checkKeyMatch[2], checkKeyMatch[1]}, "-") + checkKeyMatch[3]
	checkKey := fmt.Sprintf("%s-%s-%s", checkKeyMatch[1], checkKeyMatch[2], checkKeyMatch[3])
	//checkKey := reversedPart + "-bubayf"

	return checkKey, nil
}

func (f *FanslyHeaders) RefreshSecurityHeaders() error {
	var err error

	f.DeviceID, err = GetDeviceID()
	if err != nil {
		return err
	}

	f.SessionID, err = GetSessionID(f.AuthToken)
	if err != nil {
		return err
	}

	err = f.SetCheckKey()
	if err != nil {
		return err
	}

	// Update the config
	f.Config.SecurityHeaders.DeviceID = f.DeviceID
	f.Config.SecurityHeaders.SessionID = f.SessionID
	f.Config.SecurityHeaders.CheckKey = f.CheckKey
	f.Config.SecurityHeaders.LastUpdated = time.Now()

	// Save the updated config
	err = config.SaveConfig(f.Config)
	if err != nil {
		return err
	}

	return nil
}
