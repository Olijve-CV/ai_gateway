package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

// NewOpenAIAdapterWithConfig creates a new OpenAI adapter with configurable timeout
func NewOpenAIAdapterWithConfig(apiKey, baseURL string, timeout time.Duration) *OpenAIAdapter {
	return &OpenAIAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
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

	start := time.Now()
	prettyBody := string(jsonBody)
	var prettyBuf bytes.Buffer
	if err := json.Indent(&prettyBuf, jsonBody, "", "  "); err == nil {
		prettyBody = prettyBuf.String()
	}
	log.Printf("[OpenAIAdapter] ChatCompletions start: url=%s, requestBytes=%d", url, len(jsonBody))
	log.Printf("[OpenAIAdapter] ChatCompletions requestBody:\n%s", prettyBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

	log.Printf("[OpenAIAdapter] ChatCompletions HeaderApiKey: %s", a.apiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[OpenAIAdapter] ChatCompletions error after %s: %v", time.Since(start), err)
		return nil, 0, err
	}
	log.Printf("[OpenAIAdapter] ChatCompletions response: statusCode=%d, elapsed=%s", resp.StatusCode, time.Since(start))
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[OpenAIAdapter] ChatCompletions decode error: %v", err)
		return nil, resp.StatusCode, err
	}

	// Log response content
	prettyResponse, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("[OpenAIAdapter] ChatCompletions response (raw): %v", result)
	} else {
		log.Printf("[OpenAIAdapter] ChatCompletions response:\n%s", string(prettyResponse))
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

	start := time.Now()
	prettyBody := string(jsonBody)
	var prettyBuf bytes.Buffer
	if err := json.Indent(&prettyBuf, jsonBody, "", "  "); err == nil {
		prettyBody = prettyBuf.String()
	}
	log.Printf("[OpenAIAdapter] ChatCompletionsStream start: url=%s, requestBytes=%d", url, len(jsonBody))
	log.Printf("[OpenAIAdapter] ChatCompletionsStream requestBody:\n%s", prettyBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	req.Header.Set("Accept", "text/event-stream")

	log.Printf("[OpenAIAdapter] ChatCompletionsStream HeaderApiKey: %s", a.apiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[OpenAIAdapter] ChatCompletionsStream error after %s: %v", time.Since(start), err)
		return nil, 0, err
	}
	log.Printf("[OpenAIAdapter] ChatCompletionsStream opened: statusCode=%d, elapsed=%s", resp.StatusCode, time.Since(start))

	streamReader := &StreamReader{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}

	// Start logging stream response in background
	streamStart := time.Now()
	go func() {
		defer func() {
			log.Printf("[OpenAIAdapter] ChatCompletionsStream completed after %s", time.Since(streamStart))
		}()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("[OpenAIAdapter] ChatCompletionsStream read error: %v", err)
				}
				break
			}
			if strings.TrimSpace(line) != "" {
				log.Printf("[OpenAIAdapter] ChatCompletionsStream response: %s", strings.TrimSpace(line))
			}
		}
	}()

	return streamReader, resp.StatusCode, nil
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

	// Log response content
	prettyResponse, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("[OpenAIAdapter] Responses response (raw): %v", result)
	} else {
		log.Printf("[OpenAIAdapter] Responses response:\n%s", string(prettyResponse))
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

	start := time.Now()
	prettyBody := string(jsonBody)
	var prettyBuf bytes.Buffer
	if err := json.Indent(&prettyBuf, jsonBody, "", "  "); err == nil {
		prettyBody = prettyBuf.String()
	}
	log.Printf("[OpenAIAdapter] ResponsesStream start: url=%s, requestBytes=%d", url, len(jsonBody))
	log.Printf("[OpenAIAdapter] ResponsesStream requestBody:\n%s", prettyBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	req.Header.Set("Accept", "text/event-stream")

	log.Printf("[OpenAIAdapter] ResponsesStream HeaderApiKey: %s", a.apiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[OpenAIAdapter] ResponsesStream error after %s: %v", time.Since(start), err)
		return nil, 0, err
	}
	log.Printf("[OpenAIAdapter] ResponsesStream opened: statusCode=%d, elapsed=%s", resp.StatusCode, time.Since(start))

	streamReader := &StreamReader{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}

	// Start logging stream response in background
	streamStart := time.Now()
	go func() {
		defer func() {
			log.Printf("[OpenAIAdapter] ResponsesStream completed after %s", time.Since(streamStart))
		}()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("[OpenAIAdapter] ResponsesStream read error: %v", err)
				}
				break
			}
			if strings.TrimSpace(line) != "" {
				log.Printf("[OpenAIAdapter] ResponsesStream response: %s", strings.TrimSpace(line))
			}
		}
	}()

	return streamReader, resp.StatusCode, nil
}
