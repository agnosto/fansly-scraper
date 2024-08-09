package service

import (
	//"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	//"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/logger"
)

type MonitoringService struct {
	activeMonitors   map[string]string
	mu               sync.Mutex
	activeRecordings map[string]bool
	storagePath      string
	recordingsPath   string
}

func NewMonitoringService(storagePath string) *MonitoringService {
	ms := &MonitoringService{
		activeMonitors:   make(map[string]string),
		activeRecordings: make(map[string]bool),
		storagePath:      storagePath,
		recordingsPath:   filepath.Join(filepath.Dir(storagePath), "active_recordings.json"),
	}
	ms.loadState()
	ms.loadActiveRecordings()
	//go ms.runMonitoring()
	return ms
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
	if err := json.Unmarshal(data, &ms.activeMonitors); err != nil {
		logger.Logger.Printf("Error unmarshaling monitoring state: %v", err)
	}

	for modelID, username := range ms.activeMonitors {
		go ms.monitorModel(modelID, username)
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
	logger.Logger.Printf("Starting to monitor %s (%s)", username, modelID)
	for ms.IsMonitoring(modelID) {
		isLive, playbackUrl, err := core.CheckIfModelIsLive(modelID)
		if err != nil {
			logger.Logger.Printf("Error checking if %s is live: %v", username, err)
		} else if isLive {
			logger.Logger.Printf("%s is live. Attempting to start recording.", username)
			ms.mu.Lock()
			isRecording := ms.activeRecordings[modelID]
			ms.mu.Unlock()
			if !isRecording {
				go ms.startRecording(modelID, username, playbackUrl)
			} else {
				logger.Logger.Printf("%s is already being recorded", username)
			}
		} else {
			logger.Logger.Printf("%s is not live. Checking again in 2 minutes.", username)
		}
		time.Sleep(2 * time.Minute)
	}
	logger.Logger.Printf("Stopped monitoring %s (%s)", username, modelID)
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

/*
func (ms *MonitoringService) runMonitoring(username) {
    for {
        ms.mu.Lock()
        for modelID := range ms.activeMonitors {
            go func(id string) {
                isLive, playbackUrl, err := core.CheckIfModelIsLive(id)
                if err != nil {
                    logger.Logger.Printf("Error checking if %s is live: %v", id, err)
                } else if isLive {
                    logger.Logger.Printf("%s is now live!", id)
                    ms.startRecording(id, username, playbackUrl)
                }
            }(modelID)
        }
        ms.mu.Unlock()
        time.Sleep(2 * time.Minute)
    }
}
*/

func (ms *MonitoringService) startRecording(modelID, username, playbackUrl string) {
	ms.mu.Lock()
	if ms.activeRecordings[modelID] {
		ms.mu.Unlock()
		logger.Logger.Printf("%s is already being recorded", username)
		return
	}
	ms.activeRecordings[modelID] = true
	ms.saveActiveRecordings()
	ms.mu.Unlock()

	defer func() {
		ms.mu.Lock()
		delete(ms.activeRecordings, modelID)
		ms.saveActiveRecordings()
		ms.mu.Unlock()
		logger.Logger.Printf("Finished recording for %s", username)
	}()

	// Fetch stream data
	streamData, err := core.GetStreamData(modelID)
	logger.Logger.Printf("STREAM DATA: %v", streamData)
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

	logger.Logger.Printf("Starting recording for %s", username)
	err = cmd.Start()
	if err != nil {
		logger.Logger.Printf("Error starting ffmpeg for %s: %v", username, err)
		//ms.mu.Lock()
		//delete(ms.activeRecordings, modelID)
		//ms.mu.Unlock()
		return
	}

	// Wait for ffmpeg to finish
	/*
	   go func() {
	       err := cmd.Wait()
	       ms.mu.Lock()
	       delete(ms.activeRecordings, modelID)
	       ms.saveActiveRecordings()
	       ms.mu.Unlock()
	       if err != nil {
	           logger.Logger.Printf("Error during ffmpeg recording for %s: %v", username, err)
	       } else {
	           logger.Logger.Printf("Recording complete for %s", username)
	       }

	       // Perform post-recording tasks (convert to MP4, generate contact sheet) as before...
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

	       if cfg.Options.GenerateContactSheet {
	           mp4Filename := filepath.Join(dir, filename+".mp4")
	           if err := ms.generateContactSheet(mp4Filename); err != nil {
	               logger.Logger.Printf("Error generating contact sheet for %s: %v", username, err)
	           }
	       }

	       logger.Logger.Printf("Recording complete for %s", username)

	   }()
	*/

	err = cmd.Wait()
	if err != nil {
		logger.Logger.Printf("Error during ffmpeg recording for %s: %v", username, err)
	} else {
		logger.Logger.Printf("Recording process completed for %s", username)
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

	logger.Logger.Printf("Recording complete for %s", username)
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
	ms.loadState()
	ms.loadActiveRecordings()

	if isProcessRunning("ffmpeg") {
		logger.Logger.Println("Existing ffmpeg processes detected. Assuming recordings are in progress.")
	}

	sem := make(chan struct{}, 5)

	for {
		ms.mu.Lock()
		models := make([]struct{ id, username string }, 0, len(ms.activeMonitors))
		for modelID, username := range ms.activeMonitors {
			models = append(models, struct{ id, username string }{modelID, username})
		}
		ms.mu.Unlock()

		for _, model := range models {
			sem <- struct{}{}
			go func(id, username string) {
				defer func() { <-sem }()
				ms.monitorModel(id, username)
			}(model.id, model.username)
		}

		time.Sleep(2 * time.Minute)
	}
}

func (ms *MonitoringService) Shutdown() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.saveState()
}
