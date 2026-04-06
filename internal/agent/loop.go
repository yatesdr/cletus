package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cletus/internal/api"
	"cletus/internal/config"
	"cletus/internal/permissions"
	"cletus/internal/pipeline"
	"cletus/internal/prompt"
	"cletus/internal/tools"
)

type Event struct {
	Type         string
	Thinking     string
	Content      string
	ContentBlock *api.ContentBlock
	ToolUse      *api.ContentBlock
	ToolResult   *ToolResult
	Usage        api.Usage
	Done         bool
	Error        error
}

type EventHandler func(Event)

type Loop struct {
	client                    api.LLMClient
	registry                  *tools.Registry
	messages                  []api.APIMessage
	cfg                       *config.Config
	executor                  *Executor
	compactFn                 func([]api.APIMessage) ([]api.APIMessage, error)
	compactor                 *Compactor
	pipeline                  *pipeline.Pipeline
	inputTokens, outputTokens int
}

func NewLoop(client api.LLMClient, registry *tools.Registry, cfg *config.Config, pipeline *pipeline.Pipeline) *Loop {
	execCfg := &ExecutorConfig{
		Registry: registry,
		PermMode: permissions.ModeDefault,
	}
	executor := NewExecutor(execCfg)
	return &Loop{
		client:   client,
		registry: registry,
		messages: []api.APIMessage{},
		cfg:      cfg,
		executor: executor,
		pipeline: pipeline,
	}
}

// SetCompactor sets the compactor for automatic context management
func (l *Loop) SetCompactor(c *Compactor) {
	l.compactor = c
}

func (l *Loop) RunWithTools(ctx context.Context, input string, handler EventHandler) error {
	l.messages = append(l.messages, api.NewTextMessage("user", input))

	// Track the last usage for compaction decisions
	var lastUsage api.Usage

	// Get the model for this request using role resolution
	modelName := l.cfg.ResolveModel("large")

	for {
		systemPrompt := l.buildSystemPrompt()

		req := &api.Request{
			Model:     modelName,
			Messages:  l.messages,
			System:    []api.SystemBlock{{Type: "system", Text: systemPrompt}},
			Tools:     l.buildToolSchemas(),
			MaxTokens: l.cfg.MaxTokens,
		}

		var assistantContent strings.Builder
		var toolUses []api.ContentBlock

		err := l.client.Stream(ctx, req, func(event api.StreamEvent) {
			switch event.Type {
			case "message_start":
				handler(Event{Type: "message_start", Usage: event.Usage})
			case "content_block_delta":
				if event.Content != "" {
					assistantContent.WriteString(event.Content)
					handler(Event{Type: "content_block_delta", Content: event.Content})
				}
			case "tool_use":
				toolUses = append(toolUses, *event.ToolUse)
				handler(Event{Type: "tool_use", ToolUse: event.ToolUse})
			case "message_delta":
				lastUsage = event.Usage
				l.outputTokens = event.Usage.OutputTokens
				handler(Event{Type: "message_delta", Usage: event.Usage})
			case "message_stop":
				// Check if compaction is needed after the message completes
				if l.compactor != nil {
					totalTokens := lastUsage.InputTokens + lastUsage.OutputTokens
					if l.compactor.ShouldCompact(lastUsage) {
						handler(Event{Type: "compact_triggered", Content: fmt.Sprintf("Context usage: %d tokens - triggering compaction", totalTokens)})
						compacted, err := l.compactor.CompactWithSummary(l.messages)
						if err == nil {
							l.messages = compacted
							handler(Event{Type: "compact_completed", Content: "Context compacted successfully"})
						} else {
							handler(Event{Type: "compact_error", Content: fmt.Sprintf("Compaction failed: %v", err)})
						}
					}
				}
				handler(Event{Type: "message_stop", Done: true})
			}
		})

		if err != nil {
			return err
		}

		var assistantMsg api.APIMessage
		assistantMsg.Role = "assistant"
		assistantMsg.Content = make([]api.ContentBlock, 0, 1+len(toolUses))

		if assistantContent.Len() > 0 {
			assistantMsg.Content = append(assistantMsg.Content, api.ContentBlock{Type: "text", Text: assistantContent.String()})
		}

		assistantMsg.Content = append(assistantMsg.Content, toolUses...)

		if len(assistantMsg.Content) > 0 {
			l.messages = append(l.messages, assistantMsg)
		}

		if len(toolUses) == 0 {
			return nil
		}

		results := l.executor.ExecuteToolsSequential(ctx, toolUses)

		for _, result := range results {
			handler(Event{Type: "tool_result", ToolResult: result})

			l.messages = append(l.messages, api.APIMessage{
				Role: "user",
				Content: []api.ContentBlock{
					{Type: "tool_result", ToolUseID: result.ToolUseID, Content: result.Content, IsError: result.IsError},
				},
			})
		}
	}
}

func (l *Loop) buildSystemPrompt() string {
	modelName := l.cfg.ResolveModel("large")
	data := prompt.CollectEnvData(modelName)

	// Tools description
	data.ToolsDescription = prompt.FormatToolsDescription(l.registry.ToSchema())

	// Project context (CLETUS.md + detected language/build tool)
	cwd, _ := os.Getwd()
	if ctx, err := GetProjectContext(cwd); err == nil && ctx != nil {
		data.ProjectContext = FormatProjectContext(ctx)
	}

	// MCP servers
	if l.executor != nil {
		if names := l.executor.GetMCPServerNames(); len(names) > 0 {
			data.MCPServers = strings.Join(names, "\n")
		}
		data.Memories = l.executor.GetMemories()
	}

	// Language preference and hooks
	data.Language = l.cfg.Language
	data.HooksEnabled = l.cfg.Hooks.Enabled

	// Allow config.md System Prompt to override the template
	builder := prompt.NewSystemPromptBuilder()
	if l.cfg.MD != nil {
		if sp := l.cfg.MD.GetSystemPrompt(); sp != "" {
			builder.SetTemplate(sp)
		}
	}

	return builder.SetData(data).Build()
}

func (l *Loop) buildToolSchemas() []api.ToolSchema {
	schemas := make([]api.ToolSchema, 0)
	for _, name := range l.registry.List() {
		tool, ok := l.registry.Get(name)
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

// Reset clears all conversation messages, starting a fresh context.
func (l *Loop) Reset() {
	l.messages = []api.APIMessage{}
	l.inputTokens = 0
	l.outputTokens = 0
}

func (l *Loop) AddMessage(role, content string) {
	l.messages = append(l.messages, api.NewTextMessage(role, content))
}

// SetMessages replaces the message history wholesale (e.g. when restoring a session).
func (l *Loop) SetMessages(messages []api.APIMessage) {
	l.messages = messages
}

func (l *Loop) GetMessages() []api.APIMessage {
	return l.messages
}

func (l *Loop) SetCompactFn(fn func([]api.APIMessage) ([]api.APIMessage, error)) {
	l.compactFn = fn
}

func (l *Loop) Compact() error {
	if l.compactor != nil {
		compacted, err := l.compactor.CompactWithSummary(l.messages)
		if err != nil {
			return err
		}
		l.messages = compacted
		return nil
	}
	if l.compactFn == nil {
		return nil
	}
	compacted, err := l.compactFn(l.messages)
	if err != nil {
		return err
	}
	l.messages = compacted
	return nil
}

// GetUsageStats returns the current usage statistics
func (l *Loop) GetUsageStats() (int, int) {
	return l.inputTokens, l.outputTokens
}

// GetClient returns the underlying LLM client
func (l *Loop) GetClient() api.LLMClient {
	return l.client
}
