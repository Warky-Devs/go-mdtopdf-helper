package tomd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// node is a namespace-less XML element, enough to walk WordprocessingML.
type node struct {
	name     string
	attrs    map[string]string
	children []*node
	text     string
}

func parseXML(data []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	root := &node{}
	stack := []*node{root}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			stack[len(stack)-1].text += string(t)
		}
	}
	return root, nil
}

func (n *node) child(name string) *node {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func (n *node) attr(name string) string {
	if n == nil {
		return ""
	}
	return n.attrs[name]
}

// docxDoc holds what is needed from the package to interpret the body.
type docxDoc struct {
	styles    map[string]string // styleId -> lower-cased style name
	listKinds map[string]bool   // "numId:ilvl" -> ordered
	links     map[string]string // relationship id -> target
}

// DOCX converts a Word document to Markdown.
func DOCX(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid docx file: %w", err)
	}

	parts := map[string][]byte{}
	for _, f := range zr.File {
		switch f.Name {
		case "word/document.xml", "word/styles.xml", "word/numbering.xml", "word/_rels/document.xml.rels":
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			b, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
			parts[f.Name] = b
		}
	}
	if parts["word/document.xml"] == nil {
		return nil, fmt.Errorf("docx has no word/document.xml")
	}

	doc := &docxDoc{styles: map[string]string{}, listKinds: map[string]bool{}, links: map[string]string{}}
	doc.loadStyles(parts["word/styles.xml"])
	doc.loadNumbering(parts["word/numbering.xml"])
	doc.loadLinks(parts["word/_rels/document.xml.rels"])

	root, err := parseXML(parts["word/document.xml"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse document.xml: %w", err)
	}

	w := &mdWriter{}
	w.body(doc, root.child("document").child("body"))
	return []byte(w.String()), nil
}

func (d *docxDoc) loadStyles(data []byte) {
	root, err := parseXML(data)
	if err != nil {
		return
	}
	for _, s := range root.child("styles").children {
		if s.name == "style" {
			d.styles[s.attr("styleId")] = strings.ToLower(s.child("name").attr("val"))
		}
	}
}

func (d *docxDoc) loadNumbering(data []byte) {
	root, err := parseXML(data)
	if err != nil {
		return
	}

	// abstractNumId -> level -> ordered
	abstract := map[string]map[string]bool{}
	var nums []*node
	for _, n := range root.child("numbering").children {
		switch n.name {
		case "abstractNum":
			levels := map[string]bool{}
			for _, lvl := range n.children {
				if lvl.name == "lvl" {
					levels[lvl.attr("ilvl")] = lvl.child("numFmt").attr("val") != "bullet"
				}
			}
			abstract[n.attr("abstractNumId")] = levels
		case "num":
			nums = append(nums, n)
		}
	}

	for _, n := range nums {
		for lvl, ordered := range abstract[n.child("abstractNumId").attr("val")] {
			d.listKinds[n.attr("numId")+":"+lvl] = ordered
		}
	}
}

func (d *docxDoc) loadLinks(data []byte) {
	root, err := parseXML(data)
	if err != nil {
		return
	}
	for _, r := range root.child("Relationships").children {
		if r.attr("TargetMode") == "External" {
			d.links[r.attr("Id")] = r.attr("Target")
		}
	}
}

var headingStyle = regexp.MustCompile(`^heading (\d)$`)

type paraKind int

const (
	kindParagraph paraKind = iota
	kindHeading
	kindList
	kindCode
	kindQuote
	kindRule
)

// mdWriter assembles blocks, keeping consecutive code lines and list items together.
type mdBlock struct {
	kind paraKind
	text string
}

type mdWriter struct {
	blocks []mdBlock
	last   paraKind
	// code and list blocks grow as consecutive paragraphs arrive.
	open bool
}

func (w *mdWriter) String() string {
	texts := make([]string, len(w.blocks))
	for i, b := range w.blocks {
		texts[i] = b.text
		if b.kind == kindCode {
			texts[i] = "```\n" + b.text + "\n```"
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n\n")) + "\n"
}

func (w *mdWriter) add(kind paraKind, text string) {
	w.blocks = append(w.blocks, mdBlock{kind, text})
	w.last, w.open = kind, kind == kindCode || kind == kindList
}

// extend appends to the previous block when it is of the same grouped kind.
func (w *mdWriter) extend(kind paraKind, text, sep string) {
	if w.open && w.last == kind {
		w.blocks[len(w.blocks)-1].text += sep + text
		return
	}
	w.add(kind, text)
}

func (w *mdWriter) body(d *docxDoc, body *node) {
	if body == nil {
		return
	}
	for _, n := range body.children {
		switch n.name {
		case "p":
			w.paragraph(d, n)
		case "tbl":
			if t := d.table(n); t != "" {
				w.add(kindParagraph, t)
			}
		}
	}
}

func (w *mdWriter) paragraph(d *docxDoc, p *node) {
	ppr := p.child("pPr")
	styleID := ppr.child("pStyle").attr("val")
	style := d.styles[styleID]
	if style == "" {
		style = strings.ToLower(styleID)
	}

	segs := d.runs(p)
	text := strings.TrimSpace(renderSegs(segs))
	plain := plainText(segs)

	switch {
	case ppr.child("pBdr").child("bottom") != nil && plain == "":
		w.add(kindRule, "---")
	case isCodeStyle(style):
		w.extend(kindCode, plain, "\n")
	case plain == "":
		w.open = false
	default:
		if m := headingStyle.FindStringSubmatch(style); m != nil {
			level, _ := strconv.Atoi(m[1])
			w.add(kindHeading, strings.Repeat("#", min(max(level, 1), 6))+" "+collapseSpace(text))
		} else if style == "title" {
			w.add(kindHeading, "# "+collapseSpace(text))
		} else if numPr := ppr.child("numPr"); numPr != nil {
			w.extend(kindList, d.listItem(numPr, text), "\n")
		} else if strings.Contains(style, "quote") {
			w.add(kindQuote, "> "+strings.ReplaceAll(text, "\n", "\n> "))
		} else {
			w.add(kindParagraph, escapeBlockStart(text))
		}
	}
}

func isCodeStyle(style string) bool {
	return style == "code" || strings.Contains(style, "source code") || strings.Contains(style, "preformatted") ||
		strings.Contains(style, "html code")
}

func (d *docxDoc) listItem(numPr *node, text string) string {
	level, _ := strconv.Atoi(numPr.child("ilvl").attr("val"))
	numID := numPr.child("numId").attr("val")
	marker := "-"
	if d.listKinds[numID+":"+strconv.Itoa(level)] {
		marker = "1."
	}
	return strings.Repeat("    ", level) + marker + " " + collapseSpace(text)
}

// runs collects the formatted text of a paragraph, resolving hyperlinks.
func (d *docxDoc) runs(p *node) []seg {
	var segs []seg
	var walk func(n *node, href string)
	walk = func(n *node, href string) {
		for _, c := range n.children {
			switch c.name {
			case "r":
				segs = append(segs, d.run(c, href)...)
			case "hyperlink":
				walk(c, d.links[c.attr("id")])
			case "ins", "smartTag", "sdt", "sdtContent":
				walk(c, href)
			}
		}
	}
	walk(p, "")
	return segs
}

func (d *docxDoc) run(r *node, href string) []seg {
	rpr := r.child("rPr")
	base := seg{
		bold:   on(rpr.child("b")),
		italic: on(rpr.child("i")),
		strike: on(rpr.child("strike")),
		code:   isMono(rpr.child("rFonts").attr("ascii")),
		href:   href,
	}

	var out []seg
	for _, c := range r.children {
		s := base
		switch c.name {
		case "t":
			s.text = c.text
		case "tab":
			s.text = " "
		case "br":
			s.text = "\n"
		default:
			continue
		}
		out = append(out, s)
	}
	return out
}

// on reports whether a toggle property such as <w:b/> is switched on.
func on(n *node) bool {
	if n == nil {
		return false
	}
	switch n.attr("val") {
	case "0", "false", "off":
		return false
	}
	return true
}

func isMono(font string) bool {
	f := strings.ToLower(font)
	for _, m := range []string{"courier", "consolas", "mono", "menlo", "lucida console"} {
		if strings.Contains(f, m) {
			return true
		}
	}
	return false
}

func plainText(segs []seg) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.text)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (d *docxDoc) table(tbl *node) string {
	var rows [][]string
	for _, tr := range tbl.children {
		if tr.name != "tr" {
			continue
		}
		var cells []string
		for _, tc := range tr.children {
			if tc.name != "tc" {
				continue
			}
			var parts []string
			for _, p := range tc.children {
				if p.name == "p" {
					segs := d.runs(p)
					if len(rows) == 0 {
						// The first row becomes the Markdown header, which is already bold.
						for i := range segs {
							segs[i].bold = false
						}
					}
					if t := collapseSpace(renderSegs(segs)); t != "" {
						parts = append(parts, t)
					}
				}
			}
			cell := strings.Join(parts, "<br>")
			cells = append(cells, strings.ReplaceAll(cell, "|", `\|`))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return ""
	}

	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}

	line := func(cells []string) string {
		for len(cells) < cols {
			cells = append(cells, "")
		}
		return "| " + strings.Join(cells, " | ") + " |"
	}

	var b strings.Builder
	b.WriteString(line(rows[0]) + "\n")
	b.WriteString("|" + strings.Repeat(" --- |", cols))
	for _, r := range rows[1:] {
		b.WriteString("\n" + line(r))
	}
	return b.String()
}
