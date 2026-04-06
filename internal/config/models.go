package config

// ModelProfile represents a model's capabilities (matches configmd.go)
type ModelProfile struct {
	Name             string `json:"name"`
	ContextWindow    int    `json:"context_window"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	SupportsVision   bool   `json:"supports_vision"`
	SupportsThinking bool   `json:"supports_thinking"`
	SupportsToolUse  bool   `json:"supports_tool_use"`
	JSONMode         bool   `json:"json_mode"`
	KnowledgeCutoff  string `json:"knowledge_cutoff"`
}

// Known model profiles
var ModelProfiles = map[string]ModelProfile{
	"claude-3-5-sonnet-20241022": {
		Name:             "claude-3-5-sonnet-20241022",
		ContextWindow:    200000,
		MaxOutputTokens:  8192,
		SupportsVision:   true,
		SupportsThinking: false,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2024-10",
	},
	"claude-3-opus-20240229": {
		Name:             "claude-3-opus-20240229",
		ContextWindow:    200000,
		MaxOutputTokens:  4096,
		SupportsVision:   true,
		SupportsThinking: false,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2024-02",
	},
	"claude-sonnet-4-6": {
		Name:             "claude-sonnet-4-6",
		ContextWindow:    200000,
		MaxOutputTokens:  16384,
		SupportsVision:   true,
		SupportsThinking: true,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2025-05",
	},
	"claude-opus-4-6": {
		Name:             "claude-opus-4-6",
		ContextWindow:    200000,
		MaxOutputTokens:  16384,
		SupportsVision:   true,
		SupportsThinking: true,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2025-05",
	},
	// Local models
	"local-default": {
		Name:             "local-default",
		ContextWindow:    32768,
		MaxOutputTokens:  4096,
		SupportsVision:   false,
		SupportsThinking: false,
		SupportsToolUse:  true,
		JSONMode:         false,
		KnowledgeCutoff:  "2024-01",
	},
	"Qwen/Qwen2.5-72B-Instruct": {
		Name:             "Qwen/Qwen2.5-72B-Instruct",
		ContextWindow:    32768,
		MaxOutputTokens:  8192,
		SupportsVision:   false,
		SupportsThinking: false,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2023-12",
	},
	"DeepSeek-V3": {
		Name:             "DeepSeek-V3",
		ContextWindow:    64000,
		MaxOutputTokens:  8192,
		SupportsVision:   false,
		SupportsThinking: true,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2024-06",
	},
	"MiniMax/MiniMax-M2.1": {
		Name:             "MiniMax/MiniMax-M2.1",
		ContextWindow:    128000,
		MaxOutputTokens:  8192,
		SupportsVision:   true,
		SupportsThinking: false,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2025-01",
	},
}

// GetProfile returns the profile for a model, or a default profile
func GetProfile(model string) ModelProfile {
	if profile, ok := ModelProfiles[model]; ok {
		return profile
	}
	// Return default local model profile
	return ModelProfiles["local-default"]
}

// MergeWithConfigMD merges profiles from config.md into the default profiles
func MergeWithConfigMD(cfgMD *ConfigMD) {
	if cfgMD == nil || len(cfgMD.Models) == 0 {
		return
	}
	for name, profile := range cfgMD.Models {
		ModelProfiles[name] = profile
	}
}

// SupportsVision checks if a model supports vision
func SupportsVision(model string) bool {
	return GetProfile(model).SupportsVision
}

// SupportsThinking checks if a model supports thinking/reasoning
func SupportsThinking(model string) bool {
	return GetProfile(model).SupportsThinking
}

// SupportsTools checks if a model supports tool use
func SupportsTools(model string) bool {
	return GetProfile(model).SupportsToolUse
}

// GetContextLimit returns the context window size
func GetContextLimit(model string) int {
	return GetProfile(model).ContextWindow
}

// GetMaxOutput returns the max output tokens
func GetMaxOutput(model string) int {
	return GetProfile(model).MaxOutputTokens
}
