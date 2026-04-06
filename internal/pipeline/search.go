package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SearchProvider is the interface for web search providers
type SearchProvider interface {
	Search(ctx context.Context, query string, numResults int) ([]SearchResult, error)
	Name() string
}

// SearchResult represents a search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// TavilyProvider uses Tavily API for web search
type TavilyProvider struct {
	apiKey string
	client *http.Client
}

// NewTavilyProvider creates a new Tavily search provider
func NewTavilyProvider(apiKey string) *TavilyProvider {
	return &TavilyProvider{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs a web search using Tavily
func (t *TavilyProvider) Search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("Tavily API key not configured")
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"query":               query,
		"max_results":         numResults,
		"include_answer":      true,
		"include_raw_content": false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(reqBody)), nil
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tavily API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	searchResults := make([]SearchResult, len(result.Results))
	for i, r := range result.Results {
		searchResults[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
		}
	}

	return searchResults, nil
}

// Name returns the provider name
func (t *TavilyProvider) Name() string {
	return "Tavily"
}

// BraveProvider uses Brave Search API for web search
type BraveProvider struct {
	apiKey string
	client *http.Client
}

// NewBraveProvider creates a new Brave search provider
func NewBraveProvider(apiKey string) *BraveProvider {
	return &BraveProvider{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs a web search using Brave
func (b *BraveProvider) Search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("Brave API key not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.search.brave.com/res/v1/web/search", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	q := req.URL.Query()
	q.Add("q", query)
	q.Add("count", fmt.Sprintf("%d", numResults))
	req.URL.RawQuery = q.Encode()

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Brave API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	searchResults := make([]SearchResult, len(result.Web.Results))
	for i, r := range result.Web.Results {
		searchResults[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Description,
		}
	}

	return searchResults, nil
}

// Name returns the provider name
func (b *BraveProvider) Name() string {
	return "Brave"
}

// SearchPipeline coordinates search operations
type SearchPipeline struct {
	provider SearchProvider
	cache    *Cache
}

// NewSearchPipeline creates a new search pipeline
func NewSearchPipeline(provider SearchProvider, cacheSize int) *SearchPipeline {
	return &SearchPipeline{
		provider: provider,
		cache:    NewCache(cacheSize),
	}
}

// Search performs a web search with caching
func (p *SearchPipeline) Search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d", query, numResults)
	if results, ok := p.cache.Get(cacheKey); ok {
		return results.([]SearchResult), nil
	}

	results, err := p.provider.Search(ctx, query, numResults)
	if err != nil {
		return nil, err
	}

	// Cache the results
	p.cache.Put(cacheKey, results)

	return results, nil
}

// GetProvider returns the underlying search provider
func (p *SearchPipeline) GetProvider() SearchProvider {
	return p.provider
}
