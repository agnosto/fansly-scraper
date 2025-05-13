package interactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

// LikeAllPosts likes all posts in the given slice
func LikeAllPosts(ctx context.Context, allPosts []posts.Post, configPath string) error {
	return processAllPosts(ctx, allPosts, configPath, "https://apiv3.fansly.com/api/v1/likes?ngsw-bypass=true", "Liking")
}

// UnlikeAllPosts unlikes all posts in the given slice
func UnlikeAllPosts(ctx context.Context, allPosts []posts.Post, configPath string) error {
	return processAllPosts(ctx, allPosts, configPath, "https://apiv3.fansly.com/api/v1/likes/remove?ngsw-bypass=true", "Unliking")
}

func processAllPosts(ctx context.Context, allPosts []posts.Post, configPath, url, action string) error {
	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	// Create FanslyHeaders instance
	fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
	if err != nil {
		return fmt.Errorf("error creating headers: %v", err)
	}

	// Start with a more conservative rate limit: 1 request every 3 seconds
	limiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

	bar := progressbar.NewOptions(len(allPosts),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(15*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s posts[reset]", action)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	var failedPosts []string
	consecutiveFailures := 0
	backoffDuration := 5 * time.Second

	for i, post := range allPosts {
		// Wait for rate limiter
		if err := limiter.Wait(ctx); err != nil {
			logger.Logger.Printf("[WARN] Rate Limit Error: %v", err)
			return fmt.Errorf("rate limiter error: %v", err)
		}

		// Process the post with retry logic
		success := false
		retries := 0
		maxRetries := 3

		for !success && retries < maxRetries {
			err := processPost(post.ID, url, fanslyHeaders)
			if err != nil {
				if retries < maxRetries-1 {
					// Check if it's a rate limit error
					if err.Error() == "request failed with status code 429" {
						consecutiveFailures++
						// Exponential backoff
						waitTime := backoffDuration * time.Duration(consecutiveFailures)
						waitTime = min(waitTime, 2*time.Minute)
						logger.Logger.Printf("[WARN] Rate limited. Waiting for %v before retrying...", waitTime)
						bar.Describe(fmt.Sprintf("[yellow]Rate limited. Waiting %v...[reset]", waitTime))
						select {
						case <-time.After(waitTime):
							// Continue after waiting
						case <-ctx.Done():
							return ctx.Err()
						}
						// Adjust the rate limiter to be more conservative
						newRate := rate.Every(time.Duration(3+consecutiveFailures) * time.Second)
						limiter.SetLimit(newRate)
						logger.Logger.Printf("[INFO] Adjusted rate limit to 1 request every %v", newRate)
						retries++
						continue
					}
				}
				failedPosts = append(failedPosts, post.ID)
				logger.Logger.Printf("[ERROR] Failed to %s post %s: %v", action, post.ID, err)
				bar.Describe(fmt.Sprintf("[red]Failed[reset]: %s", post.ID))
				break
			} else {
				// Success, reset consecutive failures counter
				consecutiveFailures = 0
				success = true
				bar.Add(1)
				// Every 10 successful requests, add a small random delay
				if i > 0 && i%10 == 0 {
					randomDelay := time.Duration(1000+time.Now().UnixNano()%2000) * time.Millisecond
					logger.Logger.Printf("[INFO] Adding random delay of %v after 10 successful requests", randomDelay)
					time.Sleep(randomDelay)
				}
			}
		}
	}

	bar.Finish()
	if len(failedPosts) > 0 {
		logger.Logger.Printf("[WARN] Failed to process %d posts out of %d", len(failedPosts), len(allPosts))
	}
	logger.Logger.Printf("[INFO] Finished processing all posts for %s", action)
	return nil
}

func processPost(postID, url string, fanslyHeaders *headers.FanslyHeaders) error {
	payload := map[string]string{"postId": postID}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set the headers using the FanslyHeaders struct
	fanslyHeaders.AddHeadersToRequest(req, true)

	// Add Content-Type header which isn't included in the FanslyHeaders
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body for better error reporting
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
