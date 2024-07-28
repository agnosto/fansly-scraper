package posts

import (
	"context"
	"encoding/json"
	"fmt"
	//"log"
	"net/http"
	"os"
	"time"

    //"github.com/agnosto/fansly-scraper/logger"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
	//"github.com/k0kubun/go-ansi"
	//"github.com/agnosto/fansly-scraper/headers"
	//"strings"
)

type Post struct {
    ID      string  `json:"id"`
    Content string  `json:"content"`
    CreatedAt int64 `json:"createdAt"`
}

type TimelineResponse struct {
    Success  bool `json:"success"`
    Response struct {
        Posts []Post `json:"posts"`
        TimelineReadPermissionFlags []struct {
            ID       string `json:"id"`
            AccountID string `json:"accountId"`
            Type     int    `json:"type"`
            Flags    int    `json:"flags"`
            Metadata string `json:"metadata"`
        } `json:"timelineReadPermissionFlags"`
        AccountTimelineReadPermissionFlags struct {
            Flags    int    `json:"flags"`
            Metadata string `json:"metadata"`
        } `json:"accountTimelineReadPermissionFlags"`
    } `json:"response"`
}

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

func GetAllTimelinePosts(modelId string, authToken string, userAgent string) ([]Post, error) {
    var allPosts []Post
    before := "0"
    hasMore := true

    initialResponse, _, err := getTimelinePostsBatch(modelId, "0", authToken, userAgent)
    if err != nil {
        return nil, err
    }

    // Check permissions
    if !hasTimelineAccess(initialResponse) {
        return nil, fmt.Errorf("no access to timeline")
    }

    limiter := rate.NewLimiter(rate.Every(3*time.Second), 2)

    // Create an indeterminate progress bar
    bar := progressbar.NewOptions(-1,
        progressbar.OptionSetDescription("Fetching posts"),
        progressbar.OptionSetWriter(os.Stderr),
        progressbar.OptionSetWidth(15),
        progressbar.OptionThrottle(65*time.Millisecond),
        progressbar.OptionShowCount(),
        progressbar.OptionSpinnerType(14),
        progressbar.OptionFullWidth(),
    )

    //allPosts = append(allPosts, initialResponse.Response.Posts...)
    //bar.Add(len(initialResponse.Response.Posts))

    for hasMore {
        if err := limiter.Wait(context.Background()); err != nil {
            return nil, fmt.Errorf("rate limiter error: %v", err)
        }

        response, nextBefore, err := getTimelinePostsBatch(modelId, before, authToken, userAgent)
        if err != nil {
            if err.Error() == "not subscribed or followed: unable to get timeline feed" {
                bar.Finish()
                return nil, err // Return the error to be handled by the caller
            }
            return nil, err
        }
        //log.Printf("[GetAllTimelinePosts] posts: %v", posts)

        allPosts = append(allPosts, response.Response.Posts...)

        if nextBefore == "" || len(response.Response.Posts) == 0 {
            hasMore = false
        } else {
            before = nextBefore
        }
        bar.Add(len(response.Response.Posts))
    }
    //log.Printf("[GetAllTimelinePosts] All Posts: %v", allPosts)
    bar.Finish()

    return allPosts, nil
}

func getTimelinePostsBatch(modelId, before string, authToken string, userAgent string) (TimelineResponse, string, error) { 
    headerMap := map[string]string{
		"Authorization": authToken,
		"User-Agent":    userAgent,
	}
	client := &http.Client{}
    url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/timelinenew/%s?before=%s&after=0&wallId&contentSearch&ngsw-bypass=true", modelId, before)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TimelineResponse{}, "", err
	}

    //headers.AddHeadersToRequest(req, true)
	for key, value := range headerMap {
		req.Header.Add(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return TimelineResponse{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TimelineResponse{}, "", fmt.Errorf("failed to fetch model timeline with status code %d", resp.StatusCode)
	}

    var timelineResp TimelineResponse 
    err = json.NewDecoder(resp.Body).Decode(&timelineResp)
    if err != nil {
        return TimelineResponse{}, "", err
    }

    //if timelineResp.Response.AccountTimelineReadPermissionFlags.Flags == 0 {
    //    return nil, "", fmt.Errorf("not subscribed: unable to get timeline feed")
    //}

    if len(timelineResp.Response.Posts) == 0 {
        return timelineResp, "", nil
    }

    //nextBefore := posts[len(posts)-1].ID
    nextBefore := timelineResp.Response.Posts[len(timelineResp.Response.Posts)-1].ID 
    //log.Printf("[Timeline Batch] Last Post Id in resposne: %v", nextBefore)

    return timelineResp, nextBefore, nil

}
