package database

import (
	"time"
)

// User represents a user account
type User struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	Username        string           `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Email           string           `gorm:"uniqueIndex;size:100;not null" json:"email"`
	HashedPassword  string           `gorm:"size:100;not null" json:"-"`
	IsActive        bool             `gorm:"default:true" json:"is_active"`
	IsAdmin         bool             `gorm:"default:false" json:"is_admin"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ProviderConfigs []ProviderConfig `gorm:"foreignKey:UserID" json:"-"`
	APIKeys         []APIKey         `gorm:"foreignKey:UserID" json:"-"`
}

// ProviderConfig represents a user's provider configuration
type ProviderConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Provider     string    `gorm:"size:20;index;not null" json:"provider"` // openai, anthropic, gemini, custom
	Protocol     string    `gorm:"size:20;default:openai_chat" json:"protocol"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	BaseURL      string    `gorm:"size:255" json:"base_url"`
	EncryptedKey string    `gorm:"size:500;not null" json:"-"`
	KeyHint      string    `gorm:"size:20" json:"key_hint"`
	ModelCodes   string    `gorm:"type:text" json:"model_codes"` // JSON array of model codes, comma-separated
	IsDefault    bool      `gorm:"default:false" json:"is_default"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	User         User      `gorm:"foreignKey:UserID" json:"-"`
	APIKeys      []APIKey  `gorm:"many2many:api_key_providers;" json:"-"`
}

// APIKey represents a gateway-issued API key
type APIKey struct {
	ID                  uint             `gorm:"primaryKey" json:"id"`
	UserID              uint             `gorm:"index;not null" json:"user_id"`
	Name                string           `gorm:"size:100;not null" json:"name"`
	KeyHash             string           `gorm:"uniqueIndex;size:64;not null" json:"-"`
	KeyPrefix           string           `gorm:"size:20;not null" json:"key_prefix"`
	ExpiresAt           *time.Time       `json:"expires_at"`
	IsActive            bool             `gorm:"default:true" json:"is_active"`
	DailyRequestLimit   *int             `json:"daily_request_limit"`
	MonthlyRequestLimit *int             `json:"monthly_request_limit"`
	DailyTokenLimit     *int             `json:"daily_token_limit"`
	MonthlyTokenLimit   *int             `json:"monthly_token_limit"`
	DailyRequestsUsed   int              `gorm:"default:0" json:"daily_requests_used"`
	MonthlyRequestsUsed int              `gorm:"default:0" json:"monthly_requests_used"`
	DailyTokensUsed     int              `gorm:"default:0" json:"daily_tokens_used"`
	MonthlyTokensUsed   int              `gorm:"default:0" json:"monthly_tokens_used"`
	DailyResetAt        time.Time        `json:"daily_reset_at"`
	MonthlyResetAt      time.Time        `json:"monthly_reset_at"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
	User                User             `gorm:"foreignKey:UserID" json:"-"`
	ProviderConfigs     []ProviderConfig `gorm:"many2many:api_key_providers;" json:"-"`
	UsageRecords        []UsageRecord    `gorm:"foreignKey:APIKeyID" json:"-"`
}

// UsageRecord represents an API usage record
type UsageRecord struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	APIKeyID         uint      `gorm:"index;not null" json:"api_key_id"`
	Endpoint         string    `gorm:"size:100" json:"endpoint"`
	Model            string    `gorm:"size:50" json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	StatusCode       int       `json:"status_code"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
	APIKey           APIKey    `gorm:"foreignKey:APIKeyID" json:"-"`
}

// TableName overrides the table name for User
func (User) TableName() string {
	return "users"
}

// TableName overrides the table name for ProviderConfig
func (ProviderConfig) TableName() string {
	return "provider_configs"
}

// TableName overrides the table name for APIKey
func (APIKey) TableName() string {
	return "api_keys"
}

// TableName overrides the table name for UsageRecord
func (UsageRecord) TableName() string {
	return "usage_records"
}
