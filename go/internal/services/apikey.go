package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"ai_gateway/internal/database"
	"ai_gateway/internal/utils"

	"gorm.io/gorm"
)

// APIKeyService handles API key operations
type APIKeyService struct {
	db *gorm.DB
}

// NewAPIKeyService creates a new APIKeyService
func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// APIKeyCreate represents a request to create an API key
type APIKeyCreate struct {
	ProviderConfigIDs   []uint     `json:"provider_config_ids" validate:"required,min=1"`
	Name                string     `json:"name" validate:"required,min=1,max=100"`
	ExpiresAt           *time.Time `json:"expires_at"`
	DailyRequestLimit   *int       `json:"daily_request_limit"`
	MonthlyRequestLimit *int       `json:"monthly_request_limit"`
	DailyTokenLimit     *int       `json:"daily_token_limit"`
	MonthlyTokenLimit   *int       `json:"monthly_token_limit"`
}

// APIKeyUpdate represents a request to update an API key
type APIKeyUpdate struct {
	Name                *string    `json:"name"`
	ExpiresAt           *time.Time `json:"expires_at"`
	IsActive            *bool      `json:"is_active"`
	ProviderConfigIDs   []uint     `json:"provider_config_ids"`
	DailyRequestLimit   *int       `json:"daily_request_limit"`
	MonthlyRequestLimit *int       `json:"monthly_request_limit"`
	DailyTokenLimit     *int       `json:"daily_token_limit"`
	MonthlyTokenLimit   *int       `json:"monthly_token_limit"`
}

// APIKeyUsageStats represents usage statistics for an API key
type APIKeyUsageStats struct {
	DailyRequestsUsed    int        `json:"daily_requests_used"`
	MonthlyRequestsUsed  int        `json:"monthly_requests_used"`
	DailyTokensUsed      int        `json:"daily_tokens_used"`
	MonthlyTokensUsed    int        `json:"monthly_tokens_used"`
	DailyRequestLimit    *int       `json:"daily_request_limit"`
	MonthlyRequestLimit  *int       `json:"monthly_request_limit"`
	DailyTokenLimit      *int       `json:"daily_token_limit"`
	MonthlyTokenLimit    *int       `json:"monthly_token_limit"`
	DailyResetAt         time.Time  `json:"daily_reset_at"`
	MonthlyResetAt       time.Time  `json:"monthly_reset_at"`
	RecentRecords        []database.UsageRecord `json:"recent_records"`
}

// GenerateAPIKey generates a new API key
func (s *APIKeyService) GenerateAPIKey() (fullKey, keyHash, keyPrefix string, err error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}

	// Create the full key with agw_ prefix
	randomPart := base64.URLEncoding.EncodeToString(bytes)[:32]
	fullKey = "agw_" + randomPart

	// Create hash for storage
	keyHash = utils.HashAPIKey(fullKey)

	// Create prefix for display
	keyPrefix = fullKey[:12] + "..."

	return fullKey, keyHash, keyPrefix, nil
}

// CreateAPIKey creates a new API key
func (s *APIKeyService) CreateAPIKey(userID uint, req *APIKeyCreate) (*database.APIKey, string, error) {
	// Verify all provider configs belong to user
	var configs []database.ProviderConfig
	if err := s.db.Where("id IN ? AND user_id = ?", req.ProviderConfigIDs, userID).Find(&configs).Error; err != nil {
		return nil, "", err
	}
	if len(configs) != len(req.ProviderConfigIDs) {
		return nil, "", errors.New("one or more provider configs not found")
	}

	// Generate API key
	fullKey, keyHash, keyPrefix, err := s.GenerateAPIKey()
	if err != nil {
		return nil, "", err
	}

	now := time.Now()

	apiKey := &database.APIKey{
		UserID:              userID,
		Name:                req.Name,
		KeyHash:             keyHash,
		KeyPrefix:           keyPrefix,
		ExpiresAt:           req.ExpiresAt,
		IsActive:            true,
		DailyRequestLimit:   req.DailyRequestLimit,
		MonthlyRequestLimit: req.MonthlyRequestLimit,
		DailyTokenLimit:     req.DailyTokenLimit,
		MonthlyTokenLimit:   req.MonthlyTokenLimit,
		DailyResetAt:        now.Add(24 * time.Hour),
		MonthlyResetAt:      now.AddDate(0, 1, 0),
		ProviderConfigs:     configs,
	}

	if err := s.db.Create(apiKey).Error; err != nil {
		return nil, "", err
	}

	return apiKey, fullKey, nil
}

