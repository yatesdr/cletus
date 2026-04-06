package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectContext holds project-specific context
type ProjectContext struct {
	Path        string
	CLETUSMD   string
	Files       []string
	Dirs        []string
	HasReadme   bool
	HasTests    bool
	Language    string
	BuildTool   string
	PackageMgr  string
}

// ProjectScanner scans and extracts project context
type ProjectScanner struct {
	projectRoot string
}

// NewProjectScanner creates a new project scanner
func NewProjectScanner(projectRoot string) *ProjectScanner {
	return &ProjectScanner{
		projectRoot: projectRoot,
	}
}

// Scan scans the project and extracts context
func (s *ProjectScanner) Scan() (*ProjectContext, error) {
	ctx := &ProjectContext{
		Path: s.projectRoot,
	}

	// Load CLETUS.md from all levels (user, parent dirs, project root)
	if content, err := GetCLETUSMD(s.projectRoot); err == nil {
		ctx.CLETUSMD = content
	}

	// Scan for key files
	entries, err := os.ReadDir(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("read project dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files and common non-project dirs
		if strings.HasPrefix(name, ".") && name != ".git" {
			continue
		}
		if name == "node_modules" || name == "vendor" || name == "build" || name == "dist" {
			continue
		}

		if entry.IsDir() {
			ctx.Dirs = append(ctx.Dirs, name)
		} else {
			ctx.Files = append(ctx.Files, name)
			
			// Check for readme
			if strings.ToLower(name) == "readme.md" || strings.ToLower(name) == "readme.txt" {
				ctx.HasReadme = true
			}
			
			// Check for tests
			if strings.HasPrefix(name, "test") || strings.HasPrefix(name, "spec") || 
			   strings.Contains(name, "_test.") || strings.Contains(name, ".test.") {
				ctx.HasTests = true
			}
		}
	}

	// Detect language/build tool
	ctx.Language = detectLanguage(ctx.Files)
	ctx.BuildTool = detectBuildTool(ctx.Files, ctx.Dirs)
	ctx.PackageMgr = detectPackageMgr(ctx.Files)

	return ctx, nil
}

// GetCLETUSMD loads and merges CLETUS.md content from multiple levels:
//  1. ~/.config/cletus/CLETUS.md (user-level, lowest priority)
//  2. Any CLETUS.md found walking up from projectRoot to the git root
//  3. projectRoot/CLETUS.md (highest priority, can override)
//
// Contents are concatenated in order (lowest → highest priority) so that
// more-specific instructions can append to or override broader ones.
func GetCLETUSMD(projectRoot string) (string, error) {
	var parts []string

	// 1. User-level
	home, _ := os.UserHomeDir()
	userLevel := filepath.Join(home, ".config", "cletus", "CLETUS.md")
	if data, err := os.ReadFile(userLevel); err == nil {
		parts = append(parts, strings.TrimSpace(string(data)))
	}

	// 2. Walk from git root up to (but not including) projectRoot
	gitRoot := findGitRoot(projectRoot)
	if gitRoot != "" && gitRoot != projectRoot {
		for dir := gitRoot; dir != projectRoot; dir = filepath.Dir(dir) {
			if data, err := os.ReadFile(filepath.Join(dir, "CLETUS.md")); err == nil {
				parts = append(parts, strings.TrimSpace(string(data)))
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}

	// 3. Project root (and legacy .cletus.md)
	for _, name := range []string{"CLETUS.md", ".cletus.md"} {
		if data, err := os.ReadFile(filepath.Join(projectRoot, name)); err == nil {
			parts = append(parts, strings.TrimSpace(string(data)))
			break
		}
	}

	return strings.Join(parts, "\n\n"), nil
}

// findGitRoot walks up from dir until it finds a .git directory.
func findGitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// GetProjectContext returns project context for the given directory
func GetProjectContext(projectRoot string) (*ProjectContext, error) {
	scanner := NewProjectScanner(projectRoot)
	return scanner.Scan()
}

func detectLanguage(files []string) string {
	extMap := map[string]string{
		".go": "Go",
		
		".py": "Python",
		".js": "JavaScript",
		".ts": "TypeScript",
		".jsx": "React",
		".tsx": "React",
		".java": "Java",
		".c": "C",
		".cpp": "C++",
		".cs": "C#",
		".rb": "Ruby",
		".php": "PHP",
		".swift": "Swift",
		".kt": "Kotlin",
		".scala": "Scala",
		
	}

	counts := make(map[string]int)
	for _, file := range files {
		ext := filepath.Ext(file)
		if lang, ok := extMap[ext]; ok {
			counts[lang]++
		}
	}

	// Return most common
	maxCount := 0
	var lang string
	for l, c := range counts {
		if c > maxCount {
			maxCount = c
			lang = l
		}
	}

	return lang
}

func detectBuildTool(files, dirs []string) string {
	// Check for build files
	buildFiles := map[string]string{
		"go.mod": "Go",
		"Cargo.toml": "Rust",
		"package.json": "NPM",
		"pom.xml": "Maven",
		"build.gradle": "Gradle",
		"CMakeLists.txt": "CMake",
		"Makefile": "Make",
		"setup.py": "Python",
		"PyProject.toml": "Poetry",
		
	}

	for _, file := range files {
		if tool, ok := buildFiles[file]; ok {
			return tool
		}
	}

	// Check for directories
	dirSet := make(map[string]bool)
	for _, d := range dirs {
		dirSet[d] = true
	}

	if dirSet["gradle"] {
		return "Gradle"
	}
	if dirSet["maven"] {
		return "Maven"
	}

	return ""
}

func detectPackageMgr(files []string) string {
	pkgFiles := map[string]string{
		"package.json": "npm",
		"yarn.lock": "yarn",
		"pnpm-lock.yaml": "pnpm",
		"go.mod": "Go modules",
		
		"Gemfile": "Bundler",
		"requirements.txt": "pip",
		"pyproject.toml": "Poetry",
		"Podfile": "CocoaPods",
		"Carthage": "Carthage",
	}

	for _, file := range files {
		if pkg, ok := pkgFiles[file]; ok {
			return pkg
		}
	}

	return ""
}

// FormatProjectContext formats project context for the system prompt
func FormatProjectContext(ctx *ProjectContext) string {
	var parts []string

	if ctx.CLETUSMD != "" {
		parts = append(parts, "## Project Instructions (CLETUS.md)\n"+ctx.CLETUSMD)
	}

	if ctx.Language != "" {
		parts = append(parts, "## Project\nLanguage: "+ctx.Language)
	}

	if ctx.BuildTool != "" {
		parts = append(parts, "Build tool: "+ctx.BuildTool)
	}

	if ctx.PackageMgr != "" {
		parts = append(parts, "Package manager: "+ctx.PackageMgr)
	}

	if ctx.HasReadme {
		parts = append(parts, "Has README.md")
	}

	if ctx.HasTests {
		parts = append(parts, "Has test files")
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}
