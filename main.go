package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
)

type Converter struct {
	inputDir  string
	recursive bool
	parallel  bool
	hookMode  bool
}

func init() {
	if err := ensureWkhtmltopdfInPath(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up PATH: %v\n", err)
		os.Exit(1)
	}
}

func ensureWkhtmltopdfInPath() error {
	wkhtmlPath := `C:\Program Files\wkhtmltopdf\bin`

	if _, err := os.Stat(wkhtmlPath); os.IsNotExist(err) {
		return fmt.Errorf("wkhtmltopdf directory not found at: %s", wkhtmlPath)
	}

	currentPath := os.Getenv("PATH")

	if strings.Contains(currentPath, wkhtmlPath) {
		return nil // Already in PATH
	}

	newPath := currentPath + ";" + wkhtmlPath
	if err := os.Setenv("PATH", newPath); err != nil {
		return fmt.Errorf("failed to update PATH: %w", err)
	}

	fmt.Printf("Added wkhtmltopdf to PATH: %s\n", wkhtmlPath)
	return nil
}

func main() {
	conv := &Converter{}

	// Parse command line flags
	flag.StringVar(&conv.inputDir, "dir", ".", "Directory to scan for markdown files")
	flag.BoolVar(&conv.recursive, "recursive", true, "Scan directories recursively")
	flag.BoolVar(&conv.parallel, "parallel", true, "Convert files in parallel")
	flag.BoolVar(&conv.hookMode, "hook", false, "Run as git pre-commit hook")
	flag.Parse()

	if conv.hookMode {
		if err := conv.runAsHook(); err != nil {
			fmt.Fprintf(os.Stderr, "Hook error: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := conv.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func (c *Converter) runAsHook() error {
	// Ask user if they want to proceed
	if !c.confirmConversion() {
		fmt.Println("Skipping PDF conversion")
		return nil
	}

	// Get staged markdown files
	files, err := c.getStagedMarkdownFiles()
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(files) == 0 {
		return nil
	}

	// Convert files
	if err := c.convertFiles(files); err != nil {
		return err
	}

	// Stage generated PDFs
	return c.stageGeneratedPDFs(files)
}

func (c *Converter) confirmConversion() bool {
	fmt.Print("Convert Markdown files to PDF? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "" || response == "y" || response == "yes"
}

func (c *Converter) getStagedMarkdownFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=d")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		file := scanner.Text()
		if strings.HasSuffix(file, ".md") || strings.HasSuffix(file, ".markdown") {
			files = append(files, file)
		}
	}

	return files, scanner.Err()
}

func (c *Converter) stageGeneratedPDFs(files []string) error {
	for _, file := range files {
		pdfFile := strings.TrimSuffix(file, filepath.Ext(file)) + ".pdf"
		cmd := exec.Command("git", "add", pdfFile)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Could not stage %s\n", pdfFile)
		}
	}
	return nil
}

func (c *Converter) Run() error {
	files, err := c.findMarkdownFiles()
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No markdown files found")
		return nil
	}

	return c.convertFiles(files)
}

func (c *Converter) convertFiles(files []string) error {
	if c.parallel {
		return c.convertFilesParallel(files)
	}
	return c.convertFilesSerial(files)
}

func (c *Converter) findMarkdownFiles() ([]string, error) {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && !c.recursive && path != c.inputDir {
			return filepath.SkipDir
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".markdown")) {
			files = append(files, path)
		}

		return nil
	}

	if err := filepath.Walk(c.inputDir, walkFn); err != nil {
		return nil, err
	}

	return files, nil
}

func (c *Converter) convertFilesParallel(files []string) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(files))

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			if err := c.convertFile(f); err != nil {
				errors <- fmt.Errorf("failed to convert %s: %w", f, err)
			}
		}(file)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			return err
		}
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

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	html := markdown.ToHTML(mdContent, p, nil)

	outputFile := strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + ".pdf"

	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return fmt.Errorf("failed to create PDF generator: %w", err)
	}

	page := wkhtmltopdf.NewPageReader(strings.NewReader(string(html)))
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

	fmt.Printf("Successfully converted %s to %s\n", inputFile, outputFile)
	return nil
}
