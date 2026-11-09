package models

import (
	"time"
)

// Account is a cached creator account record. It persists the account ID →
// username mapping so folder names stay stable across runs even when the API
// is rate limiting or an account has since been deleted.
type Account struct {
	ID          string `gorm:"primaryKey"`
	Username    string `gorm:"index"`
	DisplayName string
	AvatarURL   string
	BannerURL   string
	UpdatedAt   time.Time
}

// TableName overrides the table name
func (Account) TableName() string {
	return "accounts"
}
