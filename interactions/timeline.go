package interactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agnosto/fansly-scraper/posts"
	//"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

// LikeAllPosts likes all posts in the given slice
func LikeAllPosts(ctx context.Context, allPosts []posts.Post, authToken, userAgent string) error {
	return processAllPosts(ctx, allPosts, authToken, userAgent, "https://apiv3.fansly.com/api/v1/likes?ngsw-bypass=true", "Liking")
}

// UnlikeAllPosts unlikes all posts in the given slice
func UnlikeAllPosts(ctx context.Context, allPosts []posts.Post, authToken, userAgent string) error {
	return processAllPosts(ctx, allPosts, authToken, userAgent, "https://apiv3.fansly.com/api/v1/likes/remove?ngsw-bypass=true", "Unliking")
}

func processAllPosts(ctx context.Context, allPosts []posts.Post, authToken, userAgent, url, action string) error {

	limiter := rate.NewLimiter(rate.Every(2*time.Second), 3)

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

	for _, post := range allPosts {
		if err := limiter.Wait(ctx); err != nil {
			logger.Logger.Printf("[WARN] Rate Limit Error: %v", err)
			return fmt.Errorf("rate limiter error: %v", err)
		}

		err := processPost(post.ID, url, authToken, userAgent)
		if err != nil {
			failedPosts = append(failedPosts, post.ID)
			logger.Logger.Printf("[ERROR] Failed to %s post %s: %v", action, post.ID, err)
			bar.Describe(fmt.Sprintf("[red]Failed[reset]: %s", post.ID))
		} else {
			bar.Add(1)
		}

		//if err := processPost(post.ID, url, authToken, userAgent); err != nil {
		//    logger.Logger.Printf("[ERROR] Failed to %s post %s: %v", action, post.ID, err)
		//	return fmt.Errorf("failed to %s post %s: %v", action, post.ID, err)
		//}
	}

	bar.Finish()

	if len(failedPosts) > 0 {
		logger.Logger.Printf("[WARN] Failed to process some posts: %v", failedPosts)
	}

	logger.Logger.Printf("[INFO] Finished processing all posts for %s", action)
	return nil
}

func processPost(postID, url, authToken, userAgent string) error {
	payload := map[string]string{"postId": postID}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	//headers.AddHeadersToRequest(req, true)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authToken)
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return nil
}
