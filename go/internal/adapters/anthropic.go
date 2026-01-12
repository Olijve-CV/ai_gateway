package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// AnthropicAdapter handles communication with Anthropic API
type AnthropicAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropicAdapter creates a new Anthropic adapter
func NewAnthropicAdapter(apiKey, baseURL string) *AnthropicAdapter {
	return &AnthropicAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Messages sends a messages request
func (a *AnthropicAdapter) Messages(ctx context.Context, request interface{}) (map[string]interface{}, int, error) {
	url := fmt.Sprintf("%s/messages", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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

// MessagesStream sends a streaming messages request
func (a *AnthropicAdapter) MessagesStream(ctx context.Context, request interface{}) (*StreamReader, int, error) {
	url := fmt.Sprintf("%s/messages", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
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
