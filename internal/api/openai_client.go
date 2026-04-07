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

	"cletus/internal/util"
)

// OpenAIClient handles OpenAI Chat Completions API communication
type OpenAIClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
}

// OpenAIClientConfig holds configuration for OpenAI client
type OpenAIClientConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// NewOpenAIClient creates a new OpenAI API client
func NewOpenAIClient(cfg *OpenAIClientConfig) *OpenAIClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: cfg.Timeout,
	}

	return &OpenAIClient{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
	}
}

// SetModel sets the default model
func (c *OpenAIClient) SetModel(model string) {
	c.model = model
}

// SetMaxTokens sets the default max tokens
func (c *OpenAIClient) SetMaxTokens(maxTokens int) {
	c.maxTokens = maxTokens
}

// buildChatRequest converts internal Request to OpenAI Chat Completions format
func (c *OpenAIClient) buildChatRequest(req *Request) (*ChatCompletionRequest, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}

	chatReq := &ChatCompletionRequest{
		Model:     req.Model,
		Stream:    req.Stream,
		MaxTokens: req.MaxTokens,
	}

	// Convert messages
	messages := c.convertMessages(req.Messages)
	chatReq.Messages = messages

	// Convert system to system message (OpenAI uses a message, not a separate field)
	if len(req.System) > 0 {
		var systemContent strings.Builder
		for _, sys := range req.System {
			systemContent.WriteString(sys.Text)
			systemContent.WriteString("\n")
		}
		chatReq.Messages = append([]ChatMessage{
			{Role: "system", Content: systemContent.String()},
		}, chatReq.Messages...)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		chatReq.Tools = make([]ChatTool, len(req.Tools))
		for i, tool := range req.Tools {
			chatReq.Tools[i] = ChatTool{
				Type: "function",
				Function: &ChatFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}
	}

	return chatReq, nil
}

// convertMessages converts internal APIMessages to OpenAI ChatMessages.
// Tool result messages (role "user" with tool_result blocks) are expanded into
// one role:"tool" message per result, which is what OpenAI expects.
func (c *OpenAIClient) convertMessages(msgs []APIMessage) []ChatMessage {
	var result []ChatMessage

	for _, msg := range msgs {
		// tool_result blocks must become role:"tool" messages in OpenAI format
		if msg.Role == "user" {
			var toolResults []ContentBlock
			var otherContent []ContentBlock
			for _, block := range msg.Content {
				if block.Type == "tool_result" {
					toolResults = append(toolResults, block)
				} else {
					otherContent = append(otherContent, block)
				}
			}
			// Emit user text/image content first (matches go-llm ordering)
			if len(otherContent) > 0 {
				tmp := APIMessage{Role: "user", Content: otherContent}
				if cm := c.convertMessage(tmp); cm != nil {
					result = append(result, *cm)
				}
			}
			// Then emit one role:"tool" message per tool result
			for _, tr := range toolResults {
				content := ""
				switch v := tr.Content.(type) {
				case string:
					content = v
				}
				result = append(result, ChatMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: tr.ToolUseID,
				})
			}
			continue
		}

		converted := c.convertMessage(msg)
		if converted != nil {
			result = append(result, *converted)
		}
	}

	return result
}

// convertMessage converts internal APIMessage to OpenAI ChatMessage format.
// tool_use blocks in assistant messages are converted to OpenAI tool_calls.
func (c *OpenAIClient) convertMessage(msg APIMessage) *ChatMessage {
	if len(msg.Content) == 0 && len(msg.ToolCalls) == 0 {
		return nil
	}

	// Separate tool_use blocks from regular content
	var toolUseBlocks []ContentBlock
	var otherContent []ContentBlock
	for _, block := range msg.Content {
		if block.Type == "tool_use" {
			toolUseBlocks = append(toolUseBlocks, block)
		} else if block.Type != "tool_result" {
			otherContent = append(otherContent, block)
		}
	}

	// Build content from text/image blocks
	var content interface{}
	if len(otherContent) == 0 {
		content = nil
	} else if len(otherContent) == 1 && otherContent[0].Type == "text" {
		content = otherContent[0].Text
	} else {
		parts := make([]ContentPart, 0, len(otherContent))
		for _, block := range otherContent {
			switch block.Type {
			case "text":
				parts = append(parts, ContentPart{Type: "text", Text: block.Text})
			case "image":
				if block.Source != nil {
					parts = append(parts, ContentPart{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL:    fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data),
							Detail: "auto",
						},
					})
				}
			}
		}
		if len(parts) == 0 {
			content = nil
		} else if len(parts) == 1 && parts[0].Type == "text" {
			content = parts[0].Text
		} else {
			content = parts
		}
	}

	// Convert tool_use content blocks → OpenAI tool_calls
	var toolCalls []OpenAIToolCall
	for _, block := range toolUseBlocks {
		args := string(block.Input)
		if args == "" {
			args = "{}"
		}
		toolCalls = append(toolCalls, OpenAIToolCall{
			ID:   block.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      block.Name,
				Arguments: args,
			},
		})
	}
	// Also handle explicit ToolCalls field (legacy path)
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, OpenAIToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}

	chatMsg := &ChatMessage{
		Role:       msg.Role,
		Content:    content,
		ToolCalls:  toolCalls,
		ToolCallID: msg.ToolCallID,
	}

	return chatMsg
}

