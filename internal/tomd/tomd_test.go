package tomd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/document"
	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/engine"
)

const sample = "# Title\n\nSome **bold**, *italic* and `code` with a [link](http://example.com).\n\n" +
	"## Lists\n\n- one\n- two\n    - nested\n\n1. first\n2. second\n\n" +
	"```\nfunc main() {\n    run()\n}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"

func requireContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

func TestDOCXRoundTrip(t *testing.T) {
	data, err := document.DOCX([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	md, err := DOCX(data)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, string(md),
		"# Title", "## Lists", "**bold**", "*italic*", "`code`", "[link](http://example.com)",
		"- one\n- two\n    - nested", "1. first\n1. second",
		"```\nfunc main() {\n    run()\n}\n```", "| a | b |\n| --- | --- |\n| 1 | 2 |")
}

func TestHTMLRoundTrip(t *testing.T) {
	md, err := HTML(document.HTML([]byte(sample)))
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, string(md),
		"# Title", "## Lists", "**bold**", "*italic*", "`code`", "[link](http://example.com)", "- one", "1. first", "```")
	if strings.Contains(string(md), "font-family") {
		t.Error("stylesheet leaked into Markdown")
	}
}

func TestPDFRoundTrip(t *testing.T) {
	eng, err := engine.New(engine.GoPDF)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "doc.pdf")
	if err := eng.Render([]byte(sample), out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	md, err := PDF(data)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, string(md),
		"# Title", "## Lists", "**bold**", "*italic*", "`code`", "- one", "    - nested", "1. first", "2. second",
		"```\nfunc main() {\n    run()\n}\n```")
}

func TestInvalidInputs(t *testing.T) {
	if _, err := DOCX([]byte("not a zip")); err == nil {
		t.Error("expected error for invalid docx")
	}
	if _, err := PDF([]byte("not a pdf")); err == nil {
		t.Error("expected error for invalid pdf")
	}
}

func TestParseFormats(t *testing.T) {
	got, err := ParseFormats("HTML, docx,html")
	if err != nil || len(got) != 2 || got[0] != FormatHTML || got[1] != FormatDOCX {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"", "md", "pdf,txt"} {
		if _, err := ParseFormats(bad); err == nil {
			t.Errorf("ParseFormats(%q) should fail", bad)
		}
	}
}

func TestRunnerSkipsExistingUnlessForced(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("page.html", "<h1>From HTML</h1>")
	write("page.md", "original")

	r := &Runner{InputDir: dir, Recursive: true, Formats: FormatNames}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "page.md")); string(got) != "original" {
		t.Errorf("existing markdown was overwritten: %q", got)
	}

	r.Force = true
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "page.md")); !strings.Contains(string(got), "# From HTML") {
		t.Errorf("force did not overwrite: %q", got)
	}
}

func TestRunnerPrefersFormatOnClash(t *testing.T) {
	dir := t.TempDir()
	docx, err := document.DOCX([]byte("# From DOCX"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.docx"), docx, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.html"), []byte("<h1>From HTML</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{InputDir: dir, Recursive: true, Formats: FormatNames}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "x.md"))
	if !strings.Contains(string(got), "From DOCX") {
		t.Errorf("expected docx to win the clash, got %q", got)
	}
}

func TestRunnerFormatFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("<p>hi</p>"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{InputDir: dir, Formats: []string{FormatDOCX}}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); err == nil {
		t.Error("html should be ignored when only docx is selected")
	}
}

func writeDoc(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestInputFileWithOutputName(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, filepath.Join(dir, "a.html"), "<h1>A</h1>")
	writeDoc(t, filepath.Join(dir, "b.html"), "<h1>B</h1>")

	// The format filter does not apply to an explicit input.
	r := &Runner{InputFile: filepath.Join(dir, "a.html"), Output: filepath.Join(dir, "out", "notes"), Formats: []string{FormatDOCX}}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "out", "notes.md"))
	if err != nil || !strings.Contains(string(got), "# A") {
		t.Fatalf("expected out/notes.md with the converted content: %v %q", err, got)
	}
	if fileExists(filepath.Join(dir, "a.md")) || fileExists(filepath.Join(dir, "b.md")) {
		t.Error("only the named output should be written")
	}
}

func TestOutputNameKeepsMarkdownExtension(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, filepath.Join(dir, "a.html"), "<h1>A</h1>")

	r := &Runner{InputFile: filepath.Join(dir, "a.html"), Output: filepath.Join(dir, "x.markdown")}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dir, "x.markdown")) || fileExists(filepath.Join(dir, "x.markdown.md")) {
		t.Error("a .markdown output name should be used as is")
	}
}

func TestOutputValidation(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, filepath.Join(dir, "a.html"), "<p>x</p>")
	writeDoc(t, filepath.Join(dir, "notes.txt"), "text")
	in := filepath.Join(dir, "a.html")

	cases := map[string]*Runner{
		"missing input":       {InputFile: filepath.Join(dir, "nope.html")},
		"input is a dir":      {InputFile: dir},
		"unsupported input":   {InputFile: filepath.Join(dir, "notes.txt")},
		"output is a dir":     {InputFile: in, Output: dir},
		"output not markdown": {InputFile: in, Output: filepath.Join(dir, "x.pdf")},
		"scan output is file": {InputDir: dir, Output: in},
	}
	for name, r := range cases {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestOutputDirMirrorsLayout(t *testing.T) {
	src, out := t.TempDir(), filepath.Join(t.TempDir(), "docs")
	writeDoc(t, filepath.Join(src, "a.html"), "<h1>A</h1>")
	writeDoc(t, filepath.Join(src, "sub", "b.html"), "<h1>B</h1>")

	r := &Runner{InputDir: src, Recursive: true, Output: out, Formats: FormatNames}
	if err := r.Run(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"a.md", "sub/b.md"} {
		if !fileExists(filepath.Join(out, want)) {
			t.Errorf("expected %s in output dir", want)
		}
	}
	if fileExists(filepath.Join(src, "a.md")) {
		t.Error("nothing should be written next to the sources when -output is a directory")
	}
}
