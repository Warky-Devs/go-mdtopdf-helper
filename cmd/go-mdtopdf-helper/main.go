package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/converter"
	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/engine"
)

func main() {
	os.Exit(run())
}

func run() int {
	conv := &converter.Converter{}
	var hookMode bool
	var engineName, formatList string

	flag.StringVar(&conv.InputDir, "dir", ".", "Directory to scan for markdown files")
	flag.StringVar(&conv.InputFile, "input", "",
		"Convert only this markdown file (instead of scanning -dir)")
	flag.StringVar(&conv.Output, "output", "",
		"With -input: output file name, extension added per format (-output report -> report.pdf). "+
			"Without -input: directory to write results to instead of next to the sources")
	flag.BoolVar(&conv.Recursive, "recursive", true, "Scan directories recursively")
	flag.BoolVar(&conv.Parallel, "parallel", true, "Convert files in parallel")
	flag.BoolVar(&hookMode, "hook", false, "Run as git pre-commit hook")
	flag.StringVar(&engineName, "engine", engine.Default,
		"PDF engine: "+strings.Join(engine.Names, ", "))
	flag.StringVar(&formatList, "format", converter.FormatPDF,
		"Output formats, comma separated: "+strings.Join(converter.FormatNames, ", "))
	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if conv.InputFile != "" && (set["dir"] || set["recursive"]) {
		fmt.Fprintln(os.Stderr, "Error: -input cannot be combined with -dir or -recursive")
		return 2
	}

	formats, err := converter.ParseFormats(formatList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	conv.Formats = formats
	if !hookMode {
		if err := conv.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
	}

	// The engine only matters for PDF output.
	if slices.Contains(formats, converter.FormatPDF) {
		eng, err := engine.New(engineName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		defer func() { _ = eng.Close() }()
		conv.Engine = eng
	}

	if hookMode {
		if err := conv.RunAsHook(); err != nil {
			fmt.Fprintf(os.Stderr, "Hook error: %v\n", err)
			return 1
		}
		return 0
	}

	if err := conv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
