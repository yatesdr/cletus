package vision

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Processor handles image preprocessing
type Processor struct {
	mode    string
	ocrPath string
}

// Mode constants
const (
	ModeNative = "native" // Send base64 directly
	ModeOCR   = "ocr"     // Extract text via OCR
	ModeModel = "model"   // Send to vision model
)

// NewProcessor creates a new vision processor
func NewProcessor(mode string, ocrPath string) *Processor {
	if mode == "" {
		mode = ModeNative
	}
	return &Processor{
		mode:    mode,
		ocrPath: ocrPath,
	}
}

// Process processes an image and returns content for the model
func (p *Processor) Process(imagePath string) (string, error) {
	switch p.mode {
	case ModeOCR:
		return p.processOCR(imagePath)
	case ModeNative:
		return p.processNative(imagePath)
	case ModeModel:
		// Returns base64 for sending to vision-capable model
		return p.processNative(imagePath)
	default:
		return p.processNative(imagePath)
	}
}

// processNative returns base64 encoded image
func (p *Processor) processNative(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	// Detect mime type
	ext := strings.ToLower(imagePath[strings.LastIndex(imagePath, "."):])
	mediaType := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".gif":
		mediaType = "image/gif"
	case ".webp":
		mediaType = "image/webp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mediaType, encoded), nil
}

// processOCR extracts text using tesseract
func (p *Processor) processOCR(imagePath string) (string, error) {
	// Try to find tesseract executable
	ocrPath, err := exec.LookPath("tesseract")
	if err != nil {
		// Try default paths
		defaultPaths := []string{
			"/usr/local/bin/tesseract",
			"/usr/bin/tesseract",
			"/opt/homebrew/bin/tesseract",
		}
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				ocrPath = path
				break
			}
		}
		if ocrPath == "" {
			return "", fmt.Errorf("tesseract not found. Install with: brew install tesseract")
		}
	}

	// Try to run tesseract
	cmd := exec.Command(ocrPath, imagePath, "stdout")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's a permission or dependency issue
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tesseract failed to process image: %w", err)
		}
		return "", fmt.Errorf("tesseract execution error: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("tesseract returned empty text (image may not contain readable text)")
	}

	// Limit output to prevent huge responses
	lines := strings.Split(result, "\n")
	maxLines := 500
	if len(lines) > maxLines {
		result = strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n... %d more lines", len(lines)-maxLines)
	}

	return result, nil
}

// IsSupported checks if an image format is supported
func (p *Processor) IsSupported(imagePath string) bool {
	ext := strings.ToLower(imagePath[strings.LastIndex(imagePath, "."):])
	supported := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	for _, s := range supported {
		if ext == s {
			return true
		}
	}
	return false
}

// GetMode returns the current processing mode
func (p *Processor) GetMode() string {
	return p.mode
}

// SetMode sets the processing mode
func (p *Processor) SetMode(mode string) {
	p.mode = mode
}
