package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileEditTool edits files using old_string/new_string replacement
type FileEditTool struct {
	BaseTool
}

// NewFileEditTool creates a new FileEditTool
func NewFileEditTool() *FileEditTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact text to replace"
			},
			"new_string": {
				"type": "string",
				"description": "The replacement text"
			},
			"replace_all": {
				"type": "boolean",
				"description": "Replace all occurrences of old_string (default false - only first)",
				"default": false
			}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
	return &FileEditTool{
		BaseTool: NewBaseTool("Edit", "Edits a file using old_string/new_string replacement.", schema),
	}
}

// Execute edits the file
func (t *FileEditTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	filePath, ok := GetString(parsed, "file_path")
	if !ok {
		return "", ErrMissingRequiredField("file_path")
	}

	oldStr, ok := GetString(parsed, "old_string")
	if !ok {
		return "", ErrMissingRequiredField("old_string")
	}

	newStr, ok := GetString(parsed, "new_string")
	if !ok {
		return "", ErrMissingRequiredField("new_string")
	}

	replaceAll, _ := GetBool(parsed, "replace_all")

	// Make absolute
	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	contentStr := string(content)

	// Try exact match first
	found, newContent, err := t.applyEdit(contentStr, oldStr, newStr, replaceAll, false)
	if err != nil {
		return "", err
	}

	if !found {
		// Try whitespace-normalized match
		found, newContent, err = t.applyEdit(contentStr, oldStr, newStr, replaceAll, true)
		if err != nil {
			return "", err
		}
	}

	if !found {
		// Try fuzzy match with indentation-aware matching
		found, newContent, err = t.fuzzyEdit(contentStr, oldStr, newStr, replaceAll)
		if err != nil {
			return "", err
		}
	}

	if !found {
		return "", fmt.Errorf("old_string not found in file. Tried exact, whitespace-normalized, and fuzzy matching")
	}

	// Write back
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename file: %w", err)
	}

	// Count replacements
	replacements := strings.Count(newContent, newStr) - strings.Count(contentStr, newStr)
	if replacements < 0 {
		replacements = 0
	}
	if replaceAll {
		return fmt.Sprintf("File edited: %s (%d replacements)", filePath, replacements), nil
	}
	return fmt.Sprintf("File edited: %s", filePath), nil
}

// applyEdit attempts to replace old with new in content
// normalize: if true, uses whitespace-normalized comparison
func (t *FileEditTool) applyEdit(content, oldStr, newStr string, replaceAll, normalize bool) (bool, string, error) {
	if normalize {
		normalizedContent := normalizeWhitespace(content)
		normalizedOld := normalizeWhitespace(oldStr)

		if !strings.Contains(normalizedContent, normalizedOld) {
			return false, content, nil
		}

		// Need to map normalized position back to original
		// This is complex - for simplicity, use exact match after normalization check
		if replaceAll {
			content = strings.ReplaceAll(content, oldStr, newStr)
		} else {
			content = strings.Replace(content, oldStr, newStr, 1)
		}
		return true, content, nil
	}

	// Exact match
	if !strings.Contains(content, oldStr) {
		return false, content, nil
	}

	if replaceAll {
		content = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		content = strings.Replace(content, oldStr, newStr, 1)
	}
	return true, content, nil
}

// fuzzyEdit performs indentation-aware fuzzy matching
func (t *FileEditTool) fuzzyEdit(content, oldStr, newStr string, replaceAll bool) (bool, string, error) {
	// Split content into lines
	lines := strings.Split(content, "\n")
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Find the best matching position
	bestMatch := t.findBestMatch(lines, oldLines)
	if bestMatch.start < 0 {
		return false, content, nil
	}

	// Get indentation from the matched lines
	indentation := make([]string, len(oldLines))
	for i := bestMatch.start; i < bestMatch.end && i-bestMatch.start < len(oldLines); i++ {
		indented := getIndentation(lines[i])
		indentation[i-bestMatch.start] = indented
	}

	// Apply new string with proper indentation
	var newContentLines []string

	// Add lines before match
	newContentLines = append(newContentLines, lines[:bestMatch.start]...)

	// Add new lines with indentation
	for i, line := range newLines {
		indentedLine := indentation[i%len(indentation)] + strings.TrimLeft(line, " \t")
		newContentLines = append(newContentLines, indentedLine)
	}

	// Add lines after match
	newContentLines = append(newContentLines, lines[bestMatch.end:]...)

	if replaceAll {
		// For replace all, we need to find and replace all occurrences
		// This is a simplified version - could be improved
		return t.fuzzyEditReplaceAll(content, oldStr, newStr)
	}

	return true, strings.Join(newContentLines, "\n"), nil
}

