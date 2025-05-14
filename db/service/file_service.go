package service

import (
	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/logger"
)

// FileService handles file-related operations
type FileService struct {
	repo repository.FileRepository
}

// NewFileService creates a new file service
func NewFileService(repo repository.FileRepository) *FileService {
	return &FileService{repo: repo}
}

// SaveFile saves a file to the database
func (s *FileService) SaveFile(model, hash, path, fileType string) error {
	file := &models.File{
		Model:    model,
		Hash:     hash,
		Path:     path,
		FileType: fileType,
	}

	return s.repo.Create(file)
}

// FileExists checks if a file exists in the database
func (s *FileService) FileExists(path string) bool {
	exists, err := s.repo.Exists(path)
	if err != nil {
		logger.Logger.Printf("Error checking if file exists: %v", err)
		return false
	}
	return exists
}

// GetFilesByModel retrieves all files for a specific model
func (s *FileService) GetFilesByModel(model string) ([]models.File, error) {
	return s.repo.FindByModel(model)
}

// GetFilesByModelAndType retrieves all files of a specific type for a model
func (s *FileService) GetFilesByModelAndType(model, fileType string) ([]models.File, error) {
	return s.repo.FindByModelAndType(model, fileType)
}
