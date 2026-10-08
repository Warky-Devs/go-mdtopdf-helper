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
	flag.BoolVar(&conv.Recursive, "recursive", true, "Scan directories recursively")
	flag.BoolVar(&conv.Parallel, "parallel", true, "Convert files in parallel")
	flag.BoolVar(&hookMode, "hook", false, "Run as git pre-commit hook")
	flag.StringVar(&engineName, "engine", engine.Default,
		"PDF engine: "+strings.Join(engine.Names, ", "))
	flag.StringVar(&formatList, "format", converter.FormatPDF,
		"Output formats, comma separated: "+strings.Join(converter.FormatNames, ", "))
	flag.Parse()

	formats, err := converter.ParseFormats(formatList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	conv.Formats = formats

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
