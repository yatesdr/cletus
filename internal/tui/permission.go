package tui

import (
	"github.com/rivo/tview"
)

// PermissionMode represents the user's response to a permission request
type PermissionMode int

const (
	PermissionDenied PermissionMode = iota
	PermissionAllowed
	PermissionAlways
)

// PermissionModal shows a permission dialog
type PermissionModal struct {
	modal      *tview.Modal
	resultChan chan PermissionMode
}

// NewPermissionModal creates a new permission modal
func NewPermissionModal() *PermissionModal {
	modal := tview.NewModal()

	pm := &PermissionModal{
		modal:      modal,
		resultChan: make(chan PermissionMode, 1),
	}

	// Deny first (safer default), then Allow, then Always Allow
	pm.modal.AddButtons([]string{"Deny", "Allow", "Always Allow"})
	pm.modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonIndex {
		case 0:
			pm.resultChan <- PermissionDenied
		case 1:
			pm.resultChan <- PermissionAllowed
		case 2:
			pm.resultChan <- PermissionAlways
		}
	})

	return pm
}

// GetPrimitive returns the tview primitive
func (p *PermissionModal) GetPrimitive() tview.Primitive {
	return p.modal
}

// Ask prompts the user with a permission request and blocks until answered.
func (p *PermissionModal) Ask(toolName, details string) PermissionMode {
	text := "Tool requires permission:\n\n[#FFAA00]" + toolName + "[white]"
	if details != "" {
		text += "\n\n" + details
	}
	p.modal.SetText(text)
	return <-p.resultChan
}

// SetText sets the modal text
func (p *PermissionModal) SetText(text string) {
	p.modal.SetText(text)
}
