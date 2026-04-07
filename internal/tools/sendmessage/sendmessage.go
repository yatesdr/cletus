package sendmessage

import (
	"context"
	"encoding/json"
	"fmt"

	"cletus/internal/tools"
)

// SendMessageTool sends messages (e.g., to Slack, teams, etc.)
type SendMessageTool struct {
	tools.BaseTool
}

// NewSendMessageTool creates SendMessageTool
func NewSendMessageTool() *SendMessageTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"destination": {
				"type": "string",
				"description": "Destination (e.g., slack, teams, email)"
			},
			"channel": {
				"type": "string",
				"description": "Channel or recipient"
			},
			"message": {
				"type": "string",
				"description": "Message to send"
			}
		},
		"required": ["destination", "message"]
	}`)
	return &SendMessageTool{
		BaseTool: tools.NewBaseTool("SendMessage", "Send a message to external services (Slack, Teams, etc.)", schema),
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	dest, _ := tools.GetString(parsed, "destination")
	channel, _ := tools.GetString(parsed, "channel")
	message, _ := tools.GetString(parsed, "message")

	if dest == "" || message == "" {
		return "", fmt.Errorf("destination and message required")
	}

	// Stub - would integrate with actual messaging services
	return fmt.Sprintf(`{"sent": true, "destination": "%s", "channel": "%s"}`, dest, channel), nil
}

func (t *SendMessageTool) IsReadOnly() bool { return false }
func (t *SendMessageTool) IsConcurrencySafe() bool { return true }
