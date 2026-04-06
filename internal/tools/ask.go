package tools

import (
	"context"
	"encoding/json"
)

// AskUserQuestionTool asks the user a question and waits for response
type AskUserQuestionTool struct {
	BaseTool
	callback func(string) string
}

// NewAskUserQuestionTool creates a new AskUserQuestionTool
func NewAskUserQuestionTool(callback func(string) string) *AskUserQuestionTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "The question to ask the user"
			}
		},
		"required": ["question"]
	}`)
	return &AskUserQuestionTool{
		BaseTool: NewBaseTool("AskUser", "Ask the user a question", schema),
		callback: callback,
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	question, ok := GetString(parsed, "question")
	if !ok {
		return "", ErrMissingRequiredField("question")
	}

	// Call the callback to get user response
	if t.callback != nil {
		return t.callback(question), nil
	}

	// No callback - return placeholder
	return "[No callback configured - user question not supported]", nil
}

// SetCallback sets the response callback
func (t *AskUserQuestionTool) SetCallback(cb func(string) string) {
	t.callback = cb
}
