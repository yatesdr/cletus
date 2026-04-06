package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rivo/tview"
)

// ChatView displays the conversation with streaming support
type ChatView struct {
	view        *tview.TextView
	messages    []chatMessage
	maxMessages int
	// streaming state
	streamRole string
	streamRaw  strings.Builder
	streaming  bool
}

type chatMessage struct {
	role    string
	content string // raw markdown text, or pre-formatted tview text if preformatted=true
	preformatted bool
}

// NewChatView creates a new chat view
func NewChatView() *ChatView {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetScrollable(true)
	view.SetWrap(true)
	view.SetBorderPadding(0, 0, 1, 1)
	view.SetRegions(false)

	return &ChatView{
		view:        view,
		messages:    []chatMessage{},
		maxMessages: 1000,
	}
}

// GetPrimitive returns the tview primitive
func (c *ChatView) GetPrimitive() tview.Primitive {
	return c.view
}

// AddMessage adds a complete message to the chat.
func (c *ChatView) AddMessage(role, content string) {
	c.finishStream()
	c.messages = append(c.messages, chatMessage{role: role, content: content})
	if len(c.messages) > c.maxMessages {
		c.messages = c.messages[len(c.messages)-c.maxMessages:]
	}
	c.redraw()
}

// StartStream opens a new streaming message slot for the given role.
func (c *ChatView) StartStream(role string) {
	c.finishStream()
	c.streamRole = role
	c.streamRaw.Reset()
	c.streaming = true
	c.messages = append(c.messages, chatMessage{role: role, content: ""})
	if len(c.messages) > c.maxMessages {
		c.messages = c.messages[len(c.messages)-c.maxMessages:]
	}
	c.redraw()
}

// AppendContent appends a chunk to the streaming message, re-rendering markdown live.
func (c *ChatView) AppendContent(content string) {
	if !c.streaming {
		c.StartStream("assistant")
	}
	c.streamRaw.WriteString(content)
	if len(c.messages) > 0 {
		c.messages[len(c.messages)-1].content = c.streamRaw.String()
	}
	c.redraw()
}

// FinishStream closes the current streaming message.
func (c *ChatView) FinishStream() {
	c.finishStream()
	c.redraw()
}

func (c *ChatView) finishStream() {
	if !c.streaming {
		return
	}
	if len(c.messages) > 0 {
		c.messages[len(c.messages)-1].content = c.streamRaw.String()
	}
	c.streamRole = ""
	c.streamRaw.Reset()
	c.streaming = false
}

// AddThinking adds a collapsed thinking block indicator.
func (c *ChatView) AddThinking(text string) {
	line := strings.TrimSpace(text)
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}
	if len(line) > 80 {
		line = line[:80] + "…"
	}
	content := "[#555555]  ▸ thinking: " + tviewEscape(line) + "[white]"
	c.messages = append(c.messages, chatMessage{role: "thinking", content: content, preformatted: true})
	c.redraw()
}

// AddToolUse adds a compact tool call indicator.
func (c *ChatView) AddToolUse(name, input string) {
	display := formatToolInput(input)
	content := "[#FFAA00]  ⟩ " + name + "[white] " + display
	c.messages = append(c.messages, chatMessage{role: "tool", content: content, preformatted: true})
	c.redraw()
}

// AddToolResult adds a tool result indicator.
func (c *ChatView) AddToolResult(result string, isError bool) {
	result = strings.TrimSpace(result)
	if len(result) > 400 {
		result = result[:400] + "…"
	}
	var content string
	if isError {
		content = "[red]  ✗ " + tviewEscape(result) + "[white]"
	} else {
		lines := strings.Split(result, "\n")
		if len(lines) > 5 {
			extra := len(lines) - 5
			lines = lines[:5]
			lines = append(lines, fmt.Sprintf("[#444444]  … %d more lines[white]", extra))
		}
		var sb strings.Builder
		for i, ln := range lines {
			if i == 0 {
				sb.WriteString("[#555555]  ← " + tviewEscape(ln) + "[white]")
			} else {
				sb.WriteString("\n     " + tviewEscape(ln))
			}
		}
		content = sb.String()
	}
	c.messages = append(c.messages, chatMessage{role: "tool_result", content: content, preformatted: true})
	c.redraw()
}

// ScrollUp scrolls the chat view up by lines.
func (c *ChatView) ScrollUp(lines int) {
	row, col := c.view.GetScrollOffset()
	if row-lines < 0 {
		lines = row
	}
	c.view.ScrollTo(row-lines, col)
}

// ScrollDown scrolls the chat view down by lines.
func (c *ChatView) ScrollDown(lines int) {
	row, col := c.view.GetScrollOffset()
	c.view.ScrollTo(row+lines, col)
}

// Clear clears the chat
func (c *ChatView) Clear() {
	c.finishStream()
	c.messages = []chatMessage{}
	c.view.SetText("")
}

