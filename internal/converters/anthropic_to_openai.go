package converters

import (
	"encoding/json"
	"fmt"
	"time"

	"ai_gateway/internal/models"
)

// AnthropicToOpenAIRequest converts an Anthropic request to OpenAI format
// Enhanced version based on reference implementation
func AnthropicToOpenAIRequest(req *models.MessagesRequest) (*models.ChatCompletionRequest, error) {
	// Validate input request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid anthropic request: %w", err)
	}

	openaiReq := &models.ChatCompletionRequest{
		Model:  req.Model,
		Stream: req.Stream,
	}

	// Convert parameters with enhanced handling
	if req.Temperature != nil {
		openaiReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		openaiReq.TopP = req.TopP
	}
	if req.TopK != nil {
		openaiReq.TopK = req.TopK
	}
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = &req.MaxTokens
	}
	if len(req.StopSequences) > 0 {
		openaiReq.Stop = req.StopSequences
	}

	// Convert messages
	var messages []models.ChatMessage

	// Add system message if present (reference behavior)
	if systemContent := extractSystemText(req.System); systemContent != "" {
		messages = append(messages, models.ChatMessage{
			Role:    "system",
			Content: systemContent,
		})
	}

	for _, msg := range req.Messages {
		openaiMsg := models.ChatMessage{
			Role: msg.Role,
		}

		var contentParts []interface{}
		var toolCalls []models.ToolCall

		switch content := msg.Content.(type) {
		case string:
			openaiMsg.Content = content
		default:
			blocks := normalizeAnthropicBlocks(content)
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if block.Text != "" {
						contentParts = append(contentParts, map[string]interface{}{
							"type": "text",
							"text": block.Text,
						})
					}
				case "image":
					if block.Source != nil {
						url := getString(block.Source, "data")
						if url != "" {
							contentParts = append(contentParts, map[string]interface{}{
								"type": "image_url",
								"image_url": map[string]interface{}{
									"url": url,
								},
							})
						}
					}
				case "tool_use":
					toolCallID := block.ID
					if toolCallID == "" {
						toolCallID = fmt.Sprintf("call_%d", time.Now().UnixNano())
					}
					argsBytes, _ := json.Marshal(block.Input)
					toolCalls = append(toolCalls, models.ToolCall{
						ID:   toolCallID,
						Type: "function",
						Function: models.FunctionCall{
							Name:      block.Name,
							Arguments: string(argsBytes),
						},
					})
				case "tool_result":
					if msg.Role == "user" {
						toolMsg := models.ChatMessage{
							Role:       "tool",
							ToolCallID: blockToolResultID(block),
							Content:    stringifyContent(block.Content),
						}
						messages = append(messages, toolMsg)
					} else {
						toolResultText := stringifyContent(block.Content)
						if toolResultText != "" {
							contentParts = append(contentParts, map[string]interface{}{
								"type": "text",
								"text": fmt.Sprintf("Tool result: %s", toolResultText),
							})
						}
					}
				}
			}
		}

		if len(toolCalls) > 0 {
			openaiMsg.ToolCalls = toolCalls
		}

		if len(contentParts) > 0 {
			if len(contentParts) == 1 {
				if part, ok := contentParts[0].(map[string]interface{}); ok && getString(part, "type") == "text" {
					openaiMsg.Content = getString(part, "text")
				} else {
					openaiMsg.Content = contentParts
				}
			} else {
				openaiMsg.Content = contentParts
			}
		}

		hasContent := openaiMsg.Content != nil
		if contentStr, ok := openaiMsg.Content.(string); ok {
			hasContent = contentStr != ""
		}
		if !hasContent && len(openaiMsg.ToolCalls) == 0 {
			continue
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

	// Handle tool choice conversion (enhanced)
	if req.ToolChoice != nil {
		if err := convertToolChoice(req.ToolChoice, openaiReq); err != nil {
			return nil, fmt.Errorf("tool choice conversion failed: %w", err)
		}
	} else if len(openaiReq.Tools) > 0 {
		openaiReq.ToolChoice = "auto"
	}

	return openaiReq, nil
}

