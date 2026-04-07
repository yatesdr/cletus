package brief

import (
	"context"
	"encoding/json"
	"time"

	"cletus/internal/tools"
)

// BriefTool sends a message to the user - primary output channel
type BriefTool struct {
	tools.BaseTool
}

// NewBriefTool creates a new BriefTool
func NewBriefTool() *BriefTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {
				"type": "string",
				"description": "The message for the user. Supports markdown formatting."
			},
			"status": {
				"type": "string",
				"enum": ["normal", "proactive"],
				"description": "Use 'proactive' when surfacing something the user hasn't asked for",
				"default": "normal"
			}
		},
		"required": ["message"]
	}`)

	return &BriefTool{
		BaseTool: tools.NewBaseTool("Brief", "Send a message to the user - primary output channel", schema),
	}
}

// Execute sends a message to the user
func (t *BriefTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, err := tools.ParseInput(input)
	if err != nil {
		return "", err
	}

	message, ok := tools.GetString(parsed, "message")
	if !ok {
		return "", tools.ErrMissingRequiredField("message")
	}

	status, _ := tools.GetString(parsed, "status")
	if status == "" {
		status = "normal"
	}

	// Return the message to be displayed to the user
	result := map[string]any{
		"message": message,
		"status": status,
		"sentAt": time.Now().Format(time.RFC3339),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultJSON), nil
}

// IsReadOnly returns true
func (t *BriefTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe returns true
func (t *BriefTool) IsConcurrencySafe() bool {
	return true
}
