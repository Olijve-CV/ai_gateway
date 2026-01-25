package converters

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ai_gateway/internal/models"
)

// OpenAIChatToOpenAIResponsesRequest converts OpenAI chat request to Responses API format.
func OpenAIChatToOpenAIResponsesRequest(req *models.ChatCompletionRequest) (map[string]interface{}, error) {
	if req == nil {
		return nil, errors.New("request is nil")
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
	if req.MaxTokens != nil {
		result["max_output_tokens"] = *req.MaxTokens
	}
	if req.Stop != nil {
		result["stop"] = req.Stop
	}
	if req.User != "" {
		result["user"] = req.User
	}
	if req.Seed != nil {
		result["seed"] = *req.Seed
	}
	if req.LogProbs != nil {
		result["logprobs"] = *req.LogProbs
	}
	if req.TopLogProbs != nil {
		result["top_logprobs"] = *req.TopLogProbs
	}
	if req.ToolChoice != nil {
		result["tool_choice"] = req.ToolChoice
	}
	if req.ResponseFormat != nil {
		result["response_format"] = map[string]interface{}{
			"type": req.ResponseFormat.Type,
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []map[string]interface{}
		for _, tool := range req.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			})
		}
		result["tools"] = tools
	}

	// Convert messages
	var input []map[string]interface{}
	var instructions string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			instructions += getTextContent(msg.Content)
			continue
		}

		item := map[string]interface{}{}
		if msg.Role == "tool" {
			item["type"] = "function_call_output"
			if msg.ToolCallID != "" {
				item["call_id"] = msg.ToolCallID
			}
			item["output"] = msg.Content
		} else {
			item["role"] = msg.Role
			item["content"] = msg.Content
			if len(msg.ToolCalls) > 0 {
				var toolCalls []map[string]interface{}
				for _, tc := range msg.ToolCalls {
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Function.Name,
							"arguments": tc.Function.Arguments,
						},
					})
				}
				item["tool_calls"] = toolCalls
			}
		}

		input = append(input, item)
	}

	if instructions != "" {
		result["instructions"] = instructions
	}
	result["input"] = input

	return result, nil
}

