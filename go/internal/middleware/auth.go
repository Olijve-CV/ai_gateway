package middleware

import (
	"net/http"
	"strings"
	"time"

	"ai_gateway/internal/config"
	"ai_gateway/internal/database"
	"ai_gateway/internal/utils"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	ContextKeyUser           = "user"
	ContextKeyAPIKey         = "api_key"
	ContextKeyProviderConfig = "provider_config"
)

// AuthResult contains the authentication result
type AuthResult struct {
	User           *database.User
	APIKey         *database.APIKey
	ProviderConfig *database.ProviderConfig
}

// JWTAuth is a middleware that validates JWT tokens
func JWTAuth(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			// Extract token from "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
			}

			token := parts[1]

			// Skip if it's an API key (starts with agw_)
			if strings.HasPrefix(token, "agw_") {
				return echo.NewHTTPError(http.StatusUnauthorized, "API key not allowed for this endpoint")
			}

			// Decode JWT token
			claims, err := utils.DecodeAccessToken(token, cfg.JWTSecret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// Get user from database
			db := c.Get("db").(*gorm.DB)
			var user database.User
			if err := db.First(&user, claims.UserID).Error; err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
			}

			if !user.IsActive {
				return echo.NewHTTPError(http.StatusUnauthorized, "user is inactive")
			}

			c.Set(ContextKeyUser, &user)
			return next(c)
		}
	}
}

// GatewayAuth is a middleware that validates both API keys and JWT tokens
func GatewayAuth(db *gorm.DB, cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Store db in context for other middleware/handlers
			c.Set("db", db)

			// Try to get API key from headers
			apiKeyStr := extractAPIKey(c)

			if apiKeyStr != "" && strings.HasPrefix(apiKeyStr, "agw_") {
				// API Key authentication
				return authenticateWithAPIKey(c, db, cfg, apiKeyStr, next)
			}

			// Try JWT authentication
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token := parts[1]
					if !strings.HasPrefix(token, "agw_") {
						return authenticateWithJWT(c, db, cfg, token, next)
					}
				}
			}

			return echo.NewHTTPError(http.StatusUnauthorized, "missing or invalid authentication")
		}
	}
}

// extractAPIKey extracts the API key from request headers
func extractAPIKey(c echo.Context) string {
	// Try X-API-Key header first
	apiKey := c.Request().Header.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Try Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	return ""
}

// authenticateWithAPIKey authenticates using an API key
func authenticateWithAPIKey(c echo.Context, db *gorm.DB, cfg *config.Config, apiKeyStr string, next echo.HandlerFunc) error {
	keyHash := utils.HashAPIKey(apiKeyStr)

	var apiKey database.APIKey
	if err := db.Preload("User").Preload("ProviderConfig").Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid API key")
	}

	if !apiKey.IsActive {
		return echo.NewHTTPError(http.StatusUnauthorized, "API key is inactive")
	}

	// Check expiration
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "API key has expired")
	}

	c.Set(ContextKeyUser, &apiKey.User)
	c.Set(ContextKeyAPIKey, &apiKey)
	c.Set(ContextKeyProviderConfig, &apiKey.ProviderConfig)

	return next(c)
}

// authenticateWithJWT authenticates using a JWT token
func authenticateWithJWT(c echo.Context, db *gorm.DB, cfg *config.Config, token string, next echo.HandlerFunc) error {
	claims, err := utils.DecodeAccessToken(token, cfg.JWTSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
	}

	var user database.User
	if err := db.First(&user, claims.UserID).Error; err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
	}

	if !user.IsActive {
		return echo.NewHTTPError(http.StatusUnauthorized, "user is inactive")
	}

	c.Set(ContextKeyUser, &user)

	return next(c)
}

// DBMiddleware injects the database into the context
func DBMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("db", db)
			return next(c)
		}
	}
}

// GetUser gets the user from context
func GetUser(c echo.Context) *database.User {
	user, ok := c.Get(ContextKeyUser).(*database.User)
	if !ok {
		return nil
	}
	return user
}

// GetAPIKey gets the API key from context
func GetAPIKey(c echo.Context) *database.APIKey {
	apiKey, ok := c.Get(ContextKeyAPIKey).(*database.APIKey)
	if !ok {
		return nil
	}
	return apiKey
}

// GetProviderConfig gets the provider config from context
func GetProviderConfig(c echo.Context) *database.ProviderConfig {
	cfg, ok := c.Get(ContextKeyProviderConfig).(*database.ProviderConfig)
	if !ok {
		return nil
	}
	return cfg
}
