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
    "github.com/agnosto/fansly-scraper/service"
    ksvc "github.com/kardianos/service"

	"log"
	"os"
    "context"
    "path/filepath"
    //"flag"

	tea "github.com/charmbracelet/bubbletea"
)

var ffmpegAvailable bool
const version = "v0.2.3"

func main() {
    flags, subcommand := cmd.ParseFlags()

    if flags.Version {
        fmt.Printf("Fansly Scraper version %s\n", version)
        return
    }

    switch subcommand {
	case "update":
		if err := updater.CheckForUpdate(version); err != nil {
			fmt.Printf("Error updating: %v\n", err)
			os.Exit(1)
		}
		return
	case "service":
		cmd.RunService()
		return
	} 

    if flags.Service != "" {
        prg := &cmd.Program{}
        s, err := ksvc.New(prg, &ksvc.Config{
            Name:        "FanslyScraper",
            DisplayName: "Fansly Scraper Service",
            Description: "This service monitors and records Fansly streams.",
        })
        if err != nil {
            fmt.Printf("Error creating service: %v\n", err)
            os.Exit(1)
        }

		switch flags.Service {
		case "install":
			err = s.Install()
            if err != nil {
                fmt.Printf("Error installing service: %v\n", err)
            } else {
                fmt.Println("Service installed successfully")
            }
		case "uninstall":
			err = s.Uninstall()
            if err != nil {
                fmt.Printf("Error uninstalling service: %v\n", err)
            } else {
                fmt.Println("Service uninstalled successfully")
            }
		case "start":
			err = s.Start()
            if err != nil {
                fmt.Printf("Error starting service: %v\n", err)
            } else {
                fmt.Println("Service started successfully")
            }
		case "stop":
			err = s.Stop()
            if err != nil {
                fmt.Printf("Error stopping service: %v\n", err)
            } else {
                fmt.Println("Service stopped successfully")
            }

		case "restart":
			err = s.Restart()
            if err != nil {
                fmt.Printf("Error restarting service: %v\n", err)
            } else {
                fmt.Println("Service restarted successfully")
            }
        case "status":
            status, err := s.Status()
            if err != nil {
                fmt.Printf("Error getting service status: %v\n", err)
            } else {
                fmt.Printf("Service status: %v\n", status)
            }
		default:
			fmt.Printf("Unknown service command: %s\n", flags.Service)
			os.Exit(1)
		}
		return
	}

    err := config.EnsureConfigExists(config.GetConfigPath())
    if err != nil {
        log.Fatal(err)
    }
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

    monitoringService := service.NewMonitoringService(filepath.Join(config.GetConfigDir(), "monitoring_state.json"))

    if flags.Monitor != "" {
        modelID, err := core.GetModelIDFromUsername(flags.Monitor)
        if err != nil {
            logger.Logger.Printf("Error getting model ID: %v", err)
            os.Exit(1)
        }
        monitoringService.ToggleMonitoring(modelID, flags.Monitor)
        fmt.Printf("Toggled monitoring for %s\n", flags.Monitor)
        return
    }

    model := ui.NewMainModel(downloader, version, monitoringService)
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
