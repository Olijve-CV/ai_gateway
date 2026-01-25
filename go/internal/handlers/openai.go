package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai_gateway/internal/adapters"
	"ai_gateway/internal/converters"
	"ai_gateway/internal/database"
	"ai_gateway/internal/middleware"
	"ai_gateway/internal/models"

	"github.com/labstack/echo/v4"
)

// OpenAIChatCompletions handles POST /v1/chat/completions
func (h *Handler) OpenAIChatCompletions(c echo.Context) error {
	middleware.LogTrace(c, "OpenAI", "Handling chat completions request")

	// Log headers
	middleware.LogHeaders(c, "OpenAI")

	// Parse request
	var req models.ChatCompletionRequest
	if err := c.Bind(&req); err != nil {
		middleware.LogTrace(c, "OpenAI", "Failed to parse request body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Log request body
	middleware.LogRequestBody(c, "OpenAI", req)

	middleware.LogTrace(c, "OpenAI", "Parsed request: model=%s, messages=%d, stream=%v", req.Model, len(req.Messages), req.Stream)

	// Determine target provider from model name
	provider := getTargetProvider(req.Model)
	if provider == "" {
		middleware.LogTrace(c, "OpenAI", "Unsupported model: %s", req.Model)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported model")
	}

	middleware.LogTrace(c, "OpenAI", "Target provider: %s", provider)

	// Get credentials
	baseURL, apiKey, protocol, err := h.getCredentials(c, provider)
	if err != nil {
		middleware.LogTrace(c, "OpenAI", "Failed to get credentials: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	middleware.LogTrace(c, "OpenAI", "Got credentials: baseURL=%s, apiKeyLen=%d, protocol=%s", baseURL, len(apiKey), protocol)

	// Route to appropriate handler
	switch protocol {
	case "openai_chat":
		middleware.LogTrace(c, "OpenAI", "Routing to OpenAI chat handler")
		return h.handleOpenAIToOpenAI(c, &req, baseURL, apiKey)
	case "openai_code":
		middleware.LogTrace(c, "OpenAI", "Routing to OpenAI responses handler")
		return h.handleOpenAIToOpenAIResponses(c, &req, baseURL, apiKey)
	case "anthropic":
		middleware.LogTrace(c, "OpenAI", "Routing to Anthropic handler")
		return h.handleOpenAIToAnthropic(c, &req, baseURL, apiKey)
	case "gemini":
		middleware.LogTrace(c, "OpenAI", "Routing to Gemini handler")
		return h.handleOpenAIToGemini(c, &req, baseURL, apiKey)
	default:
		middleware.LogTrace(c, "OpenAI", "Unsupported protocol: %s", protocol)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported protocol")
	}
}

// OpenAICodeResponses handles POST /v1/responses - forwards directly to OpenAI
func (h *Handler) OpenAICodeResponses(c echo.Context) error {
	middleware.LogTrace(c, "OpenAI-Responses", "Handling responses request")

	// Log headers
	middleware.LogHeaders(c, "OpenAI-Responses")

	// Parse request body as generic map (to preserve all fields)
	var reqBody map[string]interface{}
	if err := c.Bind(&reqBody); err != nil {
		middleware.LogTrace(c, "OpenAI-Responses", "Failed to parse request body: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Log request body
	middleware.LogRequestBody(c, "OpenAI-Responses", reqBody)

	// Get model from request
	model, _ := reqBody["model"].(string)
	middleware.LogTrace(c, "OpenAI-Responses", "Parsed request: model=%s", model)

	// Determine target provider from model name
	provider := getTargetProvider(model)
	if provider == "" {
		middleware.LogTrace(c, "OpenAI-Responses", "Unsupported model: %s", model)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported model")
	}

	// Get credentials for provider
	baseURL, apiKey, protocol, err := h.getCredentials(c, provider)
	if err != nil {
		middleware.LogTrace(c, "OpenAI-Responses", "Failed to get credentials: %v", err)
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	middleware.LogTrace(c, "OpenAI-Responses", "Got credentials: baseURL=%s, apiKeyLen=%d, protocol=%s", baseURL, len(apiKey), protocol)

	// Create adapters
	openaiAdapter := adapters.NewOpenAIAdapter(apiKey, baseURL)
	anthropicAdapter := adapters.NewAnthropicAdapter(apiKey, baseURL)
	geminiAdapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	// Check if streaming
	stream, _ := reqBody["stream"].(bool)
	switch protocol {
	case "openai_code":
		if stream {
			middleware.LogTrace(c, "OpenAI-Responses", "Starting streaming request")
			return h.streamResponses(c, openaiAdapter, reqBody)
		}

		middleware.LogTrace(c, "OpenAI-Responses", "Sending non-streaming request")
		resp, statusCode, err := openaiAdapter.Responses(c.Request().Context(), reqBody)
		if err != nil {
			middleware.LogTrace(c, "OpenAI-Responses", "Upstream error: %v", err)
			return echo.NewHTTPError(http.StatusBadGateway, err.Error())
		}

		middleware.LogTrace(c, "OpenAI-Responses", "Received response: statusCode=%d", statusCode)

		// Record usage
		h.recordUsage(c, "/v1/responses", model, resp, statusCode)

		return c.JSON(statusCode, resp)
	case "openai_chat":
		middleware.LogTrace(c, "OpenAI-Responses", "Converting request to chat completions")
		chatReq, err := converters.OpenAIResponsesToOpenAIChatRequest(reqBody)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if stream {
			middleware.LogTrace(c, "OpenAI-Responses", "Starting streaming chat request")
			return h.streamResponsesFromOpenAIChat(c, openaiAdapter, chatReq, model)
		}

		middleware.LogTrace(c, "OpenAI-Responses", "Sending non-streaming chat request")
		chatRespMap, statusCode, err := openaiAdapter.ChatCompletions(c.Request().Context(), chatReq)
		if err != nil {
			middleware.LogTrace(c, "OpenAI-Responses", "Upstream error: %v", err)
			return echo.NewHTTPError(http.StatusBadGateway, err.Error())
		}

		resp, err := converters.OpenAIChatMapToOpenAIResponsesResponse(chatRespMap, model)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		// Record usage
		h.recordUsage(c, "/v1/responses", model, resp, statusCode)

		return c.JSON(statusCode, resp)
	case "anthropic":
		middleware.LogTrace(c, "OpenAI-Responses", "Converting request to Anthropic")
		chatReq, err := converters.OpenAIResponsesToOpenAIChatRequest(reqBody)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		anthropicReq, err := converters.OpenAIToAnthropicRequest(chatReq)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if stream {
			middleware.LogTrace(c, "OpenAI-Responses", "Starting streaming Anthropic request")
			return h.streamResponsesFromAnthropic(c, anthropicAdapter, anthropicReq, model)
		}

		respMap, statusCode, err := anthropicAdapter.Messages(c.Request().Context(), anthropicReq)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, err.Error())
		}

		chatResp, err := converters.AnthropicToOpenAIResponse(respMap, model)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		resp, err := converters.OpenAIChatResponseToOpenAIResponsesResponse(chatResp)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		h.recordUsage(c, "/v1/responses", model, resp, statusCode)

		return c.JSON(statusCode, resp)
	case "gemini":
		middleware.LogTrace(c, "OpenAI-Responses", "Converting request to Gemini")
		chatReq, err := converters.OpenAIResponsesToOpenAIChatRequest(reqBody)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		geminiReq, err := converters.OpenAIToGeminiRequest(chatReq)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if stream {
			middleware.LogTrace(c, "OpenAI-Responses", "Starting streaming Gemini request")
			return h.streamResponsesFromGemini(c, geminiAdapter, geminiReq, model)
		}

		respMap, statusCode, err := geminiAdapter.GenerateContent(c.Request().Context(), model, geminiReq)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, err.Error())
		}

		chatResp, err := converters.GeminiToOpenAIResponse(respMap, model)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		resp, err := converters.OpenAIChatResponseToOpenAIResponsesResponse(chatResp)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		h.recordUsage(c, "/v1/responses", model, resp, statusCode)

		return c.JSON(statusCode, resp)
	default:
		middleware.LogTrace(c, "OpenAI-Responses", "Unsupported protocol: %s", protocol)
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported protocol")
	}
}

// streamResponses streams response from OpenAI /v1/responses
func (h *Handler) streamResponses(c echo.Context, adapter *adapters.OpenAIAdapter, req map[string]interface{}) error {
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

// handleOpenAIToOpenAI forwards request directly to OpenAI
func (h *Handler) handleOpenAIToOpenAI(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "OpenAI->OpenAI", "Creating adapter with baseURL=%s", baseURL)
	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "OpenAI->OpenAI", "Starting streaming request")
		return h.streamOpenAI(c, adapter, req)
	}

	middleware.LogTrace(c, "OpenAI->OpenAI", "Sending non-streaming request")
	resp, statusCode, err := adapter.ChatCompletions(c.Request().Context(), req)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->OpenAI", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "OpenAI->OpenAI", "Received response: statusCode=%d", statusCode)

	// Record usage
	h.recordUsage(c, "/v1/chat/completions", req.Model, resp, statusCode)

	return c.JSON(statusCode, resp)
}

// handleOpenAIToOpenAIResponses converts and forwards to OpenAI /responses endpoint
func (h *Handler) handleOpenAIToOpenAIResponses(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Converting request to Responses API format")
	responsesReq, err := converters.OpenAIChatToOpenAIResponsesRequest(req)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	adapter := adapters.NewOpenAIAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Starting streaming request")
		return h.streamOpenAIFromOpenAIResponses(c, adapter, responsesReq, req.Model)
	}

	middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Sending non-streaming request")
	resp, statusCode, err := adapter.Responses(c.Request().Context(), responsesReq)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	openaiResp, err := converters.OpenAIResponsesToOpenAIChatResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->OpenAIResponses", "Response conversion error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.recordUsageFromOpenAI(c, "/v1/chat/completions", req.Model, openaiResp, statusCode)

	return c.JSON(statusCode, openaiResp)
}

