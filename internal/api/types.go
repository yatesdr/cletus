package api

import (
	"encoding/json"
)

// ContentBlock represents content in API messages
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Source    *ImageSource    `json:"source,omitempty"`
}

// ImageSource describes base64 image data
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ToolCall represents a tool call from the assistant
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`       // JSON string
	Input     json.RawMessage `json:"input,omitempty"` // parsed arguments
}

// APIMessage represents a message in the API
// Content is always []ContentBlock for type safety
type APIMessage struct {
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Parts      []ContentBlock `json:"parts,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`   // tool calls from assistant
	ToolCallID string         `json:"tool_call_id,omitempty"` // for tool result messages
}

// NewTextMessage creates a new message with text content
func NewTextMessage(role, text string) APIMessage {
	return APIMessage{
		Role: role,
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
	}
}

// NewToolResultMessage creates a new message with tool result content
func NewToolResultMessage(toolUseID, content string, isError bool) APIMessage {
	return APIMessage{
		Role:       "user",
		ToolCallID: toolUseID,
		Content: []ContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content:   content,
				IsError:   isError,
			},
		},
	}
}

// SystemBlock represents system prompt content
type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl for prompt caching
type CacheControl struct {
	Type string `json:"type"`
}

// ToolSchema defines a tool for the API
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolInput represents parsed tool input
type ToolInput map[string]any

// Request represents an API request
type Request struct {
	Model     string        `json:"model"`
	Messages  []APIMessage  `json:"messages"`
	System    []SystemBlock `json:"system,omitempty"`
	Tools     []ToolSchema  `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
}

// Response represents an API response
type Response struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// Usage tracks token usage
type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// SSE Event types
type SSEEventType string

const (
	SSEEventMessageStart      SSEEventType = "message_start"
	SSEEventContentBlockStart SSEEventType = "content_block_start"
	SSEEventContentBlockDelta SSEEventType = "content_block_delta"
	SSEEventContentBlockStop  SSEEventType = "content_block_stop"
	SSEEventMessageStop       SSEEventType = "message_stop"
	SSEEventMessageDelta      SSEEventType = "message_delta"
	SSEEventError             SSEEventType = "error"
)

// SSEEvent represents a Server-Sent Events message
type SSEEvent struct {
	Type    SSEEventType `json:"type"`
	Index   int          `json:"index,omitempty"`
	Content any          `json:"content,omitempty"`
	Usage   Usage        `json:"usage,omitempty"`
	Done    bool         `json:"done,omitempty"`
}

// Delta types for streaming
type TextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type InputJSONDelta struct {
	Type        string `json:"type"`
	PartialJSON string `json:"partial_json"`
}

type ThinkingDelta struct {
	Type     string `json:"type"`
	Thinking string `json:"thinking"`
}

type ContentBlockStart struct {
	Type string `json:"type"`
	ContentBlock
}

// APIError represents an API error
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// StreamEvent carries a single event from a streaming response.
type StreamEvent struct {
	Type         string
	Usage        Usage
	Content      string
	ContentBlock *ContentBlock
	ToolUse      *ContentBlock
	ToolUseDone  bool
	Done         bool
}

// StreamResponseHandler is a callback invoked for each streaming event.
type StreamResponseHandler func(StreamEvent)
