package synthetic

import (
	"context"
	"encoding/json"
	"fmt"

	"cletus/internal/tools"
)

// SyntheticOutputTool generates synthetic output for testing
type SyntheticOutputTool struct {
	tools.BaseTool
}

// NewSyntheticOutputTool creates SyntheticOutputTool
func NewSyntheticOutputTool() *SyntheticOutputTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"type": {
				"type": "string",
				"enum": ["error", "warning", "info", "success"],
				"description": "Output type"
			},
			"content": {
				"type": "string",
				"description": "Content to output"
			}
		},
		"required": ["type", "content"]
	}`)
	return &SyntheticOutputTool{
		BaseTool: tools.NewBaseTool("SyntheticOutput", "Generate synthetic output for testing/debugging", schema),
	}
}

func (t *SyntheticOutputTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	outputType, _ := tools.GetString(parsed, "type")
	content, _ := tools.GetString(parsed, "content")

	if content == "" {
		return "", tools.ErrMissingRequiredField("content")
	}

	// Emit progress
	progress <- tools.ToolProgress{
		Type:    outputType,
		Content: content,
	}

	return fmt.Sprintf(`{"type": "%s", "content": "%s"}`, outputType, content), nil
}

func (t *SyntheticOutputTool) IsReadOnly() bool { return true }
func (t *SyntheticOutputTool) IsConcurrencySafe() bool { return true }
