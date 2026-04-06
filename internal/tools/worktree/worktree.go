package worktree

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"cletus/internal/tools"
)

// EnterWorktreeTool enters a git worktree
type EnterWorktreeTool struct {
	tools.BaseTool
}

// NewEnterWorktreeTool creates EnterWorktreeTool
func NewEnterWorktreeTool() *EnterWorktreeTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the worktree"
			},
			"branch": {
				"type": "string",
				"description": "Branch name"
			}
		},
		"required": ["path"]
	}`)
	return &EnterWorktreeTool{
		BaseTool: tools.NewBaseTool("EnterWorktree", "Enter a git worktree directory", schema),
	}
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	path, _ := tools.GetString(parsed, "path")
	branch, _ := tools.GetString(parsed, "branch")

	if path == "" {
		return "", tools.ErrMissingRequiredField("path")
	}

	// Create worktree if needed
	if branch != "" {
		cmd := exec.CommandContext(ctx, "git", "worktree", "add", path, branch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}
	}

	out, _ := json.Marshal(map[string]string{"status": "entered", "path": path})
	return string(out), nil
}

func (t *EnterWorktreeTool) IsReadOnly() bool { return false }
func (t *EnterWorktreeTool) IsConcurrencySafe() bool { return false }

// ExitWorktreeTool exits a git worktree
type ExitWorktreeTool struct {
	tools.BaseTool
}

// NewExitWorktreeTool creates ExitWorktreeTool
func NewExitWorktreeTool() *ExitWorktreeTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the worktree to remove"
			}
		}
	}`)
	return &ExitWorktreeTool{
		BaseTool: tools.NewBaseTool("ExitWorktree", "Exit and optionally remove a git worktree", schema),
	}
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	path, _ := tools.GetString(parsed, "path")

	if path == "" {
		return "", tools.ErrMissingRequiredField("path")
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", path)
	cmdOut, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(cmdOut), "already clean") {
		return string(cmdOut), err
	}

	out, _ := json.Marshal(map[string]string{"status": "exited", "path": path})
	return string(out), nil
}

func (t *ExitWorktreeTool) IsReadOnly() bool { return false }
func (t *ExitWorktreeTool) IsConcurrencySafe() bool { return false }
