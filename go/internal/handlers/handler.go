package handlers

import (
	"ai_gateway/internal/config"
	"ai_gateway/internal/services"

	"gorm.io/gorm"
)

// Handler contains all route handlers
type Handler struct {
	db            *gorm.DB
	cfg           *config.Config
	authService   *services.AuthService
	configService *services.ConfigService
	apiKeyService *services.APIKeyService
}

// New creates a new Handler instance
func New(db *gorm.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:            db,
		cfg:           cfg,
		authService:   services.NewAuthService(db, cfg),
		configService: services.NewConfigService(db, cfg),
		apiKeyService: services.NewAPIKeyService(db),
	}
}
