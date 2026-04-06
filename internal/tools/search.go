package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSearchTool searches for tools by keyword
type ToolSearchTool struct {
	BaseTool
	registry *Registry
}

// NewToolSearchTool creates a new ToolSearchTool
func NewToolSearchTool(registry *Registry) *ToolSearchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Keyword to search for"
			},
			"limit": {
				"type": "number",
				"description": "Maximum results to return",
				"default": 10
			}
		},
		"required": ["query"]
	}`)
	return &ToolSearchTool{
		BaseTool: NewBaseTool("ToolSearch", "Search for available tools by keyword", schema),
		registry: registry,
	}
}

func (t *ToolSearchTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	query, ok := GetString(parsed, "query")
	if !ok {
		return "", ErrMissingRequiredField("query")
	}

	limit, _ := GetInt(parsed, "limit")
	if limit == 0 {
		limit = 10
	}

	query = strings.ToLower(query)
	var results []string
	var count int

	for _, name := range t.registry.List() {
		tool, _ := t.registry.Get(name)
		if tool == nil {
			continue
		}

		desc := strings.ToLower(tool.Description())
		nameLower := strings.ToLower(name)

		// Match query in name or description
		if strings.Contains(nameLower, query) || strings.Contains(desc, query) {
			results = append(results, fmt.Sprintf("- **%s**: %s", name, tool.Description()))
			count++
			if count >= limit {
				break
			}
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("No tools found matching '%s'", query), nil
	}

	return "Available tools:\n" + strings.Join(results, "\n"), nil
}
