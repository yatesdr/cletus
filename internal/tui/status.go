package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// StatusBar displays model, working directory, permission mode, and token info
type StatusBar struct {
	view       *tview.TextView
	model      string
	workingDir string
	mode       string
	prompt     int
	completion int
	cost       float64
	activity   string // current activity phrase, empty when idle
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBackgroundColor(tcell.ColorDefault)

	return &StatusBar{
		view: view,
		mode: "default",
	}
}

// GetPrimitive returns the tview primitive
func (s *StatusBar) GetPrimitive() tview.Primitive {
	return s.view
}

// Update updates the status display
func (s *StatusBar) Update(model string, promptTokens, completionTokens int, cost float64) {
	s.model = model
	s.prompt = promptTokens
	s.completion = completionTokens
	s.cost = cost
	s.redraw()
}

// UpdateModel updates just the model
func (s *StatusBar) UpdateModel(model string) {
	s.model = model
	s.redraw()
}

// UpdateTokens updates token counts
func (s *StatusBar) UpdateTokens(prompt, completion int) {
	s.prompt = prompt
	s.completion = completion
	s.redraw()
}

// UpdateCost updates cost display
func (s *StatusBar) UpdateCost(cost float64) {
	s.cost = cost
	s.redraw()
}

// SetWorkingDir sets the working directory shown in the status bar
func (s *StatusBar) SetWorkingDir(dir string) {
	s.workingDir = dir
	s.redraw()
}

// SetMode sets the permission mode label
func (s *StatusBar) SetMode(mode string) {
	s.mode = mode
	s.redraw()
}

func (s *StatusBar) redraw() {
	var parts []string

	parts = append(parts, "[#FFAA00]Cletus[white]")

	if s.model != "" {
		parts = append(parts, "[#555555]│[white] "+tviewEscape(s.model))
	}

	if s.workingDir != "" {
		dir := abbreviatePath(s.workingDir)
		parts = append(parts, "[#555555]│[white] [#888888]"+tviewEscape(dir)+"[white]")
	}

	if s.prompt > 0 || s.completion > 0 {
		parts = append(parts, fmt.Sprintf("[#555555]│[white] [#888888]↑%d ↓%d[white]", s.prompt, s.completion))
	}

	if s.mode != "" && s.mode != "default" {
		modeColor := "[#888888]"
		switch s.mode {
		case "bypassPermissions", "bypass":
			modeColor = "[red]"
		case "dontAsk", "acceptEdits":
			modeColor = "[#FFAA00]"
		}
		parts = append(parts, "[#555555]│[white] "+modeColor+s.mode+"[white]")
	}

	if s.activity != "" {
		parts = append(parts, "[#555555]│[white] [#888888]"+tviewEscape(s.activity)+"[white]")
	}

	result := "  " + strings.Join(parts, "  ")
	s.view.SetText(result)
}

// abbreviatePath shortens a path for display.
func abbreviatePath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	if len(path) > 40 {
		components := strings.Split(path, "/")
		n := len(components)
		if n >= 3 {
			path = "…/" + strings.Join(components[n-2:], "/")
		}
	}
	return path
}

// SetActivity shows a transient activity phrase in the status bar (e.g. while the model is working).
// Pass an empty string to clear it.
func (s *StatusBar) SetActivity(msg string) {
	s.activity = msg
	s.redraw()
}

// ClearActivity removes the activity indicator.
func (s *StatusBar) ClearActivity() {
	s.activity = ""
	s.redraw()
}

// SetStatus sets a custom status message (used for transient messages)
func (s *StatusBar) SetStatus(msg string) {
	s.view.SetText("  " + msg)
}

// Clear resets the status bar
func (s *StatusBar) Clear() {
	s.model = ""
	s.prompt = 0
	s.completion = 0
	s.cost = 0
	s.redraw()
}

// GetModel returns the current model
func (s *StatusBar) GetModel() string {
	return s.model
}