// convertToolChoice converts Anthropic tool choice to OpenAI format
func convertToolChoice(choice interface{}, req *models.ChatCompletionRequest) error {
	switch toolChoice := choice.(type) {
	case models.ToolChoiceAuto:
		if toolChoice.Type == "auto" {
			req.ToolChoice = "auto"
		}
	case models.ToolChoiceAny:
		if toolChoice.Type == "any" {
			req.ToolChoice = "required"
		}
	case models.ToolChoiceTool:
		if toolChoice.Type == "tool" {
			req.ToolChoice = models.ToolChoiceObject{
				Type: "function",
				Function: models.ToolChoiceFunction{
					Name: toolChoice.Name,
				},
			}
		}
	case map[string]interface{}:
		// Handle JSON unmarshaled tool choice
		if choiceType, ok := toolChoice["type"].(string); ok {
			switch choiceType {
			case "auto":
				req.ToolChoice = "auto"
			case "any":
				req.ToolChoice = "required"
			case "tool":
				if name, ok := toolChoice["name"].(string); ok && name != "" {
					req.ToolChoice = models.ToolChoiceObject{
						Type: "function",
						Function: models.ToolChoiceFunction{
							Name: name,
						},
					}
				}
			}
		}
	default:
		return fmt.Errorf("unsupported tool choice type: %T", choice)
	}
	return nil
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

	contentBlocks := make([]models.ContentBlock, 0)

	// Handle content parts
	switch content := message["content"].(type) {
	case string:
		if content != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "text",
				Text: content,
			})
		}
	case []models.ContentPart:
		for _, part := range content {
			switch part.Type {
			case "text":
				if part.Text != "" {
					if len(contentBlocks) > 0 && contentBlocks[len(contentBlocks)-1].Type == "text" {
						contentBlocks[len(contentBlocks)-1].Text += part.Text
					} else {
						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type: "text",
							Text: part.Text,
						})
					}
				}
			case "image_url":
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					contentBlocks = append(contentBlocks, models.ContentBlock{
						Type: "image",
						Source: &models.ImageSource{
							Type: "base64",
							Data: part.ImageURL.URL,
						},
					})
				}
			}
		}
	case []interface{}:
		for _, part := range content {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(partMap, "type") {
			case "text":
				text := getString(partMap, "text")
				if text != "" {
					if len(contentBlocks) > 0 && contentBlocks[len(contentBlocks)-1].Type == "text" {
						contentBlocks[len(contentBlocks)-1].Text += text
					} else {
						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type: "text",
							Text: text,
						})
					}
				}
			case "image_url":
				if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
					url := getString(imageURL, "url")
					if url != "" {
						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type: "image",
							Source: &models.ImageSource{
								Type: "base64",
								Data: url,
							},
						})
					}
				}
			}
		}
	case []map[string]interface{}:
		for _, partMap := range content {
			switch getString(partMap, "type") {
			case "text":
				text := getString(partMap, "text")
				if text != "" {
					if len(contentBlocks) > 0 && contentBlocks[len(contentBlocks)-1].Type == "text" {
						contentBlocks[len(contentBlocks)-1].Text += text
					} else {
						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type: "text",
							Text: text,
						})
					}
				}
			case "image_url":
				if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
					url := getString(imageURL, "url")
					if url != "" {
						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type: "image",
							Source: &models.ImageSource{
								Type: "base64",
								Data: url,
							},
						})
					}
				}
			}
		}
	}

	// Handle tool calls
	if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			tcMap := tc.(map[string]interface{})
			function, _ := tcMap["function"].(map[string]interface{})
			var input interface{}
			if function != nil {
				if args, ok := function["arguments"].(string); ok {
					json.Unmarshal([]byte(args), &input)
				}
			} else {
				input = map[string]interface{}{}
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

// OpenAIToAnthropicStreamState stores stream conversion state.
type OpenAIToAnthropicStreamState struct {
	contentBlockIndex   int
	contentBlockStarted bool
	currentBlockType    string
	finishReason        string
	finished            bool
	startSent           bool
}

// NewOpenAIToAnthropicStreamState creates a new stream state.
func NewOpenAIToAnthropicStreamState() *OpenAIToAnthropicStreamState {
	return &OpenAIToAnthropicStreamState{}
}

// OpenAIStreamToAnthropicStream converts an OpenAI stream chunk to Anthropic format.
func OpenAIStreamToAnthropicStream(data map[string]interface{}, state *OpenAIToAnthropicStreamState) ([][]byte, error) {
	if state == nil {
		state = NewOpenAIToAnthropicStreamState()
	}

	var events [][]byte

	if !state.startSent {
		messageID := getString(data, "id")
		modelName := getString(data, "model")
		inputTokens := 0
		if usageMap, ok := data["usage"].(map[string]interface{}); ok {
			inputTokens = getInt(usageMap, "prompt_tokens")
		}

		startEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":          messageID,
				"type":        "message",
				"role":        "assistant",
				"content":     []interface{}{},
				"model":       modelName,
				"stop_reason": nil,
				"usage": map[string]interface{}{
					"input_tokens":  inputTokens,
					"output_tokens": 0,
				},
			},
		}
		startBytes, _ := json.Marshal(startEvent)
		events = append(events, startBytes)
		state.startSent = true
	}

	if state.finished {
		return events, nil
	}

	choices, _ := data["choices"].([]interface{})
	if len(choices) == 0 {
		if state.contentBlockStarted {
			stopEvent := map[string]interface{}{
				"type":  "content_block_stop",
				"index": state.contentBlockIndex,
			}
			stopBytes, _ := json.Marshal(stopEvent)
			events = append(events, stopBytes)
			state.contentBlockStarted = false
			state.currentBlockType = ""
		}

		stopReason := mapFinishReason(state.finishReason)
		messageDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": stopReason,
			},
		}
		if usageMap, ok := data["usage"].(map[string]interface{}); ok {
			messageDelta["usage"] = map[string]interface{}{
				"input_tokens":  getInt(usageMap, "prompt_tokens"),
				"output_tokens": getInt(usageMap, "completion_tokens"),
			}
		}
		messageDeltaBytes, _ := json.Marshal(messageDelta)
		events = append(events, messageDeltaBytes)

		messageStopBytes, _ := json.Marshal(map[string]interface{}{"type": "message_stop"})
		events = append(events, messageStopBytes)
		state.finished = true
		return events, nil
	}

	choice := choices[0].(map[string]interface{})
	if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
		state.finishReason = finishReason
	}

	delta, _ := choice["delta"].(map[string]interface{})
	if delta != nil {
		if content, ok := delta["content"].(string); ok && content != "" {
			if !state.contentBlockStarted || state.currentBlockType != "text" {
				if state.contentBlockStarted {
					stopEvent := map[string]interface{}{
						"type":  "content_block_stop",
						"index": state.contentBlockIndex,
					}
					stopBytes, _ := json.Marshal(stopEvent)
					events = append(events, stopBytes)
					state.contentBlockIndex++
				}
				startEvent := map[string]interface{}{
					"type":  "content_block_start",
					"index": state.contentBlockIndex,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}
				startBytes, _ := json.Marshal(startEvent)
				events = append(events, startBytes)
				state.contentBlockStarted = true
				state.currentBlockType = "text"
			}

			deltaEvent := map[string]interface{}{
				"type":  "content_block_delta",
				"index": state.contentBlockIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			}
			deltaBytes, _ := json.Marshal(deltaEvent)
			events = append(events, deltaBytes)
		}

		if toolCalls, ok := delta["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
			for _, tc := range toolCalls {
				tcMap, ok := tc.(map[string]interface{})
				if !ok {
					continue
				}
				functionMap, _ := tcMap["function"].(map[string]interface{})
				toolCallID := getString(tcMap, "id")
				toolName := getString(functionMap, "name")
				arguments := getString(functionMap, "arguments")

				if toolCallID != "" {
					if state.contentBlockStarted {
						stopEvent := map[string]interface{}{
							"type":  "content_block_stop",
							"index": state.contentBlockIndex,
						}
						stopBytes, _ := json.Marshal(stopEvent)
						events = append(events, stopBytes)
						state.contentBlockIndex++
					}
					startEvent := map[string]interface{}{
						"type":  "content_block_start",
						"index": state.contentBlockIndex,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    toolCallID,
							"name":  toolName,
							"input": map[string]interface{}{},
						},
					}
					startBytes, _ := json.Marshal(startEvent)
					events = append(events, startBytes)
					state.contentBlockStarted = true
					state.currentBlockType = "tool_use"
					if arguments != "" {
						deltaEvent := map[string]interface{}{
							"type":  "content_block_delta",
							"index": state.contentBlockIndex,
							"delta": map[string]interface{}{
								"type":         "input_json_delta",
								"partial_json": arguments,
							},
						}
						deltaBytes, _ := json.Marshal(deltaEvent)
						events = append(events, deltaBytes)
					}
					continue
				}

				if arguments != "" && state.contentBlockStarted && state.currentBlockType == "tool_use" {
					deltaEvent := map[string]interface{}{
						"type":  "content_block_delta",
						"index": state.contentBlockIndex,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": arguments,
						},
					}
					deltaBytes, _ := json.Marshal(deltaEvent)
					events = append(events, deltaBytes)
				}
			}
		}
	}

	if state.finishReason != "" {
		if state.contentBlockStarted {
			stopEvent := map[string]interface{}{
				"type":  "content_block_stop",
				"index": state.contentBlockIndex,
			}
			stopBytes, _ := json.Marshal(stopEvent)
			events = append(events, stopBytes)
			state.contentBlockStarted = false
			state.currentBlockType = ""
		}

		messageDelta := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": mapFinishReason(state.finishReason),
			},
		}
		if usageMap, ok := data["usage"].(map[string]interface{}); ok {
			messageDelta["usage"] = map[string]interface{}{
				"output_tokens": getInt(usageMap, "completion_tokens"),
			}
		}
		messageDeltaBytes, _ := json.Marshal(messageDelta)
		events = append(events, messageDeltaBytes)

		messageStopBytes, _ := json.Marshal(map[string]interface{}{"type": "message_stop"})
		events = append(events, messageStopBytes)
		state.finished = true
	}

	return events, nil
}

func mapFinishReason(finishReason string) string {
	switch finishReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "":
		return "end_turn"
	default:
		return finishReason
	}
}
