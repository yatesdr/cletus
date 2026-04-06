package agent

import (
	"cletus/internal/api"
)

// InternalMessage represents a message in the agent's internal conversation
type InternalMessage struct {
	Role      string             `json:"role"`       // user, assistant, system
	Content   string             `json:"content"`   // simple text content
	Blocks    []api.ContentBlock `json:"blocks,omitempty"` // structured content blocks
}

// ToolUse represents a tool call from the model
type ToolUse struct {
	ID     string       `json:"id"`     // unique ID
	Name   string       `json:"name"`   // tool name
	Input  api.ToolInput `json:"input"` // parsed tool input
	Result any          `json:"result,omitempty"` // tool execution result
	Error  string       `json:"error,omitempty"`  // error message if failed
}

// MessageBuilder helps construct messages for the API
type MessageBuilder struct {
	messages []api.APIMessage
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		messages: make([]api.APIMessage, 0),
	}
}

// AddUser adds a user message
func (mb *MessageBuilder) AddUser(content string) *MessageBuilder {
	mb.messages = append(mb.messages, api.NewTextMessage("user", content))
	return mb
}

// AddUserWithBlocks adds a user message with content blocks
func (mb *MessageBuilder) AddUserWithBlocks(blocks []api.ContentBlock) *MessageBuilder {
	mb.messages = append(mb.messages, api.APIMessage{
		Role:    "user",
		Content: blocks,
	})
	return mb
}

// AddAssistant adds an assistant message
func (mb *MessageBuilder) AddAssistant(content string) *MessageBuilder {
	mb.messages = append(mb.messages, api.NewTextMessage("assistant", content))
	return mb
}

// AddAssistantWithBlocks adds an assistant message with content blocks
func (mb *MessageBuilder) AddAssistantWithBlocks(blocks []api.ContentBlock) *MessageBuilder {
	mb.messages = append(mb.messages, api.APIMessage{
		Role:    "assistant",
		Content: blocks,
	})
	return mb
}

// AddToolResult adds a tool result as a content block
func (mb *MessageBuilder) AddToolResult(toolUseID string, content string, isError bool) *MessageBuilder {
	lastIdx := len(mb.messages) - 1
	if lastIdx < 0 {
		return mb
	}
	
	// Convert string content to tool_result block
	resultBlock := api.ContentBlock{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
	
	// Content is always []ContentBlock now
	mb.messages[lastIdx].Content = append(mb.messages[lastIdx].Content, resultBlock)
	
	return mb
}

// Build returns the built messages
func (mb *MessageBuilder) Build() []api.APIMessage {
	return mb.messages
}

// ConvertInternalMessages converts internal messages to API format
func ConvertInternalMessages(internalMsgs []InternalMessage) []api.APIMessage {
	apiMsgs := make([]api.APIMessage, len(internalMsgs))
	for i, im := range internalMsgs {
		if len(im.Blocks) > 0 {
			apiMsgs[i] = api.APIMessage{
				Role:    im.Role,
				Content: im.Blocks,
			}
		} else {
			apiMsgs[i] = api.NewTextMessage(im.Role, im.Content)
		}
	}
	return apiMsgs
}
