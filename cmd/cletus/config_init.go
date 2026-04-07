package main

import (
	"fmt"
	"os"
	"path/filepath"

	_ "embed"

	"cletus/internal/prompt"
)

//go:embed default-config.md
var defaultConfigMD string

// ensureConfigMD ensures a config.md exists, writing the embedded default if none found.
// Users can edit the created file to override built-in defaults section by section.
func ensureConfigMD(configMDPath string) (string, error) {
	if configMDPath != "" {
		if fileExists(configMDPath) {
			return configMDPath, nil
		}
		if err := writeConfigMD(configMDPath); err != nil {
			return "", fmt.Errorf("create config.md at %s: %w", configMDPath, err)
		}
		return configMDPath, nil
	}

	searchPaths := []string{
		".cletus/config.md",
		filepath.Join(getConfigDir(), "config.md"),
	}

	wd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	for _, p := range searchPaths {
		if fullPath := filepath.Join(wd, p); fileExists(fullPath) {
			return fullPath, nil
		}
		if fullPath := filepath.Join(home, p); fileExists(fullPath) {
			return fullPath, nil
		}
	}

	// Not found — create from embedded default so users have something to edit
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	configPath := filepath.Join(configDir, "config.md")
	if err := writeConfigMD(configPath); err != nil {
		return "", fmt.Errorf("write default config.md: %w", err)
	}
	return configPath, nil
}

func writeConfigMD(path string) error {
	if err := os.WriteFile(path, []byte(defaultConfigMD), 0644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Created default config.md at %s\n", path)
	return nil
}

// ensureDefaultsMD ensures ~/.config/cletus/defaults.md exists.
// On first run it writes the embedded default so users have a file to edit.
func ensureDefaultsMD() (string, error) {
	configDir := getConfigDir()
	path := filepath.Join(configDir, "defaults.md")
	if fileExists(path) {
		return path, nil
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(prompt.EmbeddedDefaultsMD()), 0644); err != nil {
		return "", fmt.Errorf("write defaults.md: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Created default system prompt at %s\n", path)
	return path, nil
}

func getConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cletus")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
