package repository

import (
	"github.com/agnosto/fansly-scraper/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AccountRepository defines the interface for cached account operations
type AccountRepository interface {
	Upsert(account *models.Account) error
	FindByID(id string) (*models.Account, error)
	FindByIDs(ids []string) ([]models.Account, error)
}

// GormAccountRepository implements AccountRepository using GORM
type GormAccountRepository struct {
	db *gorm.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &GormAccountRepository{db: db}
}

// Upsert inserts or updates an account record keyed by account ID
func (r *GormAccountRepository) Upsert(account *models.Account) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(account).Error
}

// FindByID finds an account by its ID
func (r *GormAccountRepository) FindByID(id string) (*models.Account, error) {
	var account models.Account
	err := r.db.Where("id = ?", id).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// FindByIDs finds all accounts matching any of the given IDs
func (r *GormAccountRepository) FindByIDs(ids []string) ([]models.Account, error) {
	var accounts []models.Account
	if len(ids) == 0 {
		return accounts, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&accounts).Error
	return accounts, err
}
