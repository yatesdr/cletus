package api

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries int           // default 3
	BaseDelay  time.Duration // default 1s
	MaxDelay   time.Duration // default 30s
}

// RetryClient wraps an LLMClient with retry logic
type RetryClient struct {
	inner      LLMClient
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewRetryClient creates a new retry-wrapped client
func NewRetryClient(inner LLMClient, cfg *RetryConfig) *RetryClient {
	if cfg == nil {
		cfg = &RetryConfig{}
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay == 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	return &RetryClient{
		inner:      inner,
		maxRetries: cfg.MaxRetries,
		baseDelay:  cfg.BaseDelay,
		maxDelay:   cfg.MaxDelay,
	}
}

// Stream sends a streaming request with retry logic
func (c *RetryClient) Stream(ctx context.Context, req *Request, handler StreamResponseHandler) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		err := c.inner.Stream(ctx, req, handler)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryable(err) {
			return err
		}

		// Don't wait after last attempt
		if attempt == c.maxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := c.calculateDelay(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Send sends a non-streaming request with retry logic
func (c *RetryClient) Send(ctx context.Context, req *Request) (*Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.inner.Send(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryable(err) {
			return nil, err
		}

		// Don't wait after last attempt
		if attempt == c.maxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := c.calculateDelay(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel delegates to inner client
func (c *RetryClient) SetModel(model string) {
	c.inner.SetModel(model)
}

// SetMaxTokens delegates to inner client
func (c *RetryClient) SetMaxTokens(maxTokens int) {
	c.inner.SetMaxTokens(maxTokens)
}

// isRetryable determines if an error should trigger a retry
func (c *RetryClient) isRetryable(err error) bool {
	// Check for connection errors
	if netErr, ok := err.(net.Error); ok {
		// Retry on temporary network errors
		if netErr.Temporary() || netErr.Timeout() {
			return true
		}
	}

	// Check if error contains HTTP response info (wrapped in custom error)
	errStr := err.Error()

	// Check for retryable HTTP status codes in error message
	retryableCodes := []int{502, 503, 504, 429}
	for _, code := range retryableCodes {
		if strings.Contains(errStr, fmt.Sprintf(" %d ", code)) || 
		   strings.Contains(errStr, fmt.Sprintf("%d", code)) {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay for exponential backoff
func (c *RetryClient) calculateDelay(attempt int) time.Duration {
	delay := c.baseDelay * time.Duration(math.Pow(2, float64(attempt)))
	if delay > c.maxDelay {
		delay = c.maxDelay
	}
	return delay
}
