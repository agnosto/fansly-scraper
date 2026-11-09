package service

import (
	"time"

	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/db/repository"
	"github.com/agnosto/fansly-scraper/logger"
)

// AccountService handles cached creator account operations
type AccountService struct {
	repo repository.AccountRepository
}

// NewAccountService creates a new account service
func NewAccountService(repo repository.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// SaveAccount upserts a cached account record
func (s *AccountService) SaveAccount(id, username, displayName, avatarURL, bannerURL string) error {
	if id == "" {
		return nil
	}
	account := &models.Account{
		ID:          id,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		BannerURL:   bannerURL,
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Upsert(account); err != nil {
		logger.Logger.Printf("Error caching account %s: %v", id, err)
		return err
	}
	return nil
}

// GetCachedUsername returns the cached username for an account ID, or ""
func (s *AccountService) GetCachedUsername(id string) string {
	account, err := s.repo.FindByID(id)
	if err != nil || account == nil {
		return ""
	}
	return account.Username
}

// GetCachedUsernames returns the cached usernames for the given account IDs.
// IDs without a cached (non-empty) username are absent from the map.
func (s *AccountService) GetCachedUsernames(ids []string) map[string]string {
	result := make(map[string]string)
	accounts, err := s.repo.FindByIDs(ids)
	if err != nil {
		logger.Logger.Printf("Error loading cached accounts: %v", err)
		return result
	}
	for _, account := range accounts {
		if account.Username != "" {
			result[account.ID] = account.Username
		}
	}
	return result
}
