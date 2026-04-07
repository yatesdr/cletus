package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings represents user settings
type Settings struct {
	AllowRules        []string `json:"allow_rules"`
	DenyRules         []string `json:"deny_rules"`
	AutoAcceptTools   bool     `json:"auto_accept_tools"`
	CompactThreshold float64  `json:"compact_threshold"`
	MaxContextTokens  int      `json:"max_context_tokens"`
}

// DefaultSettings returns default settings
func DefaultSettings() *Settings {
	return &Settings{
		AllowRules:        []string{},
		DenyRules:         []string{},
		AutoAcceptTools:   false,
		CompactThreshold: 0.8,
		MaxContextTokens:  200000,
	}
}

// LoadSettings loads settings from file or returns defaults
func LoadSettings(path string) (*Settings, error) {
	settings := DefaultSettings()
	
	if path == "" {
		locations := []string{
			filepath.Join(os.Getenv("HOME"), ".config", "cletus", "settings.json"),
			".cletus.json",
			".cletus/settings.json",
		}
		
		for _, loc := range locations {
			data, err := os.ReadFile(loc)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(data, settings); err != nil {
				return nil, fmt.Errorf("parse settings %s: %w", loc, err)
			}
			return settings, nil
		}
		return settings, nil
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	
	return settings, nil
}
