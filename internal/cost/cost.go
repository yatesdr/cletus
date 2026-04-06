package cost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cletus/internal/api"
)

// Tracker tracks token usage and costs
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

// NewTracker creates a new cost tracker
func NewTracker(model string) *Tracker {
	return &Tracker{
		model:     model,
		startTime: time.Now(),
	}
}

// RecordUsage records API usage
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

// GetUsage returns current usage stats
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

// GetCost returns estimated cost in USD
func (t *Tracker) GetCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return CalculateCost(t.inputTokens, t.outputTokens, t.cacheRead, t.cacheWrite, t.model)
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
	return fmt.Sprintf(`Input: %d | Output: %d | Cache R: %d | Cache W: %d | Total: %d | API Calls: %d | Tool Calls: %d | Duration: %s`,
		u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens, u.TotalTokens,
		u.APICalls, u.ToolCalls, u.Duration.Round(time.Second))
}

// ModelPricing provides pricing per 1M tokens
var ModelPricing = map[string]ModelPrice{
	"claude-opus-4-6":   {Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 7.5},
	"claude-sonnet-4-6": {Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 1.5},
	"claude-haiku-4-5":  {Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 0.4},
	"claude-3-5-sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 1.5},
	"claude-3-opus":     {Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 7.5},
	"llama-3":           {Input: 0.0, Output: 0.0, CacheRead: 0.0, CacheWrite: 0.0},
	"llama":             {Input: 0.0, Output: 0.0, CacheRead: 0.0, CacheWrite: 0.0},
	"gpt-4":             {Input: 30.0, Output: 60.0, CacheRead: 0.0, CacheWrite: 0.0},
	"gpt-4o":            {Input: 5.0, Output: 15.0, CacheRead: 0.0, CacheWrite: 0.0},
}

// ModelPrice holds pricing for a model
type ModelPrice struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// CalculateCost calculates cost in USD
func CalculateCost(input, output, cacheRead, cacheWrite int, model string) float64 {
	pricing, ok := ModelPricing[model]
	if !ok {
		// Default pricing for unknown models
		pricing = ModelPrice{Input: 3.0, Output: 15.0}
	}

	inputCost := float64(input) / 1_000_000 * pricing.Input
	outputCost := float64(output) / 1_000_000 * pricing.Output
	readCost := float64(cacheRead) / 1_000_000 * pricing.CacheRead
	writeCost := float64(cacheWrite) / 1_000_000 * pricing.CacheWrite

	return inputCost + outputCost + readCost + writeCost
}

// FormatCost formats cost as string
func FormatCost(cost float64) string {
	if cost < 0.001 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.4f", cost)
}

// Store saves cost data to disk
type Store struct {
	mu       sync.Mutex
	dir      string
	sessions map[string]*SessionCost
}

// SessionCost holds cost for a session
type SessionCost struct {
	ID           string    `json:"id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewStore creates a cost store
func NewStore(dir string) *Store {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "cletus", "costs")
	}
	return &Store{dir: dir}
}

// EnsureDir ensures the directory exists
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.dir, 0755)
}

// Save saves session cost
func (s *Store) Save(id string, cost *Tracker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.EnsureDir(); err != nil {
		return err
	}

	usage := cost.GetUsage()
	sessionCost := &SessionCost{
		ID:           id,
		Model:        cost.model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CostUSD:      cost.GetCost(),
		UpdatedAt:    time.Now(),
	}

	data, _ := json.MarshalIndent(sessionCost, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, id+".json"), data, 0644)
}

// Load loads session cost
func (s *Store) Load(id string) (*SessionCost, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var cost SessionCost
	if err := json.Unmarshal(data, &cost); err != nil {
		return nil, err
	}
	return &cost, nil
}
