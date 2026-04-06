package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cletus/internal/tools"
)

// SleepTool waits for a specified duration
type SleepTool struct {
	tools.BaseTool
}

// Input represents the tool input
type Input struct {
	Duration int `json:"duration"` // Duration in milliseconds
}

// NewSleepTool creates a new SleepTool
func NewSleepTool() *SleepTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"duration": {
				"type": "number",
				"description": "Duration to wait in milliseconds"
			}
		},
		"required": ["duration"]
	}`)

	return &SleepTool{
		BaseTool: tools.NewBaseTool("Sleep", "Wait for a specified duration", schema),
	}
}

// Execute waits for the specified duration
func (t *SleepTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	var parsed Input
	if err := json.Unmarshal(input, &parsed); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if parsed.Duration <= 0 {
		parsed.Duration = 1000 // Default 1 second
	}

	// Cap at 5 minutes
	if parsed.Duration > 300000 {
		parsed.Duration = 300000
	}

	progress <- tools.ToolProgress{Type: "output", Content: fmt.Sprintf("Sleeping for %dms...", parsed.Duration)}

	select {
	case <-time.After(time.Duration(parsed.Duration) * time.Millisecond):
		return fmt.Sprintf("Slept for %d milliseconds", parsed.Duration), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Schema returns the tool schema
func (t *SleepTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "Sleep",
		Description: t.BaseTool.Description(),
		InputSchema: t.BaseTool.InputSchema(),
	}
}

// IsReadOnly returns true
func (t *SleepTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe returns true
func (t *SleepTool) IsConcurrencySafe() bool {
	return true
}
