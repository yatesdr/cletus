package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GrepTool searches for patterns in files
type GrepTool struct {
	BaseTool
}

// NewGrepTool creates a new GrepTool
func NewGrepTool() *GrepTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The regex pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "The path to search in (file or directory)"
			},
			"options": {
				"type": "object",
				"properties": {
					"i": {"type": "boolean", "description": "Case insensitive"},
					"n": {"type": "boolean", "description": "Show line numbers"},
					"C": {"type": "number", "description": "Context lines"},
					"max_count": {"type": "number", "description": "Maximum matches"}
				}
			}
		},
		"required": ["pattern"]
	}`)
	return &GrepTool{
		BaseTool: NewBaseTool("Grep", "Searches for text patterns in files.", schema),
	}
}

// Execute searches for the pattern
func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	pattern, ok := GetString(parsed, "pattern")
	if !ok {
		return "", ErrMissingRequiredField("pattern")
	}

	path, _ := GetString(parsed, "path")
	if path == "" {
		path = "."
	}

	// Try ripgrep first
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return t.runRipgrep(rgPath, pattern, path, parsed)
	}

	// Fall back to basic grep
	return t.runBasicGrep(pattern, path, parsed)
}

func (t *GrepTool) runRipgrep(rgPath, pattern, path string, parsed map[string]any) (string, error) {
	args := []string{"--color=never", "-n"}
	
	if opts, ok := parsed["options"].(map[string]any); ok {
		if _, ok := opts["i"]; ok {
			args = append(args, "-i")
		}
		if c, ok := GetInt(opts, "C"); ok {
			args = append(args, "-C", fmt.Sprintf("%d", c))
		}
		if max, ok := GetInt(opts, "max_count"); ok {
			args = append(args, "-m", fmt.Sprintf("%d", max))
		}
	} else {
		// Default limit
		args = append(args, "-m", "250")
	}
	
	args = append(args, pattern, path)
	
	cmd := exec.Command(rgPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return "", fmt.Errorf("grep: %w", err)
	}
	
	return string(output), nil
}

func (t *GrepTool) runBasicGrep(pattern, path string, parsed map[string]any) (string, error) {
	// Basic fallback using find + regexp
	cwd, _ := os.Getwd()
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	var matches []string
	err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(walkPath, ".go") {
			return nil
		}
		
		content, err := os.ReadFile(walkPath)
		if err != nil {
			return nil
		}
		
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, pattern) {
				rel, _ := filepath.Rel(cwd, walkPath)
				matches = append(matches, fmt.Sprintf("%s:%d:%s", rel, i+1, line))
			}
		}
		return nil
	})
	
	if err != nil {
		return "", err
	}
	
	if len(matches) == 0 {
		return "No matches found", nil
	}
	
	// Limit results
	if len(matches) > 250 {
		matches = matches[:250]
	}
	
	return strings.Join(matches, "\n"), nil
}
