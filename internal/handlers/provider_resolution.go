package handlers

import (
	"fmt"

	"ai_gateway/internal/database"
	"ai_gateway/internal/middleware"

	"github.com/labstack/echo/v4"
)

type resolvedProvider struct {
	Provider string
	Model    string
	Config   *database.ProviderConfig
	Matched  bool
}

func (h *Handler) resolveProviderForAPIKey(c echo.Context, model string) (*resolvedProvider, error) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return nil, nil
	}

	middleware.LogTrace(c, "ResolveProvider", "Resolving provider for API key model=%s", model)

	if len(apiKey.ProviderConfigs) == 0 {
		return nil, fmt.Errorf("API key has no provider configs")
	}

	var firstActive *database.ProviderConfig

	for i := range apiKey.ProviderConfigs {
		cfg := &apiKey.ProviderConfigs[i]
		if !cfg.IsActive {
			continue
		}
		if firstActive == nil {
			firstActive = cfg
		}

		modelCodes, err := h.configService.GetModelCodes(cfg)
		if err != nil {
			middleware.LogTrace(c, "ResolveProvider", "Failed to get model codes for config %d: %v", cfg.ID, err)
			continue
		}

		for _, modelCode := range modelCodes {
			if modelCode == model {
				middleware.LogTrace(c, "ResolveProvider", "Matched model=%s to config ID=%d Provider=%s", model, cfg.ID, cfg.Provider)
				return &resolvedProvider{
					Provider: cfg.Provider,
					Model:    model,
					Config:   cfg,
					Matched:  true,
				}, nil
			}
		}
	}

	if firstActive == nil {
		return nil, fmt.Errorf("API key has no active provider configs")
	}

	resolvedModel := model
	modelCodes, err := h.configService.GetModelCodes(firstActive)
	if err != nil {
		middleware.LogTrace(c, "ResolveProvider", "Failed to get model codes for default config %d: %v", firstActive.ID, err)
	} else if len(modelCodes) > 0 {
		resolvedModel = modelCodes[0]
	} else {
		middleware.LogTrace(c, "ResolveProvider", "Default config %d has no model codes; keeping model=%s", firstActive.ID, model)
	}

	if resolvedModel != model {
		middleware.LogTrace(c, "ResolveProvider", "No model match for %s; defaulting to config ID=%d Provider=%s model=%s", model, firstActive.ID, firstActive.Provider, resolvedModel)
	} else {
		middleware.LogTrace(c, "ResolveProvider", "No model match for %s; defaulting to config ID=%d Provider=%s", model, firstActive.ID, firstActive.Provider)
	}

	return &resolvedProvider{
		Provider: firstActive.Provider,
		Model:    resolvedModel,
		Config:   firstActive,
		Matched:  false,
	}, nil
}
