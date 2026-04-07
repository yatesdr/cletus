package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cletus/internal/tools"
)

// SkillTool manages and executes skills
type SkillTool struct {
	tools.BaseTool
	skillsDir string
}

// NewSkillTool creates SkillTool
func NewSkillTool() *SkillTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "invoke", "add", "remove"],
				"description": "Skill action: list (list all skills), invoke (run a skill), add (create a new skill), remove (delete a skill)"
			},
			"name": {
				"type": "string",
				"description": "Skill name (required for invoke, add, remove)"
			},
			"description": {
				"type": "string",
				"description": "Skill description (for add action)"
			},
			"prompt": {
				"type": "string",
				"description": "System prompt for the skill (for add action)"
			},
			"args": {
				"type": "object",
				"description": "Arguments for skill invocation"
			}
		},
		"required": ["action"]
	}`)
	return &SkillTool{
		BaseTool:  tools.NewBaseTool("Skill", "Manage and invoke custom skills", schema),
		skillsDir: getDefaultSkillsDir(),
	}
}

func getDefaultSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cletus", "skills")
}

func (t *SkillTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	parsed, _ := tools.ParseInput(input)
	action, _ := tools.GetString(parsed, "action")
	name, _ := tools.GetString(parsed, "name")

	switch action {
	case "list":
		return t.listSkills()
	case "invoke":
		if name == "" {
			return "", tools.ErrMissingRequiredField("name")
		}
		return t.invokeSkill(name, parsed)
	case "add":
		if name == "" {
			return "", tools.ErrMissingRequiredField("name")
		}
		return t.addSkill(name, parsed)
	case "remove":
		if name == "" {
			return "", tools.ErrMissingRequiredField("name")
		}
		return t.removeSkill(name)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *SkillTool) listSkills() (string, error) {
	entries, err := os.ReadDir(t.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return `{"skills": []}`, nil
		}
		return "", err
	}

	type skillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}

	var skills []skillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()
		descPath := filepath.Join(t.skillsDir, name, "description.md")
		desc := ""
		if data, err := os.ReadFile(descPath); err == nil {
			desc = string(data)
		}

		skills = append(skills, skillInfo{
			Name:        name,
			Description: desc,
		})
	}
	return fmt.Sprintf(`{"skills": %s}`, toJSON(skills)), nil
}

func (t *SkillTool) addSkill(name string, args map[string]any) (string, error) {
	skillPath := filepath.Join(t.skillsDir, name)
	
	// Check if skill already exists
	if _, err := os.Stat(skillPath); err == nil {
		return "", fmt.Errorf("skill already exists: %s", name)
	}
	
	// Create skill directory
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		return "", fmt.Errorf("create skill directory: %w", err)
	}
	
	// Create description.md if provided
	if desc, _ := tools.GetString(args, "description"); desc != "" {
		descPath := filepath.Join(skillPath, "description.md")
		if err := os.WriteFile(descPath, []byte(desc), 0644); err != nil {
			os.RemoveAll(skillPath)
			return "", fmt.Errorf("write description: %w", err)
		}
	}
	
	// Create prompt.md if provided
	if prompt, _ := tools.GetString(args, "prompt"); prompt != "" {
		promptPath := filepath.Join(skillPath, "prompt.md")
		if err := os.WriteFile(promptPath, []byte(prompt), 0644); err != nil {
			os.RemoveAll(skillPath)
			return "", fmt.Errorf("write prompt: %w", err)
		}
	}
	
	// Create placeholder run script
	runPath := filepath.Join(skillPath, "run")
	runScript := `#!/bin/bash
# Skill: ` + name + `
# Add your implementation here

echo "Skill executed"
`
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		os.RemoveAll(skillPath)
		return "", fmt.Errorf("write run script: %w", err)
	}
	
	return fmt.Sprintf(`{"added": true, "name": "%s", "path": "%s"}`, name, skillPath), nil
}

func (t *SkillTool) removeSkill(name string) (string, error) {
	skillPath := filepath.Join(t.skillsDir, name)
	
	if _, err := os.Stat(skillPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("skill not found: %s", name)
		}
		return "", err
	}
	
	if err := os.RemoveAll(skillPath); err != nil {
		return "", fmt.Errorf("remove skill: %w", err)
	}
	
	return fmt.Sprintf(`{"removed": true, "name": "%s"}`, name), nil
}

func (t *SkillTool) invokeSkill(name string, args map[string]any) (string, error) {
	skillPath := filepath.Join(t.skillsDir, name)
	info, err := os.Stat(skillPath)
	if err != nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("skill must be a directory")
	}

	// Look for prompt.md
	promptPath := filepath.Join(skillPath, "prompt.md")
	prompt := ""
	if data, err := os.ReadFile(promptPath); err == nil {
		prompt = string(data)
	}

	// Look for run executable
	execPath := filepath.Join(skillPath, "run")
	if _, err := os.Stat(execPath); err != nil {
		return "", fmt.Errorf("skill has no run executable at: %s", execPath)
	}

	// Return skill prompt for execution
	// Note: Actual execution would need to be done by the agent
	return fmt.Sprintf(`{"invoked": true, "name": "%s", "prompt": "%s", "executable": "%s"}`, 
		name, prompt, execPath), nil
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func (t *SkillTool) IsReadOnly() bool { return false }
func (t *SkillTool) IsConcurrencySafe() bool { return true }
