package converters

import (
	"encoding/json"
	"testing"

	"ai_gateway/internal/models"
)

func TestAnthropicToOpenAIRequest_SystemToolsAndMessages(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxTokens := 128

	req := &models.MessagesRequest{
		Model:         "claude-3",
		MaxTokens:     maxTokens,
		Temperature:   &temp,
		TopP:          &topP,
		StopSequences: []string{"stop1", "stop2"},
		Stream:        true,
		System: []interface{}{
			map[string]interface{}{"type": "text", "text": "sys1"},
			map[string]interface{}{"type": "text", "text": "sys2"},
		},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "hi"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "hello"},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "call1",
						"name":  "sum",
						"input": map[string]interface{}{"a": 1},
					},
				},
			},
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"id":          "call1",
						"content":     "42",
					},
				},
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "sum",
				Description: "sum",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}

	openaiReq, err := AnthropicToOpenAIRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAIRequest error: %v", err)
	}

	if openaiReq.Model != req.Model {
		t.Fatalf("model mismatch: got %s", openaiReq.Model)
	}
	if !openaiReq.Stream {
		t.Fatalf("expected stream to be true")
	}
	if openaiReq.MaxTokens == nil || *openaiReq.MaxTokens != maxTokens {
		t.Fatalf("max_tokens mismatch: got %v", openaiReq.MaxTokens)
	}
	if openaiReq.Temperature == nil || *openaiReq.Temperature != temp {
		t.Fatalf("temperature mismatch: got %v", openaiReq.Temperature)
	}
	if openaiReq.TopP == nil || *openaiReq.TopP != topP {
		t.Fatalf("top_p mismatch: got %v", openaiReq.TopP)
	}

	stopSequences, ok := openaiReq.Stop.([]string)
	if !ok || len(stopSequences) != 2 {
		t.Fatalf("stop sequences mismatch: %#v", openaiReq.Stop)
	}

	if len(openaiReq.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(openaiReq.Messages))
	}

	if openaiReq.Messages[0].Role != "system" || openaiReq.Messages[0].Content != "sys1sys2" {
		t.Fatalf("system message mismatch: %#v", openaiReq.Messages[0])
	}
	if openaiReq.Messages[1].Role != "user" || openaiReq.Messages[1].Content != "hi" {
		t.Fatalf("user message mismatch: %#v", openaiReq.Messages[1])
	}

	assistantMsg := openaiReq.Messages[2]
	if assistantMsg.Role != "assistant" || assistantMsg.Content != "hello" {
		t.Fatalf("assistant message mismatch: %#v", assistantMsg)
	}
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Function.Name != "sum" {
		t.Fatalf("tool name mismatch: %#v", assistantMsg.ToolCalls[0])
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(assistantMsg.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}
	if args["a"] != float64(1) {
		t.Fatalf("tool args mismatch: %#v", args)
	}

	toolMsg := openaiReq.Messages[3]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call1" || toolMsg.Content != "42" {
		t.Fatalf("tool result message mismatch: %#v", toolMsg)
	}

	if len(openaiReq.Tools) != 1 || openaiReq.Tools[0].Function.Name != "sum" {
		t.Fatalf("tools mismatch: %#v", openaiReq.Tools)
	}
}

