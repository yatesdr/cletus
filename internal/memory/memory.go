package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Memory represents a memory file with metadata
type Memory struct {
	Path      string
	Content   string
	UpdatedAt time.Time
}

// Scanner scans for memory files
type Scanner struct {
	memoryDir string
	maxCount  int
}

// NewScanner creates a new memory scanner
func NewScanner(memoryDir string, maxCount int) *Scanner {
	if maxCount == 0 {
		maxCount = 200
	}
	return &Scanner{
		memoryDir: memoryDir,
		maxCount:  maxCount,
	}
}

// Scan finds memory files in the directory
func (s *Scanner) Scan() ([]Memory, error) {
	if s.memoryDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(s.memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read memory dir: %w", err)
	}

	var memories []Memory
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".mdx") {
			continue
		}

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		path := filepath.Join(s.memoryDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Parse frontmatter
		parsed := parseFrontmatter(string(content))

		memories = append(memories, Memory{
			Path:      path,
			Content:   parsed.content,
			UpdatedAt: info.ModTime(),
		})
	}

	// Sort by mtime, newest first
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].UpdatedAt.After(memories[j].UpdatedAt)
	})

	// Limit count
	if len(memories) > s.maxCount {
		memories = memories[:s.maxCount]
	}

	return memories, nil
}

// parseFrontmatter extracts content after YAML frontmatter
func parseFrontmatter(content string) struct{ content string } {
	lines := strings.Split(content, "\n")

	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return struct{ content string }{content}
	}

	// Find closing ---
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return struct{ content string }{
				content: strings.Join(lines[i+1:], "\n"),
			}
		}
	}

	return struct{ content string }{content}
}

// FormatMemories formats memories for inclusion in system prompt
func FormatMemories(memories []Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var parts []string
	for _, m := range memories {
		// Include filename and content
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", filepath.Base(m.Path), m.Content))
	}

	return strings.Join(parts, "\n\n---\n\n")
}
