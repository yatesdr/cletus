package tui

import (
	"regexp"
	"strings"
)

// MarkdownRenderer converts markdown to tview tags
type MarkdownRenderer struct {
	codeBlock *regexp.Regexp
	link      *regexp.Regexp
	bold      *regexp.Regexp
	italic    *regexp.Regexp
	heading   *regexp.Regexp
	list      *regexp.Regexp
}

// NewMarkdownRenderer creates a new markdown renderer
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{
		codeBlock: regexp.MustCompile("```[\\s\\S]*?```"),
		link:      regexp.MustCompile("\\[([^\\]]+)\\]\\(([^)]+)\\)"),
		bold:      regexp.MustCompile("\\*\\*([^*]+)\\*\\*"),
		italic:    regexp.MustCompile("\\*([^*]+)\\*"),
		heading:   regexp.MustCompile("^#{1,6}\\s+(.+)$"),
		list:      regexp.MustCompile("^[\\-\\*]\\s+(.+)$"),
	}
}

// Render converts markdown to tview color tags
func (m *MarkdownRenderer) Render(md string) string {
	// Code blocks (preserve formatting)
	md = m.codeBlock.ReplaceAllString(md, "[yellow]$0[white]")
	
	// Links
	md = m.link.ReplaceAllString(md, "[blue]$1[white]")
	
	// Bold
	md = m.bold.ReplaceAllString(md, "[::b]$1[::]")
	
	// Italic
	md = m.italic.ReplaceAllString(md, "[i]$1[white]")
	
	// Headings
	md = m.heading.ReplaceAllString(md, "[::b]$1[::]")
	
	// Lists
	md = m.list.ReplaceAllString(md, "• $1")
	
	// Inline code
	md = strings.ReplaceAll(md, "`", "[yellow]")
	
	return md
}

// RenderANSI converts markdown to ANSI escape sequences (for non-tview use)
func (m *MarkdownRenderer) RenderANSI(md string) string {
	// Similar to Render but using ANSI codes
	md = m.codeBlock.ReplaceAllString(md, "\x1b[33m$0\x1b[0m")
	md = m.link.ReplaceAllString(md, "\x1b[36m$1\x1b[0m")
	md = m.bold.ReplaceAllString(md, "\x1b[1m$1\x1b[0m")
	md = m.italic.ReplaceAllString(md, "\x1b[3m$1\x1b[0m")
	md = m.heading.ReplaceAllString(md, "\x1b[1m$1\x1b[0m")
	md = m.list.ReplaceAllString(md, "• $1")
	
	return md
}
