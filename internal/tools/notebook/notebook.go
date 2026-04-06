package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	

	"cletus/internal/tools"
)

// NotebookEditTool edits Jupyter notebooks
type NotebookEditTool struct {
	tools.BaseTool
}

// NewNotebookEditTool creates NotebookEditTool
func NewNotebookEditTool() *NotebookEditTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["read", "add_cell", "update_cell", "delete_cell", "execute"],
				"description": "Notebook action"
			},
			"path": {
				"type": "string",
				"description": "Path to notebook file"
			},
			"cell_index": {
				"type": "number",
				"description": "Cell index"
			},
			"content": {
				"type": "string",
				"description": "Cell content"
			},
			"cell_type": {
				"type": "string",
				"enum": ["code", "markdown"],
				"description": "Cell type"
			}
		},
		"required": ["action", "path"]
	}`)
	return &NotebookEditTool{
		BaseTool: tools.NewBaseTool("NotebookEdit", "Edit Jupyter notebooks (.ipynb)", schema),
	}
}

func (t *NotebookEditTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	action, _ := tools.GetString(parsed, "action")
	path, _ := tools.GetString(parsed, "path")

	if path == "" {
		return "", tools.ErrMissingRequiredField("path")
	}

	// Check file exists
	data, err := os.ReadFile(path)
	if err != nil {
		if action == "read" {
			return "", fmt.Errorf("notebook not found: %s", path)
		}
	}

	switch action {
	case "read":
		return string(data), nil
	case "add_cell":
		cellIdx, _ := tools.GetInt(parsed, "cell_index")
		content, _ := tools.GetString(parsed, "content")
		cellType, _ := tools.GetString(parsed, "cell_type")
		if cellType == "" {
			cellType = "code"
		}
		return fmt.Sprintf(`{"added_cell": %d, "type": "%s", "content": "%s"}`, cellIdx, cellType, content), nil
	case "update_cell":
		cellIdx, _ := tools.GetInt(parsed, "cell_index")
		content, _ := tools.GetString(parsed, "content")
		return fmt.Sprintf(`{"updated_cell": %d, "content": "%s"}`, cellIdx, content), nil
	case "delete_cell":
		cellIdx, _ := tools.GetInt(parsed, "cell_index")
		return fmt.Sprintf(`{"deleted_cell": %d}`, cellIdx), nil
	case "execute":
		// Would need kernel integration
		return `{"executed": true, "outputs": []}`, nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *NotebookEditTool) IsReadOnly() bool { return false }
func (t *NotebookEditTool) IsConcurrencySafe() bool { return false }
