package download

import (
	"context"
	//"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
)

func (d *Downloader) DownloadMessages(ctx context.Context, modelId, modelName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	messagesWithMedia, err := posts.GetAllMessagesWithMedia(modelId, d.headers)
	if err != nil {
		logger.Logger.Printf("[ERROR] [%s] Failed to get message media: %v", modelName, err)
		return err
	}

	totalMediaItems := 0
	for _, msg := range messagesWithMedia {
		totalMediaItems += len(msg.Media)
	}

	if totalMediaItems == 0 {
		logger.Logger.Printf("[INFO] [%s] No new message media to download.", modelName)
		return nil
	}

	baseDir := filepath.Join(d.saveLocation, strings.ToLower(modelName), "messages")
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
			return err
		}
	}

	d.progressBar = progressbar.NewOptions(totalMediaItems,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetDescription("[green]Downloading Messages[reset]"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(15*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, msgWithMedia := range messagesWithMedia {
		// The index `i` is for each media item within a single message.
		for i, mediaItem := range msgWithMedia.Media {
			wg.Add(1)
			go func(media posts.AccountMedia, message posts.Message, index int) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				err := d.DownloadMediaItem(ctx, media, baseDir, modelName, message, index)
				if err != nil {
					logger.Logger.Printf("[ERROR] [%s] Failed to download message media item %s: %v", modelName, media.ID, err)
				}
				d.progressBar.Add(1)
			}(mediaItem, msgWithMedia.Message, i)
		}
	}

	wg.Wait()
	d.progressBar.Clear()

	return nil
}
