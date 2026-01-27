package converters

import (
	"encoding/json"

	"ai_gateway/internal/models"
)

// AnthropicToGeminiRequest converts an Anthropic request to Gemini format
func AnthropicToGeminiRequest(req *models.MessagesRequest) (*models.GenerateContentRequest, error) {
	geminiReq := &models.GenerateContentRequest{}

	// Set generation config
	geminiReq.GenerationConfig = &models.GenerationConfig{}
	if req.Temperature != nil {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}
	if req.TopP != nil {
		geminiReq.GenerationConfig.TopP = req.TopP
	}
	if req.TopK != nil {
		geminiReq.GenerationConfig.TopK = req.TopK
	}
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = &req.MaxTokens
	}
	if len(req.StopSequences) > 0 {
		geminiReq.GenerationConfig.StopSequences = req.StopSequences
	}

	// Convert system message
	if req.System != nil {
		var systemText string
		switch v := req.System.(type) {
		case string:
			systemText = v
		case []interface{}:
			for _, block := range v {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if blockMap["type"] == "text" {
						systemText += getString(blockMap, "text")
					}
				}
			}
		}
		if systemText != "" {
			geminiReq.SystemInstruction = &models.GeminiContent{
				Parts: []models.GeminiPart{{Text: systemText}},
			}
		}
	}

	// Convert messages
	var contents []models.GeminiContent
	for _, msg := range req.Messages {
		geminiContent := models.GeminiContent{}

		// Map role
		switch msg.Role {
		case "assistant":
			geminiContent.Role = "model"
		default:
			geminiContent.Role = "user"
		}

		// Handle content
		switch content := msg.Content.(type) {
		case string:
			geminiContent.Parts = []models.GeminiPart{{Text: content}}
		case []interface{}:
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType := getString(blockMap, "type")
					switch blockType {
					case "text":
						geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{
							Text: getString(blockMap, "text"),
						})
					case "tool_use":
						args, _ := blockMap["input"].(map[string]interface{})
						geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{
							FunctionCall: &models.GeminiFunctionCall{
								Name: getString(blockMap, "name"),
								Args: args,
							},
						})
					case "tool_result":
						var responseContent interface{}
						if c, ok := blockMap["content"].(string); ok {
							json.Unmarshal([]byte(c), &responseContent)
						} else {
							responseContent = blockMap["content"]
						}
						// Tool results go in a user message
						geminiContent.Role = "user"
						geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{
							FunctionResponse: &models.FunctionResponse{
								Name:     getString(blockMap, "name"),
								Response: map[string]interface{}{"result": responseContent},
							},
						})
					case "image":
						if source, ok := blockMap["source"].(map[string]interface{}); ok {
							geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{
								InlineData: &models.InlineData{
									MimeType: getString(source, "media_type"),
									Data:     getString(source, "data"),
								},
							})
						}
					}
				}
			}
		}

		if len(geminiContent.Parts) > 0 {
			contents = append(contents, geminiContent)
		}
	}
	geminiReq.Contents = contents

	// Convert tools
	if len(req.Tools) > 0 {
		var declarations []models.FunctionDeclaration
		for _, tool := range req.Tools {
			declarations = append(declarations, models.FunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
		geminiReq.Tools = []models.GeminiTool{{
			FunctionDeclarations: declarations,
		}}
	}

	return geminiReq, nil
}

// GeminiToAnthropicResponse converts a Gemini response to Anthropic format
func GeminiToAnthropicResponse(resp map[string]interface{}, model string) (*models.MessagesResponse, error) {
	anthropicResp := &models.MessagesResponse{
		ID:    generateID(),
		Type:  "message",
		Role:  "assistant",
		Model: model,
	}

	candidates, ok := resp["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return anthropicResp, nil
	}

	candidate := candidates[0].(map[string]interface{})
	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return anthropicResp, nil
	}

	parts, ok := content["parts"].([]interface{})
	if !ok {
		return anthropicResp, nil
	}

	var contentBlocks []models.ContentBlock
	for _, part := range parts {
		partMap := part.(map[string]interface{})
		if text, ok := partMap["text"].(string); ok {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "text",
				Text: text,
			})
		}
		if fc, ok := partMap["functionCall"].(map[string]interface{}); ok {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type:  "tool_use",
				ID:    generateToolCallID(len(contentBlocks)),
				Name:  getString(fc, "name"),
				Input: fc["args"],
			})
		}
	}
	anthropicResp.Content = contentBlocks

	// Convert finish reason
	if fr, ok := candidate["finishReason"].(string); ok {
		var stopReason string
		switch fr {
		case "STOP":
			stopReason = "end_turn"
		case "MAX_TOKENS":
			stopReason = "max_tokens"
		default:
			if len(contentBlocks) > 0 {
				for _, block := range contentBlocks {
					if block.Type == "tool_use" {
						stopReason = "tool_use"
						break
					}
				}
			}
			if stopReason == "" {
				stopReason = "end_turn"
			}
		}
		anthropicResp.StopReason = &stopReason
	}

	// Convert usage
	if usage, ok := resp["usageMetadata"].(map[string]interface{}); ok {
		anthropicResp.Usage = models.AnthropicUsage{
			InputTokens:  getInt(usage, "promptTokenCount"),
			OutputTokens: getInt(usage, "candidatesTokenCount"),
		}
	}

	return anthropicResp, nil
}

// GeminiStreamToAnthropicStream converts a Gemini stream event to Anthropic format
func GeminiStreamToAnthropicStream(data map[string]interface{}, isFirst bool, model string) ([][]byte, error) {
	var events [][]byte

	candidates, ok := data["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return nil, nil
	}

	candidate := candidates[0].(map[string]interface{})
	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return nil, nil
	}

	if isFirst {
		// Send message_start event
		startEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":          generateID(),
				"type":        "message",
				"role":        "assistant",
				"content":     []interface{}{},
				"model":       model,
				"stop_reason": nil,
				"usage":       map[string]interface{}{"input_tokens": 0, "output_tokens": 0},
			},
		}
		startBytes, _ := json.Marshal(startEvent)
		events = append(events, startBytes)

		// Send content_block_start
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

	part := parts[0].(map[string]interface{})
	if text, ok := part["text"].(string); ok {
		deltaEvent := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": text,
			},
		}
		deltaBytes, _ := json.Marshal(deltaEvent)
		events = append(events, deltaBytes)
	}

	// Handle finish
	if fr, ok := candidate["finishReason"].(string); ok && fr != "" {
		// content_block_stop
		stopBlockEvent := map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}
		stopBlockBytes, _ := json.Marshal(stopBlockEvent)
		events = append(events, stopBlockBytes)

		// message_delta
		var stopReason string
		switch fr {
		case "STOP":
			stopReason = "end_turn"
		case "MAX_TOKENS":
			stopReason = "max_tokens"
		default:
			stopReason = "end_turn"
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

		// message_stop
		messageStopEvent := map[string]interface{}{
			"type": "message_stop",
		}
		messageStopBytes, _ := json.Marshal(messageStopEvent)
		events = append(events, messageStopBytes)
	}

	return events, nil
}
