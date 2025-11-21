package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/logger"
)

// DownloadProfileContent downloads the avatar and banner for a model
func (d *Downloader) DownloadProfileContent(ctx context.Context, model auth.FollowedModel) error {
	if !d.cfg.Options.DownloadProfilePic {
		return nil
	}

	logger.Logger.Printf("[INFO] Checking profile images for %s", model.Username)

	baseDir := filepath.Join(d.saveLocation, strings.ToLower(model.Username), "profile")
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create profile directory: %v", err)
	}

	if model.Avatar != nil {
		if err := d.processProfileImage(ctx, model.Avatar, baseDir, model.Username, "avatar"); err != nil {
			logger.Logger.Printf("[WARN] Failed to download avatar for %s: %v", model.Username, err)
		}
	}

	if model.Banner != nil {
		if err := d.processProfileImage(ctx, model.Banner, baseDir, model.Username, "banner"); err != nil {
			logger.Logger.Printf("[WARN] Failed to download banner for %s: %v", model.Username, err)
		}
	}

	return nil
}

func (d *Downloader) processProfileImage(ctx context.Context, img *auth.ProfileImage, baseDir, modelName, imgType string) error {
	var bestVariant auth.ImageVariant
	maxPixels := 0
	found := false

	for _, v := range img.Variants {
		if len(v.Locations) == 0 {
			continue
		}

		pixels := v.Width * v.Height
		if pixels > maxPixels {
			maxPixels = pixels
			bestVariant = v
			found = true
		}
	}

	if !found {
		return fmt.Errorf("no downloadable variants found")
	}

	// Get URL and Extension
	url := bestVariant.Locations[0].Location
	ext := ".jpg"
	if bestVariant.Mimetype == "image/png" {
		ext = ".png"
	} else if bestVariant.Mimetype == "image/jpeg" {
		ext = ".jpg"
	}

	// Naming convention: {id}_{type}.ext (e.g., 70799..._avatar.jpg)
	// This ensures if they change it (new ID), we download the new one.
	// If they haven't changed it, the ID stays the same, and we skip it (deduplication).
	filename := fmt.Sprintf("%s_%s%s", img.ID, imgType, ext)
	filePath := filepath.Join(baseDir, filename)

	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	if d.fileService.FileExists(filePath) {
		return nil
	}

	logger.Logger.Printf("[INFO] Downloading new %s for %s: %s", imgType, modelName, filename)

	return d.downloadRegularFile(url, filePath, modelName, "image", false)
}
