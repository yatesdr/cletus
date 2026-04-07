package team

import (
	"context"
	"encoding/json"
	"fmt"

	"cletus/internal/tools"
)

// TeamCreateTool creates a team
type TeamCreateTool struct {
	tools.BaseTool
}

// NewTeamCreateTool creates TeamCreateTool
func NewTeamCreateTool() *TeamCreateTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "Team name"
			},
			"members": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Team member identifiers"
			}
		},
		"required": ["name"]
	}`)
	return &TeamCreateTool{
		BaseTool: tools.NewBaseTool("TeamCreate", "Create a team of AI agents", schema),
	}
}

func (t *TeamCreateTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	name, _ := tools.GetString(parsed, "name")
	if name == "" {
		return "", tools.ErrMissingRequiredField("name")
	}
	return fmt.Sprintf(`{"created": true, "team": "%s"}`, name), nil
}

func (t *TeamCreateTool) IsReadOnly() bool { return false }
func (t *TeamCreateTool) IsConcurrencySafe() bool { return true }

// TeamDeleteTool deletes a team
type TeamDeleteTool struct {
	tools.BaseTool
}

// NewTeamDeleteTool creates TeamDeleteTool
func NewTeamDeleteTool() *TeamDeleteTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "Team name to delete"
			}
		},
		"required": ["name"]
	}`)
	return &TeamDeleteTool{
		BaseTool: tools.NewBaseTool("TeamDelete", "Delete a team", schema),
	}
}

func (t *TeamDeleteTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	name, _ := tools.GetString(parsed, "name")
	if name == "" {
		return "", tools.ErrMissingRequiredField("name")
	}
	return fmt.Sprintf(`{"deleted": true, "team": "%s"}`, name), nil
}

func (t *TeamDeleteTool) IsReadOnly() bool { return false }
func (t *TeamDeleteTool) IsConcurrencySafe() bool { return true }
