package models

import "encoding/json"

// OpenAI Chat Completion Models

// ChatCompletionRequest represents an OpenAI chat completion request
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"` // string or []string
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64     `json:"logit_bias,omitempty"`
	User             string                 `json:"user,omitempty"`
	Tools            []Tool                 `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"` // string or ToolChoiceObject
	ResponseFormat   *ResponseFormat        `json:"response_format,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	LogProbs         *bool                  `json:"logprobs,omitempty"`
	TopLogProbs      *int                   `json:"top_logprobs,omitempty"`
}

// ChatMessage represents a message in a chat conversation
type ChatMessage struct {
	Role       string      `json:"role"` // system, user, assistant, tool
	Content    interface{} `json:"content,omitempty"` // string or []ContentPart
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentPart represents a part of message content (for multimodal)
type ContentPart struct {
	Type     string    `json:"type"` // text, image_url
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in message content
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // auto, low, high
}

// Tool represents a tool/function definition
type Tool struct {
	Type     string   `json:"type"` // function
	Function Function `json:"function"`
}

// Function represents a function definition
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema object
}

// ToolCall represents a tool call from the assistant
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // function
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolChoiceObject represents a specific tool choice
type ToolChoiceObject struct {
	Type     string               `json:"type"` // function
	Function ToolChoiceFunction   `json:"function"`
}

// ToolChoiceFunction represents the function in a tool choice
type ToolChoiceFunction struct {
	Name string `json:"name"`
}

// ResponseFormat represents the response format
type ResponseFormat struct {
	Type string `json:"type"` // text, json_object
}

// ChatCompletionResponse represents an OpenAI chat completion response
type ChatCompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"` // chat.completion
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"` // For streaming
	FinishReason *string      `json:"finish_reason,omitempty"`
	LogProbs     interface{}  `json:"logprobs,omitempty"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents a streaming chunk
type ChatCompletionChunk struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"` // chat.completion.chunk
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
	Usage             *Usage   `json:"usage,omitempty"`
}

// GetMessageContent extracts text content from a ChatMessage
func (m *ChatMessage) GetTextContent() string {
	if m.Content == nil {
		return ""
	}

	// Try string first
	if str, ok := m.Content.(string); ok {
		return str
	}

	// Try content parts
	if parts, ok := m.Content.([]interface{}); ok {
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

// SetTextContent sets the text content of a ChatMessage
func (m *ChatMessage) SetTextContent(text string) {
	m.Content = text
}

// ToJSON converts the request to JSON bytes
func (r *ChatCompletionRequest) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON parses JSON bytes into a request
func (r *ChatCompletionRequest) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}
