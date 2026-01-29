package models

import (
	"encoding/json"
	"fmt"
)

// Anthropic Messages API Models

// MessagesRequest represents an Anthropic messages request
type MessagesRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	System        interface{}        `json:"system,omitempty"` // string or []ContentBlock
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Metadata      *Metadata          `json:"metadata,omitempty"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    interface{}        `json:"tool_choice,omitempty"` // ToolChoiceAuto or ToolChoiceAny or ToolChoiceTool
}

// Validate validates the request according to Anthropic API specifications
func (r *MessagesRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}

	if r.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}

	// Validate tool schemas if tools are present
	for i, tool := range r.Tools {
		if err := tool.ValidateInputSchema(); err != nil {
			return fmt.Errorf("tool %d validation failed: %w", i, err)
		}
	}

	// Validate tool choice
	if r.ToolChoice != nil {
		if err := r.validateToolChoice(); err != nil {
			return fmt.Errorf("tool_choice validation failed: %w", err)
		}
	}

	return nil
}

// validateToolChoice validates the tool choice configuration
func (r *MessagesRequest) validateToolChoice() error {
	switch choice := r.ToolChoice.(type) {
	case ToolChoiceAuto:
		if choice.Type != "auto" {
			return fmt.Errorf("tool_choice type must be 'auto'")
		}
	case ToolChoiceAny:
		if choice.Type != "any" {
			return fmt.Errorf("tool_choice type must be 'any'")
		}
	case ToolChoiceTool:
		if choice.Type != "tool" {
			return fmt.Errorf("tool_choice type must be 'tool'")
		}
		if choice.Name == "" {
			return fmt.Errorf("tool_choice name is required when type is 'tool'")
		}
	case map[string]interface{}:
		// Handle JSON unmarshaled interface
		if t, ok := choice["type"].(string); ok {
			switch t {
			case "auto", "any":
				// Valid types without additional validation
			case "tool":
				if name, ok := choice["name"].(string); !ok || name == "" {
					return fmt.Errorf("tool_choice name is required when type is 'tool'")
				}
			default:
				return fmt.Errorf("invalid tool_choice type: %s", t)
			}
		} else {
			return fmt.Errorf("tool_choice type is required")
		}
	default:
		return fmt.Errorf("invalid tool_choice format")
	}

	return nil
}

// AnthropicMessage represents a message in Anthropic format
type AnthropicMessage struct {
	Role    string      `json:"role"`    // user, assistant
	Content interface{} `json:"content"` // string or []ContentBlock
}

// ContentBlock represents a content block
type ContentBlock struct {
	Type      string       `json:"type"` // text, image, tool_use, tool_result
	Text      string       `json:"text,omitempty"`
	Source    *ImageSource `json:"source,omitempty"`      // For image blocks
	ID        string       `json:"id,omitempty"`          // For tool_use blocks
	Name      string       `json:"name,omitempty"`        // For tool_use blocks
	Input     interface{}  `json:"input,omitempty"`       // For tool_use blocks (object)
	ToolUseID string       `json:"tool_use_id,omitempty"` // Legacy tool_result id (prefers id)
	Content   interface{}  `json:"content,omitempty"`     // For tool_result blocks
	IsError   bool         `json:"is_error,omitempty"`    // For tool_result blocks
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"`       // base64
	MediaType string `json:"media_type"` // image/jpeg, image/png, etc.
	Data      string `json:"data"`
}

// SystemBlock represents a system content block
type SystemBlock struct {
	Type         string        `json:"type"` // text
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl represents cache control settings
type CacheControl struct {
	Type string `json:"type"` // ephemeral
}

// Metadata represents request metadata
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicTool represents a tool definition for Anthropic
type AnthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"` // JSON Schema object
}

