package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"cletus/internal/tools"
)

// JSONRPCMessage represents a JSON-RPC message
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client represents an MCP client
type Client struct {
	transport Transport
	server    string
	mu        sync.RWMutex
	tools     map[string]*MCPTool
}

// MCPTool wraps an MCP tool
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MCPToolWrapper wraps an MCP tool to implement Tool interface
type MCPToolWrapper struct {
	mcpTool *MCPTool
	client  *Client
}

// NewMCPToolWrapper creates a new MCP tool wrapper
func NewMCPToolWrapper(mcpTool *MCPTool, client *Client) *MCPToolWrapper {
	return &MCPToolWrapper{
		mcpTool: mcpTool,
		client:  client,
	}
}

func (t *MCPToolWrapper) Name() string {
	return t.mcpTool.Name
}

func (t *MCPToolWrapper) Description() string {
	return t.mcpTool.Description
}

func (t *MCPToolWrapper) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        t.mcpTool.Name,
		Description: t.mcpTool.Description,
		InputSchema: t.mcpTool.InputSchema,
	}
}

func (t *MCPToolWrapper) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	var inputMap map[string]any
	if err := json.Unmarshal(input, &inputMap); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	return t.client.CallTool(ctx, t.mcpTool.Name, inputMap)
}

// NewClient creates a new MCP client
func NewClient(server string, transport Transport) *Client {
	return &Client{
		server:    server,
		transport: transport,
		tools:     make(map[string]*MCPTool),
	}
}

// Connect connects to the MCP server
func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// Initialize - send initialize request
	initReq := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	_, err := c.send(ctx, initReq)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification
	initializedReq := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialized",
		ID:      nil,
	}
	c.transport.Send(ctx, []byte(initializedReq.JSON()))

	// Fetch list of tools
	if err := c.fetchTools(ctx); err != nil {
		return fmt.Errorf("fetch tools: %w", err)
	}

	return nil
}

// fetchTools requests the list of tools from the MCP server
func (c *Client) fetchTools(ctx context.Context) error {
	req := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
		ID:      2,
	}

	resp, err := c.send(ctx, req)
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}

	return c.parseTools(resp)
}

// Disconnect disconnects from the MCP server
func (c *Client) Disconnect() error {
	return c.transport.Disconnect()
}

// send sends a JSON-RPC message
func (c *Client) send(ctx context.Context, msg JSONRPCMessage) (*JSONRPCMessage, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	response, err := c.transport.Send(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	var resp JSONRPCMessage
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

// JSON returns the JSON representation of the message
func (m JSONRPCMessage) JSON() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// parseTools parses tools from the tools/list response
func (c *Client) parseTools(resp *JSONRPCMessage) error {
	if resp.Result == nil {
		return nil
	}

	// MCP tools/list response structure:
	// { "tools": [{ "name": "...", "description": "...", "inputSchema": {...} }] }
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse tools result: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tool := range result.Tools {
		c.tools[tool.Name] = &MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	return nil
}

// ListTools returns available tools
func (c *Client) ListTools() []*MCPTool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	toolsList := make([]*MCPTool, 0, len(c.tools))
	for _, t := range c.tools {
		toolsList = append(toolsList, t)
	}
	return toolsList
}

// CallTool calls a tool by name
func (c *Client) CallTool(ctx context.Context, name string, input map[string]any) (string, error) {
	params, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}

	req := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  json.RawMessage(fmt.Sprintf(`{"name": %q, "arguments": %s}`, name, params)),
		ID:      2,
	}

	resp, err := c.send(ctx, req)
	if err != nil {
		return "", err
	}

	// Extract result content
	if resp.Result == nil {
		return "", fmt.Errorf("no result")
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse result: %w", err)
	}

	if len(result.Content) == 0 {
		return "", nil
	}

	return result.Content[0].Text, nil
}

// ToToolRegistry converts MCP tools to our tool registry
func (c *Client) ToToolRegistry(registry *tools.Registry) error {
	for _, mcpTool := range c.ListTools() {
		tool := NewMCPToolWrapper(mcpTool, c)
		registry.Register(tool)
	}
	return nil
}

// NewStdioClient creates a new MCP client using stdio transport
func NewStdioClient(serverPath string) *Client {
	transport := NewStdioTransport(serverPath)
	return NewClient(serverPath, transport)
}
