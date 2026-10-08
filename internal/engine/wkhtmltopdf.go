package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
)

type wkhtmltopdfEngine struct {
	pathOnce sync.Once
	pathErr  error
}

func (e *wkhtmltopdfEngine) Name() string { return Wkhtmltopdf }

func (e *wkhtmltopdfEngine) Close() error { return nil }

func (e *wkhtmltopdfEngine) Render(md []byte, outputFile string) error {
	e.pathOnce.Do(func() { e.pathErr = ensureWkhtmltopdfInPath() })
	if e.pathErr != nil {
		return e.pathErr
	}

	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return fmt.Errorf("failed to create PDF generator: %w", err)
	}

	page := wkhtmltopdf.NewPageReader(strings.NewReader(string(markdownToHTML(md))))
	page.EnableLocalFileAccess.Set(true)
	pdfg.AddPage(page)

	pdfg.Dpi.Set(300)
	pdfg.MarginTop.Set(15)
	pdfg.MarginBottom.Set(15)
	pdfg.MarginLeft.Set(15)
	pdfg.MarginRight.Set(15)

	if err := pdfg.Create(); err != nil {
		return fmt.Errorf("failed to create PDF: %w", err)
	}
	if err := pdfg.WriteFile(outputFile); err != nil {
		return fmt.Errorf("failed to write PDF file: %w", err)
	}
	return nil
}

func ensureWkhtmltopdfInPath() error {
	var wkhtmlPath string

	switch runtime.GOOS {
	case "windows":
		wkhtmlPath = `C:\Program Files\wkhtmltopdf\bin`
	case "linux":
		wkhtmlPath = findInDirs("wkhtmltopdf", "/usr/local/bin", "/usr/bin", "/opt/wkhtmltopdf/bin")
	case "darwin":
		wkhtmlPath = findInDirs("wkhtmltopdf", "/usr/local/bin", "/opt/homebrew/bin", "/opt/local/bin")
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if wkhtmlPath == "" {
		return fmt.Errorf("wkhtmltopdf not found in common installation paths")
	}
	if _, err := os.Stat(wkhtmlPath); os.IsNotExist(err) {
		return fmt.Errorf("wkhtmltopdf directory not found at: %s", wkhtmlPath)
	}

	currentPath := os.Getenv("PATH")
	if strings.Contains(currentPath, wkhtmlPath) {
		return nil
	}

	newPath := currentPath + string(os.PathListSeparator) + wkhtmlPath
	if err := os.Setenv("PATH", newPath); err != nil {
		return fmt.Errorf("failed to update PATH: %w", err)
	}

	fmt.Printf("Added wkhtmltopdf to PATH: %s\n", wkhtmlPath)
	return nil
}

func findInDirs(binary string, dirs ...string) string {
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, binary)); err == nil {
			return dir
		}
	}
	return ""
}
