package converter

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/engine"
)

func TestRunConvertsMarkdownFiles(t *testing.T) {
	tmpDir := t.TempDir()

	content := "# Test Document\nThis is a basic test."
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	eng, err := engine.New(engine.Default)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eng.Close() }()

	conv := &Converter{InputDir: tmpDir, Engine: eng}
	if err := conv.Run(); err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	pdf, err := os.ReadFile(filepath.Join(tmpDir, "test.pdf"))
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Error("output is not a PDF")
	}
}

func TestRunNonRecursiveSkipsSubdirs(t *testing.T) {
	tmpDir := t.TempDir()
	sub := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(tmpDir, "a.md"), filepath.Join(sub, "b.md")} {
		if err := os.WriteFile(p, []byte("# hi"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	eng, _ := engine.New(engine.Default)
	conv := &Converter{InputDir: tmpDir, Engine: eng}
	if err := conv.Run(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "a.pdf")); err != nil {
		t.Errorf("expected a.pdf: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, "b.pdf")); err == nil {
		t.Error("b.pdf should not exist when not recursive")
	}
}

func TestParseFormats(t *testing.T) {
	got, err := ParseFormats("PDF, html,pdf,docx")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{FormatPDF, FormatHTML, FormatDOCX}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"", ",", "odt", "pdf,rtf"} {
		if _, err := ParseFormats(bad); err == nil {
			t.Errorf("ParseFormats(%q) should fail", bad)
		}
	}
}

func TestRunWritesAllFormats(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte("# Hi\n\ntext"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No engine needed when PDF is not requested.
	conv := &Converter{InputDir: tmpDir, Formats: []string{FormatHTML, FormatDOCX}}
	if err := conv.Run(); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{"html", "docx"} {
		if _, err := os.Stat(filepath.Join(tmpDir, "doc."+ext)); err != nil {
			t.Errorf("expected doc.%s: %v", ext, err)
		}
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "doc.pdf")); err == nil {
		t.Error("doc.pdf should not be written")
	}
}
