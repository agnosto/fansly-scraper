package service

import (
	"testing"

	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.Account{}, &models.ProcessedPost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestAccountServiceCacheRoundTrip(t *testing.T) {
	db := newTestDB(t)
	svc := NewAccountService(repository.NewAccountRepository(db))

	if got := svc.GetCachedUsername("123"); got != "" {
		t.Fatalf("expected empty username for unknown account, got %q", got)
	}

	if err := svc.SaveAccount("123", "somecreator", "Some Creator", "https://cdn/avatar.jpg", ""); err != nil {
		t.Fatalf("SaveAccount failed: %v", err)
	}

	if got := svc.GetCachedUsername("123"); got != "somecreator" {
		t.Errorf("expected cached username somecreator, got %q", got)
	}

	cached := svc.GetCachedUsernames([]string{"123", "456"})
	if cached["123"] != "somecreator" {
		t.Errorf("expected map to contain 123=somecreator, got %v", cached)
	}
	if _, ok := cached["456"]; ok {
		t.Errorf("expected map to omit unknown account 456, got %v", cached)
	}

	// Upsert should update the username (e.g. after a rename)
	if err := svc.SaveAccount("123", "renamed", "", "", ""); err != nil {
		t.Fatalf("SaveAccount (update) failed: %v", err)
	}
	if got := svc.GetCachedUsername("123"); got != "renamed" {
		t.Errorf("expected updated username renamed, got %q", got)
	}
}

func TestProcessedPostServiceContentType(t *testing.T) {
	db := newTestDB(t)
	svc := NewProcessedPostService(repository.NewProcessedPostRepository(db))

	if err := svc.MarkPostAsProcessedFull("999", "123", "somecreator", "hello", "purchases", 1785342602); err != nil {
		t.Fatalf("MarkPostAsProcessedFull failed: %v", err)
	}

	var post models.ProcessedPost
	if err := db.Where("post_id = ?", "999").First(&post).Error; err != nil {
		t.Fatalf("failed to load post: %v", err)
	}
	if post.ContentType != "purchases" {
		t.Errorf("expected content_type purchases, got %q", post.ContentType)
	}
	if post.AccountID != "123" {
		t.Errorf("expected account_id 123, got %q", post.AccountID)
	}

	// Legacy method defaults to timeline
	if err := svc.MarkPostAsProcessed("111", "somecreator", "", 1785342602); err != nil {
		t.Fatalf("MarkPostAsProcessed failed: %v", err)
	}
	var legacy models.ProcessedPost
	if err := db.Where("post_id = ?", "111").First(&legacy).Error; err != nil {
		t.Fatalf("failed to load post: %v", err)
	}
	post = legacy
	if post.ContentType != "timeline" {
		t.Errorf("expected default content_type timeline, got %q", post.ContentType)
	}
}
