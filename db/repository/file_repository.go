package repository

import (
	"github.com/agnosto/fansly-scraper/db/models"
	"gorm.io/gorm"
)

// FileRepository defines the interface for file operations
type FileRepository interface {
	Create(file *models.File) error
	FindByHash(hash string) (*models.File, error)
	FindByPath(path string) (*models.File, error)
	Exists(path string) (bool, error)
	FindByModel(model string) ([]models.File, error)
	FindByModelAndType(model, fileType string) ([]models.File, error)
}

// GormFileRepository implements FileRepository using GORM
type GormFileRepository struct {
	db *gorm.DB
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *gorm.DB) FileRepository {
	return &GormFileRepository{db: db}
}

// Create adds a new file to the database
func (r *GormFileRepository) Create(file *models.File) error {
	return r.db.Create(file).Error
}

// FindByHash finds a file by its hash
func (r *GormFileRepository) FindByHash(hash string) (*models.File, error) {
	var file models.File
	err := r.db.Where("hash = ?", hash).First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// FindByPath finds a file by its path
func (r *GormFileRepository) FindByPath(path string) (*models.File, error) {
	var file models.File
	err := r.db.Where("path = ?", path).First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// Exists checks if a file exists in the database by path
func (r *GormFileRepository) Exists(path string) (bool, error) {
	var count int64
	err := r.db.Model(&models.File{}).Where("path = ?", path).Count(&count).Error
	return count > 0, err
}

// FindByModel finds all files for a specific model
func (r *GormFileRepository) FindByModel(model string) ([]models.File, error) {
	var files []models.File
	err := r.db.Where("model = ?", model).Find(&files).Error
	return files, err
}

// FindByModelAndType finds all files for a specific model and file type
func (r *GormFileRepository) FindByModelAndType(model, fileType string) ([]models.File, error) {
	var files []models.File
	err := r.db.Where("model = ? AND file_type = ?", model, fileType).Find(&files).Error
	return files, err
}
