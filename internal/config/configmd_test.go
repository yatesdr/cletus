package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigMDParsing(t *testing.T) {
	// Use the embedded default config for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.md")

	// Write a minimal test config
	content := "# Model: test-model\n\ntest content\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	md, err := LoadConfigMD(configPath)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	fmt.Printf("Sections: %d\n", len(md.Sections))
	fmt.Printf("Tools: %d\n", len(md.ToolPrompts))
	fmt.Printf("Models: %d\n", len(md.Models))
	fmt.Printf("Defaults: %d\n", len(md.Defaults))

	sp := md.GetSystemPrompt()
	fmt.Printf("System Prompt: %d chars\n", len(sp))
}
