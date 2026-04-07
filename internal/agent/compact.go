package agent

import (
	"fmt"
	"strings"

	"cletus/internal/api"
)

// Compactor handles context compaction
type Compactor struct {
	threshold float64
	maxTokens int
	model     string
	client    api.LLMClient
}

// NewCompactor creates a new context compactor
func NewCompactor(client api.LLMClient, threshold float64, maxTokens int, model string) *Compactor {
	if threshold == 0 {
		threshold = 0.8
	}
	if maxTokens == 0 {
		maxTokens = 200000
	}
	return &Compactor{
		threshold: threshold,
		maxTokens: maxTokens,
		model:     model,
		client:    client,
	}
}

// ShouldCompact checks if compaction is needed
func (c *Compactor) ShouldCompact(usage api.Usage) bool {
	totalTokens := usage.InputTokens + usage.OutputTokens
	return float64(totalTokens) >= float64(c.maxTokens)*c.threshold
}

// Compact summarizes older messages to reduce context
func (c *Compactor) Compact(messages []api.APIMessage) ([]api.APIMessage, error) {
	if len(messages) < 4 {
		return messages, nil
	}

	keepStart := len(messages) - 2
	if keepStart < 0 {
		keepStart = 0
	}

	toCompacts := messages[:keepStart]
	toKeep := messages[keepStart:]

	if len(toCompacts) == 0 {
		return messages, nil
	}

	summary := fmt.Sprintf("[%d messages summarized - %d tokens -> estimated 200 tokens]", 
		len(toCompacts), estimateTokens(toCompacts))

	var result []api.APIMessage
	result = append(result, api.NewTextMessage("system", "Previous conversation summary: "+summary))
	result = append(result, toKeep...)

	return result, nil
}

// CompactWithSummary calls the API to summarize messages
func (c *Compactor) CompactWithSummary(messages []api.APIMessage) ([]api.APIMessage, error) {
	if c.client == nil || len(messages) < 4 {
		return c.Compact(messages)
	}

	keepStart := len(messages) - 2
	if keepStart < 0 {
		keepStart = 0
	}

	toCompacts := messages[:keepStart]
	toKeep := messages[keepStart:]

	if len(toCompacts) == 0 {
		return messages, nil
	}

	// Build summary content from messages
	var content strings.Builder
	for _, msg := range toCompacts {
		switch msg.Role {
		case "user":
			content.WriteString("User: ")
		case "assistant":
			content.WriteString("Assistant: ")
		default:
			content.WriteString(msg.Role + ": ")
		}

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				content.WriteString(block.Text)
			case "tool_use":
				content.WriteString(fmt.Sprintf("[Tool call: %s(%s)]", block.Name, string(block.Input)))
			case "tool_result":
				switch v := block.Content.(type) {
				case string:
					content.WriteString(fmt.Sprintf("[Tool result: %s]", v))
				}
			}
		}
		content.WriteString("\n\n")
	}

	// Call the model to generate a summary
	summary, err := c.generateSummary(content.String(), toCompacts)
	if err != nil {
		// If API call fails, fall back to placeholder
		summary = fmt.Sprintf("[%d messages summarized - API call failed: %v]", len(toCompacts), err)
	}

	var result []api.APIMessage
	result = append(result, api.NewTextMessage("system", "Previous conversation summary: "+summary))
	result = append(result, toKeep...)

	return result, nil
}

// generateSummary calls the API to generate a summary of the messages
func (c *Compactor) generateSummary(conversationText string, messages []api.APIMessage) (string, error) {
	// Build a summary request
	summaryPrompt := `Please summarize the following conversation in 2-3 sentences. Focus on:
- What the user was trying to accomplish
- What tools were used and what changes were made
- Any important decisions or findings

Conversation:
` + conversationText

	req := &api.Request{
		Model:     c.model,
		Messages:  []api.APIMessage{
			{Role: "user", Content: []api.ContentBlock{{Type: "text", Text: summaryPrompt}}},
		},
		MaxTokens: 500,
		Stream:    false,
	}

	resp, err := c.client.Send(nil, req)
	if err != nil {
		return "", fmt.Errorf("summary API call failed: %w", err)
	}

	// Extract text from response
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in summary response")
	}

	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in summary response")
}

func estimateTokens(messages []api.APIMessage) int {
	var total int
	for _, msg := range messages {
		// Content is always []ContentBlock
		for _, block := range msg.Content {
			total += len(block.Text) / 4
		}
	}
	return total
}
