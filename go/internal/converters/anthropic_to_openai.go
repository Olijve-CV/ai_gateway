package converters

import (
	"encoding/json"
	"time"

	"ai_gateway/internal/models"
)

// AnthropicToOpenAIRequest converts an Anthropic request to OpenAI format
func AnthropicToOpenAIRequest(req *models.MessagesRequest) (*models.ChatCompletionRequest, error) {
	openaiReq := &models.ChatCompletionRequest{
		Model:  req.Model,
		Stream: req.Stream,
	}

	if req.Temperature != nil {
		openaiReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		openaiReq.TopP = req.TopP
	}
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = &req.MaxTokens
	}
	if len(req.StopSequences) > 0 {
		openaiReq.Stop = req.StopSequences
	}

	// Convert messages
	var messages []models.ChatMessage

	// Add system message if present
	if req.System != nil {
		var systemContent string
		switch v := req.System.(type) {
		case string:
			systemContent = v
		case []interface{}:
			for _, block := range v {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if blockMap["type"] == "text" {
						systemContent += getString(blockMap, "text")
					}
				}
			}
		}
		if systemContent != "" {
			messages = append(messages, models.ChatMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiMsg := models.ChatMessage{
			Role: msg.Role,
		}

		// Handle content
		switch content := msg.Content.(type) {
		case string:
			openaiMsg.Content = content
		case []interface{}:
			var textContent string
			var toolCalls []models.ToolCall
			var toolResultContent string
			var toolUseID string

			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType := getString(blockMap, "type")
					switch blockType {
					case "text":
						textContent += getString(blockMap, "text")
					case "tool_use":
						argsBytes, _ := json.Marshal(blockMap["input"])
						toolCalls = append(toolCalls, models.ToolCall{
							ID:   getString(blockMap, "id"),
							Type: "function",
							Function: models.FunctionCall{
								Name:      getString(blockMap, "name"),
								Arguments: string(argsBytes),
							},
						})
					case "tool_result":
						toolUseID = getString(blockMap, "tool_use_id")
						if c, ok := blockMap["content"].(string); ok {
							toolResultContent = c
						} else {
							contentBytes, _ := json.Marshal(blockMap["content"])
							toolResultContent = string(contentBytes)
						}
					}
				}
			}

			if toolUseID != "" {
				// This is a tool result message
				openaiMsg.Role = "tool"
				openaiMsg.ToolCallID = toolUseID
				openaiMsg.Content = toolResultContent
			} else {
				if textContent != "" {
					openaiMsg.Content = textContent
				}
				if len(toolCalls) > 0 {
					openaiMsg.ToolCalls = toolCalls
				}
			}
		}

		messages = append(messages, openaiMsg)
	}
	openaiReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []models.Tool
		for _, tool := range req.Tools {
			tools = append(tools, models.Tool{
				Type: "function",
				Function: models.Function{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			})
		}
		openaiReq.Tools = tools
	}

	return openaiReq, nil
}

// OpenAIToAnthropicResponse converts an OpenAI response to Anthropic format
func OpenAIToAnthropicResponse(resp map[string]interface{}, model string) (*models.MessagesResponse, error) {
	anthropicResp := &models.MessagesResponse{
		ID:    getString(resp, "id"),
		Type:  "message",
		Role:  "assistant",
		Model: model,
	}

	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return anthropicResp, nil
	}

	choice := choices[0].(map[string]interface{})
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return anthropicResp, nil
	}

	var contentBlocks []models.ContentBlock

	// Handle text content
	if content, ok := message["content"].(string); ok && content != "" {
		contentBlocks = append(contentBlocks, models.ContentBlock{
			Type: "text",
			Text: content,
		})
	}

	// Handle tool calls
	if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			tcMap := tc.(map[string]interface{})
			function := tcMap["function"].(map[string]interface{})
			var input interface{}
			if args, ok := function["arguments"].(string); ok {
				json.Unmarshal([]byte(args), &input)
			}
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type:  "tool_use",
				ID:    getString(tcMap, "id"),
				Name:  getString(function, "name"),
				Input: input,
			})
		}
	}

	anthropicResp.Content = contentBlocks

	// Convert finish reason
	if finishReason, ok := choice["finish_reason"].(string); ok {
		var stopReason string
		switch finishReason {
		case "stop":
			stopReason = "end_turn"
		case "length":
			stopReason = "max_tokens"
		case "tool_calls":
			stopReason = "tool_use"
		default:
			stopReason = finishReason
		}
		anthropicResp.StopReason = &stopReason
	}

	// Convert usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		anthropicResp.Usage = models.AnthropicUsage{
			InputTokens:  getInt(usage, "prompt_tokens"),
			OutputTokens: getInt(usage, "completion_tokens"),
		}
	}

	return anthropicResp, nil
}

// OpenAIStreamToAnthropicStream converts an OpenAI stream chunk to Anthropic format
func OpenAIStreamToAnthropicStream(data map[string]interface{}, isFirst bool) ([][]byte, error) {
	var events [][]byte

	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, nil
	}

	choice := choices[0].(map[string]interface{})
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	if isFirst {
		// Send message_start event
		startEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":           getString(data, "id"),
				"type":         "message",
				"role":         "assistant",
				"content":      []interface{}{},
				"model":        getString(data, "model"),
				"stop_reason":  nil,
				"usage":        map[string]interface{}{"input_tokens": 0, "output_tokens": 0},
			},
		}
		startBytes, _ := json.Marshal(startEvent)
		events = append(events, startBytes)

		// Send content_block_start event
		blockStartEvent := map[string]interface{}{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		blockStartBytes, _ := json.Marshal(blockStartEvent)
		events = append(events, blockStartBytes)
	}

	// Handle text delta
	if content, ok := delta["content"].(string); ok && content != "" {
		deltaEvent := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": content,
			},
		}
		deltaBytes, _ := json.Marshal(deltaEvent)
		events = append(events, deltaBytes)
	}

	// Handle finish reason
	if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
		// Send content_block_stop
		stopBlockEvent := map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}
		stopBlockBytes, _ := json.Marshal(stopBlockEvent)
		events = append(events, stopBlockBytes)

		// Send message_delta
		var stopReason string
		switch finishReason {
		case "stop":
			stopReason = "end_turn"
		case "length":
			stopReason = "max_tokens"
		case "tool_calls":
			stopReason = "tool_use"
		default:
			stopReason = finishReason
		}

		messageDeltaEvent := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": stopReason,
			},
			"usage": map[string]interface{}{
				"output_tokens": 0,
			},
		}
		messageDeltaBytes, _ := json.Marshal(messageDeltaEvent)
		events = append(events, messageDeltaBytes)

		// Send message_stop
		messageStopEvent := map[string]interface{}{
			"type": "message_stop",
		}
		messageStopBytes, _ := json.Marshal(messageStopEvent)
		events = append(events, messageStopBytes)
	}

	return events, nil
}
