package util

import (
	"os"
	"path/filepath"
	"strings"
)

// GitIgnore represents a .gitignore patterns
type GitIgnore struct {
	patterns []gitPattern
}

type gitPattern struct {
	negated bool
	dir     bool
	expr    string
}

// ParseGitIgnore parses a .gitignore file
func ParseGitIgnore(path string) (*GitIgnore, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseGitIgnoreContent(string(content)), nil
}

// ParseGitIgnoreContent parses gitignore patterns from content
func ParseGitIgnoreContent(content string) *GitIgnore {
	gi := &GitIgnore{
		patterns: make([]gitPattern, 0),
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle negated patterns
		negated := strings.HasPrefix(line, "!")
		if negated {
			line = line[1:]
		}

		// Handle directory-only patterns
		dir := strings.HasSuffix(line, "/")
		if dir {
			line = line[:len(line)-1]
		}

		// Skip absolute paths
		if strings.HasPrefix(line, "/") {
			line = line[1:]
		}

		// Skip ** patterns for now (simplified)
		if strings.HasPrefix(line, "**") {
			continue
		}

		gi.patterns = append(gi.patterns, gitPattern{
			negated: negated,
			dir:     dir,
			expr:    line,
		})
	}

	return gi
}

// Matches checks if a path should be ignored
func (gi *GitIgnore) Matches(path string) bool {
	path = filepath.Clean(path)
	name := filepath.Base(path)
	dir := filepath.Dir(path)

	for _, p := range gi.patterns {
		// Simple matching
		if p.expr == name || p.expr == "*" {
			return !p.negated
		}

		// Directory matching
		if p.dir {
			if filepath.Base(dir) == p.expr {
				return !p.negated
			}
		}
	}

	return false
}

// FilterPaths filters paths based on gitignore
func (gi *GitIgnore) FilterPaths(paths []string) []string {
	var result []string
	for _, path := range paths {
		if !gi.Matches(path) {
			result = append(result, path)
		}
	}
	return result
}
