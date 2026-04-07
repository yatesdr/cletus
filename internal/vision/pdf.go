package vision

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PDFProcessor handles PDF text extraction
type PDFProcessor struct {
	pdftotextPath string
}

// NewPDFProcessor creates a new PDF processor
func NewPDFProcessor(pdftotextPath string) *PDFProcessor {
	if pdftotextPath == "" {
		pdftotextPath = "/usr/local/bin/pdftotext" // Default macOS path
	}
	return &PDFProcessor{
		pdftotextPath: pdftotextPath,
	}
}

// ExtractText extracts text from a PDF
func (p *PDFProcessor) ExtractText(pdfPath string) (string, error) {
	// Check if pdftotext is available
	if _, err := os.Stat(p.pdftotextPath); err != nil {
		return "", fmt.Errorf("pdftotext not found at %s: %w", p.pdftotextPath, err)
	}

	// Create temp output file
	tmpFile := pdfPath + ".txt"
	defer os.Remove(tmpFile)

	// Run pdftotext
	cmd := exec.Command(p.pdftotextPath, "-layout", pdfPath, tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w - %s", err, string(output))
	}

	// Read extracted text
	text, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("read extracted text: %w", err)
	}

	return string(text), nil
}

// ExtractPageRange extracts text from specific pages
func (p *PDFProcessor) ExtractPageRange(pdfPath string, startPage, endPage int) (string, error) {
	if startPage < 1 {
		startPage = 1
	}
	if endPage < startPage {
		endPage = startPage
	}

	// Use pdftotext with page range
	tmpFile := pdfPath + ".txt"
	defer os.Remove(tmpFile)

	cmd := exec.Command(p.pdftotextPath, "-f", strconv.Itoa(startPage), "-l", strconv.Itoa(endPage), "-layout", pdfPath, tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w - %s", err, string(output))
	}

	text, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("read extracted text: %w", err)
	}

	return string(text), nil
}

// GetPageCount returns the number of pages in a PDF
func (p *PDFProcessor) GetPageCount(pdfPath string) (int, error) {
	// Use pdfinfo if available
	pdfinfoPath := strings.Replace(p.pdftotextPath, "pdftotext", "pdfinfo", 1)
	
	if _, err := os.Stat(pdfinfoPath); err != nil {
		// Fallback: estimate from text extraction
		text, err := p.ExtractText(pdfPath)
		if err != nil {
			return 0, err
		}
		// Rough estimate: count form feeds
		return strings.Count(text, "\f") + 1, nil
	}

	cmd := exec.Command(pdfinfoPath, pdfPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo failed: %w", err)
	}

	// Parse "Pages: N" from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Pages:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pages, err := strconv.Atoi(fields[1])
				if err == nil {
					return pages, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("could not determine page count")
}

// IsPDF checks if a file is a PDF
func IsPDF(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".pdf")
}