// matchResult holds the result of a fuzzy match
type matchResult struct {
	start int
	end   int
	score float64
}

// findBestMatch finds the best matching position for oldLines in lines
func (t *FileEditTool) findBestMatch(lines, oldLines []string) matchResult {
	if len(oldLines) == 0 || len(lines) == 0 {
		return matchResult{-1, -1, 0}
	}

	bestScore := 0.0
	bestStart := -1

	// Slide oldLines over lines
	for start := 0; start <= len(lines)-len(oldLines); start++ {
		score := t.calculateMatchScore(lines[start:start+len(oldLines)], oldLines)
		if score > bestScore {
			bestScore = score
			bestStart = start
		}
	}

	// Also try with normalized lines (no leading whitespace)
	for start := 0; start <= len(lines)-len(oldLines); start++ {
		normalizedOriginal := make([]string, len(oldLines))
		for i, line := range oldLines {
			normalizedOriginal[i] = strings.TrimLeft(line, " \t")
		}
		normalizedTarget := make([]string, len(oldLines))
		for i := range oldLines {
			normalizedTarget[i] = strings.TrimLeft(lines[start+i], " \t")
		}
		score := t.calculateMatchScore(normalizedTarget, normalizedOriginal)
		if score > bestScore {
			bestScore = score
			bestStart = start
		}
	}

	// Threshold for acceptable match
	if bestScore < 0.7 {
		return matchResult{-1, -1, 0}
	}

	return matchResult{
		start: bestStart,
		end:   bestStart + len(oldLines),
		score: bestScore,
	}
}

// calculateMatchScore calculates how well target matches search
func (t *FileEditTool) calculateMatchScore(target, search []string) float64 {
	if len(target) != len(search) {
		return 0
	}

	matches := 0
	for i := range target {
		tLine := strings.TrimSpace(target[i])
		sLine := strings.TrimSpace(search[i])
		if tLine == sLine {
			matches++
		} else if strings.Contains(tLine, sLine) || strings.Contains(sLine, tLine) {
			matches += 1 // partial match - count as 1
		}
	}

	return float64(matches) / float64(len(search))
}

// fuzzyEditReplaceAll replaces all occurrences using fuzzy matching
func (t *FileEditTool) fuzzyEditReplaceAll(content, oldStr, newStr string) (bool, string, error) {
	// For replace all, we'll use a combination approach
	// First try exact, then whitespace normalized
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	result := content
	found := false

	for {
		lines := strings.Split(result, "\n")
		match := t.findBestMatch(lines, oldLines)

		if match.start < 0 {
			break
		}

		// Get indentation
		indentation := make([]string, len(oldLines))
		for i := match.start; i < match.end && i-match.start < len(oldLines); i++ {
			indentation[i-match.start] = getIndentation(lines[i])
		}

		// Build replacement
		var newContentLines []string
		newContentLines = append(newContentLines, lines[:match.start]...)
		for i, line := range newLines {
			indentedLine := indentation[i%len(indentation)] + strings.TrimLeft(line, " \t")
			newContentLines = append(newContentLines, indentedLine)
		}
		newContentLines = append(newContentLines, lines[match.end:]...)

		result = strings.Join(newContentLines, "\n")
		found = true
	}

	return found, result, nil
}

// getIndentation returns the leading whitespace of a line
func getIndentation(line string) string {
	re := regexp.MustCompile(`^[ \t]*`)
	return re.FindString(line)
}

// normalizeWhitespace reduces whitespace to single spaces
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
