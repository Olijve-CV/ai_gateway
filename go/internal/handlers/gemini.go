package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ai_gateway/internal/adapters"
	"ai_gateway/internal/converters"
	"ai_gateway/internal/middleware"
	"ai_gateway/internal/models"

	"github.com/labstack/echo/v4"
)

// GeminiGenerateContent handles POST /v1/models/:model
func (h *Handler) GeminiGenerateContent(c echo.Context) error {
	// Get model from path (format: model:generateContent)
	modelPath := c.Param("model")
	model := strings.TrimSuffix(modelPath, ":generateContent")
	model = strings.TrimSuffix(model, ":streamGenerateContent")

	// Check for streaming via query param
	isStream := c.QueryParam("alt") == "sse"

	// Parse request
	var req models.GenerateContentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Determine target provider from model name
	provider := getTargetProvider(model)
	if provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported model")
	}

	// Get credentials
	baseURL, apiKey, protocol, err := h.getCredentials(c, provider)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	// Route to appropriate handler
	switch protocol {
	case "gemini":
		return h.handleGeminiToGemini(c, &req, model, baseURL, apiKey, isStream)
	case "openai_chat":
		return h.handleGeminiToOpenAI(c, &req, model, baseURL, apiKey, isStream)
	case "openai_code":
		return h.handleGeminiToOpenAIResponses(c, &req, model, baseURL, apiKey, isStream)
	case "anthropic":
		return h.handleGeminiToAnthropic(c, &req, model, baseURL, apiKey, isStream)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported protocol")
	}
}