// ValidateInputSchema validates the input schema is a proper dictionary
func (t *AnthropicTool) ValidateInputSchema() error {
	if t.InputSchema == nil {
		return fmt.Errorf("input_schema cannot be nil")
	}

	schemaMap, ok := t.InputSchema.(map[string]interface{})
	if !ok {
		return fmt.Errorf("input_schema must be a dictionary/object")
	}

	// Ensure it has a type field, default to "object"
	if _, exists := schemaMap["type"]; !exists {
		schemaMap["type"] = "object"
		t.InputSchema = schemaMap
	}

	return nil
}

// ToolChoiceAuto represents auto tool choice
type ToolChoiceAuto struct {
	Type string `json:"type"` // auto
}

// ToolChoiceAny represents any tool choice
type ToolChoiceAny struct {
	Type string `json:"type"` // any
}

// ToolChoiceTool represents a specific tool choice
type ToolChoiceTool struct {
	Type string `json:"type"` // tool
	Name string `json:"name"`
}

// MessagesResponse represents an Anthropic messages response
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // message
	Role         string         `json:"role"` // assistant
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason,omitempty"` // end_turn, max_tokens, stop_sequence, tool_use
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage `json:"usage,omitempty"`
}

// AnthropicUsage represents token usage for Anthropic
type AnthropicUsage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens,omitempty"`
}

// Streaming Events

// MessageStartEvent represents a message_start event
type MessageStartEvent struct {
	Type    string           `json:"type"` // message_start
	Message MessagesResponse `json:"message"`
}

// ContentBlockStartEvent represents a content_block_start event
type ContentBlockStartEvent struct {
	Type         string       `json:"type"` // content_block_start
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// ContentBlockDeltaEvent represents a content_block_delta event
type ContentBlockDeltaEvent struct {
	Type  string       `json:"type"` // content_block_delta
	Index int          `json:"index"`
	Delta ContentDelta `json:"delta"`
}

// ContentDelta represents a content delta
type ContentDelta struct {
	Type        string `json:"type"` // text_delta, input_json_delta
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// ContentBlockStopEvent represents a content_block_stop event
type ContentBlockStopEvent struct {
	Type  string `json:"type"` // content_block_stop
	Index int    `json:"index"`
}

// MessageDeltaEvent represents a message_delta event
type MessageDeltaEvent struct {
	Type  string       `json:"type"` // message_delta
	Delta MessageDelta `json:"delta"`
	Usage *DeltaUsage  `json:"usage,omitempty"`
}

// MessageDelta represents a message delta
type MessageDelta struct {
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// DeltaUsage represents usage in a delta event
type DeltaUsage struct {
	OutputTokens int `json:"output_tokens"`
}

// MessageStopEvent represents a message_stop event
type MessageStopEvent struct {
	Type string `json:"type"` // message_stop
}

// PingEvent represents a ping event
type PingEvent struct {
	Type string `json:"type"` // ping
}

// ErrorEvent represents an error event
type ErrorEvent struct {
	Type  string         `json:"type"` // error
	Error AnthropicError `json:"error"`
}

// AnthropicError represents an error
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetTextContent extracts text content from an AnthropicMessage
func (m *AnthropicMessage) GetTextContent() string {
	if m.Content == nil {
		return ""
	}

	// Try string first
	if str, ok := m.Content.(string); ok {
		return str
	}

	if blocks, ok := m.Content.([]ContentBlock); ok {
		var text string
		for _, block := range blocks {
			if block.Type == "text" {
				text += block.Text
			}
		}
		return text
	}

	// Try content blocks
	if blocks, ok := m.Content.([]interface{}); ok {
		var text string
		for _, block := range blocks {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if t, ok := blockMap["text"].(string); ok {
						text += t
					}
				}
			}
		}
		return text
	}

	return ""
}

// ToJSON converts the request to JSON bytes
func (r *MessagesRequest) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON parses JSON bytes into a request
func (r *MessagesRequest) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// PostInit initializes fields after unmarshaling, such as generating IDs
func (r *MessagesRequest) PostInit() {
	// This would be called after unmarshaling to set default values
	// Similar to model_post_init in Python reference
}
