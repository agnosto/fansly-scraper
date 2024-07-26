package posts

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "golang.org/x/time/rate"
    "context"
    //"log"
    //"go-fansly-scraper/headers"
)

var (
    limiter = rate.NewLimiter(rate.Every(3*time.Second), 2) // Adjust these values as needed
)

type AccountMediaBundles struct {
    ID string `json:"id"`
    Access      bool `json:"access"`
    AccountMediaIDs  []string `json:"accountMediaIds"`
    BundleContent   []struct {
        AccountMediaID  string  `json:"AccountMediaId"`
        Pos int `json:"pos"`
    } `json:"bundleContent"`
    
}

type MediaItem struct {
    ID       string `json:"id"`
    Type     int    `json:"type"`
    Height   int    `json:"height"`
    Mimetype string `json:"mimetype"`
    Variants []struct {
        ID       string `json:"id"`
        Type     int    `json:"type"`
        Height   int    `json:"height"`
        Mimetype string `json:"mimetype"`
        Locations []struct {
            Location string `json:"location"`
        } `json:"locations"`
    } `json:"variants"`
    Locations []struct {
        Location string `json:"location"`
    } `json:"locations"`
}

type AccountMedia struct {
    ID    string    `json:"id"`
    Media MediaItem `json:"media"`
    Preview *MediaItem `json:"preview,omitempty"`// Added to handle optional preview 
}

type PostResponse struct {
    Success  bool `json:"success"`
    Response struct {
        Posts []struct {
            ID          string         `json:"id"`
            Attachments []struct {
                ContentType int    `json:"contentType"`
                ContentID   string `json:"contentId"`
            } `json:"attachments"`
        } `json:"posts"`
        AccountMediaBundles []AccountMediaBundles `json:"accountMediaBundles"` 
        AccountMedia []AccountMedia `json:"accountMedia"`
    } `json:"response"`
}

func GetPostMedia(postId string, authToken string, userAgent string) ([]AccountMedia, error) {
    ctx := context.Background()
    err := limiter.Wait(ctx)
    if err != nil {
        return nil, fmt.Errorf("rate limiter error: %v", err)
    }

    url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/post?ids=%s&ngsw-bypass=true", postId)
    //log.Printf("\n[INFO] Starting  media extraction for postId: %s with url: %s \n", postId, url)
    
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    //headers.AddHeadersToRequest(req, true)
    req.Header.Add("Authorization", authToken)
    req.Header.Add("User-Agent", userAgent)
    
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch post with status code %d", resp.StatusCode)
    }
    
    var postResp PostResponse
    err = json.NewDecoder(resp.Body).Decode(&postResp)
    if err != nil {
        return nil, err
    }
    
    var accountMediaItems []AccountMedia 
    for _, post := range postResp.Response.Posts {
        //log.Printf("[DEBUG] Processing post: %s", post.ID)
        for _, attachment := range post.Attachments {
            //log.Printf("[DEBUG] Processing attachment: %s", attachment.ContentID)
            for _, accountMedia := range postResp.Response.AccountMedia {
                if accountMedia.ID == attachment.ContentID {
                    //log.Printf("[DEBUG] Found matching AccountMedia: %s", accountMedia.ID)
                    
                    hasLocations := false

                    // Check main media variants for locations
                    for _, variant := range accountMedia.Media.Variants {
                        if len(variant.Locations) > 0 {
                            hasLocations = true
                            break
                        }
                    }

                    // Check preview media and its variants for locations
                    if accountMedia.Preview != nil {
                        if len(accountMedia.Preview.Locations) > 0 {
                            hasLocations = true
                        } else {
                            for _, variant := range accountMedia.Preview.Variants {
                                if len(variant.Locations) > 0 {
                                    hasLocations = true
                                    break
                                }
                            }
                        }
                    }

                    if hasLocations {
                        accountMediaItems = append(accountMediaItems, accountMedia)
                        //log.Printf("[INFO] Added AccountMedia: %s", accountMedia.ID)
                    } else {
                        //log.Printf("[WARN] Skipping AccountMedia %s: No locations found", accountMedia.ID)
                    }
                    break
                }
            }
        }

        for _, bundle := range postResp.Response.AccountMediaBundles {
            if bundle.Access {
                for _, bundleContent := range bundle.BundleContent {
                    // Find and add the corresponding AccountMedia
                    for _, accountMedia := range postResp.Response.AccountMedia {
                        if accountMedia.ID == bundleContent.AccountMediaID {
                            accountMediaItems = append(accountMediaItems, accountMedia)
                            break
                        }
                    }
                }
            }
        }
    }

    for _, accountMedia := range postResp.Response.AccountMedia {
        // Check if this accountMedia is already added
        alreadyAdded := false
        for _, addedMedia := range accountMediaItems {
            if addedMedia.ID == accountMedia.ID {
                alreadyAdded = true
                break
            }
        }

        if !alreadyAdded {
            // Add if it has locations or its preview has locations
            if hasLocations(accountMedia.Media) || (accountMedia.Preview != nil && hasLocations(*accountMedia.Preview)) {
                accountMediaItems = append(accountMediaItems, accountMedia)
            }
        }
    }
    
    //log.Printf("[INFO] Retrieved %d media items for post %s", len(accountMediaItems), postId)
    return accountMediaItems, nil
}


func hasLocations(media MediaItem) bool {
    if len(media.Locations) > 0 {
        return true
    }
    for _, variant := range media.Variants {
        if len(variant.Locations) > 0 {
            return true
        }
    }
    return false
}
