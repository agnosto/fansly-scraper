package models

import (
	"time"
)

// ProcessedPost represents a post that has been successfully downloaded.
type ProcessedPost struct {
	ID            uint   `gorm:"primaryKey"`
	PostID        string `gorm:"uniqueIndex;not null"`
	ModelUsername string `gorm:"index;not null"`
	Content       string
	Link          string
	PostCreatedAt time.Time
	CreatedAt     time.Time
}

// TableName overrides the table name
func (ProcessedPost) TableName() string {
	return "processed_posts"
}
