package vision

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// OCRProcessor handles OCR using tesseract
type OCRProcessor struct {
	tesseractPath string
	language      string
}

// NewOCRProcessor creates a new OCR processor
func NewOCRProcessor(tesseractPath, language string) *OCRProcessor {
	if tesseractPath == "" {
		tesseractPath = "/usr/local/bin/tesseract"
	}
	if language == "" {
		language = "eng"
	}
	return &OCRProcessor{
		tesseractPath: tesseractPath,
		language:      language,
	}
}

// Process performs OCR on an image file
func (o *OCRProcessor) Process(imagePath string) (string, error) {
	// Check if tesseract exists
	if _, err := os.Stat(o.tesseractPath); err != nil {
		return "", fmt.Errorf("tesseract not found at %s: %w", o.tesseractPath, err)
	}

	// Create temp output file
	tmpFile := filepath.Join(os.TempDir(), "ocr_output.txt")
	defer os.Remove(tmpFile)

	// Run tesseract
	cmd := exec.Command(o.tesseractPath, "-l", o.language, imagePath, filepath.Base(tmpFile))
	cmd.Dir = filepath.Dir(tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract failed: %w - %s", err, string(output))
	}

	// Read extracted text
	text, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("read OCR output: %w", err)
	}

	return string(text), nil
}

// ProcessPDF performs OCR on PDF pages
func (o *OCRProcessor) ProcessPDF(pdfPath string, pages string) (string, error) {
	// For PDFs, first extract images, then OCR
	// This is a simplified version - full implementation would use pdfimages
	result, err := o.Process(pdfPath)
	return result, err
}

// AvailableLanguages returns available OCR languages
func (o *OCRProcessor) AvailableLanguages() []string {
	// Check tesseract --list-languages
	cmd := exec.Command(o.tesseractPath, "--list-languages")
	output, err := cmd.Output()
	if err != nil {
		return []string{"eng"}
	}
	
	var langs []string
	for _, line := range splitLines(string(output)) {
		if line != "" && line != "Tesseract" {
			langs = append(langs, line)
		}
	}
	return langs
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
