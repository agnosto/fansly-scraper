package service

import (
	"encoding/json"
	"fmt"
	//"maps"
	"strconv"
	"syscall"

	"log"

	"strings"

	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/db"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/db/service"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/notifications"
	"github.com/agnosto/fansly-scraper/utils"
)

type MonitoringService struct {
	activeMonitors       map[string]string
	mu                   sync.Mutex
	activeRecordings     map[string]bool
	activeMonitoring     map[string]bool
	storagePath          string
	recordingsPath       string
	stopChan             chan struct{}
	logger               *log.Logger
	isTUI                bool
	notificationSvc      *notifications.NotificationService
	chatRecorder         *ChatRecorder
	fileService          *service.FileService
	processedPostService *service.ProcessedPostService
}

func (ms *MonitoringService) GetRecordingsPath() string {
	return ms.recordingsPath
}

func (ms *MonitoringService) StartRecordingStream(modelID, username, playbackUrl string) {
	ms.startRecording(modelID, username, playbackUrl)
}

func NewMonitoringService(storagePath string, logger *log.Logger) *MonitoringService {
	if logger == nil {
		// Create a default logger if none provided
		logger = log.New(os.Stdout, "monitor: ", log.LstdFlags)
	}

	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		logger.Printf("Error loading config: %v", err)
		// Create a default config if loading fails
		cfg = config.CreateDefaultConfig()
	}

	var fileService *service.FileService
	var processedPostService *service.ProcessedPostService
	database, err := db.NewDatabase(cfg.Options.SaveLocation)
	if err != nil {
		logger.Printf("Error initializing database: %v", err)
		// Continue with nil fileService, we'll check for nil before using
	} else {
		fileRepo := repository.NewFileRepository(database.DB)
		fileService = service.NewFileService(fileRepo)
		postRepo := repository.NewProcessedPostRepository(database.DB)
		processedPostService = service.NewProcessedPostService(postRepo)
		logger.Printf("Database initialized successfully for monitoring service")
	}

	mt := &MonitoringService{
		activeMonitors:       make(map[string]string),
		activeRecordings:     make(map[string]bool),
		activeMonitoring:     make(map[string]bool),
		storagePath:          storagePath,
		recordingsPath:       filepath.Join(filepath.Dir(storagePath), "active_recordings"),
		stopChan:             make(chan struct{}),
		logger:               logger,
		isTUI:                false,
		notificationSvc:      notifications.NewNotificationService(cfg),
		fileService:          fileService,
		processedPostService: processedPostService,
	}

	return mt
}

func (ms *MonitoringService) SetTUIMode(enabled bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.isTUI = enabled
}

func (ms *MonitoringService) StartMonitoring() {
	ms.loadState()
	for modelID, username := range ms.activeMonitors {
		go ms.monitorModel(modelID, username)
	}
}

func (ms *MonitoringService) saveLiveRecording(modelName, filename, postID string) error {
	if ms.fileService == nil {
		return fmt.Errorf("file service not initialized")
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filename)
	}

	// Calculate file hash
	ms.logger.Printf("Calculating hash for live recording: %s", filename)
	hash, err := utils.HashMediaFile(filename)
	if err != nil {
		return fmt.Errorf("error calculating file hash: %v", err)
	}

	ms.logger.Printf("Saving live recording to database: %s with hash %s", filename, hash)
	return ms.fileService.SaveFile(modelName, hash, filename, "livestream", postID)
}

