package api

import (
	"context"
	"time"

	"cletus/internal/config"
)

// LLMClient is the interface for all LLM API backends.
type LLMClient interface {
	// Stream sends a streaming request. The handler is called for each streaming event.
	Stream(ctx context.Context, req *Request, handler StreamResponseHandler) error

	// Send sends a non-streaming request to the API.
	Send(ctx context.Context, req *Request) (*Response, error)

	// SetModel sets the default model for requests.
	SetModel(model string)

	// SetMaxTokens sets the default max tokens for requests.
	SetMaxTokens(maxTokens int)
}

// ClientConfig holds configuration for creating an LLMClient
type ClientConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	APIType   string // "openai" or "anthropic"
	MaxTokens int
	Timeout   time.Duration
}

// NewLLMClient creates the appropriate LLMClient based on API type.
// Defaults to OpenAI client if apiType is not recognized.
func NewLLMClient(cfg *ClientConfig) LLMClient {
	if cfg.APIType == "native" || cfg.APIType == "anthropic" {
		return NewNativeClient(&NativeClientConfig{
			BaseURL:   cfg.BaseURL,
			APIKey:    cfg.APIKey,
			Model:     cfg.Model,
			MaxTokens: cfg.MaxTokens,
			Timeout:   cfg.Timeout,
		})
	}

	// Default to OpenAI client
	return NewOpenAIClient(&OpenAIClientConfig{
		BaseURL:   cfg.BaseURL,
		APIKey:    cfg.APIKey,
		Model:     cfg.Model,
		MaxTokens: cfg.MaxTokens,
		Timeout:   cfg.Timeout,
	})
}

// NewLLMClientFromConfig creates an LLMClient from a config.Config.
// Uses the resolved model and backend for the given role.
func NewLLMClientFromConfig(cfg *config.Config, role string) LLMClient {
	modelName := cfg.ResolveModel(role)
	backend := cfg.ResolveBackend(modelName)
	apiType := cfg.ResolveAPIType(modelName)

	// Apply default timeout
	timeout := backend.Timeout
	if timeout == 0 {
		timeout = 300
	}

	return NewLLMClient(&ClientConfig{
		BaseURL:   backend.BaseURL,
		APIKey:    backend.APIKey,
		Model:     modelName,
		APIType:   apiType,
		MaxTokens: cfg.MaxTokens,
		Timeout:   time.Duration(timeout) * time.Second,
	})
}
