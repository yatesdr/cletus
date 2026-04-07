package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cletus/internal/util"
)

// GlobTool finds files matching a glob pattern
type GlobTool struct {
	BaseTool
}

// NewGlobTool creates a new GlobTool
func NewGlobTool() *GlobTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The glob pattern to match (e.g., **/*.go, src/**/*.ts, *.json)"
			}
		},
		"required": ["pattern"]
	}`)
	return &GlobTool{
		BaseTool: NewBaseTool("Glob", "Finds files matching a glob pattern.", schema),
	}
}

// Execute finds matching files
func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	pattern, ok := GetString(parsed, "pattern")
	if !ok {
		return "", ErrMissingRequiredField("pattern")
	}

	cwd, _ := os.Getwd()

	// Use robust pattern matching
	matches, err := t.matchPattern(cwd, pattern)
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "No files found", nil
	}

	// Filter using .gitignore if present
	matches = t.filterGitIgnore(cwd, matches)

	if len(matches) == 0 {
		return "No files found (filtered by .gitignore)", nil
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// Limit results
	if len(matches) > 100 {
		matches = matches[:100]
	}

	// Make paths relative
	var results []string
	for _, match := range matches {
		rel, _ := filepath.Rel(cwd, match)
		results = append(results, rel)
	}

	return strings.Join(results, "\n"), nil
}

// matchPattern matches files against a glob pattern with proper ** support
func (t *GlobTool) matchPattern(cwd, pattern string) ([]string, error) {
	// Normalize the pattern
	pattern = filepath.ToSlash(pattern)

	// Check if pattern has ** (recursive match)
	hasDoubleStar := strings.Contains(pattern, "**")

	if hasDoubleStar {
		return t.matchRecursive(cwd, pattern)
	}

	// Simple pattern - use filepath.Glob
	matches, err := filepath.Glob(filepath.Join(cwd, pattern))
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	// Filter to only files
	var files []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && !info.IsDir() {
			files = append(files, m)
		}
	}

	return files, nil
}

// matchRecursive handles ** patterns (recursive matching)
func (t *GlobTool) matchRecursive(cwd, pattern string) ([]string, error) {
	// Split pattern into parts around **
	parts := strings.Split(pattern, "**")

	var baseDir string
	var filePattern string

	if len(parts) == 2 {
		// Pattern like "dir/**" or "**/*.go" or "dir/**/*.go"
		prefix := parts[0]
		suffix := parts[1]

		if prefix != "" {
			// Has directory prefix: "src/**" or "src/**/*.go"
			baseDir = filepath.Join(cwd, prefix)
			if suffix != "" {
				// Has file pattern after **
				filePattern = suffix
			} else {
				// Just "dir/**" - match all files in directory and subdirectories
				filePattern = "*"
			}
		} else {
			// Starts with **: "**/*.go"
			baseDir = cwd
			filePattern = suffix
		}
	} else {
		// Multiple ** (unusual) - treat as single **
		baseDir = cwd
		filePattern = strings.ReplaceAll(pattern, "**", "*")
	}

	// Verify base directory exists
	baseInfo, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	if !baseInfo.IsDir() {
		// Base is a file, not a directory
		return []string{}, nil
	}

	// Build regex pattern from glob
	_ = globToRegex(filePattern)

	var matches []string

	// Walk the directory tree
	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories themselves
		if d.IsDir() {
			// Skip common ignored directories
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" ||
				name == "dist" || name == "build" || name == ".cache" ||
				strings.HasPrefix(name, ".") && name != ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path from cwd
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}

		// Match against the pattern
		relSlash := filepath.ToSlash(rel)
		if matchesGlob(relSlash, pattern) {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// globToRegex converts a glob pattern to a regular expression
func globToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			// Single * matches any characters except /
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches anything including /
				result.WriteString(".*")
				i += 2
			} else {
				result.WriteString("[^/]*")
				i++
			}
		case '?':
			result.WriteString(".")
			i++
		case '.':
			result.WriteString("\\.")
			i++
		case '[':
			// Character class
			result.WriteString("[")
			i++
			for i < len(pattern) && pattern[i] != ']' {
				if pattern[i] == '\\' && i+1 < len(pattern) {
					result.WriteString(string(pattern[i]))
					i++
				}
				result.WriteString(string(pattern[i]))
				i++
			}
			result.WriteString("]")
			i++
		case '/':
			result.WriteString("/")
			i++
		default:
			if regexp.MustCompile(`[a-zA-Z0-9_]`).MatchString(string(c)) {
				result.WriteString(string(c))
			} else {
				result.WriteString("\\")
				result.WriteString(string(c))
			}
			i++
		}
	}

	result.WriteString("$")
	return result.String()
}

// matchesGlob checks if a path matches a glob pattern (supports **)
func matchesGlob(path, pattern string) bool {
	// Convert both to forward slashes for comparison
	path = filepath.ToSlash(pattern)
	pattern = filepath.ToSlash(pattern)

	// Handle ** specially
	if strings.Contains(pattern, "**") {
		// Split around **
		parts := strings.SplitN(pattern, "**", 2)
		prefix := parts[0]
		suffix := parts[1]

		// Check prefix match
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		// Check suffix match (if any)
		if suffix != "" {
			suffix = strings.TrimPrefix(suffix, "/")
			if suffix != "" && !strings.HasSuffix(path, suffix) {
				return false
			}
		}

		return true
	}

	// Simple pattern - use filepath.Match
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// filterGitIgnore filters files using .gitignore patterns
func (t *GlobTool) filterGitIgnore(cwd string, files []string) []string {
	// Look for .gitignore in cwd and parent directories
	gi := t.loadGitIgnore(cwd)
	if gi == nil {
		return files
	}

	var filtered []string
	for _, f := range files {
		rel, err := filepath.Rel(cwd, f)
		if err != nil {
			continue
		}

		// Check if file should be ignored
		if !gi.Matches(rel) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

// loadGitIgnore loads .gitignore from directory hierarchy
func (t *GlobTool) loadGitIgnore(cwd string) *util.GitIgnore {
	// Try cwd first
	if gi, err := util.ParseGitIgnore(filepath.Join(cwd, ".gitignore")); err == nil && gi != nil {
		return gi
	}

	// Try parent
	parent := filepath.Dir(cwd)
	if parent != cwd {
		if gi, err := util.ParseGitIgnore(filepath.Join(parent, ".gitignore")); err == nil && gi != nil {
			return gi
		}
	}

	return nil
}