func (ms *MonitoringService) saveContactSheet(modelName, filename, postID string) error {
	if ms.fileService == nil {
		return fmt.Errorf("file service not initialized")
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("contact sheet file does not exist: %s", filename)
	}

	ms.logger.Printf("Calculating hash for contact sheet: %s", filename)
	hash, err := utils.HashMediaFile(filename)
	if err != nil {
		return fmt.Errorf("error calculating contact sheet hash: %v", err)
	}

	ms.logger.Printf("Saving contact sheet to database: %s with hash %s", filename, hash)
	return ms.fileService.SaveFile(modelName, hash, filename, "contact_sheet", postID)
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
	//maps.Copy(ms.activeMonitors, loadedMonitors)
	ms.activeMonitors = loadedMonitors
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

		// Stop chat recording if active
		if ms.chatRecorder != nil && ms.chatRecorder.IsRecording(modelID) {
			if err := ms.chatRecorder.StopRecording(modelID); err != nil {
				logger.Logger.Printf("Error stopping chat recording for %s: %v", username, err)
			} else {
				logger.Logger.Printf("Stopped chat recording for %s", username)
			}
		}

		ms.saveState()
		return false
	} else {
		ms.activeMonitors[modelID] = username
		logger.Logger.Printf("Started monitoring %s", username)
		go ms.monitorModel(modelID, username)
		ms.saveState()
		return true
	}
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

	if !ms.isTUI {
		fmt.Printf("Starting to monitor %s (%s)\n", username, modelID)
	}

	cfg, err := config.LoadConfig(config.GetConfigPath())
	checkInterval := 60 // default safe fallback
	if err == nil && cfg.LiveSettings.CheckInterval > 0 {
		checkInterval = cfg.LiveSettings.CheckInterval
	}

	var wasLive bool = false

	for ms.IsMonitoring(modelID) {
		isLive, playbackUrl, err := core.CheckIfModelIsLive(modelID)
		if err != nil {
			if !ms.isTUI {
				fmt.Printf("Error checking if %s is live: %v\n", username, err)
			}
		} else if isLive {
			lockFile := filepath.Join(ms.recordingsPath, modelID+".lock")
			err = os.MkdirAll(ms.recordingsPath, 0755)
			if err != nil {
				return
			}

			// Check if we need to send a notification for going live
			if !wasLive {
				ms.notificationSvc.NotifyLiveStart(username, modelID)
				wasLive = true
			}

			lockFileExists := false
			if _, err := os.Stat(lockFile); err == nil {
				lockFileExists = true
				if ms.isLockStale(lockFile) {
					os.Remove(lockFile)
					lockFileExists = false
					ms.logger.Printf("Removed stale lock file for %s", username)
				}
			}

			if !lockFileExists {
				if !ms.isTUI {
					printColoredMessage(fmt.Sprintf("%s is live. Attempting to start recording.", username), true)
				}
				ms.startRecording(modelID, username, playbackUrl)

				continue
			} else if !ms.isTUI {
				fmt.Printf("%s is already being recorded\n", username)
			}
		} else {
			// When the model is no longer live, the 'wasLive' flag is reset.
			// The actual "Live End" notification is now handled exclusively by the
			// startRecording function's goroutine after all file processing is complete.
			if wasLive {
				wasLive = false
			}

			if !ms.isTUI {
				printColoredMessage(fmt.Sprintf("%s is not live. Checking again in %d seconds.", username, checkInterval), false)
			}
		}
		//time.Sleep(2 * time.Minute)
		select {
		case <-ms.stopChan:
			return
		case <-time.After(time.Duration(checkInterval) * time.Second):
			// Continue loop
		}
	}

	if !ms.isTUI {
		fmt.Printf("Stopped monitoring %s (%s)\n", username, modelID)
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
	// Write PID to lock file
	pid := os.Getpid()
	f.WriteString(strconv.Itoa(pid))
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
	historyID := streamData.HistoryID

	// Create directory structure
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		ms.logger.Printf("Error loading config: %v", err)
		return
	}

	// Build filename based on config template
	data := map[string]string{
		"model_username": username,
		"date":           time.Now().Format("20060102_150405"),
		"streamId":       streamData.StreamID,
		"streamVersion":  streamData.StreamVersion,
	}
	savePath := config.ResolveLiveSavePath(cfg, username)
	if err := os.MkdirAll(savePath, 0755); err != nil {
		ms.logger.Printf("Error creating directory: %v", err)
		return
	}
	filename := config.GetVODFilename(cfg, data)

	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	extension := filepath.Ext(filename)
	counter := 1
	originalFilename := filename

	for {
		// Check 1: Does the recording file exist? (e.g., video.ts)
		_, errTS := os.Stat(filepath.Join(savePath, filename))

		// Check 2: Does the converted MP4 file exist? (e.g., video.mp4)
		// We construct the MP4 name based on the CURRENT filename attempt
		currentBase := strings.TrimSuffix(filename, extension)
		_, errMP4 := os.Stat(filepath.Join(savePath, currentBase+".mp4"))

		// If BOTH don't exist, we are safe to use this name
		if os.IsNotExist(errTS) && os.IsNotExist(errMP4) {
			break
		}

		// If either exists, increment and try again
		filename = fmt.Sprintf("%s_%d%s", baseName, counter, extension)
		counter++
	}

	if originalFilename != filename {
		ms.logger.Printf("File %s exists, using unique filename: %s", originalFilename, filename)
	}

	recordedFilename := filepath.Join(savePath, filename)
	dir := filepath.Join(cfg.Options.SaveLocation, strings.ToLower(username), "lives")
	if err := os.MkdirAll(dir, 0755); err != nil {
		ms.logger.Printf("Error creating directory for %s: %v", username, err)
		return
	}
	// Start chat recording if we have a chat room ID
	if cfg.LiveSettings.RecordChat && streamData.ChatRoomID != "" {
		if ms.chatRecorder == nil {
			ms.chatRecorder = NewChatRecorder(ms.logger)
		}
		chatFilename := strings.TrimSuffix(recordedFilename, filepath.Ext(recordedFilename)) + "_chat.json"

		// Add debug logging
		ms.logger.Printf("Chat room ID: %s", streamData.ChatRoomID)
		ms.logger.Printf("Chat filename: %s", chatFilename)

		// Ensure directory exists
		chatDir := filepath.Dir(chatFilename)
		if err := os.MkdirAll(chatDir, 0755); err != nil {
			ms.logger.Printf("Error creating directory for chat file: %v", err)
		}

		if err := ms.chatRecorder.StartRecording(modelID, username, streamData.ChatRoomID, chatFilename); err != nil {
			ms.logger.Printf("Error starting chat recording for %s: %v", username, err)
		} else {
			ms.logger.Printf("Started chat recording for %s", username)
		}
	}

	// Log the FFmpeg command for debugging
	ms.logger.Printf("Starting FFmpeg recording for %s with URL: %s to file: %s", username, playbackUrl, recordedFilename)

	var recordingArgs []string
	if cfg.LiveSettings.FFmpegRecordingOptions != "" {
		// Use user-provided options
		recordingArgs = strings.Fields(cfg.LiveSettings.FFmpegRecordingOptions)
		ms.logger.Printf("Using custom FFmpeg recording options: %s", cfg.LiveSettings.FFmpegRecordingOptions)
	} else {
		// Use hardcoded program defaults
		recordingArgs = []string{
			"-c", "copy",
			"-movflags", "use_metadata_tags", "-map_metadata", "0",
			"-timeout", "300", "-reconnect", "300", "-reconnect_at_eof", "300",
			"-reconnect_streamed", "300", "-reconnect_delay_max", "300",
			"-rtmp_live", "live",
		}
	}

	// Build the full command
	cmdArgs := []string{"-i", playbackUrl}
	cmdArgs = append(cmdArgs, recordingArgs...)
	cmdArgs = append(cmdArgs, recordedFilename)

	// Create FFmpeg command
	cmd := exec.Command("ffmpeg", cmdArgs...)

	// Set up stdout and stderr to be logged
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr

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
	// Remove waitgroup, return from the function immediately so monitorModel can loop again
	//var wg sync.WaitGroup
	//wg.Add(1)

	// Run post-processing in a separate goroutine
	go func() {
		//defer wg.Done()
		// Stop chat recording when the stream ends
		if ms.chatRecorder != nil && ms.chatRecorder.IsRecording(modelID) {
			if err := ms.chatRecorder.StopRecording(modelID); err != nil {
				ms.logger.Printf("Error stopping chat recording for %s: %v", username, err)
			} else {
				ms.logger.Printf("Stopped chat recording for %s", username)
			}
		}

		finalFilename := recordedFilename
		var conversionSuccess bool

		// Check if the original file exists
		if _, err := os.Stat(recordedFilename); os.IsNotExist(err) {
			ms.logger.Printf("Error: Original recording file not found for %s: %s", username, recordedFilename)
			return
		} else if err != nil {
			ms.logger.Printf("Error checking original file for %s: %v", username, err)
			return
		}

		if cfg.LiveSettings.FFmpegConvert {
			if strings.ToLower(filepath.Ext(recordedFilename)) == ".mp4" {
				ms.logger.Printf("Recording is already in MP4 format, skipping conversion: %s", recordedFilename)
				conversionSuccess = true
			} else {
				mp4Filename := strings.TrimSuffix(recordedFilename, filepath.Ext(recordedFilename)) + ".mp4"
				ms.logger.Printf("Starting MP4 conversion for %s", username)
				if err := ms.convertToMP4(recordedFilename, mp4Filename); err != nil {
					ms.logger.Printf("Error converting to MP4 for %s: %v. The original file will be kept.", username, err)
					conversionSuccess = true // "Success" in the sense that we keep the original file.
				} else {
					finalFilename = mp4Filename
					conversionSuccess = true
					ms.logger.Printf("MP4 conversion complete for %s", username)
					if err := os.Remove(recordedFilename); err != nil && !os.IsNotExist(err) {
						ms.logger.Printf("Error deleting original file for %s: %v", username, err)
					}
				}
			}
		} else {
			conversionSuccess = true
		}

		// Only save to database if we have a valid file
		if conversionSuccess {
			// Check if the final file exists
			if _, err := os.Stat(finalFilename); os.IsNotExist(err) {
				ms.logger.Printf("Error: Final file not found for %s: %s", username, finalFilename)
				return
			} else if err != nil {
				ms.logger.Printf("Error checking final file for %s: %v", username, err)
				return
			}

			// Save the livestream recording to database
			ms.logger.Printf("Attempting to save live recording info to database for %s: %s", username, finalFilename)
			if err := ms.saveLiveRecording(username, finalFilename, historyID); err != nil {
				ms.logger.Printf("Error saving live recording info for %s to database: %v", username, err)
			} else {
				ms.logger.Printf("Successfully saved live recording info for %s to database", username)
			}

			var contactSheetPath string

			// Generate and save contact sheet if enabled
			if cfg.LiveSettings.GenerateContactSheet {
				sourceFile := finalFilename
				contactSheetFilename := strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile)) + "_contact_sheet.jpg"

				if err := ms.generateContactSheet(sourceFile); err != nil {
					ms.logger.Printf("Error generating contact sheet for %s: %v", username, err)
				} else {
					if _, err := os.Stat(contactSheetFilename); err == nil {
						ms.logger.Printf("Successfully generated contact sheet: %s", contactSheetFilename)
						contactSheetPath = contactSheetFilename // Set the path on success

						// Save contact sheet info to database
						if err := ms.saveContactSheet(username, contactSheetFilename, historyID); err != nil {
							ms.logger.Printf("Error saving contact sheet info for %s to database: %v", username, err)
						} else {
							ms.logger.Printf("Successfully saved contact sheet info for %s to database", username)
						}
					}
				}
			}

			if ms.processedPostService != nil {
				ms.logger.Printf("Saving stream metadata to library for: %s", historyID)
				ms.processedPostService.MarkPostAsProcessed(
					historyID,
					username,
					streamData.Title,
					time.Now().Unix(),
				)
			} else {
				ms.logger.Printf("Skipping database entry for %s: processedPostService is nil", historyID)
			}

			ms.notificationSvc.NotifyLiveEnd(username, modelID, finalFilename, contactSheetPath)
		}
	}()

	// Wait for post-processing to complete
	//wg.Wait()
	ms.logger.Printf("All processing complete for %s", username)
}

