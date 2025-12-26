package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

type Post struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
}

type TimelineResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Posts                       []Post `json:"posts"`
		TimelineReadPermissionFlags []struct {
			ID        string `json:"id"`
			AccountID string `json:"accountId"`
			Type      int    `json:"type"`
			Flags     int    `json:"flags"`
			Metadata  string `json:"metadata"`
		} `json:"timelineReadPermissionFlags"`
		AccountTimelineReadPermissionFlags struct {
			Flags    int    `json:"flags"`
			Metadata string `json:"metadata"`
		} `json:"accountTimelineReadPermissionFlags"`
	} `json:"response"`
}

// Restore the original access checking logic
func hasTimelineAccess(response TimelineResponse) bool {
	requiredFlags := response.Response.TimelineReadPermissionFlags
	userFlags := response.Response.AccountTimelineReadPermissionFlags.Flags

	// If TimelineReadPermissionFlags is empty, everyone has access
	if len(requiredFlags) == 0 {
		return true
	}

	// Check if user's flags match any of the required flags
	for _, flag := range requiredFlags {
		if userFlags&flag.Flags != 0 {
			return true
		}
	}

	return false
}

var (
	timelineLimiter = rate.NewLimiter(rate.Every(3*time.Second), 2)
)

func GetAllTimelinePosts(accountID string, wallID string, fanslyHeaders *headers.FanslyHeaders, limit int) ([]Post, error) {
	var allPosts []Post
	before := "0"
	hasMore := true

	// Get initial batch to check access
	initialResponse, err := getTimelineBatchResponse(accountID, wallID, before, fanslyHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to check timeline access: %v", err)
	}

	// Check if we have access
	if !hasTimelineAccess(initialResponse) {
		return nil, fmt.Errorf("no access to timeline for account %s", accountID)
	}

	// Add posts from initial response
	allPosts = append(allPosts, initialResponse.Response.Posts...)

	if limit > 0 && len(allPosts) >= limit {
		allPosts = allPosts[:limit]
		logger.Logger.Printf("[INFO] Reached post limit of %d. Stopping fetch", limit)
		return allPosts, nil
	}

	if len(initialResponse.Response.Posts) > 0 {
		before = initialResponse.Response.Posts[len(initialResponse.Response.Posts)-1].ID
	} else {
		hasMore = false
	}

	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Fetching Timeline"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	bar.Add(len(initialResponse.Response.Posts))

	for hasMore {
		response, err := getTimelineBatchResponse(accountID, wallID, before, fanslyHeaders)
		if err != nil {
			return nil, err
		}

		posts := response.Response.Posts
		allPosts = append(allPosts, posts...)

		if limit > 0 && len(allPosts) >= limit {
			allPosts = allPosts[:limit]
			bar.Add(len(posts))
			logger.Logger.Printf("[INFO] Reached post limit of %d inside loop. Stopping fetch", limit)
			break
		}

		if len(posts) == 0 {
			hasMore = false
		} else {
			before = posts[len(posts)-1].ID
		}

		bar.Add(len(posts))
	}

	bar.Finish()
	logger.Logger.Printf("[INFO] Retrieved %d total timeline posts for account %s", len(allPosts), accountID)
	return allPosts, nil
}

func getTimelineBatchResponse(accountID, wallID, before string, fanslyHeaders *headers.FanslyHeaders) (TimelineResponse, error) {
	ctx := context.Background()
	err := timelineLimiter.Wait(ctx)
	if err != nil {
		return TimelineResponse{}, fmt.Errorf("rate limiter error: %v", err)
	}

	// Use the original URL format that was working
	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/timelinenew/%s?before=%s&after=0&wallId=%s&contentSearch&ngsw-bypass=true", accountID, before, wallID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TimelineResponse{}, err
	}

	fanslyHeaders.AddHeadersToRequest(req, true)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return TimelineResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TimelineResponse{}, fmt.Errorf("failed to fetch timeline with status code %d", resp.StatusCode)
	}

	var timelineResp TimelineResponse
	err = json.NewDecoder(resp.Body).Decode(&timelineResp)
	if err != nil {
		return TimelineResponse{}, err
	}

	logger.Logger.Printf("[INFO] Retrieved %d posts in timeline batch", len(timelineResp.Response.Posts))
	return timelineResp, nil
}