// Stream sends a streaming request to the OpenAI API
func (c *OpenAIClient) Stream(ctx context.Context, req *Request, handler StreamResponseHandler) error {
	chatReq, err := c.buildChatRequest(req)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	chatReq.Stream = true
	chatReq.StreamOptions = &StreamOptions{IncludeUsage: true}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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

// processStream processes the streaming response with proper tool call accumulation
func (c *OpenAIClient) processStream(body io.Reader, handler StreamResponseHandler) error {
	events := util.Reader(body)

	// State tracking
	msgStartEmitted := false
	textBlockOpen := false
	thinkingBlockOpen := false
	blockIndex := 0
	_ = blockIndex // used for tracking block order
	toolCallBlocksOpen := make(map[int]bool) // index -> is open

	// Tool call accumulators: map from tool call index → accumulated state
	type toolCallAcc struct {
		id   string
		name string
		args strings.Builder
	}
	toolCalls := make(map[int]*toolCallAcc)

	// Helper to emit message_start once
	emitStart := func() {
		if !msgStartEmitted {
			handler(StreamEvent{Type: "message_start"})
			msgStartEmitted = true
		}
	}

	// Helper to close the current text block
	closeTextBlock := func() {
		if textBlockOpen {
			handler(StreamEvent{Type: "content_block_stop"})
			textBlockOpen = false
		}
	}

	// Helper to close a tool call block
	closeToolCallBlock := func(index int) {
		if toolCallBlocksOpen[index] {
			if acc, ok := toolCalls[index]; ok {
				block := &ContentBlock{
					Type:  "tool_use",
					ID:    acc.id,
					Name:  acc.name,
					Input: json.RawMessage(acc.args.String()),
				}
				handler(StreamEvent{Type: "tool_use", ToolUse: block})
			}
			handler(StreamEvent{Type: "content_block_stop"})
			toolCallBlocksOpen[index] = false
		}
	}

	for event := range events {
		if event.Type == "error" {
			return fmt.Errorf("SSE error: %s", event.Data)
		}

		data := strings.TrimRight(event.Data, "\n")
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Handle usage-only chunks (no choices)
		if len(chunk.Choices) == 0 {
			if chunk.Usage != nil {
				handler(StreamEvent{
					Type: "message_delta",
					Usage: Usage{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
					},
				})
			}
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Emit message_start on first chunk with role
		emitStart()

		// Handle reasoning (thinking) — process before text so thinking block opens first
		if delta.Reasoning != nil && *delta.Reasoning != "" {
			if !thinkingBlockOpen {
				closeTextBlock()
				handler(StreamEvent{
					Type:         "content_block_start",
					ContentBlock: &ContentBlock{Type: "thinking"},
				})
				thinkingBlockOpen = true
			}
			handler(StreamEvent{
				Type:    "content_block_delta",
				Content: *delta.Reasoning,
				ContentBlock: &ContentBlock{Type: "thinking"},
			})
		}

		// Handle content
		if delta.Content != nil && *delta.Content != "" {
			if thinkingBlockOpen {
				handler(StreamEvent{Type: "content_block_stop"})
				blockIndex++
				thinkingBlockOpen = false
			}
			if !textBlockOpen {
				handler(StreamEvent{
					Type:         "content_block_start",
					ContentBlock: &ContentBlock{Type: "text"},
				})
				textBlockOpen = true
			}
			handler(StreamEvent{
				Type:    "content_block_delta",
				Content: *delta.Content,
				ContentBlock: &ContentBlock{Type: "text"},
			})
		}

		// Handle tool calls
		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				index := tc.Index
				acc, exists := toolCalls[index]
				if !exists {
					acc = &toolCallAcc{}
					toolCalls[index] = acc
				}

				// First chunk: emit content_block_start
				if tc.ID != "" && acc.id == "" {
					closeTextBlock()
					closeToolCallBlock(index)

					acc.id = tc.ID
					if tc.Function != nil {
						acc.name = tc.Function.Name
					}

					handler(StreamEvent{
						Type: "content_block_start",
						ContentBlock: &ContentBlock{
							Type: "tool_use",
							ID:   tc.ID,
							Name: acc.name,
						},
					})
					toolCallBlocksOpen[index] = true
				}

				// Append arguments
				if tc.Function != nil && tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)

					handler(StreamEvent{
						Type:    "content_block_delta",
						Content: tc.Function.Arguments,
					})
				}
			}
		}

		// Handle finish
		if choice.FinishReason != "" {
			// Close any open blocks
			closeTextBlock()
			for i := range toolCallBlocksOpen {
				closeToolCallBlock(i)
			}

			handler(StreamEvent{Type: "message_stop", Done: true})
		}
	}

	return nil
}

// Send sends a non-streaming request to the OpenAI API
func (c *OpenAIClient) Send(ctx context.Context, req *Request) (*Response, error) {
	chatReq, err := c.buildChatRequest(req)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
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

	return c.parseResponse(resp.Body)
}

// parseResponse converts OpenAI response to internal Response format
func (c *OpenAIClient) parseResponse(body io.Reader) (*Response, error) {
	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	resp := &Response{
		ID:         chatResp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      chatResp.Model,
		StopReason: mapFinishReason(chatResp.Choices[0].FinishReason),
		Usage: Usage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
		},
	}

	// Convert message content
	msg := chatResp.Choices[0].Message

	// Handle tool calls in the response
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	// Handle message content
	if msg.Content != nil {
		switch content := msg.Content.(type) {
		case string:
			if content != "" {
				resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: content})
			}
		case []any:
			for _, item := range content {
				switch item := item.(type) {
				case map[string]any:
					if item["type"] == "text" {
						if text, ok := item["text"].(string); ok {
							resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: text})
						}
					}
				}
			}
		}
	}

	return resp, nil
}

// mapFinishReason maps OpenAI finish_reason to internal stop_reason
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}
