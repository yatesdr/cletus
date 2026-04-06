package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"cletus/internal/api"
)

// VisionProcessor handles image description using vision-capable models
type VisionProcessor struct {
	client api.LLMClient
	cache  *Cache
	prompt string
}

// NewVisionProcessor creates a new vision processor
func NewVisionProcessor(client api.LLMClient, cacheSize int) *VisionProcessor {
	prompt := `Describe this image in detail. Focus on:
- What the image contains (objects, people, text, etc.)
- The visual style and composition
- Any notable features or details

Provide a concise but comprehensive description.`

	cache := NewCache(cacheSize)
	cache.SetEvictCallback(func(key, value any) {
		// Could add cleanup here if needed
	})

	return &VisionProcessor{
		client: client,
		cache:  cache,
		prompt: prompt,
	}
}

// DescribeImage returns a description of an image file
func (v *VisionProcessor) DescribeImage(ctx context.Context, imagePath string) (string, error) {
	// Check cache first
	if desc, ok := v.cache.Get(imagePath); ok {
		return desc.(string), nil
	}

	// Read image file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	// Determine media type
	mediaType := guessMediaType(imagePath)

	// Encode as base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// Build request with image content
	messages := []api.APIMessage{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: "text", Text: v.prompt},
				{
					Type: "image",
					Source: &api.ImageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      encoded,
					},
				},
			},
		},
	}

	req := &api.Request{
		Messages:  messages,
		MaxTokens: 1024,
		Stream:    false,
	}

	resp, err := v.client.Send(ctx, req)
	if err != nil {
		return "", fmt.Errorf("vision API call: %w", err)
	}

	// Extract description from response
	var description string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			description = block.Text
			break
		}
	}

	if description == "" {
		return "", fmt.Errorf("no description in response")
	}

	// Cache the result
	v.cache.Put(imagePath, description)

	return description, nil
}

// DescribeImageURL describes an image from a URL
func (v *VisionProcessor) DescribeImageURL(ctx context.Context, imageURL string) (string, error) {
	messages := []api.APIMessage{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: "text", Text: v.prompt},
			},
		},
	}

	// For URL-based images, we'd need to modify the request format
	// This is a placeholder for URL-based image description
	req := &api.Request{
		Messages:  messages,
		MaxTokens: 1024,
		Stream:    false,
	}

	resp, err := v.client.Send(ctx, req)
	if err != nil {
		return "", fmt.Errorf("vision API call: %w", err)
	}

	var description string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			description = block.Text
			break
		}
	}

	return description, nil
}

// guessMediaType determines the media type from file extension
func guessMediaType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}
