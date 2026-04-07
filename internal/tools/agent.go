package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"cletus/internal/api"
)

// SubAgentResult represents the result from a sub-agent
type SubAgentResult struct {
	Summary  string
	Output   string
	Error    error
	ToolUses int
}

// AgentTool spawns sub-agents for parallel task execution
type AgentTool struct {
	BaseTool
	registry *Registry
	clients  map[string]api.LLMClient
	mu       sync.RWMutex
}

// NewAgentTool creates a new AgentTool
func NewAgentTool(registry *Registry) *AgentTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "The prompt/task description for the sub-agent"
			},
			"model": {
				"type": "string",
				"description": "Optional model override for the sub-agent"
			},
			"name": {
				"type": "string",
				"description": "Optional name for this sub-agent (for logging/tracking)"
			},
			"subagent_type": {
				"type": "string",
				"description": "Built-in agent type (explore, implement, verify)"
			},
			"background": {
				"type": "boolean",
				"description": "Run sub-agent in background without waiting for completion",
				"default": false
			},
			"tools": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Optional list of tools to allow (defaults to all)"
			},
			"disallowed_tools": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Tools to disallow"
			}
		},
		"required": ["prompt"]
	}`)

	return &AgentTool{
		BaseTool: NewBaseTool("Agent", "Spawns a sub-agent to handle multi-step tasks. Use for complex tasks requiring extensive exploration, multi-file changes, or parallel execution.", schema),
		registry: registry,
		clients:  make(map[string]api.LLMClient),
	}
}

// SetClient sets the API client for sub-agent execution
func (t *AgentTool) SetClient(name string, client api.LLMClient) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clients[name] = client
}

// Execute runs the sub-agent
func (t *AgentTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	prompt, ok := GetString(parsed, "prompt")
	if !ok {
		return "", ErrMissingRequiredField("prompt")
	}

	// Get optional parameters
	model, _ := GetString(parsed, "model")
	name, _ := GetString(parsed, "name")
	_, _ = GetBool(parsed, "background") // reserved for future use - background execution
	subagentType, _ := GetString(parsed, "subagent_type")

	// Get tool restrictions
	allowedTools, _ := GetStringSlice(parsed, "tools")
	disallowedTools, _ := GetStringSlice(parsed, "disallowed_tools")

	// Execute the sub-agent
	result := t.runSubAgent(ctx, &SubAgentConfig{
		Prompt:          prompt,
		Model:           model,
		Name:            name,
		SubagentType:    subagentType,
		AllowedTools:    allowedTools,
		DisallowedTools: disallowedTools,
	}, progress)

	if result.Error != nil {
		return "", result.Error
	}

	// Format the response
	if result.Summary != "" {
		return result.Summary, nil
	}

	return fmt.Sprintf("Task completed with %d tool uses", result.ToolUses), nil
}

// SubAgentConfig holds configuration for a sub-agent
type SubAgentConfig struct {
	Prompt           string
	Model            string
	Name             string
	SubagentType     string
	AllowedTools     []string
	DisallowedTools  []string
	WorkingDirectory string
	Memory           string
}

// runSubAgent executes a sub-agent with the given configuration
func (t *AgentTool) runSubAgent(ctx context.Context, cfg *SubAgentConfig, progress chan<- ToolProgress) *SubAgentResult {
	result := &SubAgentResult{}

	// Get or create API client
	t.mu.RLock()
	client := t.clients["default"]
	t.mu.RUnlock()

	if client == nil {
		// Debug fallback - client should be set in main.go via SetClient()
		result.Summary = fmt.Sprintf("[Sub-agent task received: %s]", truncate(cfg.Prompt, 50))
		return result
	}

	// Build system prompt for sub-agent
	systemPrompt := t.buildSubAgentSystemPrompt(cfg)

	// Create initial message
	messages := []api.APIMessage{
		{
			Role:    "user",
			Content: []api.ContentBlock{{Type: "text", Text: cfg.Prompt}},
		},
	}

	// Build tool schemas (filtered based on restrictions)
	tools := t.buildFilteredToolSchemas(cfg.AllowedTools, cfg.DisallowedTools)

	// Run the agent loop
	req := &api.Request{
		Model:     cfg.Model,
		Messages:  messages,
		System:    []api.SystemBlock{{Type: "text", Text: systemPrompt}},
		Tools:     tools,
		MaxTokens: 8192,
	}

	var toolUses []api.ContentBlock
	var assistantContent strings.Builder

	err := client.Stream(ctx, req, func(event api.StreamEvent) {
		switch event.Type {
		case "content_block_delta":
			if event.ContentBlock != nil && event.Content != "" {
				assistantContent.WriteString(event.Content)
			}
		case "tool_use":
			if event.ToolUse != nil {
				toolUses = append(toolUses, *event.ToolUse)
				progress <- ToolProgress{
					Type:    "subagent_tool_use",
					Content: fmt.Sprintf("[%s] %s", cfg.Name, event.ToolUse.Name),
				}
			}
		case "message_stop":
			progress <- ToolProgress{
				Type:    "subagent_done",
				Content: fmt.Sprintf("[%s] Agent completed", cfg.Name),
			}
		}
	})

	if err != nil {
		result.Error = err
		return result
	}

	// Execute tools if any
	if len(toolUses) > 0 {
		result.ToolUses = len(toolUses)

		// Execute tools using the main registry
		executor := NewSimpleExecutor(t.registry)

		for _, toolUse := range toolUses {
			execResult, err := executor.ExecuteTool(ctx, toolUse.Name, toolUse.Input)
			if err != nil {
				result.Output += fmt.Sprintf("\nError executing %s: %v", toolUse.Name, err)
			} else {
				result.Output += fmt.Sprintf("\n[%s] %s", toolUse.Name, execResult)
			}
		}
	}

	// Generate summary from the output
	if assistantContent.Len() > 0 {
		result.Summary = assistantContent.String()
	} else if result.Output != "" {
		result.Summary = result.Output
	}

	return result
}

// buildSubAgentSystemPrompt builds the system prompt for a sub-agent
func (t *AgentTool) buildSubAgentSystemPrompt(cfg *SubAgentConfig) string {
	var promptBuilder strings.Builder

	// Add name if provided
	if cfg.Name != "" {
		promptBuilder.WriteString(fmt.Sprintf("You are a sub-agent named '%s'.\n\n", cfg.Name))
	}

	// Add subagent type specific instructions
	switch cfg.SubagentType {
	case "explore":
		promptBuilder.WriteString("Your task is to explore and analyze the codebase. Focus on understanding the structure, finding relevant files, and gathering information.\n\n")
	case "implement":
		promptBuilder.WriteString("Your task is to implement changes. Make focused, surgical modifications. Verify your changes work correctly.\n\n")
	case "verify":
		promptBuilder.WriteString("Your task is to verify changes. Run tests, check outputs, and confirm the implementation is correct.\n\n")
	}

	// Add custom memory/context if provided
	if cfg.Memory != "" {
		promptBuilder.WriteString(fmt.Sprintf("## Context\n%s\n\n", cfg.Memory))
	}

	// Add tool restrictions
	if len(cfg.DisallowedTools) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("## Tool Restrictions\nDo NOT use these tools: %s\n\n", strings.Join(cfg.DisallowedTools, ", ")))
	}

	promptBuilder.WriteString("Complete your task thoroughly and provide a summary of what you found or accomplished.")

	return promptBuilder.String()
}

// buildFilteredToolSchemas builds tool schemas filtered by restrictions
func (t *AgentTool) buildFilteredToolSchemas(allowedTools, disallowedTools []string) []api.ToolSchema {
	allowedSet := make(map[string]bool)
	disallowedSet := make(map[string]bool)

	for _, tt := range allowedTools {
		allowedSet[tt] = true
	}
	for _, tt := range disallowedTools {
		disallowedSet[tt] = true
	}

	schemas := make([]api.ToolSchema, 0)
	for _, name := range t.registry.List() {
		// Skip if not in allowed list (if specified)
		if len(allowedSet) > 0 && !allowedSet[name] {
			continue
		}
		// Skip if in disallowed list
		if disallowedSet[name] {
			continue
		}

		tool, ok := t.registry.Get(name)
		if !ok {
			continue
		}
		schema := tool.Schema()
		schemas = append(schemas, api.ToolSchema{
			Name:        schema.Name,
			Description: schema.Description,
			InputSchema: schema.InputSchema,
		})
	}

	return schemas
}

// SimpleExecutor is a simple tool executor for sub-agents
type SimpleExecutor struct {
	registry *Registry
}

// NewSimpleExecutor creates a new simple executor
func NewSimpleExecutor(registry *Registry) *SimpleExecutor {
	return &SimpleExecutor{registry: registry}
}

// ExecuteTool executes a single tool
func (e *SimpleExecutor) ExecuteTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	tool, ok := e.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	return tool.Execute(ctx, input, make(chan ToolProgress, 10))
}

// IsReadOnly returns false for AgentTool (it spawns sub-agents that can modify files)
func (t *AgentTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns false (sub-agents may modify state)
func (t *AgentTool) IsConcurrencySafe() bool {
	return false
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
