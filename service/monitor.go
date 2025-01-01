package service

import (
	//"context"
	//"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	//"io"
	"log"

	//"strconv"
	"strings"

	//"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fatih/color"
	_ "modernc.org/sqlite"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/logger"
)

type MonitoringService struct {
	activeMonitors   map[string]string
	mu               sync.Mutex
	activeRecordings map[string]bool
	activeMonitoring map[string]bool
	storagePath      string
	recordingsPath   string
	stopChan         chan struct{}
	logger           *log.Logger
}

func NewMonitoringService(storagePath string, logger *log.Logger) *MonitoringService {
	if logger == nil {
		// Create a default logger if none provided
		logger = log.New(os.Stdout, "monitor: ", log.LstdFlags)
	}

	mt := &MonitoringService{
		activeMonitors:   make(map[string]string),
		activeRecordings: make(map[string]bool),
		activeMonitoring: make(map[string]bool),
		storagePath:      storagePath,
		//recordingsPath:   storagePath,
		recordingsPath: filepath.Join(filepath.Dir(storagePath), "active_recordings"),
		stopChan:       make(chan struct{}),
		logger:         logger,
	}
	return mt
}

func (ms *MonitoringService) StartMonitoring() {
	ms.loadState()
	for modelID, username := range ms.activeMonitors {
		go ms.monitorModel(modelID, username)
	}
}

func (ms *MonitoringService) saveLiveRecording(modelName, filename, streamID string) error {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(cfg.Options.SaveLocation, "downloads.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO files (model, hash, path, file_type) VALUES (?, ?, ?, ?)`,
		modelName, streamID, filename, "livestream")
	return err
}

func (ms *MonitoringService) loadActiveRecordings() {
	data, err := os.ReadFile(ms.recordingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Logger.Printf("Error loading active recordings: %v", err)
		}
		return
	}
	if err := json.Unmarshal(data, &ms.activeRecordings); err != nil {
		logger.Logger.Printf("Error unmarshaling active recordings: %v", err)
	}
}

func (ms *MonitoringService) saveActiveRecordings() {
	data, err := json.Marshal(ms.activeRecordings)
	if err != nil {
		logger.Logger.Printf("Error marshaling active recordings: %v", err)
		return
	}
	if err := os.WriteFile(ms.recordingsPath, data, 0644); err != nil {
		logger.Logger.Printf("Error saving active recordings: %v", err)
	}
}

func (ms *MonitoringService) loadState() {
	data, err := os.ReadFile(ms.storagePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Logger.Printf("Error loading monitoring state: %v", err)
		}
		return
	}

	var loadedMonitors map[string]string
	if err := json.Unmarshal(data, &loadedMonitors); err != nil {
		logger.Logger.Printf("Error unmarshaling monitoring state: %v", err)
		return
	}

	// Merge the loaded state with the existing state
	for modelID, username := range loadedMonitors {
		ms.activeMonitors[modelID] = username
	}
}

func (ms *MonitoringService) saveState() {
	data, err := json.Marshal(ms.activeMonitors)
	if err != nil {
		logger.Logger.Printf("Error marshaling monitoring state: %v", err)
		return
	}

	perm := os.FileMode(0644)
	if runtime.GOOS == "windows" {
		perm = 0666
	}

	if err := os.WriteFile(ms.storagePath, data, perm); err != nil {
		logger.Logger.Printf("Error saving monitoring state: %v", err)
	}
}

func (ms *MonitoringService) ToggleMonitoring(modelID, username string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.loadState()

	if _, exists := ms.activeMonitors[modelID]; exists {
		delete(ms.activeMonitors, modelID)
		logger.Logger.Printf("Stopped monitoring %s", username)
		ms.saveState()
		return false
	} else {
		ms.activeMonitors[modelID] = username
		logger.Logger.Printf("Started monitoring %s", username)
		go ms.monitorModel(modelID, username)
		ms.saveState()
		return true
	}
	//ms.saveState()
}

