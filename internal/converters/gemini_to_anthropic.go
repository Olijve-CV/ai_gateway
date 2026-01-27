package converters

import (
	"encoding/json"

	"ai_gateway/internal/models"
)

// GeminiToAnthropicRequest converts a Gemini request to Anthropic format
func GeminiToAnthropicRequest(req *models.GenerateContentRequest, model string) (*models.MessagesRequest, error) {
	anthropicReq := &models.MessagesRequest{
		Model:     model,
		MaxTokens: 4096, // Default
	}

	// Convert generation config
	if req.GenerationConfig != nil {
		anthropicReq.Temperature = req.GenerationConfig.Temperature
		anthropicReq.TopP = req.GenerationConfig.TopP
		anthropicReq.TopK = req.GenerationConfig.TopK
		if req.GenerationConfig.MaxOutputTokens != nil {
			anthropicReq.MaxTokens = *req.GenerationConfig.MaxOutputTokens
		}
		anthropicReq.StopSequences = req.GenerationConfig.StopSequences
	}

	// Convert system instruction
	if req.SystemInstruction != nil && len(req.SystemInstruction.Parts) > 0 {
		var systemText string
		for _, part := range req.SystemInstruction.Parts {
			systemText += part.Text
		}
		if systemText != "" {
			anthropicReq.System = systemText
		}
	}

	// Convert contents to messages
	var messages []models.AnthropicMessage
	for _, content := range req.Contents {
		msg := models.AnthropicMessage{}

		// Map role
		switch content.Role {
		case "model":
			msg.Role = "assistant"
		default:
			msg.Role = "user"
		}

		var contentBlocks []models.ContentBlock
		for _, part := range content.Parts {
			if part.Text != "" {
				contentBlocks = append(contentBlocks, models.ContentBlock{
					Type: "text",
					Text: part.Text,
				})
			}
			if part.FunctionCall != nil {
				contentBlocks = append(contentBlocks, models.ContentBlock{
					Type:  "tool_use",
					ID:    generateToolCallID(len(contentBlocks)),
					Name:  part.FunctionCall.Name,
					Input: part.FunctionCall.Args,
				})
			}
			if part.FunctionResponse != nil {
				contentBlocks = append(contentBlocks, models.ContentBlock{
					Type:      "tool_result",
					ToolUseID: generateToolCallID(0), // Gemini doesn't have IDs
					Content:   part.FunctionResponse.Response,
				})
			}
			if part.InlineData != nil {
				contentBlocks = append(contentBlocks, models.ContentBlock{
					Type: "image",
					Source: &models.ImageSource{
						Type:      "base64",
						MediaType: part.InlineData.MimeType,
						Data:      part.InlineData.Data,
					},
				})
			}
		}

		if len(contentBlocks) > 0 {
			msg.Content = contentBlocks
			messages = append(messages, msg)
		}
	}
	anthropicReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []models.AnthropicTool
		for _, tool := range req.Tools {
			for _, decl := range tool.FunctionDeclarations {
				tools = append(tools, models.AnthropicTool{
					Name:        decl.Name,
					Description: decl.Description,
					InputSchema: decl.Parameters,
				})
			}
		}
		anthropicReq.Tools = tools
	}

	return anthropicReq, nil
}

// AnthropicToGeminiResponse converts an Anthropic response to Gemini format
func AnthropicToGeminiResponse(resp map[string]interface{}) (*models.GenerateContentResponse, error) {
	geminiResp := &models.GenerateContentResponse{}

	content, ok := resp["content"].([]interface{})
	if !ok {
		return geminiResp, nil
	}

	var parts []models.GeminiPart
	for _, block := range content {
		blockMap := block.(map[string]interface{})
		blockType := getString(blockMap, "type")

		switch blockType {
		case "text":
			parts = append(parts, models.GeminiPart{
				Text: getString(blockMap, "text"),
			})
		case "tool_use":
			args, _ := blockMap["input"].(map[string]interface{})
			parts = append(parts, models.GeminiPart{
				FunctionCall: &models.GeminiFunctionCall{
					Name: getString(blockMap, "name"),
					Args: args,
				},
			})
		}
	}

	// Convert stop reason
	var finishReason string
	if stopReason, ok := resp["stop_reason"].(string); ok {
		switch stopReason {
		case "end_turn":
			finishReason = "STOP"
		case "max_tokens":
			finishReason = "MAX_TOKENS"
		case "tool_use":
			finishReason = "STOP"
		default:
			finishReason = "STOP"
		}
	}

	geminiResp.Candidates = []models.Candidate{{
		Content: &models.GeminiContent{
			Role:  "model",
			Parts: parts,
		},
		FinishReason: finishReason,
		Index:        0,
	}}

	// Convert usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		inputTokens := getInt(usage, "input_tokens")
		outputTokens := getInt(usage, "output_tokens")
		geminiResp.UsageMetadata = &models.UsageMetadata{
			PromptTokenCount:     inputTokens,
			CandidatesTokenCount: outputTokens,
			TotalTokenCount:      inputTokens + outputTokens,
		}
	}

	return geminiResp, nil
}

// AnthropicStreamToGeminiStream converts an Anthropic stream event to Gemini format
func AnthropicStreamToGeminiStream(eventType string, data map[string]interface{}) ([]byte, error) {
	switch eventType {
	case "content_block_delta":
		delta := data["delta"].(map[string]interface{})
		deltaType := getString(delta, "type")

		if deltaType == "text_delta" {
			text := getString(delta, "text")
			resp := models.GenerateContentResponse{
				Candidates: []models.Candidate{{
					Content: &models.GeminiContent{
						Role:  "model",
						Parts: []models.GeminiPart{{Text: text}},
					},
					Index: 0,
				}},
			}
			return json.Marshal(resp)
		}

	case "message_delta":
		delta := data["delta"].(map[string]interface{})
		stopReason := getString(delta, "stop_reason")

		var finishReason string
		switch stopReason {
		case "end_turn":
			finishReason = "STOP"
		case "max_tokens":
			finishReason = "MAX_TOKENS"
		default:
			finishReason = "STOP"
		}

		resp := models.GenerateContentResponse{
			Candidates: []models.Candidate{{
				Content: &models.GeminiContent{
					Role:  "model",
					Parts: []models.GeminiPart{},
				},
				FinishReason: finishReason,
				Index:        0,
			}},
		}
		return json.Marshal(resp)
	}

	return nil, nil
}
