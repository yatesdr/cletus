package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputView provides a multi-line text area with history navigation and
// slash-command completion.
// Enter submits; Ctrl+J or Shift+Enter inserts a newline.
// Pasting more than 5 lines collapses to an inline [+N lines pasted] tag;
// the full content is restored on submit.
type InputView struct {
	container        *tview.Flex
	completionBar    *tview.TextView
	area             *tview.TextArea
	history          []string
	historyPos       int
	onSubmit         func(string)
	onResize         func(lines int)
	maxHistory       int
	allCompletions   []string
	matches          []string
	matchIdx         int
	lastEscapeAt     time.Time
	pastedText       string // full text of the collapsed paste
	isCollapsed      bool
	inCollapseUpdate bool
	prevText         string // TextArea content before last change (for paste detection)
	prevLineCount    int
}

// NewInputView creates a new input view
func NewInputView(onSubmit func(string)) *InputView {
	area := tview.NewTextArea()
	area.SetPlaceholder("send a message  (Ctrl+J for newline  ·  / for commands  ·  PgUp/PgDn to scroll)")
	area.SetLabel(" ▸ ")
	area.SetLabelStyle(tcell.StyleDefault.Foreground(tcell.NewHexColor(0xFFAA00)))
	area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))
	area.SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.NewHexColor(0x444444)))
	area.SetBorder(false)
	area.SetWrap(true)

	completionBar := tview.NewTextView()
	completionBar.SetDynamicColors(true)
	completionBar.SetBorder(false)
	completionBar.SetBackgroundColor(tcell.NewHexColor(0x1a1a1a))

	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(completionBar, 0, 0, false).
		AddItem(area, 0, 1, true)

	iv := &InputView{
		container:     container,
		completionBar: completionBar,
		area:          area,
		history:       []string{},
		historyPos:    -1,
		onSubmit:      onSubmit,
		maxHistory:    100,
		matchIdx:      -1,
		prevLineCount: 1,
	}

	area.SetInputCapture(iv.handleKey)
	area.SetChangedFunc(func() {
		if iv.inCollapseUpdate {
			return
		}

		text := iv.area.GetText()
		lines := strings.Count(text, "\n") + 1

		// If collapsed, verify the tag is still present.
		if iv.isCollapsed {
			tag := iv.pasteTag()
			if !strings.Contains(text, tag) {
				// User deleted the tag — collapse is gone.
				iv.isCollapsed = false
				iv.pastedText = ""
			}
			iv.prevText = text
			iv.prevLineCount = lines
			iv.updateCompletions()
			if iv.onResize != nil {
				iv.onResize(lines)
			}
			return
		}

		// Detect a large paste: line count jumped by more than 4 at once.
		if lines > iv.prevLineCount+4 && lines > 5 {
			pre := commonPrefix(iv.prevText, text)
			suf := commonSuffix(iv.prevText[len(pre):], text[len(pre):])
			pasted := text[len(pre) : len(text)-len(suf)]

			iv.pastedText = pasted
			iv.isCollapsed = true
			tag := iv.pasteTag()
			newText := pre + tag + suf

			iv.inCollapseUpdate = true
			iv.area.SetText(newText, true)
			iv.inCollapseUpdate = false

			lines = strings.Count(newText, "\n") + 1
			iv.prevText = newText
			iv.prevLineCount = lines
			if iv.onResize != nil {
				iv.onResize(lines)
			}
			iv.updateCompletions()
			return
		}

		iv.prevText = text
		iv.prevLineCount = lines
		iv.updateCompletions()
		if iv.onResize != nil {
			iv.onResize(lines)
		}
	})

	return iv
}

// pasteTag returns the inline indicator string for the current pastedText.
func (i *InputView) pasteTag() string {
	n := strings.Count(i.pastedText, "\n") + 1
	return fmt.Sprintf("[+%d lines pasted]", n)
}

// SetCompletions sets the list of slash-command completions (e.g. "/help", "/clear").
func (i *InputView) SetCompletions(cmds []string) {
	i.allCompletions = cmds
}

// SetOnResize sets a callback invoked when the content line count changes.
func (i *InputView) SetOnResize(fn func(lines int)) {
	i.onResize = fn
}

// GetPrimitive returns the tview primitive (container wrapping bar + area).
func (i *InputView) GetPrimitive() tview.Primitive {
	return i.container
}

// handleKey intercepts key events on the text area.
func (i *InputView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		mods := event.Modifiers()
		if mods&tcell.ModShift != 0 || mods&tcell.ModAlt != 0 {
			i.insertNewline()
			return nil
		}
		if len(i.matches) > 0 && i.matchIdx >= 0 {
			cmd := i.matches[i.matchIdx]
			i.area.SetText(cmd, false)
			i.clearCompletions()
			i.doSubmit()
			return nil
		}
		i.doSubmit()
		return nil

	case tcell.KeyLF: // Ctrl+J
		i.insertNewline()
		return nil

	case tcell.KeyTab:
		if len(i.matches) > 0 {
			i.matchIdx = (i.matchIdx + 1) % len(i.matches)
			i.renderCompletions()
			return nil
		}
		return event

	case tcell.KeyBacktab:
		if len(i.matches) > 0 {
			i.matchIdx = (i.matchIdx - 1 + len(i.matches)) % len(i.matches)
			i.renderCompletions()
			return nil
		}
		return event

	case tcell.KeyEscape:
		if len(i.matches) > 0 {
			i.clearCompletions()
			i.lastEscapeAt = time.Time{}
			return nil
		}
		now := time.Now()
		if !i.lastEscapeAt.IsZero() && now.Sub(i.lastEscapeAt) < 500*time.Millisecond {
			// Double-escape: clear everything including any collapsed paste.
			i.isCollapsed = false
			i.pastedText = ""
			i.prevText = ""
			i.prevLineCount = 1
			i.area.SetText("", false)
			i.lastEscapeAt = time.Time{}
		} else {
			i.lastEscapeAt = now
		}
		return nil

	case tcell.KeyUp:
		fromRow, _, _, _ := i.area.GetCursor()
		if fromRow == 0 {
			if i.navigateHistoryUp() {
				return nil
			}
		}
		return event

	case tcell.KeyDown:
		text := i.area.GetText()
		lineCount := strings.Count(text, "\n") + 1
		fromRow, _, _, _ := i.area.GetCursor()
		if fromRow >= lineCount-1 {
			if i.navigateHistoryDown() {
				return nil
			}
		}
		return event
	}

	return event
}

