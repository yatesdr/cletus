package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// fileMtimes stores modification times of files read in this session
var fileMtimes = make(map[string]time.Time)
var fileContents = make(map[string]string)

// FileWriteTool writes files to the filesystem
type FileWriteTool struct {
	BaseTool
}

// NewFileWriteTool creates a new FileWriteTool
func NewFileWriteTool() *FileWriteTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			},
			"force": {
				"type": "boolean",
				"description": "Force write even if file was modified (skip staleness check)",
				"default": false
			}
		},
		"required": ["file_path", "content"]
	}`)
	return &FileWriteTool{
		BaseTool: NewBaseTool("Write", "Writes a file to the local filesystem.", schema),
	}
}

// RecordFileRead records that a file was read (for staleness check)
func RecordFileRead(path string, content string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	// Normalize path
	rel, _ := filepath.Rel("", path)
	fileMtimes[rel] = info.ModTime()
	fileContents[rel] = content
}

// Execute writes the file
func (t *FileWriteTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	filePath, ok := GetString(parsed, "file_path")
	if !ok {
		return "", ErrMissingRequiredField("file_path")
	}

	content, ok := GetString(parsed, "content")
	if !ok {
		return "", ErrMissingRequiredField("content")
	}

	force, _ := GetBool(parsed, "force")

	// Make absolute
	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}

	// Normalize path for lookup
	rel, _ := filepath.Rel("", filePath)

	// Check staleness if not forcing
	if !force {
		if storedMtime, exists := fileMtimes[rel]; exists {
			currentInfo, err := os.Stat(filePath)
			if err == nil && currentInfo.ModTime().After(storedMtime) {
				// File was modified since read
				return "", fmt.Errorf("file was modified since last read. Use force:true to overwrite")
			}
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	// Write atomically: temp file then rename
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename file: %w", err)
	}

	// Update stored mtime to prevent false staleness
	fileMtimes[rel] = time.Now()
	fileContents[rel] = content

	return fmt.Sprintf("File written to: %s", filePath), nil
}