// handleOpenAIToAnthropic converts and forwards to Anthropic
func (h *Handler) handleOpenAIToAnthropic(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "OpenAI->Anthropic", "Converting request")
	// Convert request
	anthropicReq, err := converters.OpenAIToAnthropicRequest(req)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Anthropic", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	middleware.LogTrace(c, "OpenAI->Anthropic", "Creating adapter with baseURL=%s", baseURL)
	adapter := adapters.NewAnthropicAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "OpenAI->Anthropic", "Starting streaming request")
		return h.streamOpenAIFromAnthropic(c, adapter, anthropicReq, req.Model)
	}

	middleware.LogTrace(c, "OpenAI->Anthropic", "Sending non-streaming request")
	resp, statusCode, err := adapter.Messages(c.Request().Context(), anthropicReq)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Anthropic", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "OpenAI->Anthropic", "Received response: statusCode=%d", statusCode)

	// Convert response
	openaiResp, err := converters.AnthropicToOpenAIResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Anthropic", "Response conversion error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Record usage
	h.recordUsageFromOpenAI(c, "/v1/chat/completions", req.Model, openaiResp, statusCode)

	return c.JSON(statusCode, openaiResp)
}

// handleOpenAIToGemini converts and forwards to Gemini
func (h *Handler) handleOpenAIToGemini(c echo.Context, req *models.ChatCompletionRequest, baseURL, apiKey string) error {
	middleware.LogTrace(c, "OpenAI->Gemini", "Converting request")
	// Convert request
	geminiReq, err := converters.OpenAIToGeminiRequest(req)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Gemini", "Conversion error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	middleware.LogTrace(c, "OpenAI->Gemini", "Creating adapter with baseURL=%s", baseURL)
	adapter := adapters.NewGeminiAdapter(apiKey, baseURL)

	if req.Stream {
		middleware.LogTrace(c, "OpenAI->Gemini", "Starting streaming request")
		return h.streamOpenAIFromGemini(c, adapter, geminiReq, req.Model)
	}

	middleware.LogTrace(c, "OpenAI->Gemini", "Sending non-streaming request")
	resp, statusCode, err := adapter.GenerateContent(c.Request().Context(), req.Model, geminiReq)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Gemini", "Upstream error: %v", err)
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	middleware.LogTrace(c, "OpenAI->Gemini", "Received response: statusCode=%d", statusCode)

	// Convert response
	openaiResp, err := converters.GeminiToOpenAIResponse(resp, req.Model)
	if err != nil {
		middleware.LogTrace(c, "OpenAI->Gemini", "Response conversion error: %v", err)
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

// streamOpenAIFromOpenAIResponses streams and converts OpenAI Responses stream to chat completion format
func (h *Handler) streamOpenAIFromOpenAIResponses(c echo.Context, adapter *adapters.OpenAIAdapter, req map[string]interface{}, model string) error {
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
				c.Response().Write([]byte("data: "))
				c.Response().Write(chunk)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}
		}
	}

	c.Response().Write([]byte("data: [DONE]\n\n"))
	c.Response().Flush()

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

