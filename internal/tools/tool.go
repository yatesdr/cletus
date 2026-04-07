package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error)
}

// ToolProgress represents progress updates during tool execution
type ToolProgress struct {
	Type      string
	Content   string
	LineCount int
	TaskID    string
}

// ToolSchema represents the tool's JSON Schema
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Registry holds all available tools
type Registry struct {
	tools        map[string]Tool
	descriptions map[string]string
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:        make(map[string]Tool),
		descriptions: make(map[string]string),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all tool names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// SetDescription overrides a tool's description
func (r *Registry) SetDescription(name, description string) {
	r.descriptions[name] = description
}

// GetDescription returns the tool description (with override if set)
func (r *Registry) GetDescription(name string) string {
	if desc, ok := r.descriptions[name]; ok {
		return desc
	}
	if tool, ok := r.tools[name]; ok {
		return tool.Description()
	}
	return ""
}

// ToSchema converts registry tools to API tool schemas
func (r *Registry) ToSchema() []map[string]any {
	schemas := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		desc := r.GetDescription(tool.Name())
		schemas = append(schemas, map[string]any{
			"name":         tool.Name(),
			"description":  desc,
			"input_schema": tool.Schema().InputSchema,
		})
	}
	return schemas
}

// BaseTool provides common functionality for tools
type BaseTool struct {
	name        string
	description string
	schema      json.RawMessage
}

// NewBaseTool creates a new base tool
func NewBaseTool(name, description string, schema json.RawMessage) BaseTool {
	return BaseTool{
		name:        name,
		description: description,
		schema:      schema,
	}
}

// Name returns the tool name
func (bt *BaseTool) Name() string {
	return bt.name
}

// Description returns the tool description
func (bt *BaseTool) Description() string {
	return bt.description
}

// SetDescription sets the tool description (for config override)
func (bt *BaseTool) SetDescription(desc string) {
	bt.description = desc
}

// Schema returns the tool input schema
func (bt *BaseTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        bt.name,
		Description: bt.description,
		InputSchema: bt.schema,
	}
}

// InputSchema returns the raw input schema
func (bt *BaseTool) InputSchema() json.RawMessage {
	return bt.schema
}

// SimpleTool is a tool that doesn't need context/progress (legacy adapter)
type SimpleTool interface {
	Execute(input json.RawMessage) (string, error)
}

// WrapSimpleTool wraps a simple tool to implement the full Tool interface
type WrapSimpleTool struct {
	BaseTool
	simple SimpleTool
}

func (w *WrapSimpleTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	return w.simple.Execute(input)
}

// WrapTool wraps a simple tool to implement Tool interface
func WrapTool(name, description string, schema json.RawMessage, simple SimpleTool) *WrapSimpleTool {
	return &WrapSimpleTool{
		BaseTool: NewBaseTool(name, description, schema),
		simple:   simple,
	}
}

// ParseInput parses JSON input into a map
func ParseInput(input json.RawMessage) (map[string]any, error) {
	if len(input) == 0 {
		return make(map[string]any), nil
	}
	
	var result map[string]any
	if err := json.Unmarshal(input, &result); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}
	return result, nil
}

// GetString gets a string field from input
func GetString(input map[string]any, key string) (string, bool) {
	v, ok := input[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetInt gets an int field from input
func GetInt(input map[string]any, key string) (int, bool) {
	v, ok := input[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// GetIntDefault gets an int with default
func GetIntDefault(input map[string]any, key string, defaultVal int) int {
	if v, ok := GetInt(input, key); ok {
		return v
	}
	return defaultVal
}

// GetBool gets a bool field from input
func GetBool(input map[string]any, key string) (bool, bool) {
	v, ok := input[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// GetStringSlice gets a string slice field from input
func GetStringSlice(input map[string]any, key string) ([]string, bool) {
	v, ok := input[key]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result[i] = s
	}
	return result, true
}

// ErrMissingRequiredField is returned when a required field is missing
func ErrMissingRequiredField(field string) error {
	return &ValidationError{Field: field, Message: "required field missing"}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