// insertNewline inserts a newline at the cursor position.
func (i *InputView) insertNewline() {
	_, pos, _ := i.area.GetSelection()
	i.area.Replace(pos, pos, "\n")
}

// updateCompletions filters allCompletions based on current input text.
func (i *InputView) updateCompletions() {
	text := i.area.GetText()
	if !strings.HasPrefix(text, "/") || strings.Contains(text, "\n") {
		i.clearCompletions()
		return
	}

	prefix := strings.ToLower(strings.TrimSpace(text))
	var matched []string
	for _, c := range i.allCompletions {
		if strings.HasPrefix(strings.ToLower(c), prefix) {
			matched = append(matched, c)
		}
	}

	if len(matched) == 0 {
		i.clearCompletions()
		return
	}

	i.matches = matched
	if i.matchIdx >= len(i.matches) {
		i.matchIdx = 0
	}
	if i.matchIdx < 0 {
		i.matchIdx = 0
	}
	i.renderCompletions()
	i.container.ResizeItem(i.completionBar, 1, 0)
}

func (i *InputView) renderCompletions() {
	var parts []string
	for idx, m := range i.matches {
		if idx == i.matchIdx {
			parts = append(parts, fmt.Sprintf("[black:#FFAA00] %s [white:-]", m))
		} else {
			parts = append(parts, fmt.Sprintf("[#888888] %s [-]", m))
		}
	}
	i.completionBar.SetText("  " + strings.Join(parts, "  "))
}

func (i *InputView) clearCompletions() {
	i.matches = nil
	i.matchIdx = -1
	i.completionBar.SetText("")
	i.container.ResizeItem(i.completionBar, 0, 0)
}

func (i *InputView) doSubmit() {
	text := strings.TrimRight(i.area.GetText(), "\n")

	// Substitute the collapse tag back to the full pasted content.
	if i.isCollapsed {
		tag := i.pasteTag()
		text = strings.Replace(text, tag, i.pastedText, 1)
		text = strings.TrimRight(text, "\n")
		i.isCollapsed = false
		i.pastedText = ""
	}

	if text == "" {
		return
	}

	i.history = append(i.history, text)
	if len(i.history) > i.maxHistory {
		i.history = i.history[len(i.history)-i.maxHistory:]
	}
	i.historyPos = -1
	i.prevText = ""
	i.prevLineCount = 1

	i.area.SetText("", false)
	i.clearCompletions()

	if i.onSubmit != nil {
		i.onSubmit(text)
	}
}

func (i *InputView) navigateHistoryUp() bool {
	if len(i.history) == 0 {
		return false
	}
	if i.historyPos < len(i.history)-1 {
		i.historyPos++
		idx := len(i.history) - 1 - i.historyPos
		i.area.SetText(i.history[idx], true)
		return true
	}
	return false
}

func (i *InputView) navigateHistoryDown() bool {
	if i.historyPos > 0 {
		i.historyPos--
		idx := len(i.history) - 1 - i.historyPos
		i.area.SetText(i.history[idx], true)
		return true
	} else if i.historyPos == 0 {
		i.historyPos = -1
		i.area.SetText("", false)
		return true
	}
	return false
}

// HandleKey is kept for compatibility (no-op; handled inside area capture).
func (i *InputView) HandleKey(event *tcell.EventKey) bool {
	return false
}

// SetText sets the input text.
func (i *InputView) SetText(text string) {
	i.area.SetText(text, true)
}

// GetText gets the input text.
func (i *InputView) GetText() string {
	return i.area.GetText()
}

// GetInputField returns the underlying TextArea primitive (for focus management).
func (i *InputView) GetInputField() tview.Primitive {
	return i.area
}

// ClearHistory clears input history.
func (i *InputView) ClearHistory() {
	i.history = []string{}
	i.historyPos = -1
}

// SetPlaceholder sets the placeholder text.
func (i *InputView) SetPlaceholder(text string) {
	i.area.SetPlaceholder(text)
}

// commonPrefix returns the longest common prefix of a and b.
func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}

// commonSuffix returns the longest common suffix of a and b.
func commonSuffix(a, b string) string {
	i, j := len(a)-1, len(b)-1
	count := 0
	for i >= 0 && j >= 0 && a[i] == b[j] {
		i--
		j--
		count++
	}
	if count == 0 {
		return ""
	}
	return a[len(a)-count:]
}
