package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Database represents the database connection
type Database struct {
	DB *gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(saveLocation string) (*Database, error) {
	dbPath := filepath.Join(saveLocation, "downloads.db")

	// Check if the database exists and has the old schema
	needsMigration, err := checkOldSchema(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check database schema: %w", err)
	}

	// Configure GORM logger
	logConfig := gormlogger.Config{
		LogLevel: gormlogger.Warn, // Log only warnings and errors
		Colorful: true,
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.New(
			logger.Logger,
			logConfig,
		),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// If we need to migrate from the old schema
	if needsMigration {
		if err := migrateOldSchema(db); err != nil {
			return nil, fmt.Errorf("failed to migrate old schema: %w", err)
		}
	} else {
		// Run normal migrations for new database
		if err := db.AutoMigrate(&models.File{}); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return &Database{DB: db}, nil
}

// checkOldSchema checks if the database has the old schema
func checkOldSchema(dbPath string) (bool, error) {
	// Check if the file exists
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		// If the database doesn't exist, no migration needed
		return false, nil
	}
	defer sqlDB.Close()

	// Check if the files table exists with the old schema
	var count int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master 
                         WHERE type='table' AND name='files' 
                         AND sql LIKE '%hash TEXT PRIMARY KEY%'`).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// migrateOldSchema migrates from the old schema to the new GORM schema
func migrateOldSchema(db *gorm.DB) error {
	// Create a temporary table with the new schema
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS files_new (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        model TEXT NOT NULL,
        hash TEXT UNIQUE NOT NULL,
        path TEXT NOT NULL,
        file_type TEXT NOT NULL,
        created_at DATETIME,
        updated_at DATETIME
    )`).Error; err != nil {
		return err
	}

	// Format current time in RFC3339 format which GORM uses
	now := time.Now().Format(time.RFC3339)

	// Copy data from the old table to the new one with properly formatted timestamps
	if err := db.Exec(`INSERT INTO files_new (model, hash, path, file_type, created_at, updated_at)
                      SELECT model, hash, path, file_type, ?, ?
                      FROM files`, now, now).Error; err != nil {
		return err
	}

	// Drop the old table
	if err := db.Exec(`DROP TABLE files`).Error; err != nil {
		return err
	}

	// Rename the new table to the original name
	if err := db.Exec(`ALTER TABLE files_new RENAME TO files`).Error; err != nil {
		return err
	}

	// Create indexes
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_files_model ON files(model)`).Error; err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
