package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai_gateway/internal/adapters"
	"ai_gateway/internal/converters"
	"ai_gateway/internal/middleware"
	"ai_gateway/internal/models"

	"github.com/labstack/echo/v4"
)

// OpenAIChatCompletions handles POST /v1/chat/completions
func (h *Handler) OpenAIChatCompletions(c echo.Context) error {
	// Parse request
	var req models.ChatCompletionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Determine target provider from model name
	provider := getTargetProvider(req.Model)
	if provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported model")
	}

	// Get credentials
	baseURL, apiKey, err := h.getCredentials(c, provider)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	// Route to appropriate handler
	switch provider {
	case "openai":
		return h.handleOpenAIToOpenAI(c, &req, baseURL, apiKey)
	case "anthropic":
		return h.handleOpenAIToAnthropic(c, &req, baseURL, apiKey)
	case "gemini":
		return h.handleOpenAIToGemini(c, &req, baseURL, apiKey)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported provider")
	}
}

// handleOpenAIToOpenAI forwards request directly to OpenAI
func (h *Handler) handleOpenAIToOpenAI(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamOpenAI(c, adapter, req)
	}

	resp, statusCode, err := adapter.ChatCompletions(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Record usage
	h.recordUsage(c, "/v1/chat/completions", req.Model, resp, statusCode)

	return c.JSON(statusCode, resp)
}

// handleOpenAIToAnthropic converts and forwards to Anthropic
func (h *Handler) handleOpenAIToAnthropic(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	// Convert request
	anthropicReq, err := converters.OpenAIToAnthropicRequest(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewAnthropicAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamOpenAIFromAnthropic(c, adapter, anthropicReq, req.Model)
	}

	resp, statusCode, err := adapter.Messages(c.Request().Context(), anthropicReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	openaiResp, err := converters.AnthropicToOpenAIResponse(resp, req.Model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordUsageFromOpenAI(c, "/v1/chat/completions", req.Model, openaiResp, statusCode)

	return c.JSON(statusCode, openaiResp)
}

// handleOpenAIToGemini converts and forwards to Gemini
func (h *Handler) handleOpenAIToGemini(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	// Convert request
	geminiReq, err := converters.OpenAIToGeminiRequest(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamOpenAIFromGemini(c, adapter, geminiReq, req.Model)
	}

	resp, statusCode, err := adapter.GenerateContent(c.Request().Context(), req.Model, geminiReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	openaiResp, err := converters.GeminiToOpenAIResponse(resp, req.Model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordUsageFromOpenAI(c, "/v1/chat/completions", req.Model, openaiResp, statusCode)

	return c.JSON(statusCode, openaiResp)
}

// streamOpenAI streams response from OpenAI
func (h *Handler) streamOpenAI(c echo.Context, adapter *adapters.OpenAIAdapter, req *models.ChatCompletionRequest) error {
	stream, statusCode, err := adapter.ChatCompletionsStream(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(statusCode)

	reader := stream.GetReader()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		c.Response().Write([]byte(line))
		c.Response().Flush()

		if strings.HasPrefix(line, "data: [DONE]") {
			break
		}
	}

	return nil
}

// streamOpenAIFromAnthropic streams and converts Anthropic response to OpenAI format
func (h *Handler) streamOpenAIFromAnthropic(c echo.Context, adapter *adapters.AnthropicAdapter, req *models.MessagesRequest, model string) error {
	req.Stream = true
	stream, statusCode, err := adapter.MessagesStream(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(statusCode)

	id := fmt.Sprintf("chatcmpl-%d", c.Request().Context().Err())
	reader := stream.GetReader()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				c.Response().Write([]byte("data: [DONE]\n\n"))
				c.Response().Flush()
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			eventType, _ := eventData["type"].(string)
			chunk, err := converters.AnthropicStreamToOpenAIStream(eventType, eventData, model, id)
			if err != nil || chunk == nil {
				continue
			}

			c.Response().Write([]byte("data: "))
			c.Response().Write(chunk)
			c.Response().Write([]byte("\n\n"))
			c.Response().Flush()
		}
	}

	return nil
}

// streamOpenAIFromGemini streams and converts Gemini response to OpenAI format
func (h *Handler) streamOpenAIFromGemini(c echo.Context, adapter *adapters.GeminiAdapter, req *models.GenerateContentRequest, model string) error {
	stream, statusCode, err := adapter.GenerateContentStream(c.Request().Context(), model, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(statusCode)

	id := fmt.Sprintf("chatcmpl-%d", c.Request().Context().Err())
	reader := stream.GetReader()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				c.Response().Write([]byte("data: [DONE]\n\n"))
				c.Response().Flush()
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			chunk, err := converters.GeminiStreamToOpenAIStream(eventData, model, id)
			if err != nil || chunk == nil {
				continue
			}

			c.Response().Write([]byte("data: "))
			c.Response().Write(chunk)
			c.Response().Write([]byte("\n\n"))
			c.Response().Flush()
		}
	}

	c.Response().Write([]byte("data: [DONE]\n\n"))
	c.Response().Flush()

	return nil
}

// getTargetProvider determines the target provider from model name
func getTargetProvider(model string) string {
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-") || strings.HasPrefix(model, "o3-") {
		return "openai"
	}
	if strings.HasPrefix(model, "claude-") {
		return "anthropic"
	}
	if strings.HasPrefix(model, "gemini-") {
		return "gemini"
	}
	return ""
}

// getCredentials gets the API credentials for the target provider
func (h *Handler) getCredentials(c echo.Context, provider string) (baseURL, apiKey string, err error) {
	// Check if using API key auth (has provider config in context)
	providerCfg := middleware.GetProviderConfig(c)
	if providerCfg != nil {
		// Verify provider matches
		if providerCfg.Provider != provider {
			return "", "", fmt.Errorf("API key is configured for %s, but model requires %s", providerCfg.Provider, provider)
		}
		apiKey, err = h.configService.DecryptAPIKey(providerCfg)
		if err != nil {
			return "", "", err
		}
		return providerCfg.BaseURL, apiKey, nil
	}

	// JWT auth - get default config for provider
	user := middleware.GetUser(c)
	if user == nil {
		return "", "", fmt.Errorf("not authenticated")
	}

	cfg, err := h.configService.GetDefaultConfig(user.ID, provider)
	if err != nil {
		return "", "", fmt.Errorf("no %s configuration found", provider)
	}

	apiKey, err = h.configService.DecryptAPIKey(cfg)
	if err != nil {
		return "", "", err
	}

	return cfg.BaseURL, apiKey, nil
}

// recordUsage records API usage
func (h *Handler) recordUsage(c echo.Context, endpoint, model string, resp map[string]interface{}, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	var promptTokens, completionTokens int
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			completionTokens = int(ct)
		}
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, promptTokens, completionTokens, statusCode)
}

// recordUsageFromOpenAI records usage from OpenAI response
func (h *Handler) recordUsageFromOpenAI(c echo.Context, endpoint, model string, resp *models.ChatCompletionResponse, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	var promptTokens, completionTokens int
	if resp.Usage != nil {
		promptTokens = resp.Usage.PromptTokens
		completionTokens = resp.Usage.CompletionTokens
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, promptTokens, completionTokens, statusCode)
}

// Helper to read SSE stream
func readSSEStream(reader *bufio.Reader) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			ch <- line
		}
	}()
	return ch
}
