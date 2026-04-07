package taskstop

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"cletus/internal/tools"
)

// TaskStopTool stops a running task by PID
type TaskStopTool struct {
	tools.BaseTool
}

// NewTaskStopTool creates a new TaskStopTool
func NewTaskStopTool() *TaskStopTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_id": {
				"type": "string",
				"description": "The task ID or PID to stop"
			},
			"force": {
				"type": "boolean",
				"description": "Force kill the process (default false)",
				"default": false
			}
		},
		"required": ["task_id"]
	}`)

	return &TaskStopTool{
		BaseTool: tools.NewBaseTool("TaskStop", "Stop a running task", schema),
	}
}

// Execute stops the task
func (t *TaskStopTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, err := tools.ParseInput(input)
	if err != nil {
		return "", err
	}

	taskID, ok := tools.GetString(parsed, "task_id")
	if !ok {
		return "", tools.ErrMissingRequiredField("task_id")
	}

	force, _ := tools.GetBool(parsed, "force")

	// Try to parse as PID first
	pid, err := strconv.Atoi(taskID)
	if err != nil {
		return fmt.Sprintf("Task '%s' not found. Provide a numeric PID.", taskID), nil
	}

	err = sendSignal(pid, force)
	if err != nil {
		return "", fmt.Errorf("failed to stop task %d: %w", pid, err)
	}

	if force {
		return fmt.Sprintf("Forcefully killed task %d", pid), nil
	}
	return fmt.Sprintf("Sent termination signal to task %d", pid), nil
}

// StopProcessByName attempts to stop a process by name (Unix only)
func StopProcessByName(name string, force bool) error {
	cmd := exec.Command("pkill", "-f", name)
	if force {
		cmd.Args = append(cmd.Args, "-9")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%s: %s", err, output)
		}
		return err
	}
	return nil
}

// IsReadOnly returns false
func (t *TaskStopTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns true
func (t *TaskStopTool) IsConcurrencySafe() bool {
	return true
}
