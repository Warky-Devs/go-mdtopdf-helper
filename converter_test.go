package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBasicCompilation(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "mdtopdf-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test markdown file
	testContent := `# Test Document
This is a basic test.`

	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create and test converter
	conv := &Converter{
		inputDir:  tmpDir,
		recursive: false,
		parallel:  false,
	}

	// Run conversion
	err = conv.Run()
	if err != nil {
		t.Fatalf("Basic conversion failed: %v", err)
	}

	// Check if PDF was created
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("PDF file was not created")
	}
}
