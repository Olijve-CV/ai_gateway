package handlers

import (
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

// AnthropicMessages handles POST /v1/messages
func (h *Handler) AnthropicMessages(c echo.Context) error {
	// Parse request
	var req models.MessagesRequest
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
	case "anthropic":
		return h.handleAnthropicToAnthropic(c, &req, baseURL, apiKey)
	case "openai":
		return h.handleAnthropicToOpenAI(c, &req, baseURL, apiKey)
	case "gemini":
		return h.handleAnthropicToGemini(c, &req, baseURL, apiKey)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported provider")
	}
}

// handleAnthropicToAnthropic forwards request directly to Anthropic
func (h *Handler) handleAnthropicToAnthropic(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	adapter := adapters.NewAnthropicAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamAnthropic(c, adapter, req)
	}

	resp, statusCode, err := adapter.Messages(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Record usage
	h.recordAnthropicUsage(c, "/v1/messages", req.Model, resp, statusCode)

	return c.JSON(statusCode, resp)
}

// handleAnthropicToOpenAI converts and forwards to OpenAI
func (h *Handler) handleAnthropicToOpenAI(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	// Convert request
	openaiReq, err := converters.AnthropicToOpenAIRequest(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamAnthropicFromOpenAI(c, adapter, openaiReq, req.Model)
	}

	resp, statusCode, err := adapter.ChatCompletions(c.Request().Context(), openaiReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	anthropicResp, err := converters.OpenAIToAnthropicResponse(resp, req.Model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordAnthropicUsageFromResp(c, "/v1/messages", req.Model, anthropicResp, statusCode)

	return c.JSON(statusCode, anthropicResp)
}

// handleAnthropicToGemini converts and forwards to Gemini
func (h *Handler) handleAnthropicToGemini(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	// Convert request
	geminiReq, err := converters.AnthropicToGeminiRequest(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	if req.Stream {
		return h.streamAnthropicFromGemini(c, adapter, geminiReq, req.Model)
	}

	resp, statusCode, err := adapter.GenerateContent(c.Request().Context(), req.Model, geminiReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	anthropicResp, err := converters.GeminiToAnthropicResponse(resp, req.Model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordAnthropicUsageFromResp(c, "/v1/messages", req.Model, anthropicResp, statusCode)

	return c.JSON(statusCode, anthropicResp)
}

// streamAnthropic streams response from Anthropic
func (h *Handler) streamAnthropic(c echo.Context, adapter *adapters.AnthropicAdapter, req *models.MessagesRequest) error {
	stream, statusCode, err := adapter.MessagesStream(c.Request().Context(), req)
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
	}

	return nil
}

// streamAnthropicFromOpenAI streams and converts OpenAI response to Anthropic format
func (h *Handler) streamAnthropicFromOpenAI(c echo.Context, adapter *adapters.OpenAIAdapter, req *models.ChatCompletionRequest, model string) error {
	req.Stream = true
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
	isFirst := true

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

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			events, err := converters.OpenAIStreamToAnthropicStream(eventData, isFirst)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("event: message\ndata: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}

			isFirst = false
		}
	}

	return nil
}

// streamAnthropicFromGemini streams and converts Gemini response to Anthropic format
func (h *Handler) streamAnthropicFromGemini(c echo.Context, adapter *adapters.GeminiAdapter, req *models.GenerateContentRequest, model string) error {
	stream, statusCode, err := adapter.GenerateContentStream(c.Request().Context(), model, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(statusCode)

	reader := stream.GetReader()
	isFirst := true

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
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			events, err := converters.GeminiStreamToAnthropicStream(eventData, isFirst, model)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("event: message\ndata: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}

			isFirst = false
		}
	}

	return nil
}

// recordAnthropicUsage records usage from Anthropic response
func (h *Handler) recordAnthropicUsage(c echo.Context, endpoint, model string, resp map[string]interface{}, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	var inputTokens, outputTokens int
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if it, ok := usage["input_tokens"].(float64); ok {
			inputTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok {
			outputTokens = int(ot)
		}
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, inputTokens, outputTokens, statusCode)
}

// recordAnthropicUsageFromResp records usage from Anthropic response struct
func (h *Handler) recordAnthropicUsageFromResp(c echo.Context, endpoint, model string, resp *models.MessagesResponse, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, resp.Usage.InputTokens, resp.Usage.OutputTokens, statusCode)
}
