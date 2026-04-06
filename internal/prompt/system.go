package prompt

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"cletus/internal/config"
)

//go:embed defaults.md
var embeddedDefaultsMD string

// PromptData holds all dynamic values injected into the system prompt template.
// Add a {{.FieldName}} token to defaults.md to surface any of these values.
type PromptData struct {
	// Environment — always populated
	WorkingDir string
	IsGitRepo  bool
	GitBranch  string
	MainBranch string
	GitStatus  string
	Platform   string
	Shell      string
	OS         string
	Date       string
	Model      string

	// Dynamic sections — empty string causes the {{if}} block to be suppressed
	ToolsDescription string
	ProjectContext   string
	MCPServers       string
	Skills           string
	Memories         string
	Language         string // if set, model responds in this language
	HooksEnabled     bool   // true if hooks are configured in settings
}

// SystemPromptBuilder renders a system prompt template with dynamic data.
type SystemPromptBuilder struct {
	templateSrc string // if empty, uses embedded default or user file
	data        PromptData
}

func NewSystemPromptBuilder() *SystemPromptBuilder {
	return &SystemPromptBuilder{}
}

// SetTemplate sets a custom template string (e.g. from config.md System Prompt override).
func (b *SystemPromptBuilder) SetTemplate(src string) *SystemPromptBuilder {
	b.templateSrc = src
	return b
}

// SetData sets all prompt data at once.
func (b *SystemPromptBuilder) SetData(data PromptData) *SystemPromptBuilder {
	b.data = data
	return b
}

// Build renders the template with the provided data.
// On template parse/execute error, returns the raw template src as a fallback.
func (b *SystemPromptBuilder) Build() string {
	src := b.templateSrc
	if src == "" {
		if loaded := LoadDefaultsMD(""); loaded != "" {
			src = loaded
		} else {
			src = embeddedDefaultsMD
		}
	}

	t, err := template.New("system").Parse(src)
	if err != nil {
		return src
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, b.data); err != nil {
		return src
	}
	return buf.String()
}

// LoadDefaultsMD loads the user's defaults.md from the given path.
// If path is empty, checks ~/.config/cletus/defaults.md.
// Returns empty string if the file does not exist.
func LoadDefaultsMD(path string) string {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "cletus", "defaults.md")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// EmbeddedDefaultsMD returns the compile-time embedded defaults template.
// Used by ensureDefaultsMD to write the file on first run.
func EmbeddedDefaultsMD() string {
	return embeddedDefaultsMD
}

// CollectEnvData gathers environment-level prompt data.
// Call this once per turn and augment the returned struct with
// tools/MCP/memory fields before passing to SystemPromptBuilder.
func CollectEnvData(model string) PromptData {
	cwd, _ := os.Getwd()

	isGit := false
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err == nil {
		isGit = true
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	osStr := ""
	if out, err := exec.Command("uname", "-sr").Output(); err == nil {
		osStr = strings.TrimSpace(string(out))
	}

	gitBranch, mainBranch, gitStatus := "", "", ""
	if isGit {
		if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			gitBranch = strings.TrimSpace(string(out))
		}
		for _, candidate := range []string{"main", "master"} {
			if out, err := exec.Command("git", "branch", "--list", candidate).Output(); err == nil {
				if strings.TrimSpace(string(out)) != "" {
					mainBranch = candidate
					break
				}
			}
		}
		if out, err := exec.Command("git", "status", "--short").Output(); err == nil {
			gitStatus = strings.TrimSpace(string(out))
		}
	}

	return PromptData{
		WorkingDir: cwd,
		IsGitRepo:  isGit,
		GitBranch:  gitBranch,
		MainBranch: mainBranch,
		GitStatus:  gitStatus,
		Platform:   runtime.GOOS,
		Shell:      shell,
		OS:         osStr,
		Date:       time.Now().Format("2006-01-02"),
		Model:      model,
	}
}

// BuildFromConfig builds the system prompt using config overrides.
// If config.md has a ## System Prompt section it is used as the template;
// otherwise the user's defaults.md (or embedded fallback) is used.
func BuildFromConfig(cfg *config.Config) string {
	builder := NewSystemPromptBuilder()
	if cfg.MD != nil {
		if sp := cfg.MD.GetSystemPrompt(); sp != "" {
			builder.SetTemplate(sp)
		}
	}
	builder.SetData(CollectEnvData(cfg.ResolveModel("large")))
	return builder.Build()
}

// DefaultSystemPrompt returns the system prompt using the user's defaults.md
// (or embedded fallback) with current environment data and no model name.
func DefaultSystemPrompt() string {
	return NewSystemPromptBuilder().SetData(CollectEnvData("")).Build()
}

// FormatToolsDescription formats a list of tool schemas as a bulleted list.
func FormatToolsDescription(tools []map[string]any) string {
	var lines []string
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		desc, _ := tool["description"].(string)
		lines = append(lines, fmt.Sprintf("- %s: %s", name, desc))
	}
	return strings.Join(lines, "\n")
}

// GetEnvironmentInfo returns a plain-text environment summary (legacy helper).
func GetEnvironmentInfo() string {
	data := CollectEnvData("")
	lines := []string{
		"Working directory: " + data.WorkingDir,
		fmt.Sprintf("Is git repo: %t", data.IsGitRepo),
		"Platform: " + data.Platform,
		"Shell: " + data.Shell,
	}
	if data.OS != "" {
		lines = append(lines, "OS: "+data.OS)
	}
	if data.GitBranch != "" {
		lines = append(lines, "Git branch: "+data.GitBranch)
	}
	lines = append(lines, "Date: "+data.Date)
	return strings.Join(lines, "\n")
}
