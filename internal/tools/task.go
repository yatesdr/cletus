package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a task
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskStore stores tasks in memory
type TaskStore struct {
	tasks map[string]Task
	mu    sync.RWMutex
}

// NewTaskStore creates a new task store
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]Task),
	}
}

// CreateTaskTool creates a new task
type CreateTaskTool struct {
	BaseTool
	store *TaskStore
}

// NewCreateTaskTool creates a new CreateTaskTool
func NewCreateTaskTool(store *TaskStore) *CreateTaskTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {
				"type": "string",
				"description": "The task title"
			},
			"description": {
				"type": "string",
				"description": "The task description"
			}
		},
		"required": ["title"]
	}`)
	return &CreateTaskTool{
		BaseTool: NewBaseTool("TaskCreate", "Create a new task", schema),
		store:    store,
	}
}

func (t *CreateTaskTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	title, ok := GetString(parsed, "title")
	if !ok {
		return "", ErrMissingRequiredField("title")
	}

	description, _ := GetString(parsed, "description")

	task := Task{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	t.store.mu.Lock()
	t.store.tasks[task.ID] = task
	t.store.mu.Unlock()

	return fmt.Sprintf("Created task: %s", task.ID), nil
}

// UpdateTaskTool updates an existing task
type UpdateTaskTool struct {
	BaseTool
	store *TaskStore
}

// NewUpdateTaskTool creates a new UpdateTaskTool
func NewUpdateTaskTool(store *TaskStore) *UpdateTaskTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "The task ID"
			},
			"title": {
				"type": "string",
				"description": "New title"
			},
			"description": {
				"type": "string",
				"description": "New description"
			},
			"status": {
				"type": "string",
				"description": "New status (pending, in_progress, completed, failed)"
			}
		},
		"required": ["id"]
	}`)
	return &UpdateTaskTool{
		BaseTool: NewBaseTool("TaskUpdate", "Update a task", schema),
		store:    store,
	}
}

func (t *UpdateTaskTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	id, ok := GetString(parsed, "id")
	if !ok {
		return "", ErrMissingRequiredField("id")
	}

	t.store.mu.Lock()
	defer t.store.mu.Unlock()

	task, exists := t.store.tasks[id]
	if !exists {
		return "", fmt.Errorf("task not found: %s", id)
	}

	if title, ok := GetString(parsed, "title"); ok {
		task.Title = title
	}
	if desc, ok := GetString(parsed, "description"); ok {
		task.Description = desc
	}
	if status, ok := GetString(parsed, "status"); ok {
		task.Status = TaskStatus(status)
	}
	task.UpdatedAt = time.Now()

	t.store.tasks[id] = task
	return fmt.Sprintf("Updated task: %s", id), nil
}

// ListTaskTool lists tasks
type ListTaskTool struct {
	BaseTool
	store *TaskStore
}

// NewListTaskTool creates a new ListTaskTool
func NewListTaskTool(store *TaskStore) *ListTaskTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"description": "Filter by status"
			}
		}
	}`)
	return &ListTaskTool{
		BaseTool: NewBaseTool("TaskList", "List tasks", schema),
		store:    store,
	}
}

func (t *ListTaskTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	t.store.mu.RLock()
	defer t.store.mu.RUnlock()

	filterStatus, hasFilter := GetString(parsed, "status")

	var lines []string
	for _, task := range t.store.tasks {
		if hasFilter && string(task.Status) != filterStatus {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", task.Status, task.ID, task.Title))
	}

	if len(lines) == 0 {
		return "No tasks found", nil
	}

	return "Tasks:\n" + join(lines, "\n"), nil
}

func join(arr []string, sep string) string {
	if len(arr) == 0 {
		return ""
	}
	result := arr[0]
	for i := 1; i < len(arr); i++ {
		result += sep + arr[i]
	}
	return result
}

// GetTaskTool gets a specific task by ID
type GetTaskTool struct {
	BaseTool
	store *TaskStore
}

// NewGetTaskTool creates a new GetTaskTool
func NewGetTaskTool(store *TaskStore) *GetTaskTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "The task ID"
			}
		},
		"required": ["id"]
	}`)
	return &GetTaskTool{
		BaseTool: NewBaseTool("TaskGet", "Get a task by ID", schema),
		store:    store,
	}
}

func (t *GetTaskTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- ToolProgress) (string, error) {
	parsed, err := ParseInput(input)
	if err != nil {
		return "", err
	}

	id, ok := GetString(parsed, "id")
	if !ok {
		return "", ErrMissingRequiredField("id")
	}

	t.store.mu.RLock()
	defer t.store.mu.RUnlock()

	task, exists := t.store.tasks[id]
	if !exists {
		return "", fmt.Errorf("task not found: %s", id)
	}

	return formatTask(task), nil
}

func formatTask(t Task) string {
	return fmt.Sprintf(`ID: %s
Title: %s
Description: %s
Status: %s
Created: %s
Updated: %s`,
		t.ID, t.Title, t.Description, t.Status, t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
}
