package main

import (
	"fmt"
    "github.com/agnosto/fansly-scraper/cmd"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/download"
	"github.com/agnosto/fansly-scraper/ui"
    "github.com/agnosto/fansly-scraper/core"
    "github.com/agnosto/fansly-scraper/updater"
    "github.com/agnosto/fansly-scraper/logger"

	"log"
	"os"
    "context"
    //"flag"

	tea "github.com/charmbracelet/bubbletea"
)

var ffmpegAvailable bool
const version = "v0.1.4"

func main() {
    flags, subcommand := cmd.ParseFlags()

    if subcommand == "update" {
        if err := updater.CheckForUpdate(version); err != nil {
            fmt.Printf("Error updating: %v\n", err)
            os.Exit(1)
        }
        return
    } 

    config.EnsureConfigExists(config.GetConfigPath())
    cfg, err := config.LoadConfig(config.GetConfigPath())
    //log.Printf("[Main Start] Loaded Config: %v", cfg)
    if err != nil {
        log.Fatal(err)
    }

    if err := logger.InitLogger(cfg); err != nil {
        log.Fatal(err)
    }

    logger.Logger.Printf("Starting Fansly Scraper version %s", version)

    ffmpegAvailable = cmd.IsFFmpegAvailable()
    logger.Logger.Printf("Ffmpeg Check Returned %v", ffmpegAvailable)

    downloader, err := download.NewDownloader(cfg, ffmpegAvailable)
    if err != nil {
        logger.Logger.Fatal(err)
    }

    if flags.Username != "" && flags.DownloadType == "" {
		flags.DownloadType = "all"
	}

    if flags.Username != "" && flags.DownloadType != "" {
        runCLIMode(flags.Username, flags.DownloadType, downloader)
        return
    }

    model := ui.NewMainModel(downloader, version)
    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        logger.Logger.Printf("Error: %v", err)
        os.Exit(1)
    }
}

func runCLIMode(username string, downloadType string, downloader *download.Downloader) {
    // Get model ID from username
    modelID, err := core.GetModelIDFromUsername(username)
    if err != nil {
        logger.Logger.Printf("Error getting model ID: %v", err)
        os.Exit(1)
    }

    ctx := context.Background()

    switch downloadType {
    case "all":
        if err := downloader.DownloadTimeline(ctx, modelID, username); err != nil {
            logger.Logger.Printf("Error downloading timeline: %v", err)
        }
        if err := downloader.DownloadMessages(ctx, modelID, username); err != nil {
            logger.Logger.Printf("Error downloading messages: %v", err)
        }
    case "timeline":
        if err := downloader.DownloadTimeline(ctx, modelID, username); err != nil {
            logger.Logger.Printf("Error downloading timeline: %v", err)
        }
    case "messages":
        if err := downloader.DownloadMessages(ctx, modelID, username); err != nil {
            logger.Logger.Printf("Error downloading messages: %v", err)
        }
    case "stories":
        if err := downloader.DownloadStories(ctx, modelID, username); err != nil {
            logger.Logger.Printf("Error downloading timeline: %v", err)
        } 
    default:
        logger.Logger.Printf("Invalid download type. Use 'all', 'timeline', or 'messages'")
        os.Exit(1)
    }
}
