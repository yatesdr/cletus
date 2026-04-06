package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ModelRoles maps generic role names to actual model identifiers.
// Users configure which model serves each role.
type ModelRoles struct {
	Large       string `json:"large"`        // Main conversation model
	Medium      string `json:"medium"`       // Lighter model for simpler tasks
	Small       string `json:"small"`        // Fast model for compaction/classification
	Vision      string `json:"vision"`       // Vision-capable model for image description
	VisionSmall string `json:"vision_small"` // Smaller vision model
	OCR         string `json:"ocr"`          // OCR model (e.g., PaddleOCR)
}

// BackendConfig defines connection settings for a specific model/endpoint.
type BackendConfig struct {
	BaseURL string `json:"base_url"` // e.g., "http://192.168.5.144:8081/v1"
	APIKey  string `json:"api_key"`  // optional
	APIType string `json:"api_type"` // "openai" (default) or "anthropic"
	Timeout int    `json:"timeout"`  // seconds, default 300
}

// Config holds the application configuration
type Config struct {
	API          APIConfig                `json:"api"`
	Model        string                   `json:"model"` // Deprecated: use Models.Large instead
	Models       ModelRoles               `json:"models"`
	Backends     map[string]BackendConfig `json:"backends"`
	MaxTokens    int                      `json:"max_tokens"`
	Tools        []string                 `json:"tools"`
	Permissions  PermissionConfig         `json:"permissions"`
	Env          map[string]string        `json:"env"`
	Hooks        HooksConfig
	WebSearchKey string                     `json:"hooks"`
	MCPServers   map[string]MCPServerConfig `json:"mcp_servers"`
	Sandbox      SandboxConfig              `json:"sandbox"`
	Language     string                     `json:"language"`  // preferred response language, e.g. "Spanish"
	MD           *ConfigMD                  `json:"-"` // loaded separately from config.md
}

// APIConfig holds API-specific configuration
type APIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	APIType string `json:"api_type"` // "openai" or "anthropic"
	Timeout int    `json:"timeout"`  // seconds
}

// PermissionConfig holds permission settings
type PermissionConfig struct {
	Mode  string   `json:"mode"`  // default, acceptEdits, dontAsk, bypassPermissions, plan
	Rules []string `json:"rules"` // Rule strings like "Bash(rm *)" -> deny
}

// HooksConfig holds hooks configuration
type HooksConfig struct {
	Enabled  bool                   `json:"enabled"`
	Commands map[string]HookCommand `json:"commands"`
	HTTP     map[string]HookHTTP    `json:"http"`
	Prompt   map[string]HookPrompt  `json:"prompt"`
}

// HookCommand holds command hook configuration
type HookCommand struct {
	Command string `json:"command"`
	Matcher string `json:"matcher"` // tool name pattern
	Timeout int    `json:"timeout"` // seconds
}

// HookHTTP holds HTTP hook configuration
type HookHTTP struct {
	URL     string `json:"url"`
	Method  string `json:"method"` // GET, POST, etc.
	Matcher string `json:"matcher"`
	Timeout int    `json:"timeout"`
}

// HookPrompt holds prompt hook configuration
type HookPrompt struct {
	Prompt  string `json:"prompt"`
	Matcher string `json:"matcher"`
}

// MCPServerConfig holds MCP server configuration
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Enabled bool              `json:"enabled"`
}

// SandboxConfig holds sandbox configuration
type SandboxConfig struct {
	Enabled          bool     `json:"enabled"`
	Network          string   `json:"network"`    // allow, deny
	Filesystem       string   `json:"filesystem"` // allow, deny
	ExcludedCommands []string `json:"excluded_commands"`
	AllowedPaths     []string `json:"allowed_paths"`
}

// DefaultLargeModel is the fallback model if nothing is configured
const DefaultLargeModel = "MiniMax-M2.5"

func DefaultModelName() string {
	return DefaultLargeModel
}

