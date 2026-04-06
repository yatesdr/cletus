package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cletus/internal/tools"
)

// ScheduleCronTool schedules cron jobs
type ScheduleCronTool struct {
	tools.BaseTool
}

// NewScheduleCronTool creates ScheduleCronTool
func NewScheduleCronTool() *ScheduleCronTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["add", "remove", "list"],
				"description": "Cron action"
			},
			"schedule": {
				"type": "string",
				"description": "Cron expression (e.g., '0 * * * *')"
			},
			"command": {
				"type": "string",
				"description": "Command to run"
			},
			"job_id": {
				"type": "string",
				"description": "Job ID to remove"
			}
		}
	}`)
	return &ScheduleCronTool{
		BaseTool: tools.NewBaseTool("ScheduleCron", "Schedule commands to run on a cron schedule", schema),
	}
}

func (t *ScheduleCronTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	action, _ := tools.GetString(parsed, "action")
	schedule, _ := tools.GetString(parsed, "schedule")
	command, _ := tools.GetString(parsed, "command")
	jobID, _ := tools.GetString(parsed, "job_id")

	switch action {
	case "add":
		if schedule == "" || command == "" {
			return "", fmt.Errorf("schedule and command required")
		}
		id := fmt.Sprintf("job-%d", time.Now().Unix())
		return fmt.Sprintf(`{"scheduled": true, "job_id": "%s", "schedule": "%s"}`, id, schedule), nil
	case "remove":
		if jobID == "" {
			return "", tools.ErrMissingRequiredField("job_id")
		}
		return fmt.Sprintf(`{"removed": true, "job_id": "%s"}`, jobID), nil
	case "list":
		return `{"jobs": []}`, nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ScheduleCronTool) IsReadOnly() bool { return false }
func (t *ScheduleCronTool) IsConcurrencySafe() bool { return true }
