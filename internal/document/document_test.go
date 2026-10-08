package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

const sample = "# Title\n\nSome **bold**, *italic*, `code` & a [link](http://example.com/?a=1&b=2).\n\n" +
	"- one\n- two\n  - nested\n\n1. first\n2. second\n   ```\n   code in item\n   ```\n\n" +
	"> quote\n\n---\n\n```\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"

func TestHTML(t *testing.T) {
	out := string(HTML([]byte(sample)))
	for _, want := range []string{"<!DOCTYPE html>", "<style>", `<h1 id="title">Title</h1>`, "<strong>bold</strong>", "</html>"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestDOCX(t *testing.T) {
	data, err := DOCX([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}

	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()

		// Every part must be well-formed XML or Word will refuse the file.
		dec := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", f.Name, err)
			}
		}
		parts[f.Name] = string(b)
	}

	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/styles.xml",
		"word/numbering.xml", "word/_rels/document.xml.rels"} {
		if _, ok := parts[name]; !ok {
			t.Errorf("missing part %s", name)
		}
	}

	doc := parts["word/document.xml"]
	for _, want := range []string{`w:val="Heading1"`, "<w:b/>", "<w:i/>", "Courier New", "w:numPr", "code in item", "<w:tbl>", `w:val="Quote"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml missing %q", want)
		}
	}
	if !strings.Contains(parts["word/_rels/document.xml.rels"], "http://example.com/?a=1&amp;b=2") {
		t.Error("hyperlink target missing or not escaped")
	}
}
