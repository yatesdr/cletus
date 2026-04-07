package api

import (
	"encoding/json"
)

// OpenAI Chat Completions types

// ChatMessage represents a message in OpenAI Chat Completions format
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentPart
	Name       string      `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`   // assistant's tool calls
	ToolCallID string           `json:"tool_call_id,omitempty"` // for role:"tool" messages
}

// OpenAIToolCall represents a tool call in an OpenAI assistant message (wire format)
type OpenAIToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and arguments
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string, NOT object
}

// ContentPart represents a content block in OpenAI format
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in content
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// ChatTool represents a tool in OpenAI Chat Completions format
type ChatTool struct {
	Type     string        `json:"type"`
	Function *ChatFunction `json:"function,omitempty"`
}

// ChatFunction represents a function tool
type ChatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ChatToolChoice represents tool choice in OpenAI format
type ChatToolChoice struct {
	Type     string        `json:"type"` // "none", "auto", "function"
	Function *FunctionCall `json:"function,omitempty"`
}

// FunctionCall represents a function call in tool_choice
type FunctionCall struct {
	Name string `json:"name"`
}

// ChatCompletionRequest represents an OpenAI Chat Completions request
type ChatCompletionRequest struct {
	Model            string          `json:"model"`
	Messages         []ChatMessage   `json:"messages"`
	Tools            []ChatTool      `json:"tools,omitempty"`
	ToolChoice       *ChatToolChoice `json:"tool_choice,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	N                int             `json:"n,omitempty"` // number of completions
	Stream           bool            `json:"stream"`
	StreamOptions    *StreamOptions  `json:"stream_options,omitempty"`
	MaxTokens        int             `json:"max_tokens,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	User             string          `json:"user,omitempty"`
}

// StreamOptions for include_usage in streaming
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatCompletionResponse represents an OpenAI Chat Completions response
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             ChatCompletionUsage    `json:"usage"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChoice represents a choice in the response
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage represents token usage
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionStreamResponse represents a streaming response chunk
type ChatCompletionStreamResponse struct {
	ID                string                       `json:"id"`
	Object            string                       `json:"object"`
	Created           int64                        `json:"created"`
	Model             string                       `json:"model"`
	Choices           []ChatCompletionStreamChoice `json:"choices"`
	Usage             *ChatCompletionUsage         `json:"usage,omitempty"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty"`
}

// ChatCompletionStreamChoice represents a streaming choice
type ChatCompletionStreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// StreamDelta is the delta object in a streaming chunk.
// Different from ChatMessage because fields are optional/partial.
type StreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   *string          `json:"content,omitempty"`   // pointer so null is different from ""
	Reasoning *string          `json:"reasoning,omitempty"` // thinking/reasoning from models that support it
	ToolCalls []StreamToolCall `json:"tool_calls,omitempty"`
}

// StreamToolCall is a partial tool call in a streaming delta.
// The first chunk has ID+Name, subsequent chunks append to Arguments.
type StreamToolCall struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`   // only in first chunk
	Type     string            `json:"type,omitempty"` // only in first chunk, "function"
	Function *StreamToolCallFn `json:"function,omitempty"`
}

// StreamToolCallFn holds the function name and arguments for streaming
type StreamToolCallFn struct {
	Name      string `json:"name,omitempty"`      // only in first chunk
	Arguments string `json:"arguments,omitempty"` // appended each chunk
}

// OpenAIError represents an OpenAI API error
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func (e *OpenAIError) Error() string {
	return e.Message
}

// OpenAIErrorResponse represents an error response from OpenAI
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}
