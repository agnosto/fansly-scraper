package main

import (
	"fmt"
	"go-fansly-scraper/config"
	"go-fansly-scraper/download"
	"go-fansly-scraper/ui"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
    cfg, err := config.LoadConfig(config.GetConfigPath())
    if err != nil {
        log.Fatal(err)
    }

    downloader, err := download.NewDownloader(cfg)
    if err != nil {
        log.Fatal(err)
    }
    model := ui.NewMainModel(downloader)
    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v", err)
        os.Exit(1)
    }
}
