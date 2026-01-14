package handlers

import (
	"net/http"
	"strconv"
	"time"

	"ai_gateway/internal/database"
	"ai_gateway/internal/middleware"
	"ai_gateway/internal/services"

	"github.com/labstack/echo/v4"
)

// ProviderConfigInfo represents provider config info in API response
type ProviderConfigInfo struct {
	ID       uint   `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

// APIKeyCreateRequest represents an API key creation request
type APIKeyCreateRequest struct {
	ProviderConfigIDs   []uint     `json:"provider_config_ids"`
	Name                string     `json:"name"`
	ExpiresAt           *time.Time `json:"expires_at"`
	DailyRequestLimit   *int       `json:"daily_request_limit"`
	MonthlyRequestLimit *int       `json:"monthly_request_limit"`
	DailyTokenLimit     *int       `json:"daily_token_limit"`
	MonthlyTokenLimit   *int       `json:"monthly_token_limit"`
}

// APIKeyUpdateRequest represents an API key update request
type APIKeyUpdateRequest struct {
	Name                *string    `json:"name"`
	ExpiresAt           *time.Time `json:"expires_at"`
	IsActive            *bool      `json:"is_active"`
	ProviderConfigIDs   []uint     `json:"provider_config_ids"`
	DailyRequestLimit   *int       `json:"daily_request_limit"`
	MonthlyRequestLimit *int       `json:"monthly_request_limit"`
	DailyTokenLimit     *int       `json:"daily_token_limit"`
	MonthlyTokenLimit   *int       `json:"monthly_token_limit"`
}

// APIKeyResponse represents an API key response
type APIKeyResponse struct {
	ID                  uint                 `json:"id"`
	Name                string               `json:"name"`
	KeyPrefix           string               `json:"key_prefix"`
	ProviderConfigs     []ProviderConfigInfo `json:"provider_configs"`
	ExpiresAt           *time.Time           `json:"expires_at"`
	IsActive            bool                 `json:"is_active"`
	DailyRequestLimit   *int                 `json:"daily_request_limit"`
	MonthlyRequestLimit *int                 `json:"monthly_request_limit"`
	DailyTokenLimit     *int                 `json:"daily_token_limit"`
	MonthlyTokenLimit   *int                 `json:"monthly_token_limit"`
	DailyRequestsUsed   int                  `json:"daily_requests_used"`
	MonthlyRequestsUsed int                  `json:"monthly_requests_used"`
	DailyTokensUsed     int                  `json:"daily_tokens_used"`
	MonthlyTokensUsed   int                  `json:"monthly_tokens_used"`
	CreatedAt           time.Time            `json:"created_at"`
}

// APIKeyCreateResponse includes the full key (only shown once)
type APIKeyCreateResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

// toProviderConfigInfos converts database ProviderConfigs to ProviderConfigInfo slice
func toProviderConfigInfos(configs []database.ProviderConfig) []ProviderConfigInfo {
	result := make([]ProviderConfigInfo, len(configs))
	for i, cfg := range configs {
		result[i] = ProviderConfigInfo{
			ID:       cfg.ID,
			Provider: cfg.Provider,
			Name:     cfg.Name,
		}
	}
	return result
}

// toAPIKeyResponse converts database APIKey to APIKeyResponse
func toAPIKeyResponse(key *database.APIKey) APIKeyResponse {
	return APIKeyResponse{
		ID:                  key.ID,
		Name:                key.Name,
		KeyPrefix:           key.KeyPrefix,
		ProviderConfigs:     toProviderConfigInfos(key.ProviderConfigs),
		ExpiresAt:           key.ExpiresAt,
		IsActive:            key.IsActive,
		DailyRequestLimit:   key.DailyRequestLimit,
		MonthlyRequestLimit: key.MonthlyRequestLimit,
		DailyTokenLimit:     key.DailyTokenLimit,
		MonthlyTokenLimit:   key.MonthlyTokenLimit,
		DailyRequestsUsed:   key.DailyRequestsUsed,
		MonthlyRequestsUsed: key.MonthlyRequestsUsed,
		DailyTokensUsed:     key.DailyTokensUsed,
		MonthlyTokensUsed:   key.MonthlyTokensUsed,
		CreatedAt:           key.CreatedAt,
	}
}

// ListAPIKeys returns all API keys for the current user
func (h *Handler) ListAPIKeys(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	keys, err := h.apiKeyService.GetAPIKeys(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var response []APIKeyResponse
	for _, key := range keys {
		response = append(response, toAPIKeyResponse(&key))
	}

	return c.JSON(http.StatusOK, response)
}

// CreateAPIKey creates a new API key
func (h *Handler) CreateAPIKey(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req APIKeyCreateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if len(req.ProviderConfigIDs) == 0 || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider_config_ids and name are required")
	}

	serviceReq := &services.APIKeyCreate{
		ProviderConfigIDs:   req.ProviderConfigIDs,
		Name:                req.Name,
		ExpiresAt:           req.ExpiresAt,
		DailyRequestLimit:   req.DailyRequestLimit,
		MonthlyRequestLimit: req.MonthlyRequestLimit,
		DailyTokenLimit:     req.DailyTokenLimit,
		MonthlyTokenLimit:   req.MonthlyTokenLimit,
	}

	key, fullKey, err := h.apiKeyService.CreateAPIKey(user.ID, serviceReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, APIKeyCreateResponse{
		APIKeyResponse: toAPIKeyResponse(key),
		Key:            fullKey,
	})
}

// GetAPIKey returns an API key by ID
func (h *Handler) GetAPIKey(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid key ID")
	}

	key, err := h.apiKeyService.GetAPIKeyByID(user.ID, uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "API key not found")
	}

	return c.JSON(http.StatusOK, toAPIKeyResponse(key))
}

// UpdateAPIKey updates an API key
func (h *Handler) UpdateAPIKey(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid key ID")
	}

	var req APIKeyUpdateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	serviceReq := &services.APIKeyUpdate{
		Name:                req.Name,
		ExpiresAt:           req.ExpiresAt,
		IsActive:            req.IsActive,
		ProviderConfigIDs:   req.ProviderConfigIDs,
		DailyRequestLimit:   req.DailyRequestLimit,
		MonthlyRequestLimit: req.MonthlyRequestLimit,
		DailyTokenLimit:     req.DailyTokenLimit,
		MonthlyTokenLimit:   req.MonthlyTokenLimit,
	}

	key, err := h.apiKeyService.UpdateAPIKey(user.ID, uint(id), serviceReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, toAPIKeyResponse(key))
}

// DeleteAPIKey deletes an API key
func (h *Handler) DeleteAPIKey(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid key ID")
	}

	if err := h.apiKeyService.DeleteAPIKey(user.ID, uint(id)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// GetAPIKeyUsage returns usage statistics for an API key
func (h *Handler) GetAPIKeyUsage(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid key ID")
	}

	stats, err := h.apiKeyService.GetUsageStats(user.ID, uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, stats)
}
