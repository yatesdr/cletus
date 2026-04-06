package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cletus/internal/tools"
)

// RemoteTriggerTool triggers remote actions
type RemoteTriggerTool struct {
	tools.BaseTool
}

// NewRemoteTriggerTool creates RemoteTriggerTool
func NewRemoteTriggerTool() *RemoteTriggerTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL to trigger"
			},
			"method": {
				"type": "string",
				"enum": ["GET", "POST", "PUT", "DELETE"],
				"default": "POST"
			},
			"body": {
				"type": "string",
				"description": "Request body"
			},
			"headers": {
				"type": "object",
				"description": "HTTP headers"
			}
		},
		"required": ["url"]
	}`)
	return &RemoteTriggerTool{
		BaseTool: tools.NewBaseTool("RemoteTrigger", "Trigger a remote HTTP endpoint", schema),
	}
}

func (t *RemoteTriggerTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	url, _ := tools.GetString(parsed, "url")
	method, _ := tools.GetString(parsed, "method")
	body, _ := tools.GetString(parsed, "body")

	if url == "" {
		return "", tools.ErrMissingRequiredField("url")
	}
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return fmt.Sprintf(`{"status": %d, "url": "%s"}`, resp.StatusCode, url), nil
}

func (t *RemoteTriggerTool) IsReadOnly() bool { return false }
func (t *RemoteTriggerTool) IsConcurrencySafe() bool { return true }
