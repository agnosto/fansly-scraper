package main

import (
	"fmt"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/download"
	"github.com/agnosto/fansly-scraper/ui"
    "github.com/agnosto/fansly-scraper/updater"
    "github.com/agnosto/fansly-scraper/logger"

	"log"
	"os"
    "os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

var ffmpegAvailable bool
const version = "v0.1.0"

func main() {
    if len(os.Args) > 1 && os.Args[1] == "update" {
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

    ffmpegAvailable = isFFmpegAvailable()
    logger.Logger.Printf("Ffmpeg Check Returned %v", ffmpegAvailable)

    downloader, err := download.NewDownloader(cfg, ffmpegAvailable)
    if err != nil {
        logger.Logger.Fatal(err)
    }

    model := ui.NewMainModel(downloader, version)
    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        logger.Logger.Printf("Error: %v", err)
        os.Exit(1)
    }
}

func isFFmpegAvailable() bool {
    _, err := exec.LookPath("ffmpeg")
    return err == nil
}
