package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cletus/internal/pipeline"
	"cletus/internal/tools"
)

// WebSearchTool searches the web
type WebSearchTool struct {
	tools.BaseTool
	httpClient *http.Client
	provider   pipeline.SearchProvider // nil means use DuckDuckGo fallback
}

// Input represents the tool input
type Input struct {
	Query string `json:"query"`
}

// Output represents the tool output
type Output struct {
	Results []SearchResult `json:"results"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// NewWebSearchTool creates a new WebSearchTool with optional search provider
func NewWebSearchTool(searchKey string) *WebSearchTool {
	var provider pipeline.SearchProvider
	if searchKey != "" {
		if strings.HasPrefix(searchKey, "tvly-") {
			provider = pipeline.NewTavilyProvider(searchKey)
		} else if strings.HasPrefix(searchKey, "BSA") {
			provider = pipeline.NewBraveProvider(searchKey)
		}
	}

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query"
			}
		},
		"required": ["query"]
	}`)

	return &WebSearchTool{
		BaseTool:   tools.NewBaseTool("WebSearch", "Search the web for information", schema),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		provider:   provider,
	}
}

// Execute performs a web search
func (t *WebSearchTool) Execute(ctx context.Context, input json.RawMessage, progress chan<- tools.ToolProgress) (string, error) {
	var parsed Input
	if err := json.Unmarshal(input, &parsed); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Try the configured provider first
	if t.provider != nil {
		results, err := t.provider.Search(ctx, parsed.Query, 5)
		if err == nil {
			// Convert pipeline.SearchResult to our SearchResult
			output := Output{}
			for _, r := range results {
				output.Results = append(output.Results, SearchResult{
					Title:   r.Title,
					URL:     r.URL,
					Snippet: r.Content,
				})
			}
			outputJSON, _ := json.Marshal(output)
			return string(outputJSON), nil
		}
		// Fall through to DuckDuckGo if provider fails
	}

	// Use DuckDuckGo HTML search (fallback)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(parsed.Query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Cletus/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform search: %w", err)
	}
	defer resp.Body.Close()

	// Read body first
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	// Simple HTML parsing to extract results
	results := t.parseResults(body)

	output := Output{Results: results}
	outputJSON, _ := json.Marshal(output)
	return string(outputJSON), nil
}

// parseResults extracts search results from HTML
func (t *WebSearchTool) parseResults(body []byte) []SearchResult {
	var results []SearchResult
	html := string(body)

	// Simple regex-based parsing for DuckDuckGo results
	// Look for result blocks
	resultStart := strings.Index(html, `class="result__body"`)
	for resultStart != -1 {
		// Extract title and URL
		titleStart := strings.Index(html[resultStart:], `class="result__a"`)
		if titleStart == -1 {
			break
		}
		titleStart += resultStart

		// Find link
		hrefStart := strings.Index(html[titleStart:], `href="`)
		if hrefStart == -1 {
			break
		}
		hrefStart += titleStart + 6
		hrefEnd := strings.Index(html[hrefStart:], `"`)
		if hrefEnd == -1 {
			break
		}
		link := html[hrefStart : hrefStart+hrefEnd]

		// Find title text
		titleTextStart := strings.Index(html[titleStart+hrefStart-titleStart:], ">")
		if titleTextStart == -1 {
			break
		}
		titleTextStart += titleStart + hrefStart - titleStart + 1
		titleTextEnd := strings.Index(html[titleTextStart:], "<")
		if titleTextEnd == -1 {
			break
		}
		title := html[titleTextStart : titleTextStart+titleTextEnd]

		results = append(results, SearchResult{
			Title: title,
			URL:   link,
		})

		resultStart = strings.Index(html[resultStart+1:], `class="result__body"`)
	}

	// Fallback: if no results parsed, return a simple message
	if len(results) == 0 {
		maxLen := 100
		if len(html) < maxLen {
			maxLen = len(html)
		}
		return []SearchResult{
			{Title: "Search performed for: " + html[:maxLen], URL: ""},
		}
	}

	return results
}

// Schema returns the tool schema
func (t *WebSearchTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "WebSearch",
		Description: t.BaseTool.Description(),
		InputSchema: t.BaseTool.InputSchema(),
	}
}

// IsReadOnly returns true
func (t *WebSearchTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe returns true
func (t *WebSearchTool) IsConcurrencySafe() bool {
	return true
}
