package tui

import (
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App is the main TUI application
type App struct {
	app        *tview.Application
	pages      *tview.Pages
	chat       *ChatView
	input      *InputView
	status     *StatusBar
	permission *PermissionModal
	eventChan  chan interface{}
	shutdown   chan struct{}
	// cancelMu protects cancelFn
	cancelMu sync.Mutex
	cancelFn func()
}

// Config holds app configuration
type Config struct {
	Model      string
	WorkingDir string
	Mode       string
	OnSubmit   func(string)
	OnQuit     func()
}

// NewApp creates a new TUI application
func NewApp(config Config) *App {
	app := tview.NewApplication()

	chat := NewChatView()
	status := NewStatusBar()
	permission := NewPermissionModal()

	input := NewInputView(func(text string) {
		if config.OnSubmit != nil {
			config.OnSubmit(text)
		}
	})

	// Initialize status bar
	if config.Model != "" {
		status.UpdateModel(config.Model)
	}
	if config.WorkingDir != "" {
		status.SetWorkingDir(config.WorkingDir)
	}
	if config.Mode != "" {
		status.SetMode(config.Mode)
	}

	divider := tview.NewTextView()
	divider.SetDynamicColors(true)
	divider.SetBackgroundColor(tcell.ColorDefault)
	divider.SetText("[#FFAA00]" + strings.Repeat("─", 300) + "[white]")

	const inputMin, inputMax = 5, 20
	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status.GetPrimitive(), 1, 0, false).
		AddItem(chat.GetPrimitive(), 0, 1, false).
		AddItem(divider, 1, 0, false).
		AddItem(input.GetPrimitive(), inputMin, 0, true)

	input.SetOnResize(func(lines int) {
		h := lines + 2 // +1 for label row, +1 padding
		if h < inputMin {
			h = inputMin
		}
		if h > inputMax {
			h = inputMax
		}
		mainLayout.ResizeItem(input.GetPrimitive(), h, 0)
	})

	pages := tview.NewPages()
	pages.AddPage("main", mainLayout, true, true)
	pages.AddPage("permission", permission.GetPrimitive(), false, false)

	app.SetRoot(pages, true)
	app.SetFocus(input.GetInputField())
	app.EnablePaste(true)

	a := &App{
		app:        app,
		pages:      pages,
		chat:       chat,
		input:      input,
		status:     status,
		permission: permission,
		eventChan:  make(chan interface{}, 100),
		shutdown:   make(chan struct{}),
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			if config.OnQuit != nil {
				config.OnQuit()
			}
			app.Stop()
			return nil

		case tcell.KeyEscape:
			a.cancelMu.Lock()
			active := a.cancelFn != nil
			a.cancelMu.Unlock()
			if active {
				a.Cancel()
				return nil
			}
			// No active agent — pass through so the input can handle double-escape
			return event

		case tcell.KeyCtrlL:
			chat.Clear()
			return nil

		case tcell.KeyPgUp:
			chat.ScrollUp(10)
			return nil
		case tcell.KeyPgDn:
			chat.ScrollDown(10)
			return nil
		}

		return event
	})

	return a
}

// Run starts the application
func (a *App) Run() error {
	return a.app.Run()
}

// Stop stops the application
func (a *App) Stop() {
	a.app.Stop()
}

// Chat returns the chat view
func (a *App) Chat() *ChatView {
	return a.chat
}

// Status returns the status bar
func (a *App) Status() *StatusBar {
	return a.status
}

// Input returns the input view
func (a *App) Input() *InputView {
	return a.input
}

// ShowPermission shows a permission modal
func (a *App) ShowPermission(toolName, details string, callback func(PermissionMode)) {
	a.pages.ShowPage("permission")
	a.app.Draw()

	go func() {
		result := a.permission.Ask(toolName, details)
		a.app.QueueUpdate(func() {
			a.pages.HidePage("permission")
			a.app.SetFocus(a.input.GetInputField())
		})
		callback(result)
	}()
}

// FocusInput focuses the input field
func (a *App) FocusInput() {
	a.app.SetFocus(a.input.GetInputField())
}

// SetSlashCompletions sets the available slash-command completions shown in the input bar.
func (a *App) SetSlashCompletions(cmds []string) {
	a.input.SetCompletions(cmds)
}

// EventChan returns the event channel
func (a *App) EventChan() chan<- interface{} {
	return a.eventChan
}

// QueueUpdate queues a function to run on the UI thread
func (a *App) QueueUpdate(f func()) {
	a.app.QueueUpdate(f)
}

// QueueUpdateDraw queues a function to run and then redraw
func (a *App) QueueUpdateDraw(f func()) {
	a.app.QueueUpdateDraw(f)
}

// Shutdown closes the shutdown channel
func (a *App) Shutdown() {
	close(a.shutdown)
}

// SetCancelFn stores a cancel function for the currently running agent turn.
// Pass nil to clear it once the turn completes.
func (a *App) SetCancelFn(fn func()) {
	a.cancelMu.Lock()
	a.cancelFn = fn
	a.cancelMu.Unlock()
}

// Cancel calls the current cancel function if one is set.
func (a *App) Cancel() {
	a.cancelMu.Lock()
	fn := a.cancelFn
	a.cancelMu.Unlock()
	if fn != nil {
		fn()
	}
}