// OpenAIResponsesToOpenAIChatRequest converts a Responses API request to OpenAI chat request.
func OpenAIResponsesToOpenAIChatRequest(req map[string]interface{}) (*models.ChatCompletionRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	chatReq := &models.ChatCompletionRequest{}

	if model, ok := req["model"].(string); ok {
		chatReq.Model = model
	}
	if stream, ok := req["stream"].(bool); ok {
		chatReq.Stream = stream
	}
	if temperature, ok := req["temperature"].(float64); ok {
		chatReq.Temperature = &temperature
	}
	if topP, ok := req["top_p"].(float64); ok {
		chatReq.TopP = &topP
	}
	if maxOutputTokens, ok := req["max_output_tokens"].(float64); ok {
		maxTokens := int(maxOutputTokens)
		chatReq.MaxTokens = &maxTokens
	}
	if stop, ok := req["stop"]; ok {
		chatReq.Stop = stop
	}
	if toolChoice, ok := req["tool_choice"]; ok {
		chatReq.ToolChoice = toolChoice
	}
	if responseFormat, ok := req["response_format"].(map[string]interface{}); ok {
		if formatType, ok := responseFormat["type"].(string); ok {
			chatReq.ResponseFormat = &models.ResponseFormat{Type: formatType}
		}
	}
	if user, ok := req["user"].(string); ok {
		chatReq.User = user
	}
	if seed, ok := req["seed"].(float64); ok {
		seedInt := int(seed)
		chatReq.Seed = &seedInt
	}
	if logprobs, ok := req["logprobs"].(bool); ok {
		chatReq.LogProbs = &logprobs
	}
	if topLogprobs, ok := req["top_logprobs"].(float64); ok {
		topLogprobsInt := int(topLogprobs)
		chatReq.TopLogProbs = &topLogprobsInt
	}

	// Convert tools
	if tools, ok := req["tools"].([]interface{}); ok {
		var result []models.Tool
		for _, tool := range tools {
			toolMap, ok := tool.(map[string]interface{})
			if !ok {
				continue
			}
			functionMap, _ := toolMap["function"].(map[string]interface{})
			result = append(result, models.Tool{
				Type: "function",
				Function: models.Function{
					Name:        getString(functionMap, "name"),
					Description: getString(functionMap, "description"),
					Parameters:  functionMap["parameters"],
				},
			})
		}
		chatReq.Tools = result
	}

	var messages []models.ChatMessage

	if instructions, ok := req["instructions"].(string); ok && instructions != "" {
		messages = append(messages, models.ChatMessage{
			Role:    "system",
			Content: instructions,
		})
	}

	switch input := req["input"].(type) {
	case string:
		messages = append(messages, models.ChatMessage{
			Role:    "user",
			Content: input,
		})
	case []interface{}:
		for _, item := range input {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			if getString(itemMap, "type") == "function_call_output" {
				msg := models.ChatMessage{
					Role:      "tool",
					ToolCallID: getString(itemMap, "call_id"),
				}
				if output, ok := itemMap["output"]; ok {
					msg.Content = output
				}
				messages = append(messages, msg)
				continue
			}

			role := getString(itemMap, "role")
			if role == "" {
				continue
			}

			msg := models.ChatMessage{
				Role: role,
			}

			if content, ok := itemMap["content"]; ok {
				msg.Content = content
			}

			if toolCalls, ok := itemMap["tool_calls"].([]interface{}); ok {
				var parsed []models.ToolCall
				for _, tc := range toolCalls {
					tcMap, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					functionMap, _ := tcMap["function"].(map[string]interface{})
					parsed = append(parsed, models.ToolCall{
						ID:   getString(tcMap, "id"),
						Type: "function",
						Function: models.FunctionCall{
							Name:      getString(functionMap, "name"),
							Arguments: getString(functionMap, "arguments"),
						},
					})
				}
				msg.ToolCalls = parsed
			}

			messages = append(messages, msg)
		}
	}

	chatReq.Messages = messages

	return chatReq, nil
}

// OpenAIResponsesToOpenAIChatResponse converts a Responses API response to OpenAI chat response.
func OpenAIResponsesToOpenAIChatResponse(resp map[string]interface{}, model string) (*models.ChatCompletionResponse, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}

	response := &models.ChatCompletionResponse{
		ID:      getString(resp, "id"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	if created := getInt(resp, "created"); created > 0 {
		response.Created = int64(created)
	}
	if modelValue := getString(resp, "model"); modelValue != "" {
		response.Model = modelValue
	}

	var contentText string
	var toolCalls []models.ToolCall

	if output, ok := resp["output"].([]interface{}); ok {
		for _, item := range output {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(itemMap, "type") {
			case "message":
				if contentArr, ok := itemMap["content"].([]interface{}); ok {
					for _, contentItem := range contentArr {
						contentMap, ok := contentItem.(map[string]interface{})
						if !ok {
							continue
						}
						contentType := getString(contentMap, "type")
						if contentType == "output_text" || contentType == "text" {
							contentText += getString(contentMap, "text")
						}
					}
				}
			case "function_call":
				toolCalls = append(toolCalls, models.ToolCall{
					ID:   getString(itemMap, "call_id"),
					Type: "function",
					Function: models.FunctionCall{
						Name:      getString(itemMap, "name"),
						Arguments: getString(itemMap, "arguments"),
					},
				})
			}
		}
	}

	message := &models.ChatMessage{Role: "assistant"}
	if contentText != "" {
		message.Content = contentText
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	finishReason := "stop"
	status := getString(resp, "status")
	if status == "incomplete" {
		finishReason = "length"
		if details, ok := resp["incomplete_details"].(map[string]interface{}); ok {
			if getString(details, "reason") != "max_output_tokens" {
				finishReason = "stop"
			}
		}
	}
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	response.Choices = []models.Choice{{
		Index:        0,
		Message:      message,
		FinishReason: &finishReason,
	}}

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		inputTokens := getInt(usage, "input_tokens")
		outputTokens := getInt(usage, "output_tokens")
		if inputTokens == 0 && outputTokens == 0 {
			inputTokens = getInt(usage, "prompt_tokens")
			outputTokens = getInt(usage, "completion_tokens")
		}
		response.Usage = &models.Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		}
	}

	return response, nil
}

