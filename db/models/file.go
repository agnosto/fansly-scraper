package models

import (
	"time"
)

// File represents a downloaded file in the database
type File struct {
	ID        uint   `gorm:"primaryKey"`
	Model     string `gorm:"index;not null"`
	Hash      string `gorm:"uniqueIndex;not null"`
	Path      string `gorm:"not null"`
	FileType  string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName overrides the table name
func (File) TableName() string {
	return "files"
}