// streamResponsesFromOpenAIChat streams and converts OpenAI chat stream to Responses format
func (h *Handler) streamResponsesFromOpenAIChat(c echo.Context, adapter *adapters.OpenAIAdapter, req *models.ChatCompletionRequest, model string) error {
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
	state := converters.NewOpenAIChatToResponsesStreamState(model)

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

			var chunk models.ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			events, err := converters.OpenAIChatStreamToOpenAIResponsesStream(&chunk, state)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("data: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}
		}
	}

	c.Response().Write([]byte("data: [DONE]\n\n"))
	c.Response().Flush()

	return nil
}

// streamResponsesFromAnthropic streams and converts Anthropic stream to OpenAI Responses format
func (h *Handler) streamResponsesFromAnthropic(c echo.Context, adapter *adapters.AnthropicAdapter, req *models.MessagesRequest, model string) error {
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
	state := converters.NewOpenAIChatToResponsesStreamState(model)
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

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
			chunkBytes, err := converters.AnthropicStreamToOpenAIStream(eventType, eventData, model, id)
			if err != nil || chunkBytes == nil {
				continue
			}

			var chunk models.ChatCompletionChunk
			if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
				continue
			}

			events, err := converters.OpenAIChatStreamToOpenAIResponsesStream(&chunk, state)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("data: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}
		}
	}

	c.Response().Write([]byte("data: [DONE]\n\n"))
	c.Response().Flush()

	return nil
}

