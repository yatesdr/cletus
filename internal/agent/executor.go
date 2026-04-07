package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"cletus/internal/api"
	"cletus/internal/hooks"
	"cletus/internal/memory"
	"cletus/internal/mcp"
	"cletus/internal/permissions"
	"cletus/internal/tools"
)

// Executor handles tool execution with full integration
type Executor struct {
	registry  *tools.Registry
	hooksMgr  *hooks.Manager
	permMode  permissions.Mode
	permRules *permissions.RuleSet
	mcpClients map[string]*mcp.Client
	memory     *memory.Scanner
	mu         sync.Mutex
	running    map[string]*ExecutingTool
}

// ExecutingTool tracks a running tool execution
type ExecutingTool struct {
	ID        string
	Name      string
	Input     json.RawMessage
	StartTime time.Time
	Result    string
	Error     error
	Done      chan struct{}
}

// ExecutorConfig holds executor configuration
type ExecutorConfig struct {
	Registry     *tools.Registry
	HooksDir     string
	PermMode     permissions.Mode
	PermRules    []string
	MemoryDir    string
	MCPClients   map[string]*mcp.Client
}

// NewExecutor creates a new tool executor
func NewExecutor(cfg *ExecutorConfig) *Executor {
	e := &Executor{
		registry:   cfg.Registry,
		permMode:   cfg.PermMode,
		mcpClients: cfg.MCPClients,
		running:    make(map[string]*ExecutingTool),
	}

	// Initialize hooks manager
	if cfg.HooksDir != "" {
		mgr, _ := hooks.NewManager(cfg.HooksDir, 30*time.Second)
		e.hooksMgr = mgr
	}

	// Initialize permissions
	if len(cfg.PermRules) > 0 {
		rs, _ := permissions.NewRuleSet(cfg.PermRules)
		e.permRules = rs
	}

	// Initialize memory scanner
	if cfg.MemoryDir != "" {
		e.memory = memory.NewScanner(cfg.MemoryDir, 200)
	}

	return e
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// ExecuteTool executes a tool with the given input and full integration
func (e *Executor) ExecuteTool(ctx context.Context, toolUseID, toolName string, input json.RawMessage) *ToolResult {
	// Parse input
	parsedInput, _ := ParseToolInput(input)

	// Check permissions
	if !e.checkPermission(toolName, parsedInput) {
		return &ToolResult{
			ToolUseID: toolUseID,
			Content:   "Permission denied by rules",
			IsError:   true,
		}
	}

	// Execute PreToolUse hook if available
	if e.hooksMgr != nil {
		result, err := e.hooksMgr.ExecutePreToolUse(ctx, toolName, parsedInput)
		if err != nil {
			return &ToolResult{
				ToolUseID: toolUseID,
				Content:   fmt.Sprintf("Hook error: %v", err),
				IsError:   true,
			}
		}
		if result.Action == "block" {
			return &ToolResult{
				ToolUseID: toolUseID,
				Content:   "Blocked by hook",
				IsError:   true,
			}
		}
	}

	// Execute MCP tool if it's an MCP client tool
	if mcpClient, ok := e.mcpClients[toolName]; ok {
		result, err := mcpClient.CallTool(ctx, toolName, parsedInput)
		if err != nil {
			return &ToolResult{
				ToolUseID: toolUseID,
				Content:   fmt.Sprintf("MCP error: %v", err),
				IsError:   true,
			}
		}
		return &ToolResult{
			ToolUseID: toolUseID,
			Content:   result,
			IsError:   false,
		}
	}

	// Execute the tool
	tool, ok := e.registry.Get(toolName)
	if !ok {
		return &ToolResult{
			ToolUseID: toolUseID,
			Content:   "Error: tool not found: " + toolName,
			IsError:   true,
		}
	}

	result, err := tool.Execute(ctx, input, make(chan tools.ToolProgress, 1))

	// Execute PostToolUse hook if available
	if e.hooksMgr != nil {
		e.hooksMgr.ExecutePostToolUse(ctx, toolName, result, err)
	}

	if err != nil {
		return &ToolResult{
			ToolUseID: toolUseID,
			Content:   "Error: " + err.Error(),
			IsError:   true,
		}
	}

	return &ToolResult{
		ToolUseID: toolUseID,
		Content:   result,
		IsError:   false,
	}
}

// checkPermission checks if the tool can be executed
func (e *Executor) checkPermission(toolName string, input map[string]any) bool {
	// Bypass permissions mode allows everything
	if e.permMode.AllowBypass() {
		return true
	}

	// If no rules, allow by default
	if e.permRules == nil {
		return true
	}

	// Convert input to string for pattern matching
	inputStr := ""
	if inputJSON, err := json.Marshal(input); err == nil {
		inputStr = string(inputJSON)
	}

	action := e.permRules.Check(toolName, inputStr)
	switch action {
	case permissions.RuleActionAllow:
		return true
	case permissions.RuleActionDeny:
		return false
	default:
		// Ask - for now allow (TUI would prompt)
		return true
	}
}

// GetMCPServerNames returns the names of all connected MCP servers.
func (e *Executor) GetMCPServerNames() []string {
	names := make([]string, 0, len(e.mcpClients))
	for name := range e.mcpClients {
		names = append(names, name)
	}
	return names
}

// GetMemories returns memory content for system prompt
func (e *Executor) GetMemories() string {
	if e.memory == nil {
		return ""
	}
	memories, err := e.memory.Scan()
	if err != nil {
		return ""
	}
	return memory.FormatMemories(memories)
}

// ExecuteToolsConcurrent executes multiple tools concurrently
func (e *Executor) ExecuteToolsConcurrent(ctx context.Context, toolCalls []api.ContentBlock) []*ToolResult {
	results := make([]*ToolResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, block := range toolCalls {
		wg.Add(1)
		go func(index int, toolBlock api.ContentBlock) {
			defer wg.Done()
			results[index] = e.ExecuteTool(ctx, toolBlock.ID, toolBlock.Name, toolBlock.Input)
		}(i, block)
	}

	wg.Wait()
	return results
}

// ExecuteToolsSequential executes tools one by one in order
func (e *Executor) ExecuteToolsSequential(ctx context.Context, toolCalls []api.ContentBlock) []*ToolResult {
	results := make([]*ToolResult, len(toolCalls))

	for i, block := range toolCalls {
		results[i] = e.ExecuteTool(ctx, block.ID, block.Name, block.Input)
		select {
		case <-ctx.Done():
			for j := i + 1; j < len(toolCalls); j++ {
				results[j] = &ToolResult{
					ToolUseID: toolCalls[j].ID,
					Content:   "Tool execution cancelled",
					IsError:   true,
				}
			}
			return results
		default:
		}
	}

	return results
}

// ParseToolInput parses tool input from JSON
func ParseToolInput(input json.RawMessage) (map[string]any, error) {
	if len(input) == 0 {
		return make(map[string]any), nil
	}
	var result map[string]any
	if err := json.Unmarshal(input, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// RunTools executes tools and collects results
func RunTools(ctx context.Context, registry *tools.Registry, toolCalls []api.ContentBlock, concurrent bool) []*ToolResult {
	executor := NewExecutor(&ExecutorConfig{Registry: registry})
	
	if concurrent {
		return executor.ExecuteToolsConcurrent(ctx, toolCalls)
	}
	return executor.ExecuteToolsSequential(ctx, toolCalls)
}

// ToolExecutor is a function type for tool execution
type ToolExecutor func(ctx context.Context, toolName string, input json.RawMessage) (string, error)

// ToolExecutorFunc creates a ToolExecutor from a registry
func ToolExecutorFunc(registry *tools.Registry) ToolExecutor {
	return func(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
		tool, ok := registry.Get(toolName)
		if !ok {
			return "", fmt.Errorf("tool not found: %s", toolName)
		}
		return tool.Execute(ctx, input, make(chan tools.ToolProgress, 1))
	}
}
