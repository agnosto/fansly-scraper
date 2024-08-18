package main

import (
	"fmt"
	"github.com/agnosto/fansly-scraper/cmd"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/download"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/service"
	"github.com/agnosto/fansly-scraper/ui"
	"github.com/agnosto/fansly-scraper/updater"
	//ksvc "github.com/kardianos/service"

	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	//"flag"

	tea "github.com/charmbracelet/bubbletea"
)

var ffmpegAvailable bool

const version = "v0.3.6"

func main() {
	flags, subcommand := cmd.ParseFlags()

	err := config.EnsureConfigExists(config.GetConfigPath())
	if err != nil {
		log.Fatal(err)
	}

	//if len(os.Args) == 1 || (len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
	//	cmd.PrintUsage()
	//	return
	//}

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
	case "monitor":
		switch flags.MonitorCommand {
		case "start":
			startMonitoring()
		case "stop":
			stopMonitoring()
		default:
			fmt.Println("Usage: ./fansly-scraper monitor [start|stop]")
		}
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("Received interrupt signal. Shutting down...")
		stopMonitoring()
		cleanupLockFiles()
		os.Exit(0)
	}()

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

	monitoringService := service.NewMonitoringService(
		filepath.Join(config.GetConfigDir(), "monitoring_state.json"),
		logger.Logger,
	)

	if flags.Monitor != "" {
		modelID, err := core.GetModelIDFromUsername(flags.Monitor)
		if err != nil {
			logger.Logger.Printf("Error getting model ID: %v", err)
			os.Exit(1)
		}
		started := monitoringService.ToggleMonitoring(modelID, flags.Monitor)
		if started {
			fmt.Printf("Started monitoring for %s\n", flags.Monitor)
		} else {
			fmt.Printf("Stopped monitoring for %s\n", flags.Monitor)
		}
		return
	}

	if subcommand == "monitor" && flags.MonitorCommand == "start" {
		startMonitoring()
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

func startMonitoring() {
	pidFile := filepath.Join(config.GetConfigDir(), "monitor.pid")
	if _, err := os.Stat(pidFile); err == nil {
		fmt.Println("Monitoring process is already running.")
		return
	}

	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		fmt.Printf("Error writing PID file: %v\n", err)
		return
	}

	defer os.Remove(pidFile)

	fmt.Printf("Started monitoring process with PID %d\n", pid)

	monitoringService := service.NewMonitoringService(
		filepath.Join(config.GetConfigDir(), "monitoring_state.json"),
		logger.Logger,
	)
	monitoringService.StartMonitoring()
	go monitoringService.Run() // Run in a goroutine to allow the main process to continue

	// Keep the main process running
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("Received interrupt signal. Shutting down monitoring...")
		stopMonitoring()
		cleanupLockFiles()
		os.Exit(0)
	}()

	// Keep the main process running
	select {}
}

func stopMonitoring() {
	pidFile := filepath.Join(config.GetConfigDir(), "monitor.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("No monitoring process is running.")
		return
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		fmt.Printf("Error reading PID: %v\n", err)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Error finding process: %v\n", err)
		return
	}

	if err := process.Signal(os.Interrupt); err != nil {
		fmt.Printf("Error stopping process: %v\n", err)
		return
	}

	if err := os.Remove(pidFile); err != nil {
		fmt.Printf("Error removing PID file: %v\n", err)
		return
	}

	// I don't even think this function would really be used
	// startMonitoring isn't a background process
	cleanupLockFiles()

	fmt.Println("Monitoring process stopped.")
}

func cleanupLockFiles() {
	recordingsPath := filepath.Join(config.GetConfigDir(), "active_recordings")
	files, err := os.ReadDir(recordingsPath)
	if err != nil {
		fmt.Printf("Error reading recordings directory: %v\n", err)
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".lock" {
			lockFile := filepath.Join(recordingsPath, file.Name())
			if err := os.Remove(lockFile); err != nil {
				fmt.Printf("Error removing lock file %s: %v\n", lockFile, err)
			} else {
				fmt.Printf("Removed lock file: %s\n", lockFile)
			}
		}
	}
}
