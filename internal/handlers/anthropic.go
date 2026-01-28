package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ai_gateway/internal/adapters"
	"ai_gateway/internal/converters"
	"ai_gateway/internal/middleware"
	"ai_gateway/internal/models"

	"github.com/labstack/echo/v4"
)

// AnthropicMessages handles POST /v1/messages
func (h *Handler) AnthropicMessages(c echo.Context) error {
	middleware.LogTrace(c, "Anthropic", "Handling messages request")

	// Log headers
	middleware.LogHeaders(c, "Anthropic")

	// Parse request
	var req models.MessagesRequest
	if err := c.Bind(&req); err != nil {
		middleware.LogTrace(c, "Anthropic", "Failed to parse request body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Log request body
	middleware.LogRequestBody(c, "Anthropic", req)

	middleware.LogTrace(c, "Anthropic", "Parsed request: model=%s, messages=%d, stream=%v", req.Model, len(req.Messages), req.Stream)

	// Determine target provider from model name
	provider := ""
	resolved, err := h.resolveProviderForAPIKey(c, req.Model)
	if err != nil {
		middleware.LogTrace(c, "Anthropic", "Failed to resolve provider: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}
	if resolved != nil {
		c.Set(middleware.ContextKeyProviderConfig, resolved.Config)
		req.Model = resolved.Model
		provider = resolved.Provider
	}
	if provider == "" {
		provider = h.getTargetProvider(c, req.Model)
	}
	if provider == "" {
		middleware.LogTrace(c, "Anthropic", "Unsupported model: %s", req.Model)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported model")
	}

	middleware.LogTrace(c, "Anthropic", "Target provider: %s", provider)

	// Get credentials
	baseURL, apiKey, protocol, err := h.getCredentials(c, provider, req.Model)
	if err != nil {
		middleware.LogTrace(c, "Anthropic", "Failed to get credentials: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	middleware.LogTrace(c, "Anthropic", "Got credentials: baseURL=%s, apiKeyLen=%d, protocol=%s", baseURL, len(apiKey), protocol)

	// Route to appropriate handler
	switch protocol {
	case "anthropic":
		middleware.LogTrace(c, "Anthropic", "Routing to Anthropic handler")
		return h.handleAnthropicToAnthropic(c, &req, baseURL, apiKey)
	case "openai_chat":
		middleware.LogTrace(c, "Anthropic", "Routing to OpenAI chat handler")
		return h.handleAnthropicToOpenAIChat(c, &req, baseURL, apiKey)
	case "openai_code":
		middleware.LogTrace(c, "Anthropic", "Routing to OpenAI responses handler")
		return h.handleAnthropicToOpenAI(c, &req, baseURL, apiKey)
	case "gemini":
		middleware.LogTrace(c, "Anthropic", "Routing to Gemini handler")
		return h.handleAnthropicToGemini(c, &req, baseURL, apiKey)
	default:
		middleware.LogTrace(c, "Anthropic", "Unsupported protocol: %s", protocol)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported protocol")
	}
}

// handleAnthropicToAnthropic forwards request directly to Anthropic
func (h *Handler) handleAnthropicToAnthropic(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "Anthropic->Anthropic", "Creating adapter with baseURL=%s", baseURL)
	adapter := adapters.NewAnthropicAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "Anthropic->Anthropic", "Starting streaming request")
		return h.streamAnthropic(c, adapter, req)
	}

	middleware.LogTrace(c, "Anthropic->Anthropic", "Sending non-streaming request")
	resp, statusCode, err := adapter.Messages(c.Request().Context(), req)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->Anthropic", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "Anthropic->Anthropic", "Received response: statusCode=%d", statusCode)

	// Record usage
	h.recordAnthropicUsage(c, "/v1/messages", req.Model, resp, statusCode)

	return c.JSON(statusCode, resp)
}

// handleAnthropicToOpenAIChat converts and forwards to OpenAI chat completions
func (h *Handler) handleAnthropicToOpenAIChat(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "Anthropic->OpenAIChat", "Converting request to Chat Completions format")
	openaiReq, err := converters.AnthropicToOpenAIRequest(req)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAIChat", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	middleware.LogTrace(c, "Anthropic->OpenAIChat", "Creating adapter with baseURL=%s, model=%s", baseURL, req.Model)
	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "Anthropic->OpenAIChat", "Starting streaming request to /chat/completions")
		return h.streamAnthropicFromOpenAIChat(c, adapter, openaiReq, req.Model)
	}

	middleware.LogTrace(c, "Anthropic->OpenAIChat", "Sending non-streaming request to /chat/completions")
	resp, statusCode, err := adapter.ChatCompletions(c.Request().Context(), openaiReq)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAIChat", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	anthropicResp, err := converters.OpenAIToAnthropicResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAIChat", "Response conversion error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.recordAnthropicUsageFromResp(c, "/v1/messages", req.Model, anthropicResp, statusCode)

	return c.JSON(statusCode, anthropicResp)
}