// handleGeminiToGemini forwards request directly to Gemini
func (h *Handler) handleGeminiToGemini(c echo.Context, req *models.GenerateContentRequest, model, baseURL, apiKey string, isStream bool) error {
	adapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	if isStream {
		return h.streamGemini(c, adapter, req, model)
	}

	resp, statusCode, err := adapter.GenerateContent(c.Request().Context(), model, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Record usage
	h.recordGeminiUsage(c, "/v1/models/"+model, model, resp, statusCode)

	return c.JSON(statusCode, resp)
}

// handleGeminiToOpenAI converts and forwards to OpenAI
func (h *Handler) handleGeminiToOpenAI(c echo.Context, req *models.GenerateContentRequest, model, baseURL, apiKey string, isStream bool) error {
	// Convert request
	openaiReq, err := converters.GeminiToOpenAIRequest(req, model)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if isStream {
		return h.streamGeminiFromOpenAI(c, adapter, openaiReq, model)
	}

	resp, statusCode, err := adapter.ChatCompletions(c.Request().Context(), openaiReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	geminiResp, err := converters.OpenAIToGeminiResponse(resp)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordGeminiUsageFromResp(c, "/v1/models/"+model, model, geminiResp, statusCode)

	return c.JSON(statusCode, geminiResp)
}

// handleGeminiToOpenAIResponses converts and forwards to OpenAI Responses API
func (h *Handler) handleGeminiToOpenAIResponses(c echo.Context, req *models.GenerateContentRequest, model, baseURL, apiKey string, isStream bool) error {
	openaiChatReq, err := converters.GeminiToOpenAIRequest(req, model)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	openaiResponsesReq, err := converters.OpenAIChatToOpenAIResponsesRequest(openaiChatReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	enforceOpenAIReasoningHigh(openaiResponsesReq)

	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if isStream {
		return h.streamGeminiFromOpenAIResponses(c, adapter, openaiResponsesReq, model)
	}

	resp, statusCode, err := adapter.Responses(c.Request().Context(), openaiResponsesReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	chatResp, err := converters.OpenAIResponsesToOpenAIChatResponse(resp, model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	chatRespMap, err := converters.ChatCompletionResponseToMap(chatResp)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	geminiResp, err := converters.OpenAIToGeminiResponse(chatRespMap)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.recordGeminiUsageFromResp(c, "/v1/models/"+model, model, geminiResp, statusCode)

	return c.JSON(statusCode, geminiResp)
}

// handleGeminiToAnthropic converts and forwards to Anthropic
func (h *Handler) handleGeminiToAnthropic(c echo.Context, req *models.GenerateContentRequest, model, baseURL, apiKey string, isStream bool) error {
	// Convert request
	anthropicReq, err := converters.GeminiToAnthropicRequest(req, model)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewAnthropicAdapter(apiKey, baseURL)

	if isStream {
		return h.streamGeminiFromAnthropic(c, adapter, anthropicReq, model)
	}

	resp, statusCode, err := adapter.Messages(c.Request().Context(), anthropicReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	// Convert response
	geminiResp, err := converters.AnthropicToGeminiResponse(resp)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordGeminiUsageFromResp(c, "/v1/models/"+model, model, geminiResp, statusCode)

	return c.JSON(statusCode, geminiResp)
}

// streamGemini streams response from Gemini
func (h *Handler) streamGemini(c echo.Context, adapter *adapters.GeminiAdapter, req *models.GenerateContentRequest, model string) error {
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

// streamGeminiFromOpenAI streams and converts OpenAI response to Gemini format
func (h *Handler) streamGeminiFromOpenAI(c echo.Context, adapter *adapters.OpenAIAdapter, req *models.ChatCompletionRequest, model string) error {
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

			chunk, err := converters.OpenAIStreamToGeminiStream(eventData)
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

// streamGeminiFromOpenAIResponses streams and converts OpenAI Responses stream to Gemini format
func (h *Handler) streamGeminiFromOpenAIResponses(c echo.Context, adapter *adapters.OpenAIAdapter, req map[string]interface{}, model string) error {
	req["stream"] = true
	stream, statusCode, err := adapter.ResponsesStream(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(statusCode)

	reader := stream.GetReader()
	state := converters.NewOpenAIResponsesToChatStreamState(model)

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

			chunks, err := converters.OpenAIResponsesStreamToOpenAIChatStream(eventData, state)
			if err != nil {
				continue
			}

			for _, chunk := range chunks {
				var chatEvent map[string]interface{}
				if err := json.Unmarshal(chunk, &chatEvent); err != nil {
					continue
				}

				geminiChunk, err := converters.OpenAIStreamToGeminiStream(chatEvent)
				if err != nil || geminiChunk == nil {
					continue
				}

				c.Response().Write([]byte("data: "))
				c.Response().Write(geminiChunk)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}
		}
	}

	c.Response().Write([]byte("data: [DONE]\n\n"))
	c.Response().Flush()

	return nil
}

// streamGeminiFromAnthropic streams and converts Anthropic response to Gemini format
func (h *Handler) streamGeminiFromAnthropic(c echo.Context, adapter *adapters.AnthropicAdapter, req *models.MessagesRequest, model string) error {
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
				break
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			eventType, _ := eventData["type"].(string)
			chunk, err := converters.AnthropicStreamToGeminiStream(eventType, eventData)
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

// recordGeminiUsage records usage from Gemini response
func (h *Handler) recordGeminiUsage(c echo.Context, endpoint, model string, resp map[string]interface{}, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	var promptTokens, completionTokens int
	if usage, ok := resp["usageMetadata"].(map[string]interface{}); ok {
		if pt, ok := usage["promptTokenCount"].(float64); ok {
			promptTokens = int(pt)
		}
		if ct, ok := usage["candidatesTokenCount"].(float64); ok {
			completionTokens = int(ct)
		}
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, promptTokens, completionTokens, statusCode)
}

// recordGeminiUsageFromResp records usage from Gemini response struct
func (h *Handler) recordGeminiUsageFromResp(c echo.Context, endpoint, model string, resp *models.GenerateContentResponse, statusCode int) {
	apiKey := middleware.GetAPIKey(c)
	if apiKey == nil {
		return
	}

	var promptTokens, completionTokens int
	if resp.UsageMetadata != nil {
		promptTokens = resp.UsageMetadata.PromptTokenCount
		completionTokens = resp.UsageMetadata.CandidatesTokenCount
	}

	h.apiKeyService.RecordUsage(apiKey.ID, endpoint, model, promptTokens, completionTokens, statusCode)
}
