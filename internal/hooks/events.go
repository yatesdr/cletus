package hooks

// EventType represents the type of hook event
type EventType string

const (
	// Tool events
	EventPreToolUse  EventType = "PreToolUse"
	EventPostToolUse EventType = "PostToolUse"
	EventToolError   EventType = "ToolError"

	// Session events
	EventSessionStart  EventType = "SessionStart"
	EventSessionEnd    EventType = "SessionEnd"
	EventResumeSession EventType = "ResumeSession"

	// Message events
	EventUserPromptSubmit EventType = "UserPromptSubmit"
	EventUserPromptReply  EventType = "UserPromptReply"
	EventAssistantMessage EventType = "AssistantMessage"
	EventStop             EventType = "Stop"

	// Context events
	EventContextExceeded EventType = "ContextExceeded"
	EventContextCompact  EventType = "ContextCompact"

	// Permission events
	EventPermissionRequest EventType = "PermissionRequest"
	EventPermissionGranted EventType = "PermissionGranted"
	EventPermissionDenied  EventType = "PermissionDenied"

	// Notification events
	EventNotification EventType = "Notification"
	EventError        EventType = "Error"
)

// Event represents a hook event payload
type Event struct {
	Type           EventType      `json:"type"`
	Timestamp      string         `json:"timestamp"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolInput      map[string]any `json:"tool_input,omitempty"`
	ToolResult     string         `json:"tool_result,omitempty"`
	Error          string         `json:"error,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	Message        string         `json:"message,omitempty"`
	TokensUsed     int            `json:"tokens_used,omitempty"`
	TokensLimit    int            `json:"tokens_limit,omitempty"`
	PermissionMode string         `json:"permission_mode,omitempty"`
	Granted        bool           `json:"granted,omitempty"`
	Notification   string         `json:"notification,omitempty"`
}

// EventResult represents the result from a hook
type EventResult struct {
	Action         string `json:"action,omitempty"` // "continue", "block", "modify", "ask"
	Error          string `json:"error,omitempty"`
	ModifiedInput  string `json:"modified_input,omitempty"`
	ModifiedOutput string `json:"modified_output,omitempty"`
	Message        string `json:"message,omitempty"`
}

// GetEventType converts string to EventType
func GetEventType(s string) EventType {
	switch s {
	case "PreToolUse":
		return EventPreToolUse
	case "PostToolUse":
		return EventPostToolUse
	case "ToolError":
		return EventToolError
	case "SessionStart":
		return EventSessionStart
	case "SessionEnd":
		return EventSessionEnd
	case "ResumeSession":
		return EventResumeSession
	case "UserPromptSubmit":
		return EventUserPromptSubmit
	case "UserPromptReply":
		return EventUserPromptReply
	case "AssistantMessage":
		return EventAssistantMessage
	case "Stop":
		return EventStop
	case "ContextExceeded":
		return EventContextExceeded
	case "ContextCompact":
		return EventContextCompact
	case "PermissionRequest":
		return EventPermissionRequest
	case "PermissionGranted":
		return EventPermissionGranted
	case "PermissionDenied":
		return EventPermissionDenied
	case "Notification":
		return EventNotification
	case "Error":
		return EventError
	default:
		return ""
	}
}

// AllEventTypes returns all supported event types
func AllEventTypes() []EventType {
	return []EventType{
		EventPreToolUse,
		EventPostToolUse,
		EventToolError,
		EventSessionStart,
		EventSessionEnd,
		EventResumeSession,
		EventUserPromptSubmit,
		EventUserPromptReply,
		EventAssistantMessage,
		EventStop,
		EventContextExceeded,
		EventContextCompact,
		EventPermissionRequest,
		EventPermissionGranted,
		EventPermissionDenied,
		EventNotification,
		EventError,
	}
}
