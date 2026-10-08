// Package tomd converts PDF, HTML and DOCX documents back into Markdown.
package tomd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Input formats accepted by ParseFormats, in order of preference when several files share a name.
const (
	FormatDOCX = "docx"
	FormatHTML = "html"
	FormatPDF  = "pdf"
)

// FormatNames lists every supported input format.
var FormatNames = []string{FormatDOCX, FormatHTML, FormatPDF}

// ParseFormats turns a comma separated list such as "html,docx" into validated formats.
func ParseFormats(list string) ([]string, error) {
	var formats []string
	for _, f := range strings.Split(list, ",") {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" || slices.Contains(formats, f) {
			continue
		}
		if !slices.Contains(FormatNames, f) {
			return nil, fmt.Errorf("unknown format %q (available: %s)", f, strings.Join(FormatNames, ", "))
		}
		formats = append(formats, f)
	}
	if len(formats) == 0 {
		return nil, fmt.Errorf("no input format given (available: %s)", strings.Join(FormatNames, ", "))
	}
	return formats, nil
}

// formatOf returns the format of path from its extension, or "" when it is not a supported input.
func formatOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx":
		return FormatDOCX
	case ".html", ".htm":
		return FormatHTML
	case ".pdf":
		return FormatPDF
	}
	return ""
}

func isMarkdownName(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// OutputPath returns the Markdown file written next to input.
func OutputPath(input string) string {
	return strings.TrimSuffix(input, filepath.Ext(input)) + ".md"
}

// Convert converts the file at path to Markdown according to its extension.
func Convert(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	switch formatOf(path) {
	case FormatDOCX:
		return DOCX(data)
	case FormatHTML:
		return HTML(data)
	case FormatPDF:
		return PDF(data)
	default:
		return nil, fmt.Errorf("unsupported file type %q", filepath.Ext(path))
	}
}

// Runner converts documents to Markdown.
//
// When InputFile is set only that file is converted (regardless of Formats) and Output, if given,
// is the name of the Markdown file to write; ".md" is added unless it already ends in .md or
// .markdown. Otherwise every matching document under InputDir is converted, and Output, if given,
// is a directory that receives the results, mirroring the layout under InputDir. With neither,
// each Markdown file is written next to its source.
type Runner struct {
	InputDir  string
	Recursive bool
	Parallel  bool
	InputFile string
	Output    string
	// Force overwrites Markdown files that already exist; otherwise they are skipped.
	Force   bool
	Formats []string
}

// Validate checks the input and output options before any work is done.
func (r *Runner) Validate() error {
	if r.InputFile != "" {
		info, err := os.Stat(r.InputFile)
		if err != nil {
			return fmt.Errorf("input file: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf("input %s is a directory; use -dir to scan a directory", r.InputFile)
		}
		if formatOf(r.InputFile) == "" {
			return fmt.Errorf("input %s must be a .pdf, .html, .htm or .docx file", r.InputFile)
		}
	}

	if r.Output == "" {
		return nil
	}

	info, err := os.Stat(r.Output)
	exists := err == nil
	if r.InputFile == "" {
		if exists && !info.IsDir() {
			return fmt.Errorf("output %s exists and is not a directory", r.Output)
		}
		return nil
	}

	if exists && info.IsDir() {
		return fmt.Errorf("output %s is a directory; give a file name when using -input", r.Output)
	}
	if formatOf(r.Output) != "" {
		return fmt.Errorf("output %s must be a Markdown file, not a %s file", r.Output, filepath.Ext(r.Output))
	}
	return nil
}

// outputFor returns the Markdown file written for input.
func (r *Runner) outputFor(input string) string {
	switch {
	case r.InputFile != "" && r.Output != "":
		if isMarkdownName(r.Output) {
			return r.Output
		}
		return r.Output + ".md"
	case r.InputFile == "" && r.Output != "":
		rel, err := filepath.Rel(r.InputDir, input)
		if err != nil {
			rel = filepath.Base(input)
		}
		return filepath.Join(r.Output, OutputPath(rel))
	default:
		return OutputPath(input)
	}
}

// Run converts InputFile, or all matching documents under InputDir.
func (r *Runner) Run() error {
	if err := r.Validate(); err != nil {
		return err
	}

	files, err := r.inputFiles()
	if err != nil {
		return fmt.Errorf("failed to find documents: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No documents found")
		return nil
	}
	return r.convertFiles(files)
}

func (r *Runner) inputFiles() ([]string, error) {
	if r.InputFile != "" {
		return []string{r.InputFile}, nil
	}
	return r.findFiles()
}

func (r *Runner) findFiles() ([]string, error) {
	byOutput := map[string]string{}
	var order []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !r.Recursive && path != r.InputDir {
				return filepath.SkipDir
			}
			return nil
		}

		format := formatOf(path)
		// Skip Word's "~$" owner files, which carry a .docx extension but are not documents.
		if format == "" || !slices.Contains(r.Formats, format) || strings.HasPrefix(info.Name(), "~$") {
			return nil
		}

		out := r.outputFor(path)
		prev, clash := byOutput[out]
		switch {
		case !clash:
			byOutput[out] = path
			order = append(order, out)
		case slices.Index(FormatNames, format) < slices.Index(FormatNames, formatOf(prev)):
			fmt.Printf("Warning: %s and %s both produce %s, using %s\n", prev, path, out, path)
			byOutput[out] = path
		default:
			fmt.Printf("Warning: %s and %s both produce %s, using %s\n", prev, path, out, prev)
		}
		return nil
	}

	if err := filepath.Walk(r.InputDir, walkFn); err != nil {
		return nil, err
	}

	files := make([]string, 0, len(order))
	for _, out := range order {
		files = append(files, byOutput[out])
	}
	return files, nil
}

func (r *Runner) convertFiles(files []string) error {
	if !r.Parallel {
		for _, f := range files {
			if err := r.convertFile(f); err != nil {
				return fmt.Errorf("failed to convert %s: %w", f, err)
			}
		}
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(files))
	for _, f := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			if err := r.convertFile(f); err != nil {
				errs <- fmt.Errorf("failed to convert %s: %w", f, err)
			}
		}(f)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

func (r *Runner) convertFile(input string) error {
	output := r.outputFor(input)
	if _, err := os.Stat(output); err == nil && !r.Force {
		fmt.Printf("Skipping %s: %s already exists (use -force to overwrite)\n", input, output)
		return nil
	}

	md, err := Convert(input)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(output, md, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", output, err)
	}

	fmt.Printf("Successfully converted %s to %s\n", input, output)
	return nil
}
