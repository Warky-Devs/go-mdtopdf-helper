// Package converter finds Markdown files and converts them to PDF, HTML or DOCX.
package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/document"
	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/engine"
)

// Output formats accepted by ParseFormats.
const (
	FormatPDF  = "pdf"
	FormatHTML = "html"
	FormatDOCX = "docx"
)

// FormatNames lists every supported output format.
var FormatNames = []string{FormatPDF, FormatHTML, FormatDOCX}

// ParseFormats turns a comma separated list such as "pdf,html" into validated, de-duplicated formats.
func ParseFormats(list string) ([]string, error) {
	var formats []string
	seen := map[string]bool{}
	for _, f := range strings.Split(list, ",") {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" || seen[f] {
			continue
		}
		if !slices.Contains(FormatNames, f) {
			return nil, fmt.Errorf("unknown format %q (available: %s)", f, strings.Join(FormatNames, ", "))
		}
		seen[f] = true
		formats = append(formats, f)
	}
	if len(formats) == 0 {
		return nil, fmt.Errorf("no output format given (available: %s)", strings.Join(FormatNames, ", "))
	}
	return formats, nil
}

// Converter converts Markdown files found under InputDir into each of Formats.
// Engine renders the PDF format and may be nil when PDF is not requested.
type Converter struct {
	InputDir  string
	Recursive bool
	Parallel  bool
	Formats   []string
	Engine    engine.Engine
}

// outputPath returns the file written for inputFile in the given format.
func outputPath(inputFile, format string) string {
	return strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + "." + format
}

// Run converts every Markdown file under InputDir.
func (c *Converter) Run() error {
	files, err := c.findMarkdownFiles()
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No markdown files found")
		return nil
	}

	return c.ConvertFiles(files)
}

// ConvertFiles converts the given Markdown files, in parallel when enabled.
func (c *Converter) ConvertFiles(files []string) error {
	if c.Parallel {
		return c.convertFilesParallel(files)
	}
	return c.convertFilesSerial(files)
}

func isMarkdown(path string) bool {
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".markdown")
}

func (c *Converter) findMarkdownFiles() ([]string, error) {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && !c.Recursive && path != c.InputDir {
			return filepath.SkipDir
		}

		if !info.IsDir() && isMarkdown(path) {
			files = append(files, path)
		}

		return nil
	}

	if err := filepath.Walk(c.InputDir, walkFn); err != nil {
		return nil, err
	}

	return files, nil
}

func (c *Converter) convertFilesParallel(files []string) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(files))

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			if err := c.convertFile(f); err != nil {
				errs <- fmt.Errorf("failed to convert %s: %w", f, err)
			}
		}(file)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

func (c *Converter) convertFilesSerial(files []string) error {
	for _, file := range files {
		if err := c.convertFile(file); err != nil {
			return fmt.Errorf("failed to convert %s: %w", file, err)
		}
	}
	return nil
}

func (c *Converter) convertFile(inputFile string) error {
	mdContent, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	formats := c.Formats
	if len(formats) == 0 {
		formats = []string{FormatPDF}
	}

	for _, format := range formats {
		outputFile := outputPath(inputFile, format)
		if err := c.write(mdContent, format, outputFile); err != nil {
			return fmt.Errorf("%s: %w", format, err)
		}
		fmt.Printf("Successfully converted %s to %s\n", inputFile, outputFile)
	}
	return nil
}

func (c *Converter) write(md []byte, format, outputFile string) error {
	switch format {
	case FormatPDF:
		if c.Engine == nil {
			return fmt.Errorf("no PDF engine configured")
		}
		return c.Engine.Render(md, outputFile)
	case FormatHTML:
		return os.WriteFile(outputFile, document.HTML(md), 0o644)
	case FormatDOCX:
		data, err := document.DOCX(md)
		if err != nil {
			return err
		}
		return os.WriteFile(outputFile, data, 0o644)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}
