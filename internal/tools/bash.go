package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BashTool executes shell commands with full features
type BashTool struct {
	BaseTool
	cwd string
	env map[string]string
}

// NewBashTool creates a new BashTool
func NewBashTool() *BashTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"description": {
				"type": "string",
				"description": "A description of what the command does"
			},
			"timeout": {
				"type": "number",
				"description": "Timeout in milliseconds (default 120000, max 600000)",
				"default": 120000
			},
			"run_in_background": {
				"type": "boolean",
				"description": "Run command in background (default false)",
				"default": false
			}
		},
		"required": ["command"]
	}`)

	cwd, _ := os.Getwd()
	return &BashTool{
		BaseTool: NewBaseTool("Bash", "Executes a given bash command and returns its output.", schema),
		cwd:      cwd,
		env:      make(map[string]string),
	}
}

// Execute runs the bash command
func (t *BashTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	cmdStr, ok := GetString(parsed, "command")
	if !ok {
		return "", ErrMissingRequiredField("command")
	}

	// Get timeout (default 2 minutes, max 10 minutes)
	timeout := GetIntDefault(parsed, "timeout", 120000)
	if timeout > 600000 {
		timeout = 600000
	}
	if timeout <= 0 {
		timeout = 120000
	}

	// Check for background mode
	background, _ := GetBool(parsed, "run_in_background")

	// Validate for dangerous commands
	warning, err := t.checkDestructiveCommands(cmdStr)
	if err != nil {
		return "", err
	}
	if warning != "" {
		progress <- ToolProgress{Type: "warning", Content: warning}
	}

	// Set up context with timeout
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(runCtx, "bash", "-l", "-c", cmdStr)
	cmd.Dir = t.cwd

	// Build environment
	env := os.Environ()
	for k, v := range t.env {
		env = append(env, k+"="+v)
	}
	env = append(env, "TERM=dumb", "CLICOLOR=1")
	cmd.Env = env

	// Set process group for clean termination
	setProcessGroup(cmd)

	if background {
		return t.runBackground(cmd, progress)
	}

	return t.runForeground(cmd, progress, runCtx)
}

// runForeground runs command and streams output
func (t *BashTool) runForeground(cmd *exec.Cmd, progress chan<- ToolProgress, ctx context.Context) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start command: %w", err)
	}

	done := make(chan struct{})
	defer func() {
		select {
		case <-done:
		default:
			killProcessGroup(cmd)
		}
	}()

	var outputBuf bytes.Buffer

	go func() {
		defer close(done)
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			outputBuf.WriteString(line)
			outputBuf.WriteString("\n")
			progress <- ToolProgress{
				Type:      "output",
				Content:   line + "\n",
				LineCount: strings.Count(outputBuf.String(), "\n"),
			}
		}
	}()

	err = cmd.Wait()
	t.updateCWD(cmdStrFromContext(cmd))

	result := outputBuf.String()
	result = strings.TrimRight(result, "\n ")

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %v", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if result == "" {
				result = fmt.Sprintf("Command exited with code %d", exitErr.ExitCode())
			}
			return result, nil
		}
		return result, err
	}

	return result, nil
}

// runBackground runs command in background
func (t *BashTool) runBackground(cmd *exec.Cmd, progress chan<- ToolProgress) (string, error) {
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start background command: %w", err)
	}

	pid := cmd.Process.Pid
	progress <- ToolProgress{
		Type:    "started",
		Content: fmt.Sprintf("Background task started with PID %d", pid),
		TaskID:  strconv.Itoa(pid),
	}

	return fmt.Sprintf("Command started in background. Task ID: %d", pid), nil
}


func (t *BashTool) updateCWD(cmdStr string) {
	cmdStr = strings.TrimSpace(cmdStr)

	if strings.HasPrefix(cmdStr, "cd ") || cmdStr == "cd" {
		parts := strings.Fields(cmdStr)
		if len(parts) >= 2 {
			path := parts[1]
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(t.cwd, path)
			}
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				t.cwd = path
			}
		}
	}
}

// GetCWD returns the current working directory
func (t *BashTool) GetCWD() string {
	return t.cwd
}

// SetCWD sets the current working directory
func (t *BashTool) SetCWD(path string) {
	t.cwd = path
}

func (t *BashTool) checkDestructiveCommands(cmdStr string) (string, error) {
	cmdStr = strings.TrimSpace(cmdStr)

	destructivePatterns := []struct {
		pattern string
		warning string
	}{
		{`^\s*rm\s+-rf\s+/(?:\s|$)`, "WARNING: Attempting to delete root directory"},
		{`^\s*rm\s+-rf\s+\.\*(?:\s|$)`, "WARNING: Attempting to delete all files"},
		{`--no-verify`, "WARNING: Skipping git hooks may be unsafe"},
		{`--force(?:\s|$)`, "WARNING: Force flag may overwrite data"},
		{`git\s+push\s+--force`, "WARNING: Force push can overwrite remote history"},
		{`git\s+reset\s+--hard`, "WARNING: Hard reset can lose uncommitted changes"},
	}

	for _, dp := range destructivePatterns {
		re := regexp.MustCompile(dp.pattern)
		if re.MatchString(cmdStr) {
			return dp.warning, nil
		}
	}

	return "", nil
}

// IsReadOnly returns false for BashTool
func (t *BashTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns false
func (t *BashTool) IsConcurrencySafe() bool {
	return false
}

func cmdStrFromContext(cmd *exec.Cmd) string {
	if cmd.Args == nil || len(cmd.Args) < 3 {
		return ""
	}
	return cmd.Args[2]
}
