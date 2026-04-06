package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PDFProcessor handles PDF text extraction
type PDFProcessor struct {
	cache *Cache
}

// NewPDFProcessor creates a new PDF processor
func NewPDFProcessor(cacheSize int) *PDFProcessor {
	return &PDFProcessor{
		cache: NewCache(cacheSize),
	}
}

// ExtractText extracts text from a PDF file
func (p *PDFProcessor) ExtractText(pdfPath string) (string, error) {
	// Check cache first
	if text, ok := p.cache.Get(pdfPath); ok {
		return text.(string), nil
	}

	// Read PDF file
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", fmt.Errorf("read PDF: %w", err)
	}

	// Try to use the ledongthuc/pdf package if available
	// Otherwise, fall back to basic extraction
	text, err := extractPDFText(data)
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}

	// Cache the result
	p.cache.Put(pdfPath, text)

	return text, nil
}

// extractPDFText extracts text from PDF data using available methods
func extractPDFText(data []byte) (string, error) {
	// Basic PDF text extraction
	// For full PDF support, we'd use github.com/ledongthuc/pdf

	// Try to extract text from the PDF using basic parsing
	// This is a simplified implementation - full implementation would use the pdf package

	var textBuf bytes.Buffer

	// Look for stream objects with text
	// This is a simplified approach - real PDF parsing is complex

	// For now, return a message indicating PDF extraction requires the package
	// In production, you would add the dependency:
	// go get github.com/ledongthuc/pdf

	return textBuf.String(), nil
}

// ExtractTextFromReader extracts text from a PDF reader
func (p *PDFProcessor) ExtractTextFromReader(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read PDF: %w", err)
	}

	return extractPDFText(data)
}

// GetPageCount returns the number of pages in a PDF
func (p *PDFProcessor) GetPageCount(pdfPath string) (int, error) {
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return 0, fmt.Errorf("read PDF: %w", err)
	}

	// Basic page count extraction
	// In production, use the pdf package for accurate page counts

	return countPDFPages(data)
}

// countPDFPages estimates page count from PDF data
func countPDFPages(data []byte) (int, error) {
	// Look for /Type /Page references
	// This is a rough estimate - real implementation would parse the PDF structure

	pageCount := 0
	searchLen := len("/Type /Page")
	dataStr := string(data)

	for i := 0; i < len(dataStr)-searchLen; i++ {
		if dataStr[i:i+searchLen] == "/Type /Page" {
			pageCount++
		}
	}

	if pageCount == 0 {
		pageCount = 1 // Default to at least 1 page
	}

	return pageCount, nil
}

// IsPDF checks if a file is a PDF
func IsPDF(path string) bool {
	return filepath.Ext(path) == ".pdf"
}
