package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigMD holds parsed config.md sections
type ConfigMD struct {
	Sections   map[string]string
	ToolPrompts map[string]string
	Models     map[string]ModelProfile
	Defaults   map[string]string
}

// LoadConfigMD loads and parses a config.md file
func LoadConfigMD(path string) (*ConfigMD, error) {
	if path == "" {
		path = findConfigMD()
	}
	
	if path == "" || !fileExists(path) {
		return &ConfigMD{
			Sections:    make(map[string]string),
			ToolPrompts: make(map[string]string),
			Models:      make(map[string]ModelProfile),
			Defaults:    make(map[string]string),
		}, nil
	}
	
	return parseConfigMD(path)
}

func findConfigMD() string {
	paths := []string{".cletus/config.md", ".config/cletus/config.md"}
	wd, _ := os.Getwd()
	for _, p := range paths {
		if fullPath := filepath.Join(wd, p); fileExists(fullPath) {
			return fullPath
		}
	}
	home, _ := os.UserHomeDir()
	for _, p := range paths {
		if fullPath := filepath.Join(home, p); fileExists(fullPath) {
			return fullPath
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseConfigMD(path string) (*ConfigMD, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	cfg := &ConfigMD{
		Sections:    make(map[string]string),
		ToolPrompts: make(map[string]string),
		Models:      make(map[string]ModelProfile),
		Defaults:    make(map[string]string),
	}
	
	currentSection := ""
	var lines []string
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			if currentSection != "" {
				cfg.processSection(currentSection, lines)
			}
			currentSection = strings.TrimPrefix(line, "## ")
			lines = []string{}
		} else if currentSection != "" {
			lines = append(lines, line)
		}
	}
	
	if currentSection != "" {
		cfg.processSection(currentSection, lines)
	}
	
	return cfg, scanner.Err()
}

func (c *ConfigMD) processSection(name string, lines []string) {
	// Strip comment lines (lines starting with #) before joining
	filtered := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			filtered = append(filtered, l)
		}
	}
	content := strings.TrimSpace(strings.Join(filtered, "\n"))
	
	if strings.HasPrefix(name, "Tool: ") {
		toolName := strings.TrimPrefix(name, "Tool: ")
		c.ToolPrompts[toolName] = content
		return
	}
	
	if strings.HasPrefix(name, "Model: ") {
		modelName := strings.TrimPrefix(name, "Model: ")
		profile := parseModelProfile(content)
		c.Models[modelName] = profile
		return
	}
	
	if name == "Defaults" {
		c.Defaults = parseKeyValuePairs(content)
		return
	}
	
	c.Sections[name] = content
}

func parseModelProfile(content string) ModelProfile {
	profile := ModelProfile{
		ContextWindow:    200000,
		MaxOutputTokens:  16384,
		SupportsVision:   true,
		SupportsThinking: true,
		SupportsToolUse:  true,
		JSONMode:         true,
		KnowledgeCutoff:  "2025-05",
	}
	
	pairs := parseKeyValuePairs(content)
	
	if v, ok := pairs["context_window"]; ok {
		fmt.Sscanf(v, "%d", &profile.ContextWindow)
	}
	if v, ok := pairs["max_output_tokens"]; ok {
		fmt.Sscanf(v, "%d", &profile.MaxOutputTokens)
	}
	if v, ok := pairs["supports_vision"]; ok {
		profile.SupportsVision = strings.ToLower(v) == "true"
	}
	if v, ok := pairs["supports_thinking"]; ok {
		profile.SupportsThinking = strings.ToLower(v) == "true"
	}
	if v, ok := pairs["supports_tool_use"]; ok {
		profile.SupportsToolUse = strings.ToLower(v) == "true"
	}
	if v, ok := pairs["json_mode"]; ok {
		profile.JSONMode = strings.ToLower(v) == "true"
	}
	if v, ok := pairs["knowledge_cutoff"]; ok {
		profile.KnowledgeCutoff = v
	}
	
	return profile
}

func parseKeyValuePairs(content string) map[string]string {
	result := make(map[string]string)
	re := regexp.MustCompile(`^(\w+):\s*(.+)$`)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			result[matches[1]] = strings.TrimSpace(matches[2])
		}
	}
	return result
}

func (c *ConfigMD) Get(name string) string {
	return c.Sections[name]
}

func (c *ConfigMD) GetTool(name string) string {
	return c.ToolPrompts[name]
}

func (c *ConfigMD) GetModel(name string) (ModelProfile, bool) {
	profile, ok := c.Models[name]
	return profile, ok
}

func (c *ConfigMD) GetDefaults() map[string]string {
	return c.Defaults
}

func (c *ConfigMD) GetIdentity() string {
	return c.Sections["Identity"]
}

func (c *ConfigMD) GetSystemPrompt() string {
	return c.Sections["System Prompt"]
}
