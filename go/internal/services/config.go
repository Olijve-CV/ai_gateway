package services

import (
	"errors"
	"log"

	"ai_gateway/internal/config"
	"ai_gateway/internal/database"
	"ai_gateway/internal/utils"

	"gorm.io/gorm"
)

// ConfigService handles provider configuration operations
type ConfigService struct {
	db  *gorm.DB
	cfg *config.Config
}

// NewConfigService creates a new ConfigService
func NewConfigService(db *gorm.DB, cfg *config.Config) *ConfigService {
	return &ConfigService{db: db, cfg: cfg}
}

// ProviderConfigCreate represents a request to create a provider config
type ProviderConfigCreate struct {
	Provider string `json:"provider" validate:"required,oneof=openai anthropic gemini"`
	Name     string `json:"name" validate:"required,min=1,max=100"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key" validate:"required"`
}

// ProviderConfigUpdate represents a request to update a provider config
type ProviderConfigUpdate struct {
	Name    *string `json:"name"`
	BaseURL *string `json:"base_url"`
	APIKey  *string `json:"api_key"`
}

// GetConfigs returns all provider configs for a user
func (s *ConfigService) GetConfigs(userID uint) ([]database.ProviderConfig, error) {
	var configs []database.ProviderConfig
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&configs).Error
	return configs, err
}

// GetConfigsByProvider returns provider configs by provider type
func (s *ConfigService) GetConfigsByProvider(userID uint, provider string) ([]database.ProviderConfig, error) {
	var configs []database.ProviderConfig
	err := s.db.Where("user_id = ? AND provider = ?", userID, provider).Order("created_at DESC").Find(&configs).Error
	return configs, err
}

// GetConfigByID returns a provider config by ID
func (s *ConfigService) GetConfigByID(userID, configID uint) (*database.ProviderConfig, error) {
	var cfg database.ProviderConfig
	err := s.db.Where("id = ? AND user_id = ?", configID, userID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// CreateConfig creates a new provider config
func (s *ConfigService) CreateConfig(userID uint, req *ProviderConfigCreate) (*database.ProviderConfig, error) {
	// Get encryption key
	encKey, err := s.cfg.GetEncryptionKeyBytes()
	if err != nil {
		return nil, err
	}

	// Encrypt API key
	encryptedKey, err := utils.EncryptAPIKey(req.APIKey, encKey)
	if err != nil {
		return nil, err
	}

	// Set default base URL if not provided
	baseURL := req.BaseURL
	if baseURL == "" {
		switch req.Provider {
		case "openai":
			baseURL = s.cfg.OpenAIBaseURL
		case "anthropic":
			baseURL = s.cfg.AnthropicBaseURL
		case "gemini":
			baseURL = s.cfg.GeminiBaseURL
		}
	}

	// Check if this is the first config for this provider (make it default)
	var count int64
	s.db.Model(&database.ProviderConfig{}).Where("user_id = ? AND provider = ?", userID, req.Provider).Count(&count)
	isDefault := count == 0

	cfg := &database.ProviderConfig{
		UserID:       userID,
		Provider:     req.Provider,
		Name:         req.Name,
		BaseURL:      baseURL,
		EncryptedKey: encryptedKey,
		KeyHint:      utils.GetAPIKeyHint(req.APIKey),
		IsDefault:    isDefault,
		IsActive:     true,
	}

	if err := s.db.Create(cfg).Error; err != nil {
		return nil, err
	}

	return cfg, nil
}

// UpdateConfig updates a provider config
func (s *ConfigService) UpdateConfig(userID, configID uint, req *ProviderConfigUpdate) (*database.ProviderConfig, error) {
	cfg, err := s.GetConfigByID(userID, configID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.Name != nil {
		updates["name"] = *req.Name
	}

	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}

	if req.APIKey != nil {
		encKey, err := s.cfg.GetEncryptionKeyBytes()
		if err != nil {
			return nil, err
		}
		encryptedKey, err := utils.EncryptAPIKey(*req.APIKey, encKey)
		if err != nil {
			return nil, err
		}
		updates["encrypted_key"] = encryptedKey
		updates["key_hint"] = utils.GetAPIKeyHint(*req.APIKey)
	}

	if len(updates) > 0 {
		if err := s.db.Model(cfg).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetConfigByID(userID, configID)
}

// DeleteConfig deletes a provider config
func (s *ConfigService) DeleteConfig(userID, configID uint) error {
	result := s.db.Where("id = ? AND user_id = ?", configID, userID).Delete(&database.ProviderConfig{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("config not found")
	}
	return nil
}

// SetDefault sets a config as the default for its provider
func (s *ConfigService) SetDefault(userID, configID uint) (*database.ProviderConfig, error) {
	cfg, err := s.GetConfigByID(userID, configID)
	if err != nil {
		return nil, err
	}

	// Unset other defaults for this provider
	s.db.Model(&database.ProviderConfig{}).
		Where("user_id = ? AND provider = ? AND id != ?", userID, cfg.Provider, configID).
		Update("is_default", false)

	// Set this as default
	s.db.Model(cfg).Update("is_default", true)

	return s.GetConfigByID(userID, configID)
}

// ToggleActive toggles the active status of a config
func (s *ConfigService) ToggleActive(userID, configID uint) (*database.ProviderConfig, error) {
	cfg, err := s.GetConfigByID(userID, configID)
	if err != nil {
		return nil, err
	}

	s.db.Model(cfg).Update("is_active", !cfg.IsActive)

	return s.GetConfigByID(userID, configID)
}

// GetDefaultConfig returns the default config for a provider
func (s *ConfigService) GetDefaultConfig(userID uint, provider string) (*database.ProviderConfig, error) {
	var cfg database.ProviderConfig
	err := s.db.Where("user_id = ? AND provider = ? AND is_default = ? AND is_active = ?", userID, provider, true, true).First(&cfg).Error
	if err != nil {
		// Try to get any active config for this provider
		err = s.db.Where("user_id = ? AND provider = ? AND is_active = ?", userID, provider, true).First(&cfg).Error
		if err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// DecryptAPIKey decrypts the API key from a provider config
func (s *ConfigService) DecryptAPIKey(cfg *database.ProviderConfig) (string, error) {
	encKey, err := s.cfg.GetEncryptionKeyBytes()
	if err != nil {
		log.Printf("[DECRYPT] Failed to get encryption key bytes: %v", err)
		return "", err
	}
	log.Printf("[DECRYPT] ENCRYPTION_KEY (base64): %s", s.cfg.EncryptionKey)
	log.Printf("[DECRYPT] EncryptedKey from DB: %s", cfg.EncryptedKey)
	log.Printf("[DECRYPT] EncKey bytes length: %d", len(encKey))

	result, err := utils.DecryptAPIKey(cfg.EncryptedKey, encKey)
	if err != nil {
		log.Printf("[DECRYPT] Decryption failed: %v", err)
		return "", err
	}
	log.Printf("[DECRYPT] Decryption successful, key length: %d", len(result))
	return result, nil
}
