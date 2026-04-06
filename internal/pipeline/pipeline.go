package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cletus/internal/api"
	"cletus/internal/config"
)

// Pipeline is the main orchestrator for content processing
type Pipeline struct {
	cfg       *config.Config
	vision    *VisionProcessor
	pdf       *PDFProcessor
	search    *SearchPipeline
	llmClient api.LLMClient
}

// NewPipeline creates a new pipeline with all processors
func NewPipeline(cfg *config.Config, llmClient api.LLMClient) *Pipeline {
	p := &Pipeline{
		cfg:       cfg,
		llmClient: llmClient,
	}

	// Initialize vision processor if a vision model is configured
	visionModel := cfg.ResolveModel("vision")
	if visionModel != "" {
		// Create vision client if different from main
		visionClient := llmClient
		if backend := cfg.ResolveBackend(visionModel); backend.BaseURL != cfg.API.BaseURL {
			visionClient = api.NewLLMClient(&api.ClientConfig{
				BaseURL:   backend.BaseURL,
				APIKey:    backend.APIKey,
				Model:     visionModel,
				APIType:   backend.APIType,
				MaxTokens: cfg.MaxTokens,
			})
		}
		p.vision = NewVisionProcessor(visionClient, 100)
	}

	// Initialize PDF processor
	p.pdf = NewPDFProcessor(50)

	// Initialize search if API key is configured
	// Note: Search key would come from config in production
	p.search = nil

	return p
}

// ProcessFile processes a file and returns its content
// For images, returns a vision description
// For PDFs, extracts text
// For other files, returns the raw content
func (p *Pipeline) ProcessFile(ctx context.Context, filePath string) (string, error) {
	ext := filepath.Ext(filePath)

	// Check if it's an image
	if isImage(ext) {
		if p.vision == nil {
			return "", fmt.Errorf("vision not configured")
		}
		return p.vision.DescribeImage(ctx, filePath)
	}

	// Check if it's a PDF
	if ext == ".pdf" {
		return p.pdf.ExtractText(filePath)
	}

	// Default: read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Search performs a web search
func (p *Pipeline) Search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	if p.search == nil {
		return nil, fmt.Errorf("search not configured")
	}
	return p.search.Search(ctx, query, numResults)
}

// Vision returns the vision processor
func (p *Pipeline) Vision() *VisionProcessor {
	return p.vision
}

// PDF returns the PDF processor
func (p *Pipeline) PDF() *PDFProcessor {
	return p.pdf
}

// SearchProvider returns the search pipeline
func (p *Pipeline) SearchProvider() *SearchPipeline {
	return p.search
}

// SetSearchProvider sets a custom search provider
func (p *Pipeline) SetSearchProvider(provider SearchProvider) {
	p.search = NewSearchPipeline(provider, 50)
}

// isImage checks if the extension is an image type
func isImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico", ".tiff", ".tif":
		return true
	default:
		return false
	}
}
