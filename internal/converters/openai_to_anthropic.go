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
	if req.TopK != nil {
		anthropicReq.TopK = req.TopK
	}

	// Convert stop sequences
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			anthropicReq.StopSequences = []string{v}
		case []string:
			anthropicReq.StopSequences = append(anthropicReq.StopSequences, v...)
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
	var systemText string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system message
			systemText += getTextContent(msg.Content)
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
				ID:        msg.ToolCallID,
				Content:   msg.Content,
			}}
		} else {
			textContent, imageBlocks := extractOpenAIContentParts(msg.Content)
			var blocks []models.ContentBlock

			if textContent != "" {
				blocks = append(blocks, models.ContentBlock{
					Type: "text",
					Text: textContent,
				})
			}

			if len(imageBlocks) > 0 {
				blocks = append(blocks, imageBlocks...)
			}

			// Add tool use blocks if present
			if len(msg.ToolCalls) > 0 {
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
			}

			if len(blocks) > 0 {
				if len(blocks) == 1 && blocks[0].Type == "text" && len(msg.ToolCalls) == 0 && len(imageBlocks) == 0 {
					anthropicMsg.Content = blocks[0].Text
				} else {
					anthropicMsg.Content = blocks
				}
			} else {
				anthropicMsg.Content = msg.Content
			}
		}

		messages = append(messages, anthropicMsg)
	}
	anthropicReq.Messages = messages

	if systemText != "" {
		anthropicReq.System = systemText
	}

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

	// Convert tool choice (OpenAI -> Anthropic)
	if req.ToolChoice != nil {
		switch choice := req.ToolChoice.(type) {
		case string:
			switch choice {
			case "auto":
				anthropicReq.ToolChoice = models.ToolChoiceAuto{Type: "auto"}
			case "required":
				anthropicReq.ToolChoice = models.ToolChoiceAny{Type: "any"}
			}
		case models.ToolChoiceObject:
			anthropicReq.ToolChoice = models.ToolChoiceTool{
				Type: "tool",
				Name: choice.Function.Name,
			}
		case map[string]interface{}:
			if choiceType, ok := choice["type"].(string); ok {
				switch choiceType {
				case "auto":
					anthropicReq.ToolChoice = models.ToolChoiceAuto{Type: "auto"}
				case "required":
					anthropicReq.ToolChoice = models.ToolChoiceAny{Type: "any"}
				case "function":
					if fn, ok := choice["function"].(map[string]interface{}); ok {
						name := getString(fn, "name")
						if name != "" {
							anthropicReq.ToolChoice = models.ToolChoiceTool{
								Type: "tool",
								Name: name,
							}
						}
					}
				}
			}
		}
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

	var textContent string
	var toolCalls []models.ToolCall

	switch contentVal := content.(type) {
	case string:
		textContent = contentVal
	case []interface{}:
		for _, block := range contentVal {
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
	}

	if textContent != "" {
		message.Content = textContent
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
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

	case "content_block_start":
		contentBlock, ok := data["content_block"].(map[string]interface{})
		if !ok {
			return nil, nil
		}
		blockType := getString(contentBlock, "type")
		if blockType != "tool_use" {
			return nil, nil
		}

		chunk := models.ChatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []models.Choice{{
				Index: 0,
				Delta: &models.ChatMessage{
					ToolCalls: []models.ToolCall{{
						ID:   getString(contentBlock, "id"),
						Type: "function",
						Function: models.FunctionCall{
							Name: getString(contentBlock, "name"),
						},
					}},
				},
			}},
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
	if parts, ok := content.([]models.ContentPart); ok {
		var text string
		for _, part := range parts {
			if part.Type == "text" {
				text += part.Text
			}
		}
		return text
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
	switch v := m[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

func generateID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}