func DefaultConfig() *Config {
	return &Config{
		API: APIConfig{
			BaseURL: "http://localhost:8080/v1",
			APIKey:  "",
			APIType: "openai",
			Timeout: 300,
		},
		Model: DefaultLargeModel,
		Models: ModelRoles{
			Large:       DefaultLargeModel,
			Medium:      "qwen-3.5",
			Small:       "glm-5-turbo",
			Vision:      DefaultLargeModel,
			VisionSmall: "glm-4.7",
			OCR:         "paddleOCR",
		},
		Backends:  make(map[string]BackendConfig),
		MaxTokens: 8192,
		Tools:     []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Permissions: PermissionConfig{
			Mode:  "default",
			Rules: []string{},
		},
		Hooks:      HooksConfig{Commands: make(map[string]HookCommand), HTTP: make(map[string]HookHTTP), Prompt: make(map[string]HookPrompt)},
		MCPServers: make(map[string]MCPServerConfig),
		Sandbox:    SandboxConfig{Enabled: false, Network: "allow", Filesystem: "allow", ExcludedCommands: []string{}, AllowedPaths: []string{}},
		Env:        make(map[string]string),
	}
}

// Load loads configuration from file and optionally config.md
func Load(path string, configMDPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Load JSON config
	if path == "" {
		path = getDefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// Backward compatibility: if Models.Large is not set but Model is, use Model
	if cfg.Models.Large == "" && cfg.Model != "" {
		cfg.Models.Large = cfg.Model
	}

	// Backward compatibility: if APIType is not set, default to openai
	if cfg.API.APIType == "" {
		cfg.API.APIType = "openai"
	}

	// Load config.md (markdown config)
	md, err := LoadConfigMD(configMDPath)
	if err != nil {
		return nil, fmt.Errorf("load config.md: %w", err)
	}
	cfg.MD = md

	// Apply defaults from config.md if JSON didn't set them
	if defaults := md.GetDefaults(); defaults != nil {
		if cfg.Models.Large == "" && defaults["model"] != "" {
			cfg.Models.Large = defaults["model"]
		}
		if cfg.API.BaseURL == "" && defaults["base_url"] != "" {
			cfg.API.BaseURL = defaults["base_url"]
		}
		if cfg.MaxTokens == 0 && defaults["max_tokens"] != "" {
			fmt.Sscanf(defaults["max_tokens"], "%d", &cfg.MaxTokens)
		}
		if cfg.API.Timeout == 0 && defaults["timeout"] != "" {
			fmt.Sscanf(defaults["timeout"], "%d", &cfg.API.Timeout)
		}
		if cfg.API.APIType == "" && defaults["api_type"] != "" {
			cfg.API.APIType = defaults["api_type"]
		}
	}

	MergeWithConfigMD(md)

	return cfg, nil
}

// Save saves configuration to file
func Save(cfg *Config, path string) error {
	if path == "" {
		path = getDefaultConfigPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// findConfigPath searches for settings.json in priority order:
// 1. ./settings.json (current directory)
// 2. .cletus/settings.json (project-level)
// 3. ~/.config/cletus/settings.json (user-level)
// Returns the first one found, or the user-level path as default.
func getDefaultConfigPath() string {
	// Check current directory first
	if fileExists("settings.json") {
		abs, err := filepath.Abs("settings.json")
		if err == nil {
			return abs
		}
		return "settings.json"
	}

	// Check project-level .cletus/settings.json
	if fileExists(".cletus/settings.json") {
		abs, err := filepath.Abs(".cletus/settings.json")
		if err == nil {
			return abs
		}
		return ".cletus/settings.json"
	}

	// Fall back to user-level config
	home, _ := os.UserHomeDir()
	userPath := filepath.Join(home, ".config", "cletus", "settings.json")
	if fileExists(userPath) {
		return userPath
	}

	// Also check legacy config.json paths
	if fileExists("config.json") {
		abs, _ := filepath.Abs("config.json")
		return abs
	}
	legacyPath := filepath.Join(home, ".config", "cletus", "config.json")
	if fileExists(legacyPath) {
		return legacyPath
	}

	// Return default user-level path (may not exist yet)
	return userPath
}

// fileExists is defined in configmd.go
