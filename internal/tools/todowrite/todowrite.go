package todowrite

import (
	"context"
	"encoding/json"
	"fmt"

	"cletus/internal/tools"
)

// TodoWriteTool manages session task checklists
type TodoWriteTool struct {
	tools.BaseTool
	todos map[string][]Todo
}

// Todo represents a single todo item
type Todo struct {
	Content  string `json:"content"`
	Status   string `json:"status"` // "in_progress", "completed", "pending"
	ActiveForm string `json:"activeForm,omitempty"`
}

// Input represents the tool input
type Input struct {
	Todos []Todo `json:"todos"`
}

// Output represents the tool output
type Output struct {
	OldTodos []Todo `json:"oldTodos"`
	NewTodos []Todo `json:"newTodos"`
}

// NewTodoWriteTool creates a new TodoWriteTool
func NewTodoWriteTool() *TodoWriteTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"todos": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"content": {"type": "string"},
						"status": {"type": "string", "enum": ["in_progress", "completed", "pending"]},
						"activeForm": {"type": "string"}
					}
				},
				"description": "The updated todo list"
			}
		},
		"required": ["todos"]
	}`)

	return &TodoWriteTool{
		BaseTool: tools.NewBaseTool("TodoWrite", "Manage the session task checklist", schema),
		todos:    make(map[string][]Todo),
	}
}

// Execute updates the todo list
func (t *TodoWriteTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	var parsed Input
	if err := json.Unmarshal(input, &parsed); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Get old todos (for session key)
	oldTodos := t.todos["default"]
	
	// Update todos
	t.todos["default"] = parsed.Todos

	output := Output{
		OldTodos: oldTodos,
		NewTodos: parsed.Todos,
	}

	outputJSON, _ := json.Marshal(output)
	return string(outputJSON), nil
}

// Schema returns the tool schema
func (t *TodoWriteTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "TodoWrite",
		Description: t.BaseTool.Description(),
		InputSchema: t.BaseTool.InputSchema(),
	}
}

// IsReadOnly returns false
func (t *TodoWriteTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns false
func (t *TodoWriteTool) IsConcurrencySafe() bool {
	return false
}