// handleAnthropicToOpenAI converts and forwards to OpenAI using /responses endpoint
func (h *Handler) handleAnthropicToOpenAI(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "Anthropic->OpenAI", "Converting request to Responses API format")
	// Convert request to OpenAI Responses API format
	openaiReq, err := converters.AnthropicToOpenAIResponsesRequest(req)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAI", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	enforceOpenAIReasoningHigh(openaiReq)

	middleware.LogTrace(c, "Anthropic->OpenAI", "Creating adapter with baseURL=%s, model=%s", baseURL, req.Model)
	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "Anthropic->OpenAI", "Starting streaming request to /responses")
		return h.streamAnthropicFromOpenAIResponses(c, adapter, openaiReq, req.Model)
	}

	middleware.LogTrace(c, "Anthropic->OpenAI", "Sending non-streaming request to /responses")
	resp, statusCode, err := adapter.Responses(c.Request().Context(), openaiReq)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAI", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "Anthropic->OpenAI", "Received response: statusCode=%d, resp=%v", statusCode, resp)

	// Convert response from OpenAI Responses API format
	anthropicResp, err := converters.OpenAIResponsesToAnthropicResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->OpenAI", "Response conversion error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordAnthropicUsageFromResp(c, "/v1/messages", req.Model, anthropicResp, statusCode)

	return c.JSON(statusCode, anthropicResp)
}

// handleAnthropicToGemini converts and forwards to Gemini
func (h *Handler) handleAnthropicToGemini(c echo.Context, req *models.MessagesRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "Anthropic->Gemini", "Converting request")
	// Convert request
	geminiReq, err := converters.AnthropicToGeminiRequest(req)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->Gemini", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	middleware.LogTrace(c, "Anthropic->Gemini", "Creating adapter with baseURL=%s", baseURL)
	adapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "Anthropic->Gemini", "Starting streaming request")
		return h.streamAnthropicFromGemini(c, adapter, geminiReq, req.Model)
	}

	middleware.LogTrace(c, "Anthropic->Gemini", "Sending non-streaming request")
	resp, statusCode, err := adapter.GenerateContent(c.Request().Context(), req.Model, geminiReq)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->Gemini", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "Anthropic->Gemini", "Received response: statusCode=%d", statusCode)

	// Convert response
	anthropicResp, err := converters.GeminiToAnthropicResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "Anthropic->Gemini", "Response conversion error: %v", err)
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

// streamAnthropicFromOpenAIResponses streams and converts OpenAI Responses API response to Anthropic format
func (h *Handler) streamAnthropicFromOpenAIResponses(c echo.Context, adapter *adapters.OpenAIAdapter, req map[string]interface{}, model string) error {
	req["stream"] = true
	stream, statusCode, err := adapter.ResponsesStream(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	middleware.LogTrace(c, "Anthropic->OpenAI", "Starting response stream: statusCode=%d, model=%s", statusCode, model)

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	middleware.LogTrace(c, "Anthropic->OpenAI", "=== Response Headers ===")
	for name, values := range c.Response().Header() {
		for _, value := range values {
			middleware.LogTrace(c, "Anthropic->OpenAI", "  %s: %s", name, value)
		}
	}
	c.Response().WriteHeader(statusCode)

	reader := stream.GetReader()
	state := converters.NewOpenAIToAnthropicStreamState()
	start := time.Now()
	lastProgressLog := start
	var lineCount int
	var dataLineCount int
	var byteCount int
	done := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			middleware.LogTrace(c, "Anthropic->OpenAI", "Stream read error after %s: %v (lines=%d, dataLines=%d, bytes=%d)", time.Since(start), err, lineCount, dataLineCount, byteCount)
			return err
		}

		lineCount++
		byteCount += len(line)

		if time.Since(lastProgressLog) >= 5*time.Second {
			middleware.LogTrace(c, "Anthropic->OpenAI", "Stream progress: elapsed=%s, lines=%d, dataLines=%d, bytes=%d", time.Since(start), lineCount, dataLineCount, byteCount)
			lastProgressLog = time.Now()
		}

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		if strings.HasPrefix(trimmedLine, "data:") {
			dataLineCount++
			data := strings.TrimPrefix(trimmedLine, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				done = true
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			events, err := converters.OpenAIResponsesStreamToAnthropicStream(eventData, isFirst)
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

	endReason := "eof"
	if done {
		endReason = "done"
	}
	middleware.LogTrace(c, "Anthropic->OpenAI", "Stream completed: reason=%s, duration=%s, lines=%d, dataLines=%d, bytes=%d", endReason, time.Since(start), lineCount, dataLineCount, byteCount)

	return nil
}

// streamAnthropicFromOpenAIChat streams and converts OpenAI chat completion response to Anthropic format
func (h *Handler) streamAnthropicFromOpenAIChat(c echo.Context, adapter *adapters.OpenAIAdapter, req *models.ChatCompletionRequest, model string) error {
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

			events, err := converters.OpenAIStreamToAnthropicStream(eventData, state)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("event: message\ndata: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}

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
