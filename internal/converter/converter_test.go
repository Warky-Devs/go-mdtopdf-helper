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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestInputFileWithOutputName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "# A")
	writeFile(t, filepath.Join(dir, "b.md"), "# B")

	conv := &Converter{
		InputFile: filepath.Join(dir, "a.md"),
		Output:    filepath.Join(dir, "out", "report"),
		Formats:   []string{FormatHTML, FormatDOCX},
	}
	if err := conv.Run(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"out/report.html", "out/report.docx"} {
		if !exists(filepath.Join(dir, want)) {
			t.Errorf("expected %s", want)
		}
	}
	if exists(filepath.Join(dir, "b.html")) || exists(filepath.Join(dir, "a.html")) {
		t.Error("only the output name should be written, and only the input file converted")
	}
}

func TestInputFileOutputWithMatchingExtension(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "# A")

	conv := &Converter{InputFile: filepath.Join(dir, "a.md"), Output: filepath.Join(dir, "x.html"), Formats: []string{FormatHTML}}
	if err := conv.Run(); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dir, "x.html")) || exists(filepath.Join(dir, "x.html.html")) {
		t.Error("an output name that already has the format's extension should be used as is")
	}
}

func TestOutputValidation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "# A")
	writeFile(t, filepath.Join(dir, "notes.txt"), "text")
	md := filepath.Join(dir, "a.md")

	cases := map[string]*Converter{
		"missing input":       {InputFile: filepath.Join(dir, "nope.md")},
		"input is a dir":      {InputFile: dir},
		"input not markdown":  {InputFile: filepath.Join(dir, "notes.txt")},
		"output is a dir":     {InputFile: md, Output: dir},
		"extension mismatch":  {InputFile: md, Output: filepath.Join(dir, "x.docx"), Formats: []string{FormatHTML}},
		"ext with many fmts":  {InputFile: md, Output: filepath.Join(dir, "x.html"), Formats: []string{FormatHTML, FormatDOCX}},
		"scan output is file": {InputDir: dir, Output: md},
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestOutputDirMirrorsLayout(t *testing.T) {
	src, out := t.TempDir(), filepath.Join(t.TempDir(), "dist")
	writeFile(t, filepath.Join(src, "a.md"), "# A")
	writeFile(t, filepath.Join(src, "sub", "b.md"), "# B")

	conv := &Converter{InputDir: src, Recursive: true, Output: out, Formats: []string{FormatHTML}}
	if err := conv.Run(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"a.html", "sub/b.html"} {
		if !exists(filepath.Join(out, want)) {
			t.Errorf("expected %s in output dir", want)
		}
	}
	if exists(filepath.Join(src, "a.html")) {
		t.Error("nothing should be written next to the sources when -output is a directory")
	}
}

func TestHookRejectsInputAndOutput(t *testing.T) {
	if err := (&Converter{InputFile: "a.md"}).RunAsHook(); err == nil {
		t.Error("expected error combining hook with -input")
	}
}