// GetScrollPosition returns current scroll position
func (c *ChatView) GetScrollPosition() (int, int) {
	return c.view.GetScrollOffset()
}

func (c *ChatView) redraw() {
	_, _, width, _ := c.view.GetInnerRect()
	if width <= 0 {
		width = 80
	}

	var builder strings.Builder
	for _, msg := range c.messages {
		builder.WriteString(c.renderMessage(msg, width))
	}
	c.view.SetText(builder.String())
	c.view.ScrollToEnd()
}

func (c *ChatView) renderMessage(msg chatMessage, width int) string {
	switch msg.role {
	case "tool", "tool_result", "thinking":
		return msg.content + "\n"
	case "system":
		return "[#444444]── " + tviewEscape(msg.content) + "[white]\n"
	default:
		return c.renderChatMessage(msg, width)
	}
}

func (c *ChatView) renderChatMessage(msg chatMessage, width int) string {
	var sb strings.Builder

	switch msg.role {
	case "user":
		sb.WriteString("[#555555]>[white] ")
	case "assistant":
		sep := strings.Repeat("─", min(width-10, 60))
		sb.WriteString("[#FFAA00]Cletus[white] [#2a2a2a]" + sep + "[white]\n")
	default:
		sb.WriteString("[#666666]" + msg.role + "[white]\n")
	}

	if msg.content == "" {
		sb.WriteString("[#555555]…[white]\n")
	} else if msg.preformatted {
		sb.WriteString(msg.content + "\n")
	} else {
		rendered := parseMarkdown(msg.content)
		sb.WriteString(rendered)
		if !strings.HasSuffix(rendered, "\n") {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// parseMarkdown converts markdown to tview color tags.
func parseMarkdown(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	codeLang := ""

	for _, line := range lines {
		// Code block fence
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				result.WriteString("[#333333]└" + strings.Repeat("─", 40) + "[white]\n")
				inCodeBlock = false
				codeLang = ""
			} else {
				codeLang = strings.TrimSpace(line[3:])
				header := "┌── "
				if codeLang != "" {
					header += codeLang + " "
				}
				header += strings.Repeat("─", max(0, 40-len(header)))
				result.WriteString("[#333333]" + header + "[white]\n")
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			_ = codeLang
			result.WriteString("[#444444]│[white] [#88CCFF]" + tviewEscape(line) + "[white]\n")
			continue
		}

		// Headers
		if strings.HasPrefix(line, "#### ") {
			result.WriteString("[#FFAA00]" + parseInline(line[5:]) + "[white]\n")
		} else if strings.HasPrefix(line, "### ") {
			result.WriteString("[#FFAA00]" + parseInline(line[4:]) + "[white]\n")
		} else if strings.HasPrefix(line, "## ") {
			result.WriteString("[#FFAA00]" + parseInline(line[3:]) + "[white]\n")
		} else if strings.HasPrefix(line, "# ") {
			result.WriteString("[::b][#FFAA00]" + parseInline(line[2:]) + "[::]" + "[white]\n")
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString("[#FFAA00]•[white] " + parseInline(line[2:]) + "\n")
		} else if regexp.MustCompile(`^\d+\. `).MatchString(line) {
			parts := strings.SplitN(line, ". ", 2)
			result.WriteString("[#FFAA00]" + parts[0] + ".[white] " + parseInline(parts[1]) + "\n")
		} else if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "***" {
			result.WriteString("[#333333]" + strings.Repeat("─", 40) + "[white]\n")
		} else if strings.HasPrefix(line, "> ") {
			result.WriteString("[#444444]▎[white] " + parseInline(line[2:]) + "\n")
		} else {
			result.WriteString(parseInline(line) + "\n")
		}
	}

	return result.String()
}

// parseInline handles inline markdown elements.
func parseInline(text string) string {
	// Inline code
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "[#88CCFF]$1[white]")
	// Bold
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "[::b]$1[::]")
	// Italic
	text = regexp.MustCompile(`\*([^*\s][^*]*)\*`).ReplaceAllString(text, "[::i]$1[::]")
	// Strikethrough
	text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "[#555555]$1[white]")
	// Links — show text only
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "[#88CCFF]$1[white]")
	return text
}

// tviewEscape escapes characters that tview interprets as color tags.
func tviewEscape(s string) string {
	return strings.ReplaceAll(s, "[", "[[")
}

// formatToolInput formats a JSON input string for compact display.
func formatToolInput(input string) string {
	input = strings.TrimSpace(input)
	if input == "{}" || input == "" {
		return ""
	}
	input = strings.Trim(input, "{}")
	input = strings.TrimSpace(input)
	if len(input) > 80 {
		input = input[:80] + "…"
	}
	return "[#555555]" + tviewEscape(input) + "[white]"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
