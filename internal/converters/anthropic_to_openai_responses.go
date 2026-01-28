package converters

import (
	"encoding/json"
	"fmt"

	"ai_gateway/internal/models"
)

// AnthropicToOpenAIResponsesRequest converts an Anthropic request to OpenAI Responses API format
// Enhanced version based on reference implementation
func AnthropicToOpenAIResponsesRequest(req *models.MessagesRequest) (map[string]interface{}, error) {
	// Validate input request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid anthropic request: %w", err)
	}

	result := map[string]interface{}{
		"model": req.Model,
	}

	if req.Stream {
		result["stream"] = true
	}

	if req.Temperature != nil {
		result["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		result["top_p"] = *req.TopP
	}
	if req.MaxTokens > 0 {
		result["max_output_tokens"] = req.MaxTokens
	}

	// Convert system to instructions
	if instructions := extractSystemText(req.System); instructions != "" {
		result["instructions"] = instructions
	}

	// Convert messages to input array
	var input []map[string]interface{}
	for _, msg := range req.Messages {
		var contentParts []map[string]interface{}
		var toolCalls []map[string]interface{}
		var toolOutputs []map[string]interface{}

		switch content := msg.Content.(type) {
		case string:
			if content != "" {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "input_text",
					"text": content,
				})
			}
		default:
			blocks := normalizeAnthropicBlocks(content)
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if block.Text != "" {
						contentParts = append(contentParts, map[string]interface{}{
							"type": "input_text",
							"text": block.Text,
						})
					}
				case "image":
					if block.Source != nil {
						url := getString(block.Source, "data")
						if url != "" {
							contentParts = append(contentParts, map[string]interface{}{
								"type": "input_image",
								"image_url": map[string]interface{}{
									"url": url,
								},
							})
						}
					}
				case "tool_use":
					argsBytes, _ := json.Marshal(block.Input)
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   block.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      block.Name,
							"arguments": string(argsBytes),
						},
					})
				case "tool_result":
					toolOutputs = append(toolOutputs, map[string]interface{}{
						"type":    "function_call_output",
						"call_id": blockToolResultID(block),
						"output":  stringifyContent(block.Content),
					})
				}
			}
		}

		if len(contentParts) > 0 || len(toolCalls) > 0 {
			inputItem := map[string]interface{}{
				"type":    "message",
				"role":    msg.Role,
				"content": contentParts,
			}
			if len(contentParts) == 0 {
				inputItem["content"] = []interface{}{}
			}
			if len(toolCalls) > 0 {
				inputItem["tool_calls"] = toolCalls
			}
			input = append(input, inputItem)
		}

		if len(toolOutputs) > 0 {
			input = append(input, toolOutputs...)
		}
	}
	result["input"] = input

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []map[string]interface{}
		for _, tool := range req.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.InputSchema,
				},
			})
		}
		result["tools"] = tools
	}

	return result, nil
}

// OpenAIResponsesToAnthropicResponse converts an OpenAI Responses API response to Anthropic format
func OpenAIResponsesToAnthropicResponse(resp map[string]interface{}, model string) (*models.MessagesResponse, error) {
	anthropicResp := &models.MessagesResponse{
		ID:    getString(resp, "id"),
		Type:  "message",
		Role:  "assistant",
		Model: model,
	}

	// Handle output array
	output, ok := resp["output"].([]interface{})
	if !ok || len(output) == 0 {
		return anthropicResp, nil
	}

	var contentBlocks []models.ContentBlock

	for _, item := range output {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType := getString(itemMap, "type")
		switch itemType {
		case "message":
			// Handle message output
			if contentArr, ok := itemMap["content"].([]interface{}); ok {
				for _, contentItem := range contentArr {
					if contentMap, ok := contentItem.(map[string]interface{}); ok {
						contentType := getString(contentMap, "type")
						if contentType == "output_text" || contentType == "text" {
							text := getString(contentMap, "text")
							if text != "" {
								contentBlocks = append(contentBlocks, models.ContentBlock{
									Type: "text",
									Text: text,
								})
							}
						}
					}
				}
			}
		case "function_call":
			// Handle function call output
			var input interface{}
			if args, ok := itemMap["arguments"].(string); ok {
				json.Unmarshal([]byte(args), &input)
			}
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type:  "tool_use",
				ID:    getString(itemMap, "call_id"),
				Name:  getString(itemMap, "name"),
				Input: input,
			})
		}
	}

	anthropicResp.Content = contentBlocks

	// Convert status to stop_reason
	if status, ok := resp["status"].(string); ok {
		var stopReason string
		switch status {
		case "completed":
			stopReason = "end_turn"
		case "incomplete":
			if reason, ok := resp["incomplete_details"].(map[string]interface{}); ok {
				if getString(reason, "reason") == "max_output_tokens" {
					stopReason = "max_tokens"
				}
			}
		case "failed":
			stopReason = "end_turn" // Map failed to end_turn as fallback
		default:
			stopReason = "end_turn"
		}
		// Check if there are function calls
		for _, block := range contentBlocks {
			if block.Type == "tool_use" {
				stopReason = "tool_use"
				break
			}
		}
		anthropicResp.StopReason = &stopReason
	}

	// Convert usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		anthropicResp.Usage = models.AnthropicUsage{
			InputTokens:  getInt(usage, "input_tokens"),
			OutputTokens: getInt(usage, "output_tokens"),
		}
	}

	return anthropicResp, nil
}

