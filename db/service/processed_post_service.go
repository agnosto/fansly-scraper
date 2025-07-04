package service

import (
	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/logger"
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
func (s *ProcessedPostService) MarkPostAsProcessed(postID, modelUsername string) error {
	post := &models.ProcessedPost{
		PostID:        postID,
		ModelUsername: modelUsername,
	}
	return s.repo.Create(post)
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
