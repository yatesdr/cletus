package tui

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// ToolProgress displays running tool progress
type ToolProgress struct {
	view      *tview.TextView
	tools     map[string]*toolInfo
	maxTools  int
}

type toolInfo struct {
	name      string
	startTime time.Time
	status    string
	output    string
}

// NewToolProgress creates a new tool progress view
func NewToolProgress() *ToolProgress {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetScrollable(false)
	view.SetWrap(false)
	
	return &ToolProgress{
		view:     view,
		tools:    make(map[string]*toolInfo),
		maxTools: 10,
	}
}

// GetPrimitive returns the tview primitive
func (t *ToolProgress) GetPrimitive() tview.Primitive {
	return t.view
}

// StartTool marks a tool as started
func (t *ToolProgress) StartTool(id, name string) {
	t.tools[id] = &toolInfo{
		name:      name,
		startTime: time.Now(),
		status:    "running",
	}
	t.redraw()
}

// UpdateTool updates tool status
func (t *ToolProgress) UpdateTool(id, status, output string) {
	if tool, ok := t.tools[id]; ok {
		tool.status = status
		tool.output = output
		t.redraw()
	}
}

// EndTool marks a tool as completed
func (t *ToolProgress) EndTool(id string, success bool) {
	if tool, ok := t.tools[id]; ok {
		if success {
			tool.status = "✓ done"
		} else {
			tool.status = "✗ failed"
		}
		t.redraw()
		
		// Remove after a delay
		time.AfterFunc(5*time.Second, func() {
			delete(t.tools, id)
			t.redraw()
		})
	}
}

// GetToolStatus returns the status of a tool
func (t *ToolProgress) GetToolStatus(id string) string {
	if tool, ok := t.tools[id]; ok {
		return tool.status
	}
	return ""
}

func (t *ToolProgress) redraw() {
	if len(t.tools) == 0 {
		t.view.SetText("")
		return
	}
	
	var lines []string
	count := 0
	for id, tool := range t.tools {
		if count >= t.maxTools {
			break
		}
		
		duration := time.Since(tool.startTime)
		var icon string
		switch tool.status {
		case "running":
			icon = "⚙️"
		case "✓ done":
			icon = "✓"
		case "✗ failed":
			icon = "✗"
		default:
			icon = "•"
		}
		
		line := fmt.Sprintf("%s [yellow]%s[white] (%s) %s", 
			icon, tool.name, tool.status, duration.Round(time.Second))
		
		if tool.output != "" {
			// Truncate output
			output := tool.output
			if len(output) > 50 {
				output = output[:50] + "..."
			}
			line += "\n  → " + output
		}
		
		lines = append(lines, line)
		_ = id // suppress unused warning
		count++
	}
	
	t.view.SetText("[dim]Running tools:[white]\n" + joinLines(lines))
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

// Clear removes all tools
func (t *ToolProgress) Clear() {
	t.tools = make(map[string]*toolInfo)
	t.view.SetText("")
}

// SetMaxTools sets the maximum number of tools to display
func (t *ToolProgress) SetMaxTools(max int) {
	t.maxTools = max
}
