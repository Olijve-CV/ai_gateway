package handlers

import (
	"net/http"
	"strconv"

	"ai_gateway/internal/middleware"
	"ai_gateway/internal/services"

	"github.com/labstack/echo/v4"
)

// ProviderConfigRequest represents a provider config create/update request
type ProviderConfigRequest struct {
	Provider string  `json:"provider"`
	Name     string  `json:"name"`
	BaseURL  *string `json:"base_url"`
	Protocol *string `json:"protocol"`
	APIKey   *string `json:"api_key"`
}

// ProviderConfigResponse represents a provider config response
type ProviderConfigResponse struct {
	ID        uint   `json:"id"`
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Protocol  string `json:"protocol"`
	KeyHint   string `json:"key_hint"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

// GetProviderConfigs returns all provider configs for the current user
func (h *Handler) GetProviderConfigs(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	configs, err := h.configService.GetConfigs(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var response []ProviderConfigResponse
	for _, cfg := range configs {
		response = append(response, ProviderConfigResponse{
			ID:        cfg.ID,
			Provider:  cfg.Provider,
			Name:      cfg.Name,
			BaseURL:   cfg.BaseURL,
			Protocol:  normalizeProtocol(cfg.Protocol),
			KeyHint:   cfg.KeyHint,
			IsDefault: cfg.IsDefault,
			IsActive:  cfg.IsActive,
		})
	}

	return c.JSON(http.StatusOK, response)
}

// GetProviderConfigsByProvider returns provider configs by provider type
func (h *Handler) GetProviderConfigsByProvider(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	provider := c.Param("provider")
	configs, err := h.configService.GetConfigsByProvider(user.ID, provider)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var response []ProviderConfigResponse
	for _, cfg := range configs {
		response = append(response, ProviderConfigResponse{
			ID:        cfg.ID,
			Provider:  cfg.Provider,
			Name:      cfg.Name,
			BaseURL:   cfg.BaseURL,
			Protocol:  normalizeProtocol(cfg.Protocol),
			KeyHint:   cfg.KeyHint,
			IsDefault: cfg.IsDefault,
			IsActive:  cfg.IsActive,
		})
	}

	return c.JSON(http.StatusOK, response)
}

// GetProviderConfigByID returns a provider config by ID
func (h *Handler) GetProviderConfigByID(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid config ID")
	}

	cfg, err := h.configService.GetConfigByID(user.ID, uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "config not found")
	}

	return c.JSON(http.StatusOK, ProviderConfigResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		Name:      cfg.Name,
		BaseURL:   cfg.BaseURL,
		Protocol:  normalizeProtocol(cfg.Protocol),
		KeyHint:   cfg.KeyHint,
		IsDefault: cfg.IsDefault,
		IsActive:  cfg.IsActive,
	})
}

// CreateProviderConfig creates a new provider config
func (h *Handler) CreateProviderConfig(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req ProviderConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Provider == "" || req.Name == "" || req.APIKey == nil || *req.APIKey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider, name, and api_key are required")
	}

	baseURL := ""
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}

	serviceReq := &services.ProviderConfigCreate{
		Provider: req.Provider,
		Name:     req.Name,
		BaseURL:  baseURL,
		Protocol: protocolValue(req.Protocol),
		APIKey:   *req.APIKey,
	}

	cfg, err := h.configService.CreateConfig(user.ID, serviceReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, ProviderConfigResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		Name:      cfg.Name,
		BaseURL:   cfg.BaseURL,
		Protocol:  normalizeProtocol(cfg.Protocol),
		KeyHint:   cfg.KeyHint,
		IsDefault: cfg.IsDefault,
		IsActive:  cfg.IsActive,
	})
}

// UpdateProviderConfig updates a provider config
func (h *Handler) UpdateProviderConfig(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid config ID")
	}

	var req ProviderConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	serviceReq := &services.ProviderConfigUpdate{
		Name:    &req.Name,
		BaseURL: req.BaseURL,
		Protocol: req.Protocol,
		APIKey:  req.APIKey,
	}

	cfg, err := h.configService.UpdateConfig(user.ID, uint(id), serviceReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, ProviderConfigResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		Name:      cfg.Name,
		BaseURL:   cfg.BaseURL,
		Protocol:  normalizeProtocol(cfg.Protocol),
		KeyHint:   cfg.KeyHint,
		IsDefault: cfg.IsDefault,
		IsActive:  cfg.IsActive,
	})
}

// DeleteProviderConfig deletes a provider config
func (h *Handler) DeleteProviderConfig(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid config ID")
	}

	if err := h.configService.DeleteConfig(user.ID, uint(id)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// SetDefaultProviderConfig sets a provider config as default
func (h *Handler) SetDefaultProviderConfig(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid config ID")
	}

	cfg, err := h.configService.SetDefault(user.ID, uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, ProviderConfigResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		Name:      cfg.Name,
		BaseURL:   cfg.BaseURL,
		Protocol:  normalizeProtocol(cfg.Protocol),
		KeyHint:   cfg.KeyHint,
		IsDefault: cfg.IsDefault,
		IsActive:  cfg.IsActive,
	})
}

// ToggleProviderConfig toggles the active status of a provider config
func (h *Handler) ToggleProviderConfig(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid config ID")
	}

	cfg, err := h.configService.ToggleActive(user.ID, uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, ProviderConfigResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		Name:      cfg.Name,
		BaseURL:   cfg.BaseURL,
		Protocol:  normalizeProtocol(cfg.Protocol),
		KeyHint:   cfg.KeyHint,
		IsDefault: cfg.IsDefault,
		IsActive:  cfg.IsActive,
	})
}
