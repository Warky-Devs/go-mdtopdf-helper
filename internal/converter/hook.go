package converter

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunAsHook converts the staged Markdown files and stages the generated output.
func (c *Converter) RunAsHook() error {
	if c.InputFile != "" || c.Output != "" {
		return fmt.Errorf("-input and -output cannot be combined with -hook")
	}

	if !confirmConversion() {
		fmt.Println("Skipping conversion")
		return nil
	}

	files, err := stagedMarkdownFiles()
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(files) == 0 {
		return nil
	}

	if err := c.ConvertFiles(files); err != nil {
		return err
	}

	return c.stageGeneratedFiles(files)
}

func confirmConversion() bool {
	fmt.Print("Convert Markdown files? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "" || response == "y" || response == "yes"
}

func stagedMarkdownFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=d")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if file := scanner.Text(); isMarkdown(file) {
			files = append(files, file)
		}
	}

	return files, scanner.Err()
}

func (c *Converter) stageGeneratedFiles(files []string) error {
	for _, file := range files {
		for _, format := range c.formats() {
			out := filepath.FromSlash(c.outputFor(file, format))
			if err := exec.Command("git", "add", out).Run(); err != nil {
				fmt.Printf("Warning: Could not stage %s\n", out)
			}
		}
	}
	return nil
}
