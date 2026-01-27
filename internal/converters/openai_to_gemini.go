package converters

import (
	"encoding/json"
	"time"

	"ai_gateway/internal/models"
)

// OpenAIToGeminiRequest converts an OpenAI request to Gemini format
func OpenAIToGeminiRequest(req *models.ChatCompletionRequest) (*models.GenerateContentRequest, error) {
	geminiReq := &models.GenerateContentRequest{}

	// Set generation config
	geminiReq.GenerationConfig = &models.GenerationConfig{}
	if req.Temperature != nil {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}
	if req.TopP != nil {
		geminiReq.GenerationConfig.TopP = req.TopP
	}
	if req.MaxTokens != nil {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}

	// Convert stop sequences
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			geminiReq.GenerationConfig.StopSequences = []string{v}
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					geminiReq.GenerationConfig.StopSequences = append(geminiReq.GenerationConfig.StopSequences, str)
				}
			}
		}
	}

	// Convert messages
	var contents []models.GeminiContent
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system instruction
			content := getTextContent(msg.Content)
			geminiReq.SystemInstruction = &models.GeminiContent{
				Parts: []models.GeminiPart{{Text: content}},
			}
			continue
		}

		geminiContent := models.GeminiContent{}

		// Map role
		switch msg.Role {
		case "assistant":
			geminiContent.Role = "model"
		case "tool":
			// Tool response - add to previous model content or create new user content
			geminiContent.Role = "user"
			// Parse the content as function response
			var responseContent interface{}
			if str, ok := msg.Content.(string); ok {
				json.Unmarshal([]byte(str), &responseContent)
			} else {
				responseContent = msg.Content
			}
			geminiContent.Parts = []models.GeminiPart{{
				FunctionResponse: &models.FunctionResponse{
					Name:     msg.Name,
					Response: map[string]interface{}{"result": responseContent},
				},
			}}
			contents = append(contents, geminiContent)
			continue
		default:
			geminiContent.Role = "user"
		}

		// Handle tool calls from assistant
		if msg.ToolCalls != nil && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{
					FunctionCall: &models.GeminiFunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		}

		// Handle regular content
		content := getTextContent(msg.Content)
		if content != "" {
			geminiContent.Parts = append(geminiContent.Parts, models.GeminiPart{Text: content})
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
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			})
		}
		geminiReq.Tools = []models.GeminiTool{{
			FunctionDeclarations: declarations,
		}}
	}

	return geminiReq, nil
}

// GeminiToOpenAIResponse converts a Gemini response to OpenAI format
func GeminiToOpenAIResponse(resp map[string]interface{}, model string) (*models.ChatCompletionResponse, error) {
	openaiResp := &models.ChatCompletionResponse{
		ID:      generateID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	candidates, ok := resp["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return openaiResp, nil
	}

	candidate := candidates[0].(map[string]interface{})
	content := candidate["content"].(map[string]interface{})
	parts := content["parts"].([]interface{})

	var message models.ChatMessage
	message.Role = "assistant"

	var textContent string
	var toolCalls []models.ToolCall
	toolCallIndex := 0

	for _, part := range parts {
		partMap := part.(map[string]interface{})
		if text, ok := partMap["text"].(string); ok {
			textContent += text
		}
		if fc, ok := partMap["functionCall"].(map[string]interface{}); ok {
			args, _ := json.Marshal(fc["args"])
			toolCalls = append(toolCalls, models.ToolCall{
				ID:   generateToolCallID(toolCallIndex),
				Type: "function",
				Function: models.FunctionCall{
					Name:      getString(fc, "name"),
					Arguments: string(args),
				},
			})
			toolCallIndex++
		}
	}

	if textContent != "" {
		message.Content = textContent
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	// Convert finish reason
	var finishReason string
	if fr, ok := candidate["finishReason"].(string); ok {
		switch fr {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		default:
			if len(toolCalls) > 0 {
				finishReason = "tool_calls"
			} else {
				finishReason = "stop"
			}
		}
	}

	openaiResp.Choices = []models.Choice{{
		Index:        0,
		Message:      &message,
		FinishReason: &finishReason,
	}}

	// Convert usage
	if usage, ok := resp["usageMetadata"].(map[string]interface{}); ok {
		promptTokens := getInt(usage, "promptTokenCount")
		completionTokens := getInt(usage, "candidatesTokenCount")
		openaiResp.Usage = &models.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	return openaiResp, nil
}

// GeminiStreamToOpenAIStream converts a Gemini stream event to OpenAI format
func GeminiStreamToOpenAIStream(data map[string]interface{}, model string, id string) ([]byte, error) {
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

	chunk := models.ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
	}

	part := parts[0].(map[string]interface{})
	if text, ok := part["text"].(string); ok {
		chunk.Choices = []models.Choice{{
			Index: 0,
			Delta: &models.ChatMessage{Content: text},
		}}
	} else if fc, ok := part["functionCall"].(map[string]interface{}); ok {
		args, _ := json.Marshal(fc["args"])
		chunk.Choices = []models.Choice{{
			Index: 0,
			Delta: &models.ChatMessage{
				ToolCalls: []models.ToolCall{{
					ID:   generateToolCallID(0),
					Type: "function",
					Function: models.FunctionCall{
						Name:      getString(fc, "name"),
						Arguments: string(args),
					},
				}},
			},
		}}
	}

	// Check for finish reason
	if fr, ok := candidate["finishReason"].(string); ok {
		var finishReason string
		switch fr {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		default:
			finishReason = "stop"
		}
		if len(chunk.Choices) > 0 {
			chunk.Choices[0].FinishReason = &finishReason
		}
	}

	return json.Marshal(chunk)
}

func generateToolCallID(index int) string {
	return "call_" + time.Now().Format("20060102150405") + "_" + string(rune('a'+index))
}
