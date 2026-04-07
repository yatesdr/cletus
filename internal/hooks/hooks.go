package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Manager handles hook execution
type Manager struct {
	hookPaths map[EventType]string
	timeout   time.Duration
}

// NewManager creates a new hook manager
func NewManager(hookDir string, timeout time.Duration) (*Manager, error) {
	m := &Manager{
		hookPaths: make(map[EventType]string),
		timeout:   timeout,
	}

	if hookDir == "" {
		return m, nil
	}

	// Load all supported event hooks
	for _, event := range AllEventTypes() {
		hookPath := filepath.Join(hookDir, string(event))
		files, _ := filepath.Glob(hookPath + ".*")
		if len(files) > 0 {
			m.hookPaths[event] = files[0]
		}
	}

	return m, nil
}

// SetHook sets a hook for an event type
func (m *Manager) SetHook(event EventType, path string) {
	m.hookPaths[event] = path
}

// HasHook checks if a hook exists for an event
func (m *Manager) HasHook(event EventType) bool {
	_, ok := m.hookPaths[event]
	return ok
}

// GetHookPath returns the path to a hook for an event
func (m *Manager) GetHookPath(event EventType) string {
	return m.hookPaths[event]
}

// Execute runs a hook for the given event
func (m *Manager) Execute(ctx context.Context, event Event, inputOverride string) (*EventResult, error) {
	hookPath, ok := m.hookPaths[EventType(event.Type)]
	if !ok {
		return &EventResult{Action: "continue"}, nil
	}

	eventData := event
	if inputOverride != "" {
		eventData.Message = inputOverride
	}

	payload, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, hookPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "CLETUS_HOOK=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("hook timed out after %v", m.timeout)
		}
		return &EventResult{
			Action: "continue",
			Error:  fmt.Sprintf("hook error: %v - %s", err, stderr.String()),
		}, nil
	}

	if stdout.Len() == 0 {
		return &EventResult{Action: "continue"}, nil
	}

	var result EventResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse hook result: %w", err)
	}

	if result.Action == "" {
		result.Action = "continue"
	}

	return &result, nil
}

// ExecutePreToolUse runs the PreToolUse hook
func (m *Manager) ExecutePreToolUse(ctx context.Context, toolName string, toolInput map[string]any) (*EventResult, error) {
	event := Event{
		Type:      EventPreToolUse,
		Timestamp: time.Now().Format(time.RFC3339),
		ToolName:  toolName,
		ToolInput: toolInput,
	}
	return m.Execute(ctx, event, "")
}

// ExecutePostToolUse runs the PostToolUse hook
func (m *Manager) ExecutePostToolUse(ctx context.Context, toolName, toolResult string, err error) (*EventResult, error) {
	event := Event{
		Type:       EventPostToolUse,
		Timestamp:  time.Now().Format(time.RFC3339),
		ToolName:   toolName,
		ToolResult: toolResult,
	}
	if err != nil {
		event.Error = err.Error()
	}
	return m.Execute(ctx, event, "")
}

// ExecuteToolError runs the ToolError hook
func (m *Manager) ExecuteToolError(ctx context.Context, toolName, toolResult string, err error) (*EventResult, error) {
	event := Event{
		Type:       EventToolError,
		Timestamp:  time.Now().Format(time.RFC3339),
		ToolName:   toolName,
		ToolResult: toolResult,
		Error:      err.Error(),
	}
	return m.Execute(ctx, event, "")
}

