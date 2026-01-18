package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 300 * time.Second

// OpenAIAdapter handles communication with OpenAI API
type OpenAIAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(apiKey, baseURL string) *OpenAIAdapter {
	return &OpenAIAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// ChatCompletions sends a chat completion request
func (a *OpenAIAdapter) ChatCompletions(ctx context.Context, request interface{}) (map[string]interface{}, int, error) {
	url := fmt.Sprintf("%s/chat/completions", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

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

// ChatCompletionsStream sends a streaming chat completion request
func (a *OpenAIAdapter) ChatCompletionsStream(ctx context.Context, request interface{}) (*StreamReader, int, error) {
	url := fmt.Sprintf("%s/chat/completions", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
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

// StreamReader wraps a streaming response
type StreamReader struct {
	reader *bufio.Reader
	body   io.ReadCloser
}

// ReadLine reads a line from the stream
func (s *StreamReader) ReadLine() (string, error) {
	return s.reader.ReadString('\n')
}

// Read reads bytes from the stream
func (s *StreamReader) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

// Close closes the stream
func (s *StreamReader) Close() error {
	return s.body.Close()
}

// GetReader returns the underlying reader
func (s *StreamReader) GetReader() *bufio.Reader {
	return s.reader
}

// Responses sends a request to /v1/responses endpoint
func (a *OpenAIAdapter) Responses(ctx context.Context, request interface{}) (map[string]interface{}, int, error) {
	url := fmt.Sprintf("%s/responses", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

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

// ResponsesStream sends a streaming request to /v1/responses endpoint
func (a *OpenAIAdapter) ResponsesStream(ctx context.Context, request interface{}) (*StreamReader, int, error) {
	url := fmt.Sprintf("%s/responses", a.baseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
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