// OpenAIChatResponseToOpenAIResponsesResponse converts an OpenAI chat response to Responses API format.
func OpenAIChatResponseToOpenAIResponsesResponse(resp *models.ChatCompletionResponse) (map[string]interface{}, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}

	result := map[string]interface{}{
		"id":     resp.ID,
		"object": "response",
		"model":  resp.Model,
		"status": "completed",
	}

	if resp.Created != 0 {
		result["created"] = resp.Created
	}

	var output []map[string]interface{}
	var finishReason string

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.FinishReason != nil {
			finishReason = *choice.FinishReason
		}
		if choice.Message != nil {
			contentText := ""
			if content, ok := choice.Message.Content.(string); ok {
				contentText = content
			} else {
				contentText = getTextContent(choice.Message.Content)
			}
			if contentText != "" {
				output = append(output, map[string]interface{}{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{{
						"type": "output_text",
						"text": contentText,
					}},
				})
			}
			for _, tc := range choice.Message.ToolCalls {
				output = append(output, map[string]interface{}{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}
		}
	}

	if finishReason == "length" {
		result["status"] = "incomplete"
		result["incomplete_details"] = map[string]interface{}{
			"reason": "max_output_tokens",
		}
	}

	if len(output) > 0 {
		result["output"] = output
	}

	if resp.Usage != nil {
		result["usage"] = map[string]interface{}{
			"input_tokens":       resp.Usage.PromptTokens,
			"output_tokens":      resp.Usage.CompletionTokens,
			"total_tokens":       resp.Usage.TotalTokens,
			"prompt_tokens":      resp.Usage.PromptTokens,
			"completion_tokens":  resp.Usage.CompletionTokens,
		}
	}

	return result, nil
}

// OpenAIChatMapToOpenAIResponsesResponse converts a chat response map to Responses API format.
func OpenAIChatMapToOpenAIResponsesResponse(resp map[string]interface{}, model string) (map[string]interface{}, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	var chatResp models.ChatCompletionResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return nil, err
	}

	if chatResp.Model == "" {
		chatResp.Model = model
	}

	return OpenAIChatResponseToOpenAIResponsesResponse(&chatResp)
}