// ExecuteSessionStart runs the SessionStart hook
func (m *Manager) ExecuteSessionStart(ctx context.Context, sessionID string) (*EventResult, error) {
	event := Event{
		Type:      EventSessionStart,
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteSessionEnd runs the SessionEnd hook
func (m *Manager) ExecuteSessionEnd(ctx context.Context, sessionID string) (*EventResult, error) {
	event := Event{
		Type:      EventSessionEnd,
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteResumeSession runs the ResumeSession hook
func (m *Manager) ExecuteResumeSession(ctx context.Context, sessionID string) (*EventResult, error) {
	event := Event{
		Type:      EventResumeSession,
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteUserPromptSubmit runs the UserPromptSubmit hook
func (m *Manager) ExecuteUserPromptSubmit(ctx context.Context, message string) (*EventResult, error) {
	event := Event{
		Type:      EventUserPromptSubmit,
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   message,
	}
	return m.Execute(ctx, event, message)
}

// ExecuteUserPromptReply runs the UserPromptReply hook
func (m *Manager) ExecuteUserPromptReply(ctx context.Context, message string) (*EventResult, error) {
	event := Event{
		Type:      EventUserPromptReply,
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   message,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteAssistantMessage runs the AssistantMessage hook
func (m *Manager) ExecuteAssistantMessage(ctx context.Context, message string) (*EventResult, error) {
	event := Event{
		Type:      EventAssistantMessage,
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   message,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteStop runs the Stop hook
func (m *Manager) ExecuteStop(ctx context.Context) (*EventResult, error) {
	event := Event{
		Type:      EventStop,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return m.Execute(ctx, event, "")
}

// ExecuteContextExceeded runs the ContextExceeded hook
func (m *Manager) ExecuteContextExceeded(ctx context.Context, tokensUsed, tokensLimit int) (*EventResult, error) {
	event := Event{
		Type:        EventContextExceeded,
		Timestamp:   time.Now().Format(time.RFC3339),
		TokensUsed:  tokensUsed,
		TokensLimit: tokensLimit,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteContextCompact runs the ContextCompact hook
func (m *Manager) ExecuteContextCompact(ctx context.Context, tokensBefore, tokensAfter int) (*EventResult, error) {
	event := Event{
		Type:        EventContextCompact,
		Timestamp:   time.Now().Format(time.RFC3339),
		TokensUsed:  tokensBefore,
		TokensLimit: tokensAfter,
	}
	return m.Execute(ctx, event, "")
}

// ExecutePermissionRequest runs the PermissionRequest hook
func (m *Manager) ExecutePermissionRequest(ctx context.Context, toolName string, permissionMode string) (*EventResult, error) {
	event := Event{
		Type:           EventPermissionRequest,
		Timestamp:      time.Now().Format(time.RFC3339),
		ToolName:       toolName,
		PermissionMode: permissionMode,
	}
	return m.Execute(ctx, event, "")
}

// ExecutePermissionGranted runs the PermissionGranted hook
func (m *Manager) ExecutePermissionGranted(ctx context.Context, toolName string) (*EventResult, error) {
	event := Event{
		Type:      EventPermissionGranted,
		Timestamp: time.Now().Format(time.RFC3339),
		ToolName:  toolName,
		Granted:   true,
	}
	return m.Execute(ctx, event, "")
}

// ExecutePermissionDenied runs the PermissionDenied hook
func (m *Manager) ExecutePermissionDenied(ctx context.Context, toolName string) (*EventResult, error) {
	event := Event{
		Type:      EventPermissionDenied,
		Timestamp: time.Now().Format(time.RFC3339),
		ToolName:  toolName,
		Granted:   false,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteNotification runs the Notification hook
func (m *Manager) ExecuteNotification(ctx context.Context, notification string) (*EventResult, error) {
	event := Event{
		Type:         EventNotification,
		Timestamp:    time.Now().Format(time.RFC3339),
		Notification: notification,
	}
	return m.Execute(ctx, event, "")
}

// ExecuteError runs the Error hook
func (m *Manager) ExecuteError(ctx context.Context, errMsg string) (*EventResult, error) {
	event := Event{
		Type:      EventError,
		Timestamp: time.Now().Format(time.RFC3339),
		Error:     errMsg,
	}
	return m.Execute(ctx, event, "")
}

// HTTPHookConfig holds HTTP hook configuration
type HTTPHookConfig struct {
	URL     string
	Method  string
	Timeout time.Duration
}

// ExecuteHTTP runs an HTTP hook
func (m *Manager) ExecuteHTTP(ctx context.Context, config HTTPHookConfig, event Event) (*EventResult, error) {
	if config.URL == "" {
		return &EventResult{Action: "continue"}, nil
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP hook error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &EventResult{
			Action: "continue",
			Error:  fmt.Sprintf("HTTP hook returned status %d", resp.StatusCode),
		}, nil
	}

	var result EventResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse hook result: %w", err)
	}

	if result.Action == "" {
		result.Action = "continue"
	}

	return &result, nil
}
