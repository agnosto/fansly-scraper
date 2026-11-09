package service

import (
	"fmt"
	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/logger"
	"time"
)

// ProcessedPostService handles post-related operations
type ProcessedPostService struct {
	repo repository.ProcessedPostRepository
}

// NewProcessedPostService creates a new post service
func NewProcessedPostService(repo repository.ProcessedPostRepository) *ProcessedPostService {
	return &ProcessedPostService{repo: repo}
}

// MarkPostAsProcessed saves a post ID to the database
func (s *ProcessedPostService) MarkPostAsProcessed(postID, modelUsername string, content string, postCreatedAt int64) error {
	return s.MarkPostAsProcessedFull(postID, "", modelUsername, content, "timeline", postCreatedAt)
}

// MarkPostAsProcessedFull saves a post to the database with its creator
// account ID and content type (timeline, message, story, purchases, livestream)
func (s *ProcessedPostService) MarkPostAsProcessedFull(postID, accountID, modelUsername, content, contentType string, postCreatedAt int64) error {
	if contentType == "" {
		contentType = "timeline"
	}
	post := &models.ProcessedPost{
		PostID:        postID,
		AccountID:     accountID,
		ModelUsername: modelUsername,
		Content:       content,
		ContentType:   contentType,
		Link:          fmt.Sprintf("https://fansly.com/post/%s", postID),
		PostCreatedAt: time.Unix(postCreatedAt, 0),
	}
	//return s.repo.Create(post)
	return s.repo.Upsert(post)
}

// PostExists checks if a post has already been processed
func (s *ProcessedPostService) PostExists(postID string) bool {
	exists, err := s.repo.ExistsByPostID(postID)
	if err != nil {
		logger.Logger.Printf("Error checking if post exists: %v", err)
		return false // Fail-safe: attempt to download if DB check fails
	}
	return exists
}