// streamResponsesFromGemini streams and converts Gemini stream to OpenAI Responses format
func (h *Handler) streamResponsesFromGemini(c echo.Context, adapter *adapters.GeminiAdapter, req *models.GenerateContentRequest, model string) error {
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
	state := converters.NewOpenAIChatToResponsesStreamState(model)
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

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

			chunkBytes, err := converters.GeminiStreamToOpenAIStream(eventData, model, id)
			if err != nil || chunkBytes == nil {
				continue
			}

			var chunk models.ChatCompletionChunk
			if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
				continue
			}

			events, err := converters.OpenAIChatStreamToOpenAIResponsesStream(&chunk, state)
			if err != nil {
				continue
			}

			for _, event := range events {
				c.Response().Write([]byte("data: "))
				c.Response().Write(event)
				c.Response().Write([]byte("\n\n"))
				c.Response().Flush()
			}
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
func (h *Handler) getCredentials(c echo.Context, provider string) (baseURL, apiKey, protocol string, err error) {
	middleware.LogTrace(c, "GetCredentials", "Getting credentials for provider: %s", provider)

	// Check if using API key auth (has API key in context)
	apiKeyObj := middleware.GetAPIKey(c)
	if apiKeyObj != nil {
		middleware.LogTrace(c, "GetCredentials", "Using API key auth: KeyID=%d, ProviderConfigsCount=%d", apiKeyObj.ID, len(apiKeyObj.ProviderConfigs))

		// Find matching provider config from API key's associated providers
		var providerCfg *database.ProviderConfig
		for i := range apiKeyObj.ProviderConfigs {
			cfg := &apiKeyObj.ProviderConfigs[i]
			middleware.LogTrace(c, "GetCredentials", "Checking provider config: Provider=%s, IsActive=%v", cfg.Provider, cfg.IsActive)
			if cfg.Provider == provider && cfg.IsActive {
				providerCfg = cfg
				middleware.LogTrace(c, "GetCredentials", "Found matching provider config: ID=%d, Name=%s, BaseURL=%s", cfg.ID, cfg.Name, cfg.BaseURL)
				break
			}
		}
		if providerCfg == nil {
			middleware.LogTrace(c, "GetCredentials", "No matching provider config found for provider: %s", provider)
			return "", "", "", fmt.Errorf("API key does not have access to %s provider", provider)
		}
		apiKey, err = h.configService.DecryptAPIKey(providerCfg)
		if err != nil {
			middleware.LogTrace(c, "GetCredentials", "Failed to decrypt API key: %v", err)
			return "", "", "", err
		}
		middleware.LogTrace(c, "GetCredentials", "Successfully got credentials from API key")
		return providerCfg.BaseURL, apiKey, normalizeProtocol(providerCfg.Protocol), nil
	}

	// JWT auth - get default config for provider
	middleware.LogTrace(c, "GetCredentials", "Using JWT auth")
	user := middleware.GetUser(c)
	if user == nil {
		middleware.LogTrace(c, "GetCredentials", "No user found in context")
		return "", "", "", fmt.Errorf("not authenticated")
	}

	middleware.LogTrace(c, "GetCredentials", "Getting default config for user=%d, provider=%s", user.ID, provider)
	cfg, err := h.configService.GetDefaultConfig(user.ID, provider)
	if err != nil {
		middleware.LogTrace(c, "GetCredentials", "Failed to get default config: %v", err)
		return "", "", "", fmt.Errorf("no %s configuration found", provider)
	}

	apiKey, err = h.configService.DecryptAPIKey(cfg)
	if err != nil {
		middleware.LogTrace(c, "GetCredentials", "Failed to decrypt API key: %v", err)
		return "", "", "", err
	}

	middleware.LogTrace(c, "GetCredentials", "Successfully got credentials from JWT user config")
	return cfg.BaseURL, apiKey, normalizeProtocol(cfg.Protocol), nil
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
		if promptTokens == 0 {
			if it, ok := usage["input_tokens"].(float64); ok {
				promptTokens = int(it)
			}
		}
		if completionTokens == 0 {
			if ot, ok := usage["output_tokens"].(float64); ok {
				completionTokens = int(ot)
			}
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
