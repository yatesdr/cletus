package repl

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"cletus/internal/tools"
)

// REPLTool starts an interactive REPL
type REPLTool struct {
	tools.BaseTool
}

// NewREPLTool creates REPLTool
func NewREPLTool() *REPLTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"language": {
				"type": "string",
				"description": "Language for REPL (python, node, go, etc.)"
			},
			"code": {
				"type": "string",
				"description": "Code to execute"
			}
		},
		"required": ["language"]
	}`)
	return &REPLTool{
		BaseTool: tools.NewBaseTool("REPL", "Start an interactive REPL session for a language", schema),
	}
}

func (t *REPLTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	lang, _ := tools.GetString(parsed, "language")
	code, _ := tools.GetString(parsed, "code")

	if lang == "" {
		return "", tools.ErrMissingRequiredField("language")
	}

	// Execute code in REPL
	var cmd *exec.Cmd
	switch strings.ToLower(lang) {
	case "python", "python3":
		if code != "" {
			cmd = exec.CommandContext(ctx, "python3", "-c", code)
		} else {
			cmd = exec.CommandContext(ctx, "python3", "-i")
		}
	case "node", "nodejs":
		if code != "" {
			cmd = exec.CommandContext(ctx, "node", "-e", code)
		} else {
			cmd = exec.CommandContext(ctx, "node", "-i")
		}
	case "go":
		if code != "" {
			cmd = exec.CommandContext(ctx, "go", "run", "-")
		} else {
			return "Run 'go run file.go' for Go REPL", nil
		}
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, "bash", "-i")
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}

	if code == "" {
		return fmt.Sprintf("Started %s REPL. Use Bash tool to interact.", lang), nil
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (t *REPLTool) IsReadOnly() bool { return false }
func (t *REPLTool) IsConcurrencySafe() bool { return false }