// OpenAIResponsesStreamToAnthropicStream converts an OpenAI Responses API stream event to Anthropic format
func OpenAIResponsesStreamToAnthropicStream(data map[string]interface{}, isFirst bool) ([][]byte, error) {
	var events [][]byte

	eventType := getString(data, "type")

	switch eventType {
	case "response.created":
		// Send message_start event
		response, _ := data["response"].(map[string]interface{})
		startEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":          getString(response, "id"),
				"type":        "message",
				"role":        "assistant",
				"content":     []interface{}{},
				"model":       getString(response, "model"),
				"stop_reason": nil,
				"usage":       map[string]interface{}{"input_tokens": 0, "output_tokens": 0},
			},
		}
		startBytes, _ := json.Marshal(startEvent)
		events = append(events, startBytes)

	case "response.output_item.added":
		// Send content_block_start event
		index := getInt(data, "output_index")
		item, _ := data["item"].(map[string]interface{})
		itemType := getString(item, "type")

		var contentBlock map[string]interface{}
		if itemType == "message" {
			contentBlock = map[string]interface{}{
				"type": "text",
				"text": "",
			}
		} else if itemType == "function_call" {
			contentBlock = map[string]interface{}{
				"type":  "tool_use",
				"id":    getString(item, "call_id"),
				"name":  getString(item, "name"),
				"input": map[string]interface{}{},
			}
		}

		if contentBlock != nil {
			blockStartEvent := map[string]interface{}{
				"type":          "content_block_start",
				"index":         index,
				"content_block": contentBlock,
			}
			blockStartBytes, _ := json.Marshal(blockStartEvent)
			events = append(events, blockStartBytes)
		}

	case "response.content_part.added":
		// Content part started
		index := getInt(data, "output_index")
		blockStartEvent := map[string]interface{}{
			"type":  "content_block_start",
			"index": index,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		blockStartBytes, _ := json.Marshal(blockStartEvent)
		events = append(events, blockStartBytes)

	case "response.output_text.delta":
		// Text delta
		index := getInt(data, "output_index")
		delta := getString(data, "delta")
		if delta != "" {
			deltaEvent := map[string]interface{}{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": delta,
				},
			}
			deltaBytes, _ := json.Marshal(deltaEvent)
			events = append(events, deltaBytes)
		}

	case "response.function_call_arguments.delta":
		// Function call arguments delta
		index := getInt(data, "output_index")
		delta := getString(data, "delta")
		if delta != "" {
			deltaEvent := map[string]interface{}{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]interface{}{
					"type":         "input_json_delta",
					"partial_json": delta,
				},
			}
			deltaBytes, _ := json.Marshal(deltaEvent)
			events = append(events, deltaBytes)
		}

	case "response.output_item.done":
		// Content block done
		index := getInt(data, "output_index")
		stopBlockEvent := map[string]interface{}{
			"type":  "content_block_stop",
			"index": index,
		}
		stopBlockBytes, _ := json.Marshal(stopBlockEvent)
		events = append(events, stopBlockBytes)

	case "response.completed":
		// Response completed
		response, _ := data["response"].(map[string]interface{})
		status := getString(response, "status")

		var stopReason string
		switch status {
		case "completed":
			stopReason = "end_turn"
		case "incomplete":
			stopReason = "max_tokens"
		default:
			stopReason = "end_turn"
		}

		// Check for tool use
		if output, ok := response["output"].([]interface{}); ok {
			for _, item := range output {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if getString(itemMap, "type") == "function_call" {
						stopReason = "tool_use"
						break
					}
				}
			}
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
		if usage, ok := response["usage"].(map[string]interface{}); ok {
			messageDeltaEvent["usage"] = map[string]interface{}{
				"output_tokens": getInt(usage, "output_tokens"),
			}
		}
		messageDeltaBytes, _ := json.Marshal(messageDeltaEvent)
		events = append(events, messageDeltaBytes)

		messageStopEvent := map[string]interface{}{
			"type": "message_stop",
		}
		messageStopBytes, _ := json.Marshal(messageStopEvent)
		events = append(events, messageStopBytes)
	}

	return events, nil
}