// ChatCompletionResponseToMap converts a ChatCompletionResponse to a map.
func ChatCompletionResponseToMap(resp *models.ChatCompletionResponse) (map[string]interface{}, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

type toolCallMeta struct {
	id   string
	name string
}

// OpenAIResponsesToChatStreamState stores state for converting Responses stream to chat stream.
type OpenAIResponsesToChatStreamState struct {
	id         string
	model      string
	started    bool
	sawToolCall bool
	toolCalls  map[int]toolCallMeta
}

// NewOpenAIResponsesToChatStreamState creates a new stream state.
func NewOpenAIResponsesToChatStreamState(model string) *OpenAIResponsesToChatStreamState {
	return &OpenAIResponsesToChatStreamState{
		model:     model,
		toolCalls: map[int]toolCallMeta{},
	}
}

// OpenAIResponsesStreamToOpenAIChatStream converts a Responses stream event to chat completion chunks.
func OpenAIResponsesStreamToOpenAIChatStream(data map[string]interface{}, state *OpenAIResponsesToChatStreamState) ([][]byte, error) {
	if state == nil {
		state = NewOpenAIResponsesToChatStreamState("")
	}

	var chunks [][]byte
	eventType := getString(data, "type")

	startChunk := func() {
		if state.started {
			return
		}
		chunk := state.newChunk()
		chunk.Choices[0].Delta = &models.ChatMessage{Role: "assistant"}
		chunkBytes, _ := json.Marshal(chunk)
		chunks = append(chunks, chunkBytes)
		state.started = true
	}

	switch eventType {
	case "response.created":
		response, _ := data["response"].(map[string]interface{})
		if state.id == "" {
			state.id = getString(response, "id")
		}
		if state.id == "" {
			state.id = generateID()
		}
		if state.model == "" {
			state.model = getString(response, "model")
		}
		startChunk()

	case "response.output_item.added":
		startChunk()
		index := getInt(data, "output_index")
		item, _ := data["item"].(map[string]interface{})
		if getString(item, "type") == "function_call" {
			callID := getString(item, "call_id")
			name := getString(item, "name")
			state.toolCalls[index] = toolCallMeta{id: callID, name: name}
			state.sawToolCall = true

			chunk := state.newChunk()
			chunk.Choices[0].Delta = &models.ChatMessage{
				ToolCalls: []models.ToolCall{{
					ID:   callID,
					Type: "function",
					Function: models.FunctionCall{
						Name:      name,
						Arguments: "",
					},
				}},
			}
			chunkBytes, _ := json.Marshal(chunk)
			chunks = append(chunks, chunkBytes)
		}

	case "response.output_text.delta":
		startChunk()
		delta := getString(data, "delta")
		if delta != "" {
			chunk := state.newChunk()
			chunk.Choices[0].Delta = &models.ChatMessage{Content: delta}
			chunkBytes, _ := json.Marshal(chunk)
			chunks = append(chunks, chunkBytes)
		}

	case "response.function_call_arguments.delta":
		startChunk()
		index := getInt(data, "output_index")
		meta := state.toolCalls[index]
		delta := getString(data, "delta")
		if delta != "" {
			chunk := state.newChunk()
			chunk.Choices[0].Delta = &models.ChatMessage{
				ToolCalls: []models.ToolCall{{
					ID:   meta.id,
					Type: "function",
					Function: models.FunctionCall{
						Name:      meta.name,
						Arguments: delta,
					},
				}},
			}
			chunkBytes, _ := json.Marshal(chunk)
			chunks = append(chunks, chunkBytes)
		}

	case "response.completed":
		startChunk()
		response, _ := data["response"].(map[string]interface{})
		status := getString(response, "status")

		finishReason := "stop"
		if status == "incomplete" {
			finishReason = "length"
		}
		if details, ok := response["incomplete_details"].(map[string]interface{}); ok {
			if getString(details, "reason") == "max_output_tokens" {
				finishReason = "length"
			}
		}
		if state.sawToolCall {
			finishReason = "tool_calls"
		}

		chunk := state.newChunk()
		chunk.Choices[0].FinishReason = &finishReason
		chunkBytes, _ := json.Marshal(chunk)
		chunks = append(chunks, chunkBytes)
	}

	return chunks, nil
}

func (s *OpenAIResponsesToChatStreamState) newChunk() models.ChatCompletionChunk {
	if s.id == "" {
		s.id = generateID()
	}
	return models.ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []models.Choice{{
			Index: 0,
		}},
	}
}

// OpenAIChatToResponsesStreamState stores state for converting chat stream to Responses stream.
type OpenAIChatToResponsesStreamState struct {
	responseID      string
	model           string
	created         bool
	messageStarted  bool
	nextOutputIndex int
	toolCallIndices map[string]int
}

// NewOpenAIChatToResponsesStreamState creates a new stream state.
func NewOpenAIChatToResponsesStreamState(model string) *OpenAIChatToResponsesStreamState {
	return &OpenAIChatToResponsesStreamState{
		model:           model,
		nextOutputIndex: 1,
		toolCallIndices: map[string]int{},
	}
}

