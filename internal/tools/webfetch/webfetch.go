package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"cletus/internal/tools"
)

// WebFetchTool fetches content from URLs
type WebFetchTool struct {
	tools.BaseTool
	httpClient *http.Client
}

// Input represents the tool input
type Input struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

// Output represents the tool output
type Output struct {
	Bytes      int    `json:"bytes"`
	Code       int    `json:"code"`
	CodeText   string `json:"codeText"`
	Result     string `json:"result"`
	DurationMs int    `json:"durationMs"`
	URL        string `json:"url"`
}

// NewWebFetchTool creates a new WebFetchTool
func NewWebFetchTool() *WebFetchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch content from"
			},
			"prompt": {
				"type": "string",
				"description": "The prompt to run on the fetched content"
			}
		},
		"required": ["url"]
	}`)

	return &WebFetchTool{
		BaseTool:   tools.NewBaseTool("WebFetch", "Fetch and extract content from a URL", schema),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Execute fetches content from a URL
func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	var parsed Input
	if err := json.Unmarshal(input, &parsed); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Validate URL
	if _, err := url.Parse(parsed.URL); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", parsed.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Cletus/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	durationMs := int(time.Since(start).Milliseconds())

	codeText := http.StatusText(resp.StatusCode)

	// Simple content processing
	var result string
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		result = t.processHTML(string(body), parsed.Prompt)
	} else if strings.Contains(contentType, "text/markdown") {
		result = string(body)
	} else {
		result = fmt.Sprintf("Content type: %s\n\n%s", contentType, t.summarizeContent(body, parsed.Prompt))
	}

	output := Output{
		Bytes:      len(body),
		Code:       resp.StatusCode,
		CodeText:   codeText,
		Result:     result,
		DurationMs: durationMs,
		URL:        parsed.URL,
	}

	outputJSON, _ := json.Marshal(output)
	return string(outputJSON), nil
}

// processHTML extracts text from HTML
func (t *WebFetchTool) processHTML(html, prompt string) string {
	// Simple HTML tag removal
	text := html

	// Remove script and style tags along with their content
	text = removeTag(text, "script")
	text = removeTag(text, "style")

	// Remove HTML tags but keep text content
	re := regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, "\n")

	// Clean up whitespace
	text = strings.Join(strings.Fields(text), "\n")

	if prompt != "" {
		text = fmt.Sprintf("Prompt: %s\n\n%s", prompt, text)
	}

	return text
}

func removeTag(html, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>.*?</%s>`, tag, tag))
	return re.ReplaceAllString(html, "")
}

// summarizeContent provides a simple summary
func (t *WebFetchTool) summarizeContent(body []byte, prompt string) string {
	content := string(body)
	maxLen := 10000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n\n[truncated]"
	}
	if prompt != "" {
		content = fmt.Sprintf("Prompt: %s\n\n%s", prompt, content)
	}
	return content
}

// Schema returns the tool schema
func (t *WebFetchTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "WebFetch",
		Description: t.BaseTool.Description(),
		InputSchema: t.BaseTool.InputSchema(),
	}
}

// IsReadOnly returns true
func (t *WebFetchTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe returns true
func (t *WebFetchTool) IsConcurrencySafe() bool {
	return true
}
