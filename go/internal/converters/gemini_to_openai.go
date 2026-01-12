package converters

import (
	"encoding/json"

	"ai_gateway/internal/models"
)

// GeminiToOpenAIRequest converts a Gemini request to OpenAI format
func GeminiToOpenAIRequest(req *models.GenerateContentRequest, model string) (*models.ChatCompletionRequest, error) {
	openaiReq := &models.ChatCompletionRequest{
		Model: model,
	}

	// Convert generation config
	if req.GenerationConfig != nil {
		openaiReq.Temperature = req.GenerationConfig.Temperature
		openaiReq.TopP = req.GenerationConfig.TopP
		openaiReq.MaxTokens = req.GenerationConfig.MaxOutputTokens
		if len(req.GenerationConfig.StopSequences) > 0 {
			openaiReq.Stop = req.GenerationConfig.StopSequences
		}
	}

	// Convert messages
	var messages []models.ChatMessage

	// Add system message if present
	if req.SystemInstruction != nil && len(req.SystemInstruction.Parts) > 0 {
		var systemText string
		for _, part := range req.SystemInstruction.Parts {
			systemText += part.Text
		}
		if systemText != "" {
			messages = append(messages, models.ChatMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert contents
	for _, content := range req.Contents {
		msg := models.ChatMessage{}

		// Map role
		switch content.Role {
		case "model":
			msg.Role = "assistant"
		default:
			msg.Role = "user"
		}

		var textContent string
		var toolCalls []models.ToolCall
		var hasFunctionResponse bool
		var functionResponseName string
		var functionResponseContent interface{}

		for _, part := range content.Parts {
			if part.Text != "" {
				textContent += part.Text
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, models.ToolCall{
					ID:   generateToolCallID(len(toolCalls)),
					Type: "function",
					Function: models.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				})
			}
			if part.FunctionResponse != nil {
				hasFunctionResponse = true
				functionResponseName = part.FunctionResponse.Name
				functionResponseContent = part.FunctionResponse.Response
			}
		}

		if hasFunctionResponse {
			// Tool result message
			contentBytes, _ := json.Marshal(functionResponseContent)
			messages = append(messages, models.ChatMessage{
				Role:       "tool",
				Name:       functionResponseName,
				ToolCallID: generateToolCallID(0), // Gemini doesn't have tool call IDs
				Content:    string(contentBytes),
			})
		} else {
			if textContent != "" {
				msg.Content = textContent
			}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
			if msg.Content != nil || len(msg.ToolCalls) > 0 {
				messages = append(messages, msg)
			}
		}
	}
	openaiReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []models.Tool
		for _, tool := range req.Tools {
			for _, decl := range tool.FunctionDeclarations {
				tools = append(tools, models.Tool{
					Type: "function",
					Function: models.Function{
						Name:        decl.Name,
						Description: decl.Description,
						Parameters:  decl.Parameters,
					},
				})
			}
		}
		openaiReq.Tools = tools
	}

	return openaiReq, nil
}

// OpenAIToGeminiResponse converts an OpenAI response to Gemini format
func OpenAIToGeminiResponse(resp map[string]interface{}) (*models.GenerateContentResponse, error) {
	geminiResp := &models.GenerateContentResponse{}

	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return geminiResp, nil
	}

	choice := choices[0].(map[string]interface{})
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return geminiResp, nil
	}

	var parts []models.GeminiPart

	// Handle text content
	if content, ok := message["content"].(string); ok && content != "" {
		parts = append(parts, models.GeminiPart{Text: content})
	}

	// Handle tool calls
	if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			tcMap := tc.(map[string]interface{})
			function := tcMap["function"].(map[string]interface{})
			var args map[string]interface{}
			if argsStr, ok := function["arguments"].(string); ok {
				json.Unmarshal([]byte(argsStr), &args)
			}
			parts = append(parts, models.GeminiPart{
				FunctionCall: &models.GeminiFunctionCall{
					Name: getString(function, "name"),
					Args: args,
				},
			})
		}
	}

	// Convert finish reason
	var finishReason string
	if fr, ok := choice["finish_reason"].(string); ok {
		switch fr {
		case "stop":
			finishReason = "STOP"
		case "length":
			finishReason = "MAX_TOKENS"
		case "tool_calls":
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
		geminiResp.UsageMetadata = &models.UsageMetadata{
			PromptTokenCount:     getInt(usage, "prompt_tokens"),
			CandidatesTokenCount: getInt(usage, "completion_tokens"),
			TotalTokenCount:      getInt(usage, "total_tokens"),
		}
	}

	return geminiResp, nil
}

// OpenAIStreamToGeminiStream converts an OpenAI stream chunk to Gemini format
func OpenAIStreamToGeminiStream(data map[string]interface{}) ([]byte, error) {
	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, nil
	}

	choice := choices[0].(map[string]interface{})
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	var parts []models.GeminiPart

	if content, ok := delta["content"].(string); ok && content != "" {
		parts = append(parts, models.GeminiPart{Text: content})
	}

	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			tcMap := tc.(map[string]interface{})
			if function, ok := tcMap["function"].(map[string]interface{}); ok {
				var args map[string]interface{}
				if argsStr, ok := function["arguments"].(string); ok {
					json.Unmarshal([]byte(argsStr), &args)
				}
				name := getString(function, "name")
				if name != "" || args != nil {
					parts = append(parts, models.GeminiPart{
						FunctionCall: &models.GeminiFunctionCall{
							Name: name,
							Args: args,
						},
					})
				}
			}
		}
	}

	if len(parts) == 0 {
		// Check for finish reason
		if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
			var geminiFinishReason string
			switch finishReason {
			case "stop":
				geminiFinishReason = "STOP"
			case "length":
				geminiFinishReason = "MAX_TOKENS"
			default:
				geminiFinishReason = "STOP"
			}

			resp := models.GenerateContentResponse{
				Candidates: []models.Candidate{{
					Content: &models.GeminiContent{
						Role:  "model",
						Parts: []models.GeminiPart{},
					},
					FinishReason: geminiFinishReason,
					Index:        0,
				}},
			}
			return json.Marshal(resp)
		}
		return nil, nil
	}

	resp := models.GenerateContentResponse{
		Candidates: []models.Candidate{{
			Content: &models.GeminiContent{
				Role:  "model",
				Parts: parts,
			},
			Index: 0,
		}},
	}

	return json.Marshal(resp)
}