// OpenAIChatStreamToOpenAIResponsesStream converts a chat completion chunk to Responses stream events.
func OpenAIChatStreamToOpenAIResponsesStream(chunk *models.ChatCompletionChunk, state *OpenAIChatToResponsesStreamState) ([][]byte, error) {
	if chunk == nil || len(chunk.Choices) == 0 {
		return nil, nil
	}
	if state == nil {
		state = NewOpenAIChatToResponsesStreamState("")
	}

	choice := chunk.Choices[0]

	if state.responseID == "" {
		if chunk.ID != "" {
			state.responseID = chunk.ID
		} else {
			state.responseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
		}
	}
	if state.model == "" && chunk.Model != "" {
		state.model = chunk.Model
	}

	var events [][]byte

	if !state.created {
		createdEvent := map[string]interface{}{
			"type": "response.created",
			"response": map[string]interface{}{
				"id":     state.responseID,
				"model":  state.model,
				"status": "in_progress",
			},
		}
		createdBytes, _ := json.Marshal(createdEvent)
		events = append(events, createdBytes)
		state.created = true
	}

	if !state.messageStarted {
		messageStartEvent := map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item": map[string]interface{}{
				"id":      fmt.Sprintf("msg_%s", state.responseID),
				"type":    "message",
				"role":    "assistant",
				"content": []interface{}{},
			},
		}
		messageStartBytes, _ := json.Marshal(messageStartEvent)
		events = append(events, messageStartBytes)

		contentPartEvent := map[string]interface{}{
			"type":         "response.content_part.added",
			"output_index": 0,
			"content_index": 0,
			"part": map[string]interface{}{
				"type": "output_text",
				"text": "",
			},
		}
		contentPartBytes, _ := json.Marshal(contentPartEvent)
		events = append(events, contentPartBytes)

		state.messageStarted = true
	}

	if choice.Delta != nil {
		if content, ok := choice.Delta.Content.(string); ok && content != "" {
			textDeltaEvent := map[string]interface{}{
				"type":         "response.output_text.delta",
				"output_index": 0,
				"content_index": 0,
				"delta":        content,
			}
			textDeltaBytes, _ := json.Marshal(textDeltaEvent)
			events = append(events, textDeltaBytes)
		}

		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				callID := tc.ID
				if callID == "" {
					callID = fmt.Sprintf("call_%d", state.nextOutputIndex)
				}
				index, ok := state.toolCallIndices[callID]
				if !ok {
					index = state.nextOutputIndex
					state.toolCallIndices[callID] = index
					state.nextOutputIndex++

					itemAddedEvent := map[string]interface{}{
						"type":         "response.output_item.added",
						"output_index": index,
						"item": map[string]interface{}{
							"type":      "function_call",
							"call_id":   callID,
							"name":      tc.Function.Name,
							"arguments": "",
						},
					}
					itemAddedBytes, _ := json.Marshal(itemAddedEvent)
					events = append(events, itemAddedBytes)
				}

				if tc.Function.Arguments != "" {
					argsDeltaEvent := map[string]interface{}{
						"type":         "response.function_call_arguments.delta",
						"output_index": index,
						"delta":        tc.Function.Arguments,
					}
					argsDeltaBytes, _ := json.Marshal(argsDeltaEvent)
					events = append(events, argsDeltaBytes)
				}
			}
		}
	}

	if choice.FinishReason != nil && *choice.FinishReason != "" {
		finishReason := *choice.FinishReason

		if state.messageStarted {
			doneEvent := map[string]interface{}{
				"type":         "response.output_item.done",
				"output_index": 0,
			}
			doneBytes, _ := json.Marshal(doneEvent)
			events = append(events, doneBytes)
		}

		for _, index := range state.toolCallIndices {
			doneEvent := map[string]interface{}{
				"type":         "response.output_item.done",
				"output_index": index,
			}
			doneBytes, _ := json.Marshal(doneEvent)
			events = append(events, doneBytes)
		}

		status := "completed"
		response := map[string]interface{}{
			"id":     state.responseID,
			"model":  state.model,
			"status": status,
		}

		if finishReason == "length" {
			status = "incomplete"
			response["status"] = status
			response["incomplete_details"] = map[string]interface{}{
				"reason": "max_output_tokens",
			}
		}

		completedEvent := map[string]interface{}{
			"type":     "response.completed",
			"response": response,
		}
		completedBytes, _ := json.Marshal(completedEvent)
		events = append(events, completedBytes)
	}

	return events, nil
}
