package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputView provides a multi-line text area with history navigation and
// slash-command completion.
// Enter submits; Ctrl+J or Shift+Enter inserts a newline.
type InputView struct {
	container      *tview.Flex
	completionBar  *tview.TextView
	area           *tview.TextArea
	history        []string
	historyPos     int
	onSubmit       func(string)
	onResize       func(lines int)
	maxHistory     int
	allCompletions []string // set by caller via SetCompletions
	matches        []string // currently filtered matches
	matchIdx       int      // selected match index (-1 = none)
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
		AddItem(completionBar, 0, 0, false). // hidden until completions are active
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
	}

	area.SetInputCapture(iv.handleKey)
	area.SetChangedFunc(func() {
		iv.updateCompletions()
		if iv.onResize != nil {
			text := iv.area.GetText()
			lines := strings.Count(text, "\n") + 1
			iv.onResize(lines)
		}
	})

	return iv
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
		// If a completion is selected, apply it and submit immediately
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
			return nil
		}
		return event

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

// insertNewline inserts a newline at the cursor position directly.
func (i *InputView) insertNewline() {
	_, pos, _ := i.area.GetSelection()
	i.area.Replace(pos, pos, "\n")
}

// updateCompletions filters allCompletions based on current input text.
func (i *InputView) updateCompletions() {
	text := i.area.GetText()
	// Only complete on a single-line input that starts with /
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

func (i *InputView) applyCompletion(cmd string) {
	i.area.SetText(cmd+" ", true)
	i.clearCompletions()
}

func (i *InputView) doSubmit() {
	text := strings.TrimRight(i.area.GetText(), "\n")
	if text == "" {
		return
	}

	i.history = append(i.history, text)
	if len(i.history) > i.maxHistory {
		i.history = i.history[len(i.history)-i.maxHistory:]
	}
	i.historyPos = -1

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
