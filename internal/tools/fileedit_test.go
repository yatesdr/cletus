package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileEditExactMatch(t *testing.T) {
	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("func hello() {\n\treturn\n}"), 0644)

	tool := NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":  path,
		"old_string": "return",
		"new_string": "return 42",
	})

	progress := make(chan ToolProgress, 10)
	_, err := tool.Execute(context.Background(), input, progress)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(path)
	expected := "func hello() {\n\treturn 42\n}"
	if string(content) != expected {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestFileEditReplaceAll(t *testing.T) {
	// Create temp file with multiple occurrences
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("foo bar foo bar foo"), 0644)

	tool := NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":   path,
		"old_string":  "foo",
		"new_string":  "baz",
		"replace_all": true,
	})

	progress := make(chan ToolProgress, 10)
	result, err := tool.Execute(context.Background(), input, progress)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(path)
	expected := "baz bar baz bar baz"
	if string(content) != expected {
		t.Errorf("unexpected content: %s", string(content))
	}

	if !contains(result, "3 replacements") {
		t.Errorf("expected replacement count in result, got: %s", result)
	}
}

func TestFileEditNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	tool := NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":  path,
		"old_string": "not found",
		"new_string": "replacement",
	})

	progress := make(chan ToolProgress, 10)
	_, err := tool.Execute(context.Background(), input, progress)
	if err == nil {
		t.Error("expected error for not found string")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					len(s) > len(substr)+1 && contains(s[1:], substr)))
}
