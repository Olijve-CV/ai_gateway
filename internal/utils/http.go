package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"ai_gateway/internal/config"
)

const (
	DefaultTimeout = 300 * time.Second
)

// GetTimeout returns the configured HTTP timeout
func GetTimeout(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.HTTPTimeout <= 0 {
		return DefaultTimeout
	}
	return time.Duration(cfg.HTTPTimeout) * time.Second
}

// HTTPClient is a wrapper around http.Client with common functionality
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTP client with default settings
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// NewHTTPClientWithConfig creates a new HTTP client with configured timeout
func NewHTTPClientWithConfig(cfg *config.Config) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: GetTimeout(cfg),
		},
	}
}

// PostJSON sends a POST request with JSON body
func (c *HTTPClient) PostJSON(ctx context.Context, url string, headers map[string]string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}

// StreamResponse represents a streaming response
type StreamResponse struct {
	Reader *bufio.Reader
	Body   io.ReadCloser
}

// Close closes the stream response
func (s *StreamResponse) Close() error {
	return s.Body.Close()
}

// ReadLine reads a line from the stream
func (s *StreamResponse) ReadLine() (string, error) {
	line, err := s.Reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return line, nil
}

// PostJSONStream sends a POST request and returns a streaming response
func (c *HTTPClient) PostJSONStream(ctx context.Context, url string, headers map[string]string, body interface{}) (*StreamResponse, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	return &StreamResponse{
		Reader: bufio.NewReader(resp.Body),
		Body:   resp.Body,
	}, resp.StatusCode, nil
}

// ParseJSONResponse parses a JSON response body
func ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}
