package planmode

import (
	"context"
	"encoding/json"

	"cletus/internal/tools"
)

// EnterPlanModeTool enters planning mode
type EnterPlanModeTool struct {
	tools.BaseTool
}

// NewEnterPlanModeTool creates EnterPlanModeTool
func NewEnterPlanModeTool() *EnterPlanModeTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"plan": {
				"type": "string",
				"description": "The plan to execute"
			}
		}
	}`)
	return &EnterPlanModeTool{
		BaseTool: tools.NewBaseTool("EnterPlanMode", "Enter planning mode to work on a multi-step plan", schema),
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	return `{"status": "entered_plan_mode", "message": "Planning mode enabled. Use ExitPlanMode when done."}`, nil
}

func (t *EnterPlanModeTool) IsReadOnly() bool { return false }
func (t *EnterPlanModeTool) IsConcurrencySafe() bool { return true }

// ExitPlanModeTool exits planning mode
type ExitPlanModeTool struct {
	tools.BaseTool
}

// NewExitPlanModeTool creates ExitPlanModeTool
func NewExitPlanModeTool() *ExitPlanModeTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"summary": {
				"type": "string",
				"description": "Summary of completed plan"
			}
		}
	}`)
	return &ExitPlanModeTool{
		BaseTool: tools.NewBaseTool("ExitPlanMode", "Exit planning mode and summarize results", schema),
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	return `{"status": "exited_plan_mode", "message": "Planning mode ended."}`, nil
}

func (t *ExitPlanModeTool) IsReadOnly() bool { return false }
func (t *ExitPlanModeTool) IsConcurrencySafe() bool { return true }
