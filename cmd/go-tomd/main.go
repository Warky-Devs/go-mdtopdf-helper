// Command go-tomd converts PDF, HTML and DOCX documents to Markdown.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/tomd"
)

func main() {
	os.Exit(run())
}

func run() int {
	r := &tomd.Runner{}
	var formatList string

	flag.StringVar(&r.InputDir, "dir", ".", "Directory to scan for documents")
	flag.StringVar(&r.InputFile, "input", "",
		"Convert only this document (instead of scanning -dir); -format is ignored")
	flag.StringVar(&r.Output, "output", "",
		"With -input: Markdown file name to write (.md added if missing). "+
			"Without -input: directory to write results to instead of next to the sources")
	flag.BoolVar(&r.Recursive, "recursive", true, "Scan directories recursively")
	flag.BoolVar(&r.Parallel, "parallel", true, "Convert files in parallel")
	flag.BoolVar(&r.Force, "force", false, "Overwrite Markdown files that already exist")
	flag.StringVar(&formatList, "format", strings.Join(tomd.FormatNames, ","),
		"Input formats to convert, comma separated: "+strings.Join(tomd.FormatNames, ", "))
	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if r.InputFile != "" && (set["dir"] || set["recursive"]) {
		fmt.Fprintln(os.Stderr, "Error: -input cannot be combined with -dir or -recursive")
		return 2
	}

	formats, err := tomd.ParseFormats(formatList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	r.Formats = formats
	if err := r.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
