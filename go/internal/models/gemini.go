package models

import "encoding/json"

// Gemini GenerateContent API Models

// GenerateContentRequest represents a Gemini generateContent request
type GenerateContentRequest struct {
	Contents          []GeminiContent     `json:"contents"`
	SystemInstruction *GeminiContent      `json:"systemInstruction,omitempty"`
	Tools             []GeminiTool        `json:"tools,omitempty"`
	ToolConfig        *ToolConfig         `json:"toolConfig,omitempty"`
	GenerationConfig  *GenerationConfig   `json:"generationConfig,omitempty"`
	SafetySettings    []SafetySetting     `json:"safetySettings,omitempty"`
}

// GeminiContent represents content in Gemini format
type GeminiContent struct {
	Role  string       `json:"role,omitempty"` // user, model
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of content
type GeminiPart struct {
	Text             string            `json:"text,omitempty"`
	InlineData       *InlineData       `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

// InlineData represents inline data (images, etc.)
type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64 encoded
}

// GeminiFunctionCall represents a function call from Gemini
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"` // Object, not string
}

// FunctionResponse represents a function response
type FunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// GeminiTool represents a tool definition for Gemini
type GeminiTool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// FunctionDeclaration represents a function declaration
type FunctionDeclaration struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema object
}

// ToolConfig represents tool configuration
type ToolConfig struct {
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// FunctionCallingConfig represents function calling configuration
type FunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // AUTO, ANY, NONE
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// GenerationConfig represents generation configuration
type GenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
	CandidateCount  *int     `json:"candidateCount,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"` // text/plain, application/json
}

// SafetySetting represents a safety setting
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GenerateContentResponse represents a Gemini generateContent response
type GenerateContentResponse struct {
	Candidates     []Candidate     `json:"candidates,omitempty"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *UsageMetadata  `json:"usageMetadata,omitempty"`
}

// Candidate represents a response candidate
type Candidate struct {
	Content       *GeminiContent `json:"content,omitempty"`
	FinishReason  string         `json:"finishReason,omitempty"` // STOP, MAX_TOKENS, SAFETY, RECITATION, OTHER
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
	Index         int            `json:"index"`
}

// SafetyRating represents a safety rating
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// PromptFeedback represents feedback about the prompt
type PromptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

// UsageMetadata represents token usage metadata
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Streaming response (same structure, sent as SSE)

// GetTextContent extracts text content from a GeminiContent
func (c *GeminiContent) GetTextContent() string {
	var text string
	for _, part := range c.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}
	return text
}

// SetTextContent sets text content in a GeminiContent
func (c *GeminiContent) SetTextContent(text string) {
	c.Parts = []GeminiPart{{Text: text}}
}

// ToJSON converts the request to JSON bytes
func (r *GenerateContentRequest) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON parses JSON bytes into a request
func (r *GenerateContentRequest) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// HasFunctionCall checks if the response contains a function call
func (r *GenerateContentResponse) HasFunctionCall() bool {
	if len(r.Candidates) == 0 || r.Candidates[0].Content == nil {
		return false
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			return true
		}
	}
	return false
}

// GetFunctionCalls returns all function calls from the response
func (r *GenerateContentResponse) GetFunctionCalls() []GeminiFunctionCall {
	var calls []GeminiFunctionCall
	if len(r.Candidates) == 0 || r.Candidates[0].Content == nil {
		return calls
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, *part.FunctionCall)
		}
	}
	return calls
}