func (ms *MonitoringService) convertToMP4(tsFilename, mp4Filename string) error {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		// Fallback to default if config can't be loaded, though this is unlikely
		ms.logger.Printf("Could not load config for conversion options, using default: %v", err)
		cmd := exec.Command("ffmpeg", "-i", tsFilename, "-c", "copy", mp4Filename)
		return cmd.Run()
	}

	var conversionArgs []string
	if cfg.LiveSettings.FFmpegConversionOptions != "" {
		// Use user-provided options
		conversionArgs = strings.Fields(cfg.LiveSettings.FFmpegConversionOptions)
		ms.logger.Printf("Using custom FFmpeg conversion options: %s", cfg.LiveSettings.FFmpegConversionOptions)
	} else {
		// Use hardcoded program default
		conversionArgs = []string{"-c", "copy"}
	}

	// Build the full command
	cmdArgs := []string{"-i", tsFilename}
	cmdArgs = append(cmdArgs, conversionArgs...)
	cmdArgs = append(cmdArgs, mp4Filename)

	cmd := exec.Command("ffmpeg", cmdArgs...)
	return cmd.Run()
}

func (ms *MonitoringService) generateContactSheet(mp4Filename string) error {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		ms.logger.Printf("Error loading config: %v", err)
	}
	contactSheetFilename := strings.TrimSuffix(mp4Filename, ".mp4") + "_contact_sheet.jpg"

	if cfg.LiveSettings.UseMTForContactSheet {
		ms.logger.Printf("Using MT for contact sheet generation")
		cmd := exec.Command("mt", "--columns=4", "--numcaps=24", "--header-meta", "--fast",
			"--output="+contactSheetFilename, mp4Filename)
		return cmd.Run()
	}

	ms.logger.Printf("Using FFmpeg for contact sheet generation")
	durationCmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mp4Filename,
	)

	durationOutput, err := durationCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	// Parse duration and calculate interval
	durationStr := strings.TrimSpace(string(durationOutput))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse duration: %w", err)
	}

	// Define grid dimensions
	gridWidth := 4
	gridHeight := 6
	totalFrames := gridWidth * gridHeight

	// Calculate interval between frames
	interval := duration / float64(totalFrames)

	// Build the complex filter for ffmpeg
	vfFilter := fmt.Sprintf(
		"select='if(eq(0,n),1,gte(t-prev_selected_t,%f))',"+
			"setpts=PTS-STARTPTS,"+
			"scale=640:360,"+
			"drawtext=text='%%{pts\\:hms}':x=w-tw-5:y=h-th-5:fontsize=14:fontcolor=white:box=1:boxcolor=black@1.0:boxborderw=5,"+
			"tile=%dx%d",
		interval, gridWidth, gridHeight,
	)

	// Run ffmpeg command
	cmd := exec.Command(
		"ffmpeg",
		"-i", mp4Filename,
		"-vf", vfFilter,
		"-frames:v", "1",
		"-q:v", "1",
		contactSheetFilename,
	)

	// For debugging
	//ms.logger.Printf("Running ffmpeg command: %v", cmd.Args)
	ms.logger.Printf("Contact sheet generation complete for %s", mp4Filename)

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
	ms.loadState()

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

	if ms.chatRecorder != nil {
		ms.chatRecorder.StopAllRecordings()
	}

	time.Sleep(5 * time.Second)

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

func (ms *MonitoringService) isLockStale(lockFile string) bool {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return false // Can't read, assume active/problematic
	}

	// If empty, assume stale (leftover from crash or old version that didn't write PID)
	if len(data) == 0 {
		return true
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return true // Invalid content, assume stale
	}

	return !isProcessRunning(pid)
}

func isProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// On Windows, FindProcess always succeeds, so we need to try to get exit code
		processState, err := process.Wait()
		return err == nil && !processState.Exited()
	}
	// Unix-like systems (Linux, macOS)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
