package util

import (
	"fmt"
	"strings"
)

// MarkdownToANSI converts markdown to ANSI escape sequences for terminal
func MarkdownToANSI(md string) string {
	var result strings.Builder
	lines := strings.Split(md, "\n")
	inCodeBlock := false

	for _, line := range lines {
		// Code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				result.WriteString("[reset]\n")
			} else {
				result.WriteString("[dim]")
			}
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			result.WriteString(line + "\n")
			continue
		}

		// Headers
		if strings.HasPrefix(line, "### ") {
			result.WriteString("[bold]" + line[4:] + "[reset]\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			result.WriteString("[bold]" + line[3:] + "[reset]\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			result.WriteString("[bold]" + line[2:] + "[reset]\n")
			continue
		}

		// List items
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString("[yellow]*[reset] " + line[2:] + "\n")
			continue
		}

		// Inline code
		line = strings.ReplaceAll(line, "`", "[dim]")
		line = strings.ReplaceAll(line, "**", "[bold]")

		result.WriteString(line + "\n")
	}

	return result.String()
}

// ExtractCodeBlocks extracts code blocks from markdown
func ExtractCodeBlocks(md string) []string {
	var blocks []string
	lines := strings.Split(md, "\n")
	var current strings.Builder
	inBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				blocks = append(blocks, current.String())
				current.Reset()
			}
			inBlock = !inBlock
			continue
		}
		if inBlock {
			current.WriteString(line + "\n")
		}
	}

	return blocks
}

// CountTokens roughly estimates token count
func CountTokens(text string) int {
	return len(text) / 4
}

// FormatCost formats cost in USD
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}
