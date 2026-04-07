package powershell

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"cletus/internal/tools"
)

// PowerShellTool executes PowerShell commands
type PowerShellTool struct {
	tools.BaseTool
	cwd string
}

// NewPowerShellTool creates a new PowerShellTool
func NewPowerShellTool() *PowerShellTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The PowerShell command to execute"
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

	cwd, _ := exec.LookPath("pwd")
	return &PowerShellTool{
		BaseTool: tools.NewBaseTool("PowerShell", "Executes PowerShell commands and returns their output.", schema),
		cwd:      cwd,
	}
}

// Execute runs the PowerShell command
func (t *PowerShellTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, err := tools.ParseInput(input)
	if err != nil {
		return "", err
	}

	cmdStr, ok := tools.GetString(parsed, "command")
	if !ok {
		return "", tools.ErrMissingRequiredField("command")
	}

	// Get timeout (default 2 minutes, max 10 minutes)
	timeout := tools.GetIntDefault(parsed, "timeout", 120000)
	if timeout > 600000 {
		timeout = 600000
	}
	if timeout <= 0 {
		timeout = 120000
	}

	background, _ := tools.GetBool(parsed, "run_in_background")

	// Validate for dangerous commands
	warning, err := t.checkDestructiveCommands(cmdStr)
	if err != nil {
		return "", err
	}
	if warning != "" {
		progress <- tools.ToolProgress{Type: "warning", Content: warning}
	}

	// Set up context with timeout
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Find PowerShell executable
	psPath := findPowerShell()
	if psPath == "" {
		return "", fmt.Errorf("PowerShell not found. Ensure PowerShell is installed.")
	}

	cmd := exec.CommandContext(runCtx, psPath, "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	cmd.Dir = t.cwd

	// Build environment
	cmd.Env = append([]string{}, "TERM=dumb")

	if background {
		return t.runBackground(cmd, progress)
	}

	return t.runForeground(cmd, progress, runCtx)
}

func findPowerShell() string {
	// Try common PowerShell locations
	paths := []string{
		"/usr/bin/pwsh",
		"/usr/local/bin/pwsh",
		"/opt/homebrew/bin/pwsh",
		"/usr/bin/powershell",
		"pwsh",
		"powershell",
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}

	return ""
}

func (t *PowerShellTool) runForeground(cmd *exec.Cmd, progress chan<- tools.ToolProgress, ctx context.Context) (string, error) {
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
			cmd.Process.Kill()
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
			progress <- tools.ToolProgress{
				Type:      "output",
				Content:   line + "\n",
				LineCount: strings.Count(outputBuf.String(), "\n"),
			}
		}
	}()

	err = cmd.Wait()
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

func (t *PowerShellTool) runBackground(cmd *exec.Cmd, progress chan<- tools.ToolProgress) (string, error) {
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start background command: %w", err)
	}

	pid := cmd.Process.Pid
	progress <- tools.ToolProgress{
		Type:    "started",
		Content: fmt.Sprintf("Background task started with PID %d", pid),
	}

	return fmt.Sprintf("Command started in background. Task ID: %d", pid), nil
}

func (t *PowerShellTool) checkDestructiveCommands(cmdStr string) (string, error) {
	cmdStr = strings.TrimSpace(cmdStr)

	destructivePatterns := []struct {
		pattern string
		warning string
	}{
		{`Remove-Item\s+-Recurse\s+-Force`, "WARNING: Attempting recursive delete"},
		{`Format-Volume`, "WARNING: Format-Volume will erase all data"},
		{`Clear-Disk`, "WARNING: Clear-Disk will erase disk data"},
		{`Stop-Computer`, "WARNING: Shutting down computer"},
		{`Restart-Computer`, "WARNING: Restarting computer"},
	}

	for _, dp := range destructivePatterns {
		re := regexp.MustCompile(dp.pattern)
		if re.MatchString(cmdStr) {
			return dp.warning, nil
		}
	}

	return "", nil
}

// GetCWD returns the current working directory
func (t *PowerShellTool) GetCWD() string {
	return t.cwd
}

// SetCWD sets the current working directory
func (t *PowerShellTool) SetCWD(path string) {
	t.cwd = path
}

// IsReadOnly returns false for PowerShellTool
func (t *PowerShellTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns false
func (t *PowerShellTool) IsConcurrencySafe() bool {
	return false
}
