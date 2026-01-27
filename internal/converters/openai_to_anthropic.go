package converters

import (
	"encoding/json"
	"fmt"
	"time"

	"ai_gateway/internal/models"
)

// OpenAIToAnthropicRequest converts an OpenAI request to Anthropic format
func OpenAIToAnthropicRequest(req *models.ChatCompletionRequest) (*models.MessagesRequest, error) {
	anthropicReq := &models.MessagesRequest{
		Model:     req.Model,
		MaxTokens: 4096, // Default max tokens
		Stream:    req.Stream,
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		anthropicReq.TopP = req.TopP
	}

	// Convert stop sequences
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			anthropicReq.StopSequences = []string{v}
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					anthropicReq.StopSequences = append(anthropicReq.StopSequences, str)
				}
			}
		}
	}

	// Convert messages, extracting system message
	var messages []models.AnthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system message
			content := getTextContent(msg.Content)
			anthropicReq.System = content
			continue
		}

		anthropicMsg := models.AnthropicMessage{
			Role: msg.Role,
		}

		// Handle tool responses
		if msg.Role == "tool" {
			anthropicMsg.Role = "user"
			anthropicMsg.Content = []models.ContentBlock{{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}}
		} else if msg.ToolCalls != nil && len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
			var blocks []models.ContentBlock

			// Add text content if present
			content := getTextContent(msg.Content)
			if content != "" {
				blocks = append(blocks, models.ContentBlock{
					Type: "text",
					Text: content,
				})
			}

			// Add tool use blocks
			for _, tc := range msg.ToolCalls {
				var input interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = map[string]interface{}{}
				}
				blocks = append(blocks, models.ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			anthropicMsg.Content = blocks
		} else {
			// Regular message
			anthropicMsg.Content = msg.Content
		}

		messages = append(messages, anthropicMsg)
	}
	anthropicReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []models.AnthropicTool
		for _, tool := range req.Tools {
			tools = append(tools, models.AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			})
		}
		anthropicReq.Tools = tools
	}

	return anthropicReq, nil
}

// AnthropicToOpenAIResponse converts an Anthropic response to OpenAI format
func AnthropicToOpenAIResponse(resp map[string]interface{}, model string) (*models.ChatCompletionResponse, error) {
	openaiResp := &models.ChatCompletionResponse{
		ID:      getString(resp, "id"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	// Convert content to message
	content := resp["content"]
	var message models.ChatMessage
	message.Role = "assistant"

	if contentArr, ok := content.([]interface{}); ok {
		var textContent string
		var toolCalls []models.ToolCall

		for _, block := range contentArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				blockType := getString(blockMap, "type")
				if blockType == "text" {
					textContent += getString(blockMap, "text")
				} else if blockType == "tool_use" {
					argsBytes, _ := json.Marshal(blockMap["input"])
					toolCalls = append(toolCalls, models.ToolCall{
						ID:   getString(blockMap, "id"),
						Type: "function",
						Function: models.FunctionCall{
							Name:      getString(blockMap, "name"),
							Arguments: string(argsBytes),
						},
					})
				}
			}
		}

		if textContent != "" {
			message.Content = textContent
		}
		if len(toolCalls) > 0 {
			message.ToolCalls = toolCalls
		}
	}

	// Convert stop reason
	var finishReason string
	if stopReason, ok := resp["stop_reason"].(string); ok {
		switch stopReason {
		case "end_turn":
			finishReason = "stop"
		case "max_tokens":
			finishReason = "length"
		case "tool_use":
			finishReason = "tool_calls"
		default:
			finishReason = stopReason
		}
	}

	openaiResp.Choices = []models.Choice{{
		Index:        0,
		Message:      &message,
		FinishReason: &finishReason,
	}}

	// Convert usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		inputTokens := getInt(usage, "input_tokens")
		outputTokens := getInt(usage, "output_tokens")
		openaiResp.Usage = &models.Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		}
	}

	return openaiResp, nil
}

// AnthropicStreamToOpenAIStream converts an Anthropic stream event to OpenAI format
func AnthropicStreamToOpenAIStream(eventType string, data map[string]interface{}, model string, id string) ([]byte, error) {
	switch eventType {
	case "message_start":
		// Create initial chunk
		chunk := models.ChatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []models.Choice{{
				Index: 0,
				Delta: &models.ChatMessage{Role: "assistant"},
			}},
		}
		return json.Marshal(chunk)

	case "content_block_delta":
		delta := data["delta"].(map[string]interface{})
		deltaType := getString(delta, "type")

		chunk := models.ChatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
		}

		if deltaType == "text_delta" {
			chunk.Choices = []models.Choice{{
				Index: 0,
				Delta: &models.ChatMessage{Content: getString(delta, "text")},
			}}
		} else if deltaType == "input_json_delta" {
			// Tool call argument delta
			chunk.Choices = []models.Choice{{
				Index: 0,
				Delta: &models.ChatMessage{
					ToolCalls: []models.ToolCall{{
						Function: models.FunctionCall{
							Arguments: getString(delta, "partial_json"),
						},
					}},
				},
			}}
		}

		return json.Marshal(chunk)

	case "message_delta":
		delta := data["delta"].(map[string]interface{})
		stopReason := getString(delta, "stop_reason")

		var finishReason string
		switch stopReason {
		case "end_turn":
			finishReason = "stop"
		case "max_tokens":
			finishReason = "length"
		case "tool_use":
			finishReason = "tool_calls"
		default:
			finishReason = stopReason
		}

		chunk := models.ChatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []models.Choice{{
				Index:        0,
				Delta:        &models.ChatMessage{},
				FinishReason: &finishReason,
			}},
		}

		return json.Marshal(chunk)

	default:
		return nil, nil
	}
}

// Helper functions

func getTextContent(content interface{}) string {
	if content == nil {
		return ""
	}
	if str, ok := content.(string); ok {
		return str
	}
	if parts, ok := content.([]interface{}); ok {
		var text string
		for _, part := range parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partMap["type"] == "text" {
					if t, ok := partMap["text"].(string); ok {
						text += t
					}
				}
			}
		}
		return text
	}
	return ""
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func generateID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}