func (ms *MonitoringService) monitorModel(modelID, username string) {
	ms.mu.Lock()
	if ms.activeMonitoring[modelID] {
		ms.mu.Unlock()
		return // If already being monitored, return early
	}
	ms.activeMonitoring[modelID] = true
	ms.mu.Unlock()

	defer func() {
		ms.mu.Lock()
		delete(ms.activeMonitoring, modelID)
		ms.mu.Unlock()
	}()

	fmt.Printf("Starting to monitor %s (%s)\n", username, modelID)
	for ms.IsMonitoring(modelID) {
		isLive, playbackUrl, err := core.CheckIfModelIsLive(modelID)
		if err != nil {
			fmt.Printf("Error checking if %s is live: %v\n", username, err)
		} else if isLive {
			printColoredMessage(fmt.Sprintf("%s is live. Attempting to start recording.", username), true)
			lockFile := filepath.Join(ms.recordingsPath, modelID+".lock")
			err = os.MkdirAll(ms.recordingsPath, 0755)
			if err != nil {
				return
			}
			if _, err := os.Stat(lockFile); os.IsNotExist(err) {
				go ms.startRecording(modelID, username, playbackUrl)
			} else {
				fmt.Printf("%s is already being recorded\n", username)
			}
		} else {
			printColoredMessage(fmt.Sprintf("%s is not live. Checking again in 2 minutes.", username), false)
		}
		time.Sleep(2 * time.Minute)
	}
	fmt.Printf("Stopped monitoring %s (%s)\n", username, modelID)
}

func isProcessRunning(name string) bool {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", name))
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), name)
	case "darwin", "linux":
		cmd := exec.Command("pgrep", name)
		err := cmd.Run()
		return err == nil
	default:
		return false
	}
}

func (ms *MonitoringService) IsMonitoring(modelID string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	_, exists := ms.activeMonitors[modelID]
	return exists
}

func (ms *MonitoringService) startRecording(modelID, username, playbackUrl string) {
	lockFile := filepath.Join(ms.recordingsPath, modelID+".lock")

	// Ensure recordings directory exists
	if err := os.MkdirAll(ms.recordingsPath, 0755); err != nil {
		ms.logger.Printf("Error creating recordings directory: %v", err)
		return
	}

	// Create lock file with atomic operation
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			ms.logger.Printf("%s is already being recorded", username)
			return
		}
		ms.logger.Printf("Error creating lock file for %s: %v", username, err)
		return
	}
	f.Close()

	// Ensure lock file cleanup
	defer func() {
		if err := os.Remove(lockFile); err != nil {
			ms.logger.Printf("Error removing lock file for %s: %v", username, err)
		}
	}()

	// Fetch stream data with error handling
	streamData, err := core.GetStreamData(modelID)
	if err != nil {
		ms.logger.Printf("Error fetching stream data for %s: %v", modelID, err)
		return
	}

	// Create directory structure
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		ms.logger.Printf("Error loading config: %v", err)
		return
	}

	dir := filepath.Join(cfg.Options.SaveLocation, strings.ToLower(username), "lives")
	if err := os.MkdirAll(dir, 0755); err != nil {
		ms.logger.Printf("Error creating directory for %s: %v", username, err)
		return
	}

	// Set up recording filename
	currentTime := time.Now().Format("20060102_150405")
	sanitizedUsername := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, username)
	filename := fmt.Sprintf("%s_%s_v%s", sanitizedUsername, currentTime, streamData.StreamID)
	recordedFilename := filepath.Join(dir, filename+cfg.Options.VODsFileExtension)

	// Create FFmpeg command
	cmd := exec.Command("ffmpeg", "-i", playbackUrl, "-c", "copy",
		"-movflags", "use_metadata_tags", "-map_metadata", "0",
		"-timeout", "300", "-reconnect", "300", "-reconnect_at_eof", "300",
		"-reconnect_streamed", "300", "-reconnect_delay_max", "300",
		"-rtmp_live", "live", recordedFilename)

	// Capture FFmpeg output for debugging
	//var stdBuffer bytes.Buffer
	//mw := io.MultiWriter(os.Stdout, &stdBuffer)
	//cmd.Stdout = mw
	//cmd.Stderr = mw

	ms.logger.Printf("Starting FFmpeg recording for %s with URL: %s", username, playbackUrl)

	if err := cmd.Start(); err != nil {
		ms.logger.Printf("Error starting FFmpeg for %s: %v", username, err)
		return
	}

	// Wait for FFmpeg in a goroutine
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	// Monitor the recording
	select {
	case err := <-done:
		if err != nil {
			ms.logger.Printf("FFmpeg error for %s: %v", username, err)
		}
	case <-ms.stopChan:
		ms.logger.Printf("Stopping recording for %s due to stop signal", username)
		if err := cmd.Process.Kill(); err != nil {
			ms.logger.Printf("Error killing FFmpeg process for %s: %v", username, err)
		}
	}

	ms.logger.Printf("Recording complete for %s.", username)

	// Create a wait group for post-processing
	var wg sync.WaitGroup
	wg.Add(1)

	// Run post-processing in a separate goroutine
	go func() {
		defer wg.Done()

		if cfg.Options.FFmpegConvert {
			mp4Filename := filepath.Join(dir, filename+".mp4")
			ms.logger.Printf("Starting MP4 conversion for %s", username)

			if err := ms.convertToMP4(recordedFilename, mp4Filename); err != nil {
				ms.logger.Printf("Error converting to MP4 for %s: %v", username, err)
				return
			}
			ms.logger.Printf("MP4 conversion complete for %s", username)

			if err := os.Remove(recordedFilename); err != nil && !os.IsNotExist(err) {
				ms.logger.Printf("Error deleting TS file for %s: %v", username, err)
			}
		}

		if cfg.Options.GenerateContactSheet {
			mp4Filename := filepath.Join(dir, filename+".mp4")
			if err := ms.generateContactSheet(mp4Filename); err != nil {
				ms.logger.Printf("Error generating contact sheet for %s: %v", username, err)
			}
		}

		if err := ms.saveLiveRecording(username, recordedFilename, streamData.StreamID); err != nil {
			ms.logger.Printf("Error saving live recording info for %s: %v", username, err)
		}
	}()

	// Wait for post-processing to complete
	wg.Wait()
	ms.logger.Printf("All processing complete for %s", username)
}

