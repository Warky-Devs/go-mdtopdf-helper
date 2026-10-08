// Package engine converts Markdown documents to PDF using interchangeable backends.
package engine

import (
	"fmt"
	"strings"
)

// Engine names accepted by New.
const (
	GoPDF       = "gopdf"
	Wkhtmltopdf = "wkhtmltopdf"
	Chromedp    = "chromedp"
)

// Default is the engine used when none is requested. It needs no external binaries.
const Default = GoPDF

// Names lists every supported engine.
var Names = []string{GoPDF, Wkhtmltopdf, Chromedp}

// Engine renders Markdown into a PDF file.
type Engine interface {
	Name() string
	// Render converts the Markdown source and writes the PDF to outputFile.
	Render(markdown []byte, outputFile string) error
	// Close releases resources held by the engine. It is safe to call more than once.
	Close() error
}

// New returns the engine registered under name.
func New(name string) (Engine, error) {
	switch strings.ToLower(name) {
	case GoPDF:
		return &gopdfEngine{}, nil
	case Wkhtmltopdf:
		return &wkhtmltopdfEngine{}, nil
	case Chromedp:
		return &chromedpEngine{}, nil
	default:
		return nil, fmt.Errorf("unknown engine %q (available: %s)", name, strings.Join(Names, ", "))
	}
}
