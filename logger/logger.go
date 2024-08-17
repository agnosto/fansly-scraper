package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/agnosto/fansly-scraper/config"
)

const (
	maxLogSize    = 5 * 1024 * 1024 // 5MB
	maxLogBackups = 5
)

var (
	Logger *log.Logger
)

func InitLogger(cfg *config.Config) error {
	logDir := filepath.Join(cfg.Options.SaveLocation, ".logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "fansly-scraper.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	Logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

	go rotateLogFile(logFile)

	return nil
}

func rotateLogFile(logFile string) {
	for {
		time.Sleep(1 * time.Hour)

		file, err := os.Stat(logFile)
		if err != nil {
			Logger.Printf("Error checking log file: %v", err)
			continue
		}

		if file.Size() < maxLogSize {
			continue
		}

		Logger.Printf("Rotating log file")

		for i := maxLogBackups - 1; i > 0; i-- {
			oldFile := fmt.Sprintf("%s.%d", logFile, i)
			newFile := fmt.Sprintf("%s.%d", logFile, i+1)
			os.Rename(oldFile, newFile)
		}

		os.Rename(logFile, logFile+".1")

		// Close the current log file
		Logger.Writer().(*os.File).Close()

		// Open a new log file
		newFile, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			Logger.Printf("Error creating new log file: %v", err)
			continue
		}

		Logger.SetOutput(newFile)
	}
}
