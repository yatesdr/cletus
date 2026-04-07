package taskoutput

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cletus/internal/tools"
)

// TaskOutputTool gets the output of a task
type TaskOutputTool struct {
	tools.BaseTool
	outputDir string
}

// NewTaskOutputTool creates a new TaskOutputTool
func NewTaskOutputTool() *TaskOutputTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_id": {
				"type": "string",
				"description": "The task ID or PID to get output from"
			},
			"offset": {
				"type": "number",
				"description": "Line offset to start from",
				"default": 0
			},
			"limit": {
				"type": "number",
				"description": "Maximum number of lines to return",
				"default": 100
			},
			"follow": {
				"type": "boolean",
				"description": "Follow output (like tail -f)",
				"default": false
			}
		},
		"required": ["task_id"]
	}`)

	return &TaskOutputTool{
		BaseTool: tools.NewBaseTool("TaskOutput", "Get the output of a running task", schema),
		outputDir: getDefaultOutputDir(),
	}
}

// Execute gets the task output
func (t *TaskOutputTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, err := tools.ParseInput(input)
	if err != nil {
		return "", err
	}

	taskID, ok := tools.GetString(parsed, "task_id")
	if !ok {
		return "", tools.ErrMissingRequiredField("task_id")
	}

	offset := tools.GetIntDefault(parsed, "offset", 0)
	limit := tools.GetIntDefault(parsed, "limit", 100)
	follow, _ := tools.GetBool(parsed, "follow")

	// Try to parse as PID
	pid, err := strconv.Atoi(taskID)
	if err != nil {
		// Not a number, treat as task ID
		return t.getTaskOutput(taskID, offset, limit, follow)
	}

	// For a PID, we can check /proc (Linux) or use ps
	return t.getProcessOutput(pid, offset, limit, follow)
}

func (t *TaskOutputTool) getTaskOutput(taskID string, offset, limit int, follow bool) (string, error) {
	// Check for output file in standard locations
	possiblePaths := []string{
		filepath.Join(t.outputDir, taskID+".out"),
		filepath.Join(t.outputDir, taskID+".log"),
		filepath.Join(os.TempDir(), taskID+".out"),
	}

	for _, path := range possiblePaths {
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			return t.formatLines(lines, offset, limit), nil
		}
	}

	return fmt.Sprintf("No output found for task %s", taskID), nil
}

func (t *TaskOutputTool) getProcessOutput(pid, offset, limit int, follow bool) (string, error) {
	// Check if process exists
	if !processExists(pid) {
		return fmt.Sprintf("Process %d not found or terminated", pid), nil
	}

	// Try to read from /proc/<pid>/fd/1 (Linux)
	procPath := fmt.Sprintf("/proc/%d/fd/1", pid)
	if data, err := os.ReadFile(procPath); err == nil {
		lines := strings.Split(string(data), "\n")
		return t.formatLines(lines, offset, limit), nil
	}

	// For macOS/other, we can't easily read stdout
	return fmt.Sprintf("Process %d is running. Use TaskStop to terminate it.", pid), nil
}

func (t *TaskOutputTool) formatLines(lines []string, offset, limit int) string {
	if offset >= len(lines) {
		return "No output available"
	}

	if offset+limit > len(lines) {
		limit = len(lines) - offset
	}

	selected := lines[offset : offset+limit]
	return strings.Join(selected, "\n")
}

func getDefaultOutputDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "cletus", "tasks")
}

// IsReadOnly returns true
func (t *TaskOutputTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe returns true
func (t *TaskOutputTool) IsConcurrencySafe() bool {
	return true
}
