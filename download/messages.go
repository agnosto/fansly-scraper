package download

import (
	"context"
	"fmt"
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

	messageMediaItems, err := posts.GetAllMessageMedia(modelId, d.headers)
	if err != nil {
		logger.Logger.Printf("[ERROR] [%s] Failed to get message media: %v", modelName, err)
		return err
	}

	baseDir := filepath.Join(d.saveLocation, strings.ToLower(modelName), "messages")
	for _, subDir := range []string{"images", "videos", "audios"} {
		if err = os.MkdirAll(filepath.Join(baseDir, subDir), os.ModePerm); err != nil {
			return err
		}
	}

	d.progressBar = progressbar.NewOptions(len(messageMediaItems),
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

	for _, mediaItem := range messageMediaItems {
		wg.Add(1)
		go func(media posts.AccountMedia) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := d.downloadMessageMediaItem(ctx, media, baseDir, modelName)
			if err != nil {
				logger.Logger.Printf("[ERROR] [%s] Failed to download message media item %s: %v", modelName, media.ID, err)
			}
			d.progressBar.Add(1)
		}(mediaItem)
	}

	wg.Wait()
	d.progressBar.Clear()

	return nil
}

func (d *Downloader) downloadMessageMediaItem(ctx context.Context, accountMedia posts.AccountMedia, baseDir, modelName string) error {
	// Download main media
	err := d.downloadSingleItem(ctx, accountMedia.Media, baseDir, accountMedia.ID, modelName, false)
	if err != nil {
		return fmt.Errorf("error downloading main media: %v", err)
	}

	// Download preview if it exists
	if accountMedia.Preview != nil {
		err = d.downloadSingleItem(ctx, *accountMedia.Preview, baseDir, accountMedia.ID, modelName, true)
		if err != nil {
			return fmt.Errorf("error downloading preview: %v", err)
		}
	}

	return nil
}
