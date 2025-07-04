package repository

import (
	"github.com/agnosto/fansly-scraper/db/models"
	"gorm.io/gorm"
)

// ProcessedPostRepository defines the interface for post operations
type ProcessedPostRepository interface {
	Create(post *models.ProcessedPost) error
	ExistsByPostID(postID string) (bool, error)
}

// GormProcessedPostRepository implements ProcessedPostRepository using GORM
type GormProcessedPostRepository struct {
	db *gorm.DB
}

// NewProcessedPostRepository creates a new post repository
func NewProcessedPostRepository(db *gorm.DB) ProcessedPostRepository {
	return &GormProcessedPostRepository{db: db}
}

// Create adds a new processed post to the database
func (r *GormProcessedPostRepository) Create(post *models.ProcessedPost) error {
	return r.db.Create(post).Error
}

// ExistsByPostID checks if a post exists in the database by its PostID
func (r *GormProcessedPostRepository) ExistsByPostID(postID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ProcessedPost{}).Where("post_id = ?", postID).Count(&count).Error
	return count > 0, err
}
