package service

import (
	//"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
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
	ms := &MonitoringService{
		activeMonitors:   make(map[string]string),
		activeRecordings: make(map[string]bool),
		activeMonitoring: make(map[string]bool),
		storagePath:      storagePath,
		//recordingsPath:   storagePath,
		recordingsPath: filepath.Join(filepath.Dir(storagePath), "active_recordings"),
		stopChan:       make(chan struct{}),
		logger:         logger,
	}
	return ms
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
	if err := os.WriteFile(ms.storagePath, data, 0644); err != nil {
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
	cmd := exec.Command("pgrep", name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (ms *MonitoringService) IsMonitoring(modelID string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	_, exists := ms.activeMonitors[modelID]
	return exists
}

func (ms *MonitoringService) startRecording(modelID, username, playbackUrl string) {
	lockFile := filepath.Join(ms.recordingsPath, modelID+".lock")

	// Check if a lock file exists
	if _, err := os.Stat(lockFile); err == nil {
		//ms.logger.Printf("%s is already being recorded", username)
		fmt.Printf("%s is already being recorded", username)
		return
	}

	// Create a lock file
	if err := os.WriteFile(lockFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		//ms.logger.Printf("Error creating lock file for %s: %v", username, err)
		fmt.Printf("Error creating lock file for %s: %v", username, err)
		return
	}

	defer func() {
		if err := os.Remove(lockFile); err != nil {
			ms.logger.Printf("Error removing lock file for %s: %v", username, err)
		}
		//logger.Logger.Printf("Finished recording for %s", username)
		fmt.Printf("Finished recording for %s\n", username)
	}()

	// Fetch stream data
	streamData, err := core.GetStreamData(modelID)
	//logger.Logger.Printf("STREAM DATA: %v", streamData)
	if err != nil {
		logger.Logger.Printf("Error fetching stream data for %s: %v", modelID, err)
		return
	}

	// Create filename
	currentTime := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_v%s", username, currentTime, streamData.StreamID)

	// Create directory

	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		logger.Logger.Printf("Error loading config: %v", err)
	}

	dir := filepath.Join(cfg.Options.SaveLocation, strings.ToLower(username), "lives")
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Logger.Printf("Error creating directory for %s: %v", username, err)
		return
	}

	// Start ffmpeg recording
	recordedFilename := filepath.Join(dir, filename+cfg.Options.VODsFileExtension)
	cmd := exec.Command("ffmpeg", "-i", playbackUrl, "-c", "copy", "-movflags", "use_metadata_tags", "-map_metadata", "0", "-timeout", "300", "-reconnect", "300", "-reconnect_at_eof", "300", "-reconnect_streamed", "300", "-reconnect_delay_max", "300", "-rtmp_live", "live", recordedFilename)

	//logger.Logger.Printf("Starting recording for %s", username)
	fmt.Printf("Starting recording for %s\n", username)
	err = cmd.Start()
	if err != nil {
		logger.Logger.Printf("Error starting ffmpeg for %s: %v", username, err)
		return
	}

	err = cmd.Wait()
	if err != nil {
		logger.Logger.Printf("Error during ffmpeg recording for %s: %v", username, err)
	} else {
		//logger.Logger.Printf("Recording process completed for %s", username)
		fmt.Printf("Recording process completed for %s\n", username)
	}

	// Convert to MP4 if needed
	if cfg.Options.FFmpegConvert {
		mp4Filename := filepath.Join(dir, filename+".mp4")
		if err := ms.convertToMP4(recordedFilename, mp4Filename); err != nil {
			logger.Logger.Printf("Error converting to MP4 for %s: %v", username, err)
			return
		}
		// Delete TS file
		if err := os.Remove(recordedFilename); err != nil {
			logger.Logger.Printf("Error deleting TS file for %s: %v", username, err)
		}
	}

	// Generate contact sheet if needed
	if cfg.Options.GenerateContactSheet {
		mp4Filename := filepath.Join(dir, filename+".mp4")
		if err := ms.generateContactSheet(mp4Filename); err != nil {
			logger.Logger.Printf("Error generating contact sheet for %s: %v", username, err)
		}
	}

	if err := ms.saveLiveRecording(username, recordedFilename, streamData.StreamID); err != nil {
		logger.Logger.Printf("Error saving live recording info for %s: %v", username, err)
	}

	//logger.Logger.Printf("Recording complete for %s", username)
	fmt.Printf("Recording complete for %s\n", username)
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
