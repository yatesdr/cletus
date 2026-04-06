package usage

import (
	"sync"
	"time"

	"cletus/internal/api"
)

// Tracker tracks token usage and API calls
type Tracker struct {
	mu           sync.RWMutex
	inputTokens  int
	outputTokens int
	cacheRead    int
	cacheWrite   int
	apiCalls     int
	toolCalls    int
	toolDuration time.Duration
	startTime    time.Time
	model        string
}

// NewTracker creates a new usage tracker
func NewTracker(model string) *Tracker {
	return &Tracker{
		model:    model,
		startTime: time.Now(),
	}
}

// RecordUsage records API usage from a response
func (t *Tracker) RecordUsage(usage api.Usage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inputTokens += usage.InputTokens
	t.outputTokens += usage.OutputTokens
	t.cacheRead += usage.CacheReadTokens
	t.cacheWrite += usage.CacheWriteTokens
	t.apiCalls++
}

// RecordTool records a tool call
func (t *Tracker) RecordTool(name string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.toolCalls++
	t.toolDuration += duration
}

// GetUsage returns current usage statistics
func (t *Tracker) GetUsage() UsageStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return UsageStats{
		InputTokens:      t.inputTokens,
		OutputTokens:     t.outputTokens,
		CacheReadTokens:  t.cacheRead,
		CacheWriteTokens: t.cacheWrite,
		APICalls:         t.apiCalls,
		ToolCalls:        t.toolCalls,
		ToolDuration:     t.toolDuration,
		TotalTokens:      t.inputTokens + t.outputTokens,
		Duration:         time.Since(t.startTime),
	}
}

// Reset resets all counters
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inputTokens = 0
	t.outputTokens = 0
	t.cacheRead = 0
	t.cacheWrite = 0
	t.apiCalls = 0
	t.toolCalls = 0
	t.toolDuration = 0
	t.startTime = time.Now()
}

// UsageStats holds usage statistics
type UsageStats struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	APICalls         int
	ToolCalls        int
	ToolDuration     time.Duration
	TotalTokens      int
	Duration         time.Duration
}

// String returns a formatted string
func (u UsageStats) String() string {
	return `Input: %d | Output: %d | Cache R: %d | Cache W: %d | Total: %d | API Calls: %d | Tool Calls: %d | Duration: %s`
}