func TestOpenAIToAnthropicRequest_ToolUseAndResult(t *testing.T) {
	maxTokens := 256
	req := &models.ChatCompletionRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Stop:      "done",
		Messages: []models.ChatMessage{
			{Role: "system", Content: "sys"},
			{
				Role:    "assistant",
				Content: "use tool",
				ToolCalls: []models.ToolCall{
					{
						ID:   "call1",
						Type: "function",
						Function: models.FunctionCall{
							Name:      "do",
							Arguments: `{"x":1}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call1",
				Content:    `{"result":"ok"}`,
			},
		},
	}

	anthropicReq, err := OpenAIToAnthropicRequest(req)
	if err != nil {
		t.Fatalf("OpenAIToAnthropicRequest error: %v", err)
	}

	if anthropicReq.System != "sys" {
		t.Fatalf("system mismatch: %v", anthropicReq.System)
	}
	if anthropicReq.MaxTokens != maxTokens {
		t.Fatalf("max_tokens mismatch: %d", anthropicReq.MaxTokens)
	}
	if len(anthropicReq.StopSequences) != 1 || anthropicReq.StopSequences[0] != "done" {
		t.Fatalf("stop sequences mismatch: %#v", anthropicReq.StopSequences)
	}

	if len(anthropicReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(anthropicReq.Messages))
	}

	assistantMsg := anthropicReq.Messages[0]
	if assistantMsg.Role != "assistant" {
		t.Fatalf("assistant role mismatch: %s", assistantMsg.Role)
	}
	blocks, ok := assistantMsg.Content.([]models.ContentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("assistant content blocks mismatch: %#v", assistantMsg.Content)
	}
	if blocks[0].Type != "text" || blocks[0].Text != "use tool" {
		t.Fatalf("assistant text block mismatch: %#v", blocks[0])
	}
	if blocks[1].Type != "tool_use" || blocks[1].ID != "call1" || blocks[1].Name != "do" {
		t.Fatalf("assistant tool block mismatch: %#v", blocks[1])
	}
	input, ok := blocks[1].Input.(map[string]interface{})
	if !ok || input["x"] != float64(1) {
		t.Fatalf("assistant tool input mismatch: %#v", blocks[1].Input)
	}

	toolMsg := anthropicReq.Messages[1]
	if toolMsg.Role != "user" {
		t.Fatalf("tool response role mismatch: %s", toolMsg.Role)
	}
	toolBlocks, ok := toolMsg.Content.([]models.ContentBlock)
	if !ok || len(toolBlocks) != 1 {
		t.Fatalf("tool response blocks mismatch: %#v", toolMsg.Content)
	}
	if toolBlocks[0].Type != "tool_result" || toolBlocks[0].ID != "call1" || toolBlocks[0].Content != `{"result":"ok"}` {
		t.Fatalf("tool response block mismatch: %#v", toolBlocks[0])
	}
}

func TestOpenAIToAnthropicResponse_ToolCallsUsageStopReason(t *testing.T) {
	resp := map[string]interface{}{
		"id": "resp1",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content": "hi",
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id": "call_1",
							"function": map[string]interface{}{
								"name":      "sum",
								"arguments": `{"a":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(5),
			"completion_tokens": float64(7),
		},
	}

	anthropicResp, err := OpenAIToAnthropicResponse(resp, "claude")
	if err != nil {
		t.Fatalf("OpenAIToAnthropicResponse error: %v", err)
	}

	if anthropicResp.ID != "resp1" || anthropicResp.Model != "claude" {
		t.Fatalf("response metadata mismatch: %#v", anthropicResp)
	}
	if anthropicResp.StopReason == nil || *anthropicResp.StopReason != "tool_use" {
		t.Fatalf("stop reason mismatch: %#v", anthropicResp.StopReason)
	}
	if anthropicResp.Usage.InputTokens != 5 || anthropicResp.Usage.OutputTokens != 7 {
		t.Fatalf("usage mismatch: %#v", anthropicResp.Usage)
	}

	if len(anthropicResp.Content) != 2 {
		t.Fatalf("content blocks mismatch: %#v", anthropicResp.Content)
	}
	if anthropicResp.Content[0].Type != "text" || anthropicResp.Content[0].Text != "hi" {
		t.Fatalf("text block mismatch: %#v", anthropicResp.Content[0])
	}
	if anthropicResp.Content[1].Type != "tool_use" || anthropicResp.Content[1].Name != "sum" {
		t.Fatalf("tool block mismatch: %#v", anthropicResp.Content[1])
	}
	input, ok := anthropicResp.Content[1].Input.(map[string]interface{})
	if !ok || input["a"] != float64(1) {
		t.Fatalf("tool input mismatch: %#v", anthropicResp.Content[1].Input)
	}
}

func TestAnthropicToOpenAIResponse_TextToolUseUsage(t *testing.T) {
	resp := map[string]interface{}{
		"id": "a1",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
			map[string]interface{}{
				"type":  "tool_use",
				"id":    "t1",
				"name":  "calc",
				"input": map[string]interface{}{"x": 2},
			},
		},
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  float64(2),
			"output_tokens": float64(3),
		},
	}

	openaiResp, err := AnthropicToOpenAIResponse(resp, "gpt")
	if err != nil {
		t.Fatalf("AnthropicToOpenAIResponse error: %v", err)
	}

	if len(openaiResp.Choices) != 1 || openaiResp.Choices[0].Message == nil {
		t.Fatalf("choices missing: %#v", openaiResp.Choices)
	}
	if openaiResp.Choices[0].Message.Content != "hello" {
		t.Fatalf("message content mismatch: %#v", openaiResp.Choices[0].Message)
	}
	if openaiResp.Choices[0].FinishReason == nil || *openaiResp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish reason mismatch: %#v", openaiResp.Choices[0].FinishReason)
	}
	if openaiResp.Usage == nil || openaiResp.Usage.TotalTokens != 5 {
		t.Fatalf("usage mismatch: %#v", openaiResp.Usage)
	}
	if len(openaiResp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool calls mismatch: %#v", openaiResp.Choices[0].Message.ToolCalls)
	}
}

func TestOpenAIStreamToAnthropicStream_FirstChunk(t *testing.T) {
	data := map[string]interface{}{
		"id":    "chunk1",
		"model": "gpt",
		"choices": []interface{}{
			map[string]interface{}{
				"delta":         map[string]interface{}{"content": "hi"},
				"finish_reason": "stop",
			},
		},
	}

	state := NewOpenAIToAnthropicStreamState()
	events, err := OpenAIStreamToAnthropicStream(data, state)
	if err != nil {
		t.Fatalf("OpenAIStreamToAnthropicStream error: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	var event map[string]interface{}
	if err := json.Unmarshal(events[0], &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "message_start" {
		t.Fatalf("event[0] type mismatch: %#v", event)
	}

	if err := json.Unmarshal(events[1], &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "content_block_start" {
		t.Fatalf("event[1] type mismatch: %#v", event)
	}

	if err := json.Unmarshal(events[2], &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "content_block_delta" {
		t.Fatalf("event[2] type mismatch: %#v", event)
	}
	delta := event["delta"].(map[string]interface{})
	if delta["text"] != "hi" {
		t.Fatalf("delta text mismatch: %#v", delta)
	}

	if err := json.Unmarshal(events[4], &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "message_delta" {
		t.Fatalf("event[4] type mismatch: %#v", event)
	}
	stopDelta := event["delta"].(map[string]interface{})
	if stopDelta["stop_reason"] != "end_turn" {
		t.Fatalf("stop reason mismatch: %#v", stopDelta)
	}

	if err := json.Unmarshal(events[5], &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "message_stop" {
		t.Fatalf("event[5] type mismatch: %#v", event)
	}
}

func TestAnthropicStreamToOpenAIStream_Deltas(t *testing.T) {
	t.Run("input_json_delta", func(t *testing.T) {
		data := map[string]interface{}{
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": `{"a":1}`,
			},
		}

		chunkBytes, err := AnthropicStreamToOpenAIStream("content_block_delta", data, "gpt", "id1")
		if err != nil {
			t.Fatalf("AnthropicStreamToOpenAIStream error: %v", err)
		}
		if chunkBytes == nil {
			t.Fatalf("expected chunk bytes")
		}

		var chunk models.ChatCompletionChunk
		if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
			t.Fatalf("unmarshal chunk: %v", err)
		}
		if len(chunk.Choices) != 1 || chunk.Choices[0].Delta == nil {
			t.Fatalf("chunk choices mismatch: %#v", chunk)
		}
		if len(chunk.Choices[0].Delta.ToolCalls) != 1 {
			t.Fatalf("tool calls mismatch: %#v", chunk.Choices[0].Delta.ToolCalls)
		}
		if chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments != `{"a":1}` {
			t.Fatalf("tool call arguments mismatch: %#v", chunk.Choices[0].Delta.ToolCalls[0])
		}
	})

	t.Run("message_delta", func(t *testing.T) {
		data := map[string]interface{}{
			"delta": map[string]interface{}{
				"stop_reason": "tool_use",
			},
		}

		chunkBytes, err := AnthropicStreamToOpenAIStream("message_delta", data, "gpt", "id2")
		if err != nil {
			t.Fatalf("AnthropicStreamToOpenAIStream error: %v", err)
		}
		if chunkBytes == nil {
			t.Fatalf("expected chunk bytes")
		}

		var chunk models.ChatCompletionChunk
		if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
			t.Fatalf("unmarshal chunk: %v", err)
		}
		if len(chunk.Choices) != 1 || chunk.Choices[0].FinishReason == nil {
			t.Fatalf("finish reason missing: %#v", chunk)
		}
		if *chunk.Choices[0].FinishReason != "tool_calls" {
			t.Fatalf("finish reason mismatch: %#v", chunk.Choices[0].FinishReason)
		}
	})
}

func TestOpenAIChatToOpenAIResponsesRequest_MessagesAndTools(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxTokens := 64
	req := &models.ChatCompletionRequest{
		Model:       "gpt-4",
		Stream:      true,
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
		Stop:        []string{"done"},
		User:        "user-1",
		Messages: []models.ChatMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
			{
				Role:    "assistant",
				Content: "hello",
				ToolCalls: []models.ToolCall{
					{
						ID:   "call1",
						Type: "function",
						Function: models.FunctionCall{
							Name:      "sum",
							Arguments: `{"a":1}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call1",
				Content:    "42",
			},
		},
		Tools: []models.Tool{
			{
				Type: "function",
				Function: models.Function{
					Name:        "sum",
					Description: "sum",
					Parameters:  map[string]interface{}{"type": "object"},
				},
			},
		},
	}

	result, err := OpenAIChatToOpenAIResponsesRequest(req)
	if err != nil {
		t.Fatalf("OpenAIChatToOpenAIResponsesRequest error: %v", err)
	}

	if getString(result, "model") != "gpt-4" {
		t.Fatalf("model mismatch: %#v", result["model"])
	}
	if !result["stream"].(bool) {
		t.Fatalf("stream mismatch: %#v", result["stream"])
	}
	if result["temperature"].(float64) != temp {
		t.Fatalf("temperature mismatch: %#v", result["temperature"])
	}
	if result["top_p"].(float64) != topP {
		t.Fatalf("top_p mismatch: %#v", result["top_p"])
	}
	if int(result["max_output_tokens"].(int)) != maxTokens {
		t.Fatalf("max_output_tokens mismatch: %#v", result["max_output_tokens"])
	}

	if result["instructions"].(string) != "sys" {
		t.Fatalf("instructions mismatch: %#v", result["instructions"])
	}

	stop, ok := result["stop"].([]string)
	if !ok || len(stop) != 1 || stop[0] != "done" {
		t.Fatalf("stop mismatch: %#v", result["stop"])
	}

	inputItems := mapSlice(result["input"])
	if len(inputItems) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(inputItems))
	}

	if getString(inputItems[0], "role") != "user" || inputItems[0]["content"] != "hi" {
		t.Fatalf("user input mismatch: %#v", inputItems[0])
	}

	if getString(inputItems[1], "role") != "assistant" || inputItems[1]["content"] != "hello" {
		t.Fatalf("assistant input mismatch: %#v", inputItems[1])
	}
	toolCalls := mapSlice(inputItems[1]["tool_calls"])
	if len(toolCalls) != 1 {
		t.Fatalf("tool_calls mismatch: %#v", toolCalls)
	}
	if getString(toolCalls[0], "id") != "call1" {
		t.Fatalf("tool call id mismatch: %#v", toolCalls[0])
	}
	fn := toolCalls[0]["function"].(map[string]interface{})
	if getString(fn, "name") != "sum" || getString(fn, "arguments") != `{"a":1}` {
		t.Fatalf("tool call function mismatch: %#v", fn)
	}

	if getString(inputItems[2], "type") != "function_call_output" || getString(inputItems[2], "call_id") != "call1" {
		t.Fatalf("tool output mismatch: %#v", inputItems[2])
	}
	if inputItems[2]["output"] != "42" {
		t.Fatalf("tool output content mismatch: %#v", inputItems[2]["output"])
	}

	tools := mapSlice(result["tools"])
	if len(tools) != 1 {
		t.Fatalf("tools mismatch: %#v", tools)
	}
}

func TestOpenAIResponsesToOpenAIChatRequest_MessagesAndTools(t *testing.T) {
	req := map[string]interface{}{
		"model":       "gpt-4",
		"stream":      true,
		"temperature": float64(0.4),
		"top_p":       float64(0.8),
		"max_output_tokens": float64(128),
		"stop":        []string{"done"},
		"instructions": "sys",
		"input": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
			map[string]interface{}{
				"role":    "assistant",
				"content": "hello",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":   "call1",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "sum",
							"arguments": `{"a":1}`,
						},
					},
				},
			},
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": "call1",
				"output":  "42",
			},
		},
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "sum",
					"description": "sum",
					"parameters":  map[string]interface{}{"type": "object"},
				},
			},
		},
	}

	chatReq, err := OpenAIResponsesToOpenAIChatRequest(req)
	if err != nil {
		t.Fatalf("OpenAIResponsesToOpenAIChatRequest error: %v", err)
	}

	if chatReq.Model != "gpt-4" || !chatReq.Stream {
		t.Fatalf("basic fields mismatch: %#v", chatReq)
	}
	if len(chatReq.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(chatReq.Messages))
	}
	if chatReq.Messages[0].Role != "system" || chatReq.Messages[0].Content != "sys" {
		t.Fatalf("system message mismatch: %#v", chatReq.Messages[0])
	}
	if chatReq.Messages[1].Role != "user" || chatReq.Messages[1].Content != "hi" {
		t.Fatalf("user message mismatch: %#v", chatReq.Messages[1])
	}

	assistant := chatReq.Messages[2]
	if assistant.Role != "assistant" || assistant.Content != "hello" {
		t.Fatalf("assistant message mismatch: %#v", assistant)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Function.Name != "sum" {
		t.Fatalf("assistant tool_calls mismatch: %#v", assistant.ToolCalls)
	}

	toolMsg := chatReq.Messages[3]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call1" || toolMsg.Content != "42" {
		t.Fatalf("tool message mismatch: %#v", toolMsg)
	}

	if len(chatReq.Tools) != 1 || chatReq.Tools[0].Function.Name != "sum" {
		t.Fatalf("tools mismatch: %#v", chatReq.Tools)
	}
}

func TestOpenAIResponsesToOpenAIChatResponse_ToolCallsUsage(t *testing.T) {
	resp := map[string]interface{}{
		"id":    "resp1",
		"model": "gpt-4",
		"status": "completed",
		"output": []interface{}{
			map[string]interface{}{
				"type": "message",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": "hi"},
				},
			},
			map[string]interface{}{
				"type":      "function_call",
				"call_id":   "call1",
				"name":      "sum",
				"arguments": `{"a":1}`,
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  float64(3),
			"output_tokens": float64(5),
		},
	}

	chatResp, err := OpenAIResponsesToOpenAIChatResponse(resp, "gpt-4")
	if err != nil {
		t.Fatalf("OpenAIResponsesToOpenAIChatResponse error: %v", err)
	}

	if len(chatResp.Choices) != 1 || chatResp.Choices[0].Message == nil {
		t.Fatalf("choices missing: %#v", chatResp)
	}
	if chatResp.Choices[0].Message.Content != "hi" {
		t.Fatalf("message content mismatch: %#v", chatResp.Choices[0].Message)
	}
	if len(chatResp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool calls mismatch: %#v", chatResp.Choices[0].Message.ToolCalls)
	}
	if chatResp.Choices[0].FinishReason == nil || *chatResp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish reason mismatch: %#v", chatResp.Choices[0].FinishReason)
	}
	if chatResp.Usage == nil || chatResp.Usage.TotalTokens != 8 {
		t.Fatalf("usage mismatch: %#v", chatResp.Usage)
	}
}

func TestOpenAIChatResponseToOpenAIResponsesResponse_Length(t *testing.T) {
	finishReason := "length"
	resp := &models.ChatCompletionResponse{
		ID:    "chat1",
		Model: "gpt-4",
		Choices: []models.Choice{{
			Index: 0,
			Message: &models.ChatMessage{
				Role:    "assistant",
				Content: "hi",
			},
			FinishReason: &finishReason,
		}},
		Usage: &models.Usage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
		},
	}

	result, err := OpenAIChatResponseToOpenAIResponsesResponse(resp)
	if err != nil {
		t.Fatalf("OpenAIChatResponseToOpenAIResponsesResponse error: %v", err)
	}

	if getString(result, "status") != "incomplete" {
		t.Fatalf("status mismatch: %#v", result["status"])
	}
	details := result["incomplete_details"].(map[string]interface{})
	if getString(details, "reason") != "max_output_tokens" {
		t.Fatalf("incomplete_details mismatch: %#v", details)
	}
}

func TestOpenAIResponsesStreamToOpenAIChatStream_Text(t *testing.T) {
	state := NewOpenAIResponsesToChatStreamState("gpt-4")

	createdEvents, err := OpenAIResponsesStreamToOpenAIChatStream(map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":    "resp1",
			"model": "gpt-4",
		},
	}, state)
	if err != nil {
		t.Fatalf("response.created error: %v", err)
	}
	if len(createdEvents) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(createdEvents))
	}
	var chunk models.ChatCompletionChunk
	if err := json.Unmarshal(createdEvents[0], &chunk); err != nil {
		t.Fatalf("unmarshal chunk: %v", err)
	}
	if chunk.Choices[0].Delta == nil || chunk.Choices[0].Delta.Role != "assistant" {
		t.Fatalf("start chunk mismatch: %#v", chunk.Choices[0].Delta)
	}

	deltaEvents, err := OpenAIResponsesStreamToOpenAIChatStream(map[string]interface{}{
		"type":         "response.output_text.delta",
		"output_index": float64(0),
		"delta":        "hi",
	}, state)
	if err != nil {
		t.Fatalf("output_text.delta error: %v", err)
	}
	if len(deltaEvents) != 1 {
		t.Fatalf("expected 1 delta event, got %d", len(deltaEvents))
	}
	if err := json.Unmarshal(deltaEvents[0], &chunk); err != nil {
		t.Fatalf("unmarshal delta chunk: %v", err)
	}
	if chunk.Choices[0].Delta == nil || chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("delta content mismatch: %#v", chunk.Choices[0].Delta)
	}

	completedEvents, err := OpenAIResponsesStreamToOpenAIChatStream(map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"status": "completed",
		},
	}, state)
	if err != nil {
		t.Fatalf("response.completed error: %v", err)
	}
	if len(completedEvents) != 1 {
		t.Fatalf("expected 1 completed event, got %d", len(completedEvents))
	}
	if err := json.Unmarshal(completedEvents[0], &chunk); err != nil {
		t.Fatalf("unmarshal completed chunk: %v", err)
	}
	if chunk.Choices[0].FinishReason == nil || *chunk.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish reason mismatch: %#v", chunk.Choices[0].FinishReason)
	}
}

func TestOpenAIChatStreamToOpenAIResponsesStream_TextAndFinish(t *testing.T) {
	state := NewOpenAIChatToResponsesStreamState("gpt-4")
	chunk := &models.ChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "gpt-4",
		Choices: []models.Choice{{
			Delta: &models.ChatMessage{Content: "hi"},
		}},
	}

	events, err := OpenAIChatStreamToOpenAIResponsesStream(chunk, state)
	if err != nil {
		t.Fatalf("OpenAIChatStreamToOpenAIResponsesStream error: %v", err)
	}

	types := eventTypes(t, events)
	expectTypes := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
	}
	for _, expected := range expectTypes {
		if !containsString(types, expected) {
			t.Fatalf("missing event type %s: %v", expected, types)
		}
	}

	finishReason := "length"
	finishChunk := &models.ChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "gpt-4",
		Choices: []models.Choice{{
			FinishReason: &finishReason,
		}},
	}

	finishEvents, err := OpenAIChatStreamToOpenAIResponsesStream(finishChunk, state)
	if err != nil {
		t.Fatalf("finish events error: %v", err)
	}

	finishTypes := eventTypes(t, finishEvents)
	if !containsString(finishTypes, "response.output_item.done") || !containsString(finishTypes, "response.completed") {
		t.Fatalf("finish events missing: %v", finishTypes)
	}

	for _, event := range finishEvents {
		var eventMap map[string]interface{}
		if err := json.Unmarshal(event, &eventMap); err != nil {
			t.Fatalf("unmarshal finish event: %v", err)
		}
		if eventMap["type"] == "response.completed" {
			response := eventMap["response"].(map[string]interface{})
			if getString(response, "status") != "incomplete" {
				t.Fatalf("response status mismatch: %#v", response)
			}
			details := response["incomplete_details"].(map[string]interface{})
			if getString(details, "reason") != "max_output_tokens" {
				t.Fatalf("incomplete_details mismatch: %#v", details)
			}
		}
	}
}

func mapSlice(value interface{}) []map[string]interface{} {
	switch v := value.(type) {
	case []map[string]interface{}:
		return v
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				result = append(result, itemMap)
			}
		}
		return result
	default:
		return nil
	}
}

func eventTypes(t *testing.T, events [][]byte) []string {
	t.Helper()
	types := make([]string, 0, len(events))
	for _, event := range events {
		var eventMap map[string]interface{}
		if err := json.Unmarshal(event, &eventMap); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		if eventType, ok := eventMap["type"].(string); ok {
			types = append(types, eventType)
		}
	}
	return types
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
