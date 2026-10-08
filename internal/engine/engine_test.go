package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var sample = "# Title\n\n" +
	"Some *italic*, **bold**, `code` and a [link](http://example.com) in a paragraph that is long enough to wrap " +
	"across several lines so that the line layout code gets exercised properly. " + strings.Repeat("word ", 80) + "\n\n" +
	"## List\n\n- one\n- two\n  - nested\n\n1. first\n2. second\n\n" +
	"> quoted text\n\n---\n\n" +
	"```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\n\n" +
	"| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n\n" +
	"averyveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryverylongunbreakableword\n"

func TestNewUnknownEngine(t *testing.T) {
	if _, err := New("nope"); err == nil {
		t.Error("expected error for unknown engine")
	}
}

func TestNewKnownEngines(t *testing.T) {
	for _, name := range Names {
		e, err := New(name)
		if err != nil {
			t.Fatalf("New(%q): %v", name, err)
		}
		if e.Name() != name {
			t.Errorf("Name() = %q, want %q", e.Name(), name)
		}
	}
}

func renderSample(t *testing.T, name string) {
	t.Helper()
	e, err := New(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	out := filepath.Join(t.TempDir(), "out.pdf")
	if err := e.Render([]byte(sample), out); err != nil {
		t.Fatalf("render with %s: %v", name, err)
	}
	pdf, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Errorf("%s output is not a PDF", name)
	}
}

func TestGoPDFRender(t *testing.T) {
	renderSample(t, GoPDF)
}

func TestWkhtmltopdfRender(t *testing.T) {
	if _, err := exec.LookPath("wkhtmltopdf"); err != nil {
		t.Skip("wkhtmltopdf not installed")
	}
	renderSample(t, Wkhtmltopdf)
}

func TestChromedpRender(t *testing.T) {
	for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if _, err := exec.LookPath(bin); err == nil {
			renderSample(t, Chromedp)
			return
		}
	}
	t.Skip("Chrome/Chromium not installed")
}
