package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cletus/internal/vision"
)

const maxLinesToRead = 2000

// FileReadTool reads files from the filesystem
type FileReadTool struct {
	BaseTool
	readCache    map[string]cacheEntry
	pdfProcessor *vision.PDFProcessor
}

type cacheEntry struct {
	content string
	mtime   fs.FileInfo
}

// NewFileReadTool creates a new FileReadTool
func NewFileReadTool() *FileReadTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to read"
			},
			"offset": {
				"type": "number",
				"description": "Line offset to start reading from (default 0)",
				"default": 0
			},
			"limit": {
				"type": "number",
				"description": "Maximum number of lines to read (default 2000)",
				"default": 2000
			},
			"pages": {
				"type": "string",
				"description": "For PDFs: page range like '1-5'"
			}
		},
		"required": ["file_path"]
	}`)

	return &FileReadTool{
		BaseTool:     NewBaseTool("Read", "Reads a file from the local filesystem.", schema),
		readCache:    make(map[string]cacheEntry),
		pdfProcessor: vision.NewPDFProcessor(""),
	}
}

// Execute reads the file
func (t *FileReadTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	filePath, ok := GetString(parsed, "file_path")
	if !ok {
		return "", ErrMissingRequiredField("file_path")
	}

	// Make absolute
	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}

	// Check cache
	if entry, ok := t.readCache[filePath]; ok {
		info, err := os.Stat(filePath)
		if err == nil && info.ModTime().Equal(entry.mtime.ModTime()) {
			progress <- ToolProgress{Type: "info", Content: "File unchanged since last read"}
			return entry.content, nil
		}
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", filePath)
		}
		return "", fmt.Errorf("stat file: %w", err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return "", fmt.Errorf("cannot read directory: %s", filePath)
	}

	// Handle image files
	ext := strings.ToLower(filepath.Ext(filePath))
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".ico", ".tiff"}
	for _, ie := range imageExts {
		if ext == ie {
			return t.readImage(filePath)
		}
	}

	// Handle PDF files
	if ext == ".pdf" {
		return t.readPDF(filePath, parsed)
	}

	// Handle regular text files
	return t.readTextFile(filePath, parsed, info)
}

// readTextFile reads a text file
func (t *FileReadTool) readTextFile(filePath string, parsed map[string]any, info fs.FileInfo) (string, error) {
	offset := GetIntDefault(parsed, "offset", 0)
	limit := GetIntDefault(parsed, "limit", maxLinesToRead)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Apply offset and limit
	start := offset
	if start >= len(lines) {
		return "", fmt.Errorf("offset %d beyond file with %d lines", offset, len(lines))
	}

	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}

	selectedLines := lines[start:end]

	// Build output with line numbers (cat -n format)
	var builder strings.Builder
	for i, line := range selectedLines {
		lineNum := start + i + 1
		builder.WriteString(strconv.Itoa(lineNum))
		builder.WriteString("\t")
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	result := builder.String()

	// Cache the result
	t.readCache[filePath] = cacheEntry{
		content: result,
		mtime:   info,
	}

	// Add info about total lines if truncated
	if end < len(lines) {
		result += fmt.Sprintf("\n... %d more lines (use offset/limit to read more)\n", len(lines)-end)
	}

	return result, nil
}

// readImage reads an image file and returns base64
func (t *FileReadTool) readImage(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var mimeType string
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".bmp":
		mimeType = "image/bmp"
	default:
		mimeType = "application/octet-stream"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("[IMAGE:%s:%d bytes]\n%s", mimeType, len(data), encoded), nil
}

// readPDF reads a PDF file using the PDF processor
func (t *FileReadTool) readPDF(filePath string, parsed map[string]any) (string, error) {
	pages, _ := GetString(parsed, "pages")

	// Try to use PDFProcessor
	if t.pdfProcessor != nil {
		var text string
		var err error

		if pages != "" {
			// Parse page range
			parts := strings.Split(pages, "-")
			if len(parts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil {
					text, err = t.pdfProcessor.ExtractPageRange(filePath, start, end)
				} else {
					text, err = t.pdfProcessor.ExtractText(filePath)
				}
			} else {
				text, err = t.pdfProcessor.ExtractText(filePath)
			}
		} else {
			text, err = t.pdfProcessor.ExtractText(filePath)
		}

		if err == nil {
			// Limit output to prevent huge responses
			lines := strings.Split(text, "\n")
			if len(lines) > maxLinesToRead {
				text = strings.Join(lines[:maxLinesToRead], "\n") + fmt.Sprintf("\n... %d more lines", len(lines)-maxLinesToRead)
			}
			return text, nil
		}
		// Fall through to error message if PDF processor fails
	}

	// Fallback message
	if pages != "" {
		return fmt.Sprintf("[PDF file: %s, pages: %s]\n(pdf extraction not available)", filePath, pages), nil
	}
	return fmt.Sprintf("[PDF file: %s]\n(pdf extraction not available - install poppler-utils)", filePath), nil
}

// ClearCache clears the read cache
func (t *FileReadTool) ClearCache() {
	t.readCache = make(map[string]cacheEntry)
}
