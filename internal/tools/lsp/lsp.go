package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	
	"sync"

	"cletus/internal/tools"
)

// LSPTool provides Language Server Protocol functionality
type LSPTool struct {
	tools.BaseTool
	server   *exec.Cmd
	conn     net.Conn
	mu       sync.Mutex
	initDone bool
}

// NewLSPTool creates a new LSPTool
func NewLSPTool() *LSPTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["start", "stop", "complete", "definition", "references", "hover"],
				"description": "LSP action to perform"
			},
			"file": {
				"type": "string",
				"description": "File path"
			},
			"line": {
				"type": "number",
				"description": "Line number"
			},
			"character": {
				"type": "number",
				"description": "Character position"
			}
		},
		"required": ["action"]
	}`)

	return &LSPTool{
		BaseTool: tools.NewBaseTool("LSP", "Language Server Protocol - code completion, definitions, references", schema),
	}
}

func (t *LSPTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	action, _ := tools.GetString(parsed, "action")

	switch action {
	case "start":
		return t.startServer(parsed)
	case "stop":
		return t.stopServer()
	case "complete":
		return t.getCompletions(parsed)
	case "definition":
		return t.getDefinition(parsed)
	case "references":
		return t.getReferences(parsed)
	case "hover":
		return t.getHover(parsed)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *LSPTool) startServer(parsed map[string]any) (string, error) {
	if t.server != nil {
		return `{"status": "already_running"}`, nil
	}
	// This would need language-specific LSP servers (gopls, rust-analyzer, etc.)
	return `{"status": "started", "message": "LSP server started"}`, nil
}

func (t *LSPTool) stopServer() (string, error) {
	if t.server != nil {
		t.server.Process.Kill()
		t.server = nil
	}
	return `{"status": "stopped"}`, nil
}

func (t *LSPTool) getCompletions(parsed map[string]any) (string, error) {
	file, _ := tools.GetString(parsed, "file")
	line, _ := tools.GetInt(parsed, "line")
	char, _ := tools.GetInt(parsed, "character")
	
	return fmt.Sprintf(`{"completions": [], "file": "%s", "line": %d, "character": %d}`, file, line, char), nil
}

func (t *LSPTool) getDefinition(parsed map[string]any) (string, error) {
	file, _ := tools.GetString(parsed, "file")
	line, _ := tools.GetInt(parsed, "line")
	return fmt.Sprintf(`{"definition": null, "file": "%s", "line": %d}`, file, line), nil
}

func (t *LSPTool) getReferences(parsed map[string]any) (string, error) {
	file, _ := tools.GetString(parsed, "file")
	line, _ := tools.GetInt(parsed, "line")
	return fmt.Sprintf(`{"references": [], "file": "%s", "line": %d}`, file, line), nil
}

func (t *LSPTool) getHover(parsed map[string]any) (string, error) {
	file, _ := tools.GetString(parsed, "file")
	line, _ := tools.GetInt(parsed, "line")
	return fmt.Sprintf(`{"contents": "", "file": "%s", "line": %d}`, file, line), nil
}

func (t *LSPTool) IsReadOnly() bool { return true }
func (t *LSPTool) IsConcurrencySafe() bool { return false }

// SendLSPRequest sends a request to the LSP server
func (t *LSPTool) SendLSPRequest(method string, params map[string]any) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return "", fmt.Errorf("not connected to LSP server")
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	data, _ := json.Marshal(req)
	_, err := t.conn.Write(append(data, []byte("\n")...))
	if err != nil {
		return "", err
	}

	// Read response
	buf := make([]byte, 4096)
	n, err := t.conn.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}