func (ms *MonitoringService) convertToMP4(tsFilename, mp4Filename string) error {
	cmd := exec.Command("ffmpeg", "-i", tsFilename, "-c", "copy", mp4Filename)
	return cmd.Run()
}

func (ms *MonitoringService) generateContactSheet(mp4Filename string) error {
	contactSheetFilename := strings.TrimSuffix(mp4Filename, ".mp4") + "_contact_sheet.jpg"
	//cmd := exec.Command("./mt", "--columns=4", "--numcaps=24", "--header-meta", "--fast", "--comment=Archive - Fansly VODs", "--output="+contactSheetFilename, mp4Filename)
	cmd := exec.Command(
		"ffmpeg",
		"-i", mp4Filename,
		"-vf", "select='not(mod(n,300))',scale=320:180,tile=4x6",
		"-frames:v", "1",
		contactSheetFilename,
	)
	return cmd.Run()
}

func (ms *MonitoringService) Run() {
	fmt.Printf("Starting monitoring service\n")
	ms.loadState()
	//ms.loadActiveRecordings()

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			clearTerminal()
			ms.checkModels()
		case <-ms.stopChan:
			return
		}
	}
}

func (ms *MonitoringService) checkModels() {
	ms.mu.Lock()
	models := make([]struct{ id, username string }, 0, len(ms.activeMonitors))
	for modelID, username := range ms.activeMonitors {
		models = append(models, struct{ id, username string }{modelID, username})
	}
	ms.mu.Unlock()

	for _, model := range models {
		go ms.monitorModel(model.id, model.username)
	}
}

func (ms *MonitoringService) Shutdown() {
	close(ms.stopChan)
	//ms.mu.Lock()
	//defer ms.mu.Unlock()
	//ms.saveState()
}

func clearTerminal() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func printColoredMessage(message string, isLive bool) {
	if isLive {
		color.Green(message)
	} else {
		color.Red(message)
	}
}
