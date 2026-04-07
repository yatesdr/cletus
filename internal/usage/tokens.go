package usage

import (
	"regexp"
	"strings"
)

// TokenCounter estimates token counts for text
type TokenCounter struct {
	// Configuration for different tokenization approximations
	charsPerToken float64 // typically 4 for English
}

// NewTokenCounter creates a new token counter
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{
		charsPerToken: 4.0,
	}
}

// Count estimates the number of tokens in text
func (tc *TokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}
	// Simple approximation: characters / charsPerToken
	// For more accurate counting, you'd use tiktoken or similar
	return len(text) / int(tc.charsPerToken)
}

// CountMessages estimates tokens for a message array
func (tc *TokenCounter) CountMessages(messages []string) int {
	var total int
	for _, msg := range messages {
		total += tc.Count(msg)
	}
	return total
}

// EstimateCost estimates the cost in USD based on token counts and pricing
func (tc *TokenCounter) EstimateCost(inputTokens, outputTokens int, pricing ModelPricing) float64 {
	inputCost := float64(inputTokens) / 1_000_000 * pricing.Input
	outputCost := float64(outputTokens) / 1_000_000 * pricing.Output
	return inputCost + outputCost
}

// ModelPricing provides pricing per 1M tokens
type ModelPricing struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// DefaultPricing returns default pricing for unknown models
func DefaultPricing() ModelPricing {
	return ModelPricing{
		Input:  3.0,
		Output: 15.0,
	}
}

// GetPricing returns pricing for a known model
func GetPricing(model string) ModelPricing {
	pricingMap := map[string]ModelPricing{
		// Claude models (Anthropic)
		"claude-opus-4-6":   {Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 7.5},
		"claude-sonnet-4-6": {Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 1.5},
		"claude-haiku-4-5":  {Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 0.4},

		// OpenAI models
		"gpt-4":   {Input: 30.0, Output: 60.0},
		"gpt-4o":  {Input: 5.0, Output: 15.0},
		"gpt-4o-mini": {Input: 0.6, Output: 2.4},
		"gpt-3.5-turbo": {Input: 0.5, Output: 1.5},

		// Meta Llama models (typically free/local)
		"llama-3":    {Input: 0.0, Output: 0.0},
		"llama-3.1":  {Input: 0.0, Output: 0.0},
		"llama-3.2":  {Input: 0.0, Output: 0.0},
		"llama":      {Input: 0.0, Output: 0.0},

		// Qwen models (typically free/local)
		"qwen-3.5":   {Input: 0.0, Output: 0.0},
		"qwen-3":     {Input: 0.0, Output: 0.0},
		"qwen-2.5":   {Input: 0.0, Output: 0.0},

		// MiniMax models (typically free/local)
		"MiniMax-M2.5": {Input: 0.0, Output: 0.0},
		"MiniMax-M2":   {Input: 0.0, Output: 0.0},

		// GLM models (typically free/local)
		"glm-5-turbo":  {Input: 0.0, Output: 0.0},
		"glm-4.7":      {Input: 0.0, Output: 0.0},
		"glm-4":        {Input: 0.0, Output: 0.0},
	}

	// Try exact match first
	if pricing, ok := pricingMap[model]; ok {
		return pricing
	}

	// Try prefix match for model families
	for prefix, pricing := range pricingMap {
		if strings.HasPrefix(model, prefix) {
			return pricing
		}
	}

	return DefaultPricing()
}

// CleanContent removes thinking blocks and other metadata from content
func CleanContent(content string) string {
	// Remove thinking blocks (Claude 4 extended thinking)
	thinkingPattern := regexp.MustCompile(`<thinking>.*?</thinking>`)
	content = thinkingPattern.ReplaceAllString(content, "")

	// Remove redacted thinking
	redactedPattern := regexp.MustCompile(`<redacted_thinking>.*?</redacted_thinking>`)
	content = redactedPattern.ReplaceAllString(content, "")

	return strings.TrimSpace(content)
}