// GetAPIKeys returns all API keys for a user
func (s *APIKeyService) GetAPIKeys(userID uint) ([]database.APIKey, error) {
	var keys []database.APIKey
	err := s.db.Where("user_id = ?", userID).Preload("ProviderConfigs").Order("created_at DESC").Find(&keys).Error
	return keys, err
}

// GetAPIKeyByID returns an API key by ID
func (s *APIKeyService) GetAPIKeyByID(userID, keyID uint) (*database.APIKey, error) {
	var key database.APIKey
	err := s.db.Where("id = ? AND user_id = ?", keyID, userID).Preload("ProviderConfigs").First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// UpdateAPIKey updates an API key
func (s *APIKeyService) UpdateAPIKey(userID, keyID uint, req *APIKeyUpdate) (*database.APIKey, error) {
	key, err := s.GetAPIKeyByID(userID, keyID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.DailyRequestLimit != nil {
		updates["daily_request_limit"] = *req.DailyRequestLimit
	}
	if req.MonthlyRequestLimit != nil {
		updates["monthly_request_limit"] = *req.MonthlyRequestLimit
	}
	if req.DailyTokenLimit != nil {
		updates["daily_token_limit"] = *req.DailyTokenLimit
	}
	if req.MonthlyTokenLimit != nil {
		updates["monthly_token_limit"] = *req.MonthlyTokenLimit
	}

	if len(updates) > 0 {
		if err := s.db.Model(key).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// Update provider configs if provided
	if len(req.ProviderConfigIDs) > 0 {
		var configs []database.ProviderConfig
		if err := s.db.Where("id IN ? AND user_id = ?", req.ProviderConfigIDs, userID).Find(&configs).Error; err != nil {
			return nil, err
		}
		if len(configs) != len(req.ProviderConfigIDs) {
			return nil, errors.New("one or more provider configs not found")
		}
		if err := s.db.Model(key).Association("ProviderConfigs").Replace(configs); err != nil {
			return nil, err
		}
	}

	return s.GetAPIKeyByID(userID, keyID)
}

// DeleteAPIKey deletes an API key
func (s *APIKeyService) DeleteAPIKey(userID, keyID uint) error {
	result := s.db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&database.APIKey{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("API key not found")
	}
	return nil
}

// ValidateAPIKey validates an API key and returns it if valid
func (s *APIKeyService) ValidateAPIKey(keyHash string) (*database.APIKey, error) {
	var key database.APIKey
	err := s.db.Where("key_hash = ?", keyHash).Preload("ProviderConfigs").First(&key).Error
	if err != nil {
		return nil, err
	}

	if !key.IsActive {
		return nil, errors.New("API key is inactive")
	}

	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("API key has expired")
	}

	return &key, nil
}

// GetProviderConfigForProvider returns the provider config for a specific provider from an API key
func (s *APIKeyService) GetProviderConfigForProvider(apiKey *database.APIKey, provider string) (*database.ProviderConfig, error) {
	for i := range apiKey.ProviderConfigs {
		if apiKey.ProviderConfigs[i].Provider == provider && apiKey.ProviderConfigs[i].IsActive {
			return &apiKey.ProviderConfigs[i], nil
		}
	}
	return nil, errors.New("no configuration found for provider: " + provider)
}

// CheckUsageLimits checks if an API key has exceeded its usage limits
func (s *APIKeyService) CheckUsageLimits(key *database.APIKey) error {
	now := time.Now()

	// Reset daily counters if needed
	if key.DailyResetAt.Before(now) {
		s.db.Model(key).Updates(map[string]interface{}{
			"daily_requests_used": 0,
			"daily_tokens_used":   0,
			"daily_reset_at":      now.Add(24 * time.Hour),
		})
		key.DailyRequestsUsed = 0
		key.DailyTokensUsed = 0
	}

	// Reset monthly counters if needed
	if key.MonthlyResetAt.Before(now) {
		s.db.Model(key).Updates(map[string]interface{}{
			"monthly_requests_used": 0,
			"monthly_tokens_used":   0,
			"monthly_reset_at":      now.AddDate(0, 1, 0),
		})
		key.MonthlyRequestsUsed = 0
		key.MonthlyTokensUsed = 0
	}

	// Check request limits
	if key.DailyRequestLimit != nil && key.DailyRequestsUsed >= *key.DailyRequestLimit {
		return errors.New("daily request limit exceeded")
	}
	if key.MonthlyRequestLimit != nil && key.MonthlyRequestsUsed >= *key.MonthlyRequestLimit {
		return errors.New("monthly request limit exceeded")
	}

	// Check token limits
	if key.DailyTokenLimit != nil && key.DailyTokensUsed >= *key.DailyTokenLimit {
		return errors.New("daily token limit exceeded")
	}
	if key.MonthlyTokenLimit != nil && key.MonthlyTokensUsed >= *key.MonthlyTokenLimit {
		return errors.New("monthly token limit exceeded")
	}

	return nil
}

// RecordUsage records API usage for an API key
func (s *APIKeyService) RecordUsage(keyID uint, endpoint, model string, promptTokens, completionTokens, statusCode int) error {
	totalTokens := promptTokens + completionTokens

	// Create usage record
	record := &database.UsageRecord{
		APIKeyID:         keyID,
		Endpoint:         endpoint,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		StatusCode:       statusCode,
	}

	if err := s.db.Create(record).Error; err != nil {
		return err
	}

	// Update counters
	return s.db.Model(&database.APIKey{}).Where("id = ?", keyID).Updates(map[string]interface{}{
		"daily_requests_used":   gorm.Expr("daily_requests_used + 1"),
		"monthly_requests_used": gorm.Expr("monthly_requests_used + 1"),
		"daily_tokens_used":     gorm.Expr("daily_tokens_used + ?", totalTokens),
		"monthly_tokens_used":   gorm.Expr("monthly_tokens_used + ?", totalTokens),
	}).Error
}

// GetUsageStats returns usage statistics for an API key
func (s *APIKeyService) GetUsageStats(userID, keyID uint) (*APIKeyUsageStats, error) {
	key, err := s.GetAPIKeyByID(userID, keyID)
	if err != nil {
		return nil, err
	}

	// Get recent usage records
	var records []database.UsageRecord
	s.db.Where("api_key_id = ?", keyID).Order("created_at DESC").Limit(100).Find(&records)

	return &APIKeyUsageStats{
		DailyRequestsUsed:   key.DailyRequestsUsed,
		MonthlyRequestsUsed: key.MonthlyRequestsUsed,
		DailyTokensUsed:     key.DailyTokensUsed,
		MonthlyTokensUsed:   key.MonthlyTokensUsed,
		DailyRequestLimit:   key.DailyRequestLimit,
		MonthlyRequestLimit: key.MonthlyRequestLimit,
		DailyTokenLimit:     key.DailyTokenLimit,
		MonthlyTokenLimit:   key.MonthlyTokenLimit,
		DailyResetAt:        key.DailyResetAt,
		MonthlyResetAt:      key.MonthlyResetAt,
		RecentRecords:       records,
	}, nil
}
