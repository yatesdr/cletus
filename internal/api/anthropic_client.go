package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cletus/internal/config"
	"cletus/internal/util"
)

// NativeClient speaks the Messages API wire format (role/content/tool_use blocks).
// Compatible with any endpoint that implements this protocol.
type NativeClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
}

// NativeClientConfig holds configuration for NativeClient.
type NativeClientConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// DefaultNativeClientConfig returns a default configuration.
func DefaultNativeClientConfig() *NativeClientConfig {
	return &NativeClientConfig{
		BaseURL:   "http://localhost:8080/v1",
		Model:     config.DefaultLargeModel,
		MaxTokens: 8192,
		Timeout:   5 * time.Minute,
	}
}

// NewNativeClient creates a new NativeClient.
func NewNativeClient(cfg *NativeClientConfig) *NativeClient {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &NativeClient{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
	}
}

func (c *NativeClient) SetModel(model string) {
	c.model = model
}

// SetMaxTokens sets the default max tokens.
func (c *NativeClient) SetMaxTokens(maxTokens int) {
	c.maxTokens = maxTokens
}

// Stream sends a streaming request to the Messages endpoint.
func (c *NativeClient) Stream(ctx context.Context, req *Request, handler StreamResponseHandler) error {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return c.processStream(resp.Body, handler)
}

// processStream processes the SSE streaming response.
func (c *NativeClient) processStream(body io.Reader, handler StreamResponseHandler) error {
	var currentBlock *ContentBlock
	var textBuffer strings.Builder
	var inputJSONBuffer strings.Builder

	events := util.Reader(body)
	for event := range events {
		if event.Type == "error" {
			return fmt.Errorf("SSE error: %s", event.Data)
		}

		data := strings.TrimRight(event.Data, "\n")

		switch event.Type {
		case "message_start":
			var envelope struct {
				Type    string  `json:"type"`
				Message Message `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err == nil {
				handler(StreamEvent{Type: "message_start", Usage: envelope.Message.Usage})
			}

		case "content_block_start":
			var envelope struct {
				Type         string       `json:"type"`
				Index        int          `json:"index"`
				ContentBlock ContentBlock `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err == nil {
				currentBlock = &envelope.ContentBlock
				handler(StreamEvent{Type: "content_block_start", ContentBlock: currentBlock})
			}

		case "content_block_delta":
			if currentBlock == nil {
				continue
			}
			var envelope struct {
				Type  string `json:"type"`
				Index int    `json:"index"`
				Delta Delta  `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err != nil {
				continue
			}

			switch envelope.Delta.Type {
			case "text_delta":
				textBuffer.WriteString(envelope.Delta.Text)
				currentBlock.Text = textBuffer.String()
				handler(StreamEvent{Type: "content_block_delta", Content: envelope.Delta.Text, ContentBlock: currentBlock})
			case "input_json_delta":
				inputJSONBuffer.WriteString(envelope.Delta.PartialJSON)
				currentBlock.Input = json.RawMessage(inputJSONBuffer.String())
			case "thinking_delta":
				// Extended reasoning token — suppress from chat output
				handler(StreamEvent{Type: "thinking_block", Content: envelope.Delta.Thinking})
			}

		case "content_block_stop":
			if currentBlock != nil && currentBlock.Type == "tool_use" {
				handler(StreamEvent{Type: "tool_use", ToolUse: currentBlock, ToolUseDone: true})
			}
			currentBlock = nil
			textBuffer.Reset()
			inputJSONBuffer.Reset()

		case "message_delta":
			var envelope struct {
				Type  string `json:"type"`
				Delta Delta  `json:"delta"`
				Usage Usage  `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err == nil {
				handler(StreamEvent{Type: "message_delta", Usage: envelope.Usage})
			}

		case "message_stop":
			handler(StreamEvent{Type: "message_stop"})

		case "ping":
			// Keepalive — ignore

		default:
			// Unknown event type — skip silently
		}
	}

	return nil
}

// Message represents the full message object in a streaming envelope.
type Message struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	Usage   Usage          `json:"usage"`
}

// Delta represents the delta payload in content_block_delta events.
type Delta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	Thinking    string `json:"thinking"`
}

// Send sends a non-streaming request to the Messages endpoint.
func (c *NativeClient) Send(ctx context.Context, req *Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
