package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"cletus/internal/tools"
)

// MCPTool - Connect to MCP server (we already have internal/mcp/client.go for this)
// This is the tool wrapper for tool registration

// ListMcpResourcesTool lists MCP resources
type ListMcpResourcesTool struct {
	tools.BaseTool
}

// NewListMcpResourcesTool creates ListMcpResourcesTool
func NewListMcpResourcesTool() *ListMcpResourcesTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {
				"type": "string",
				"description": "MCP server name"
			}
		}
	}`)
	return &ListMcpResourcesTool{
		BaseTool: tools.NewBaseTool("ListMcpResources", "List available resources from MCP servers", schema),
	}
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	// This would integrate with internal/mcp/client.go
	return `{"resources": []}`, nil
}

func (t *ListMcpResourcesTool) IsReadOnly() bool { return true }
func (t *ListMcpResourcesTool) IsConcurrencySafe() bool { return true }

// ReadMcpResourceTool reads an MCP resource
type ReadMcpResourceTool struct {
	tools.BaseTool
}

// NewReadMcpResourceTool creates ReadMcpResourceTool
func NewReadMcpResourceTool() *ReadMcpResourceTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {
				"type": "string",
				"description": "Resource URI to read"
			}
		},
		"required": ["uri"]
	}`)
	return &ReadMcpResourceTool{
		BaseTool: tools.NewBaseTool("ReadMcpResource", "Read a specific resource from MCP server", schema),
	}
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	uri, _ := tools.GetString(parsed, "uri")
	if uri == "" {
		return "", tools.ErrMissingRequiredField("uri")
	}
	return `{"content": ""}`, nil
}

func (t *ReadMcpResourceTool) IsReadOnly() bool { return true }
func (t *ReadMcpResourceTool) IsConcurrencySafe() bool { return true }

// McpAuthTool handles MCP authentication
type McpAuthTool struct {
	tools.BaseTool
}

// NewMcpAuthTool creates McpAuthTool
func NewMcpAuthTool() *McpAuthTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["add", "remove", "list"],
				"description": "Auth action"
			},
			"server": {
				"type": "string",
				"description": "Server name"
			},
			"token": {
				"type": "string",
				"description": "Auth token"
			}
		}
	}`)
	return &McpAuthTool{
		BaseTool: tools.NewBaseTool("McpAuth", "Manage MCP server authentication", schema),
	}
}

func (t *McpAuthTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	action, _ := tools.GetString(parsed, "action")
	server, _ := tools.GetString(parsed, "server")

	switch action {
	case "add":
		return fmt.Sprintf(`{"status": "added", "server": "%s"}`, server), nil
	case "remove":
		return fmt.Sprintf(`{"status": "removed", "server": "%s"}`, server), nil
	case "list":
		return `{"servers": []}`, nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *McpAuthTool) IsReadOnly() bool { return false }
func (t *McpAuthTool) IsConcurrencySafe() bool { return true }
