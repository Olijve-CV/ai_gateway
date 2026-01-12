package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GeminiAdapter handles communication with Gemini API
type GeminiAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewGeminiAdapter creates a new Gemini adapter
func NewGeminiAdapter(apiKey, baseURL string) *GeminiAdapter {
	return &GeminiAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// GenerateContent sends a generateContent request
func (a *GeminiAdapter) GenerateContent(ctx context.Context, model string, request interface{}) (map[string]interface{}, int, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", a.baseURL, model, a.apiKey)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, err
	}

	return result, resp.StatusCode, nil
}

// GenerateContentStream sends a streaming generateContent request
func (a *GeminiAdapter) GenerateContentStream(ctx context.Context, model string, request interface{}) (*StreamReader, int, error) {
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", a.baseURL, model, a.apiKey)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	return &StreamReader{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, resp.StatusCode, nil
}
