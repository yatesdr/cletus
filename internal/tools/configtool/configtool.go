package configtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cletus/internal/config"
	"cletus/internal/tools"
)

// ConfigTool reads and writes configuration
type ConfigTool struct {
	tools.BaseTool
	cfgPath string
}

// NewConfigTool creates a new ConfigTool
func NewConfigTool() *ConfigTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"description": "Action to perform: read, write, or get-path",
				"enum": ["read", "write", "get-path"]
			},
			"key": {
				"type": "string",
				"description": "Configuration key to read (e.g., 'model', 'api.base_url')"
			},
			"value": {
				"type": "string",
				"description": "Configuration value to write (for write action)"
			}
		},
		"required": ["action"]
	}`)

	return &ConfigTool{
		BaseTool: tools.NewBaseTool("Config", "Read and write configuration values", schema),
		cfgPath:  getDefaultConfigPath(),
	}
}

// Execute performs the config action
func (t *ConfigTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, err := tools.ParseInput(input)
	if err != nil {
		return "", err
	}

	action, ok := tools.GetString(parsed, "action")
	if !ok {
		return "", tools.ErrMissingRequiredField("action")
	}

	switch action {
	case "read":
		return t.readConfig(parsed)
	case "write":
		return t.writeConfig(parsed)
	case "get-path":
		return t.cfgPath, nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ConfigTool) readConfig(parsed map[string]any) (string, error) {
	key, _ := tools.GetString(parsed, "key")

	cfg, err := config.Load(t.cfgPath, "")
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	if key == "" {
		// Return entire config
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal config: %w", err)
		}
		return string(data), nil
	}

	// Handle dot notation for nested keys
	switch key {
	case "model":
		return cfg.Model, nil
	case "api.base_url", "base_url":
		return cfg.API.BaseURL, nil
	case "api.api_key", "api_key":
		return cfg.API.APIKey, nil
	case "api.timeout":
		return fmt.Sprintf("%d", cfg.API.Timeout), nil
	case "max_tokens":
		return fmt.Sprintf("%d", cfg.MaxTokens), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func (t *ConfigTool) writeConfig(parsed map[string]any) (string, error) {
	key, ok := tools.GetString(parsed, "key")
	if !ok {
		return "", tools.ErrMissingRequiredField("key")
	}

	value, ok := tools.GetString(parsed, "value")
	if !ok {
		return "", tools.ErrMissingRequiredField("value")
	}

	cfg, err := config.Load(t.cfgPath, "")
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	// Handle dot notation for nested keys
	switch key {
	case "model":
		cfg.Model = value
	case "api.base_url", "base_url":
		cfg.API.BaseURL = value
	case "api.api_key", "api_key":
		cfg.API.APIKey = value
	case "api.timeout":
		fmt.Sscanf(value, "%d", &cfg.API.Timeout)
	case "max_tokens":
		fmt.Sscanf(value, "%d", &cfg.MaxTokens)
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}

	if err := config.Save(cfg, t.cfgPath); err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}

	return fmt.Sprintf("Updated %s = %s", key, value), nil
}

func getDefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cletus", "config.json")
}

// IsReadOnly returns false for ConfigTool
func (t *ConfigTool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe returns true
func (t *ConfigTool) IsConcurrencySafe() bool {
	return true
}
