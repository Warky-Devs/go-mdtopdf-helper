package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/gomarkdown/markdown/ast"
)

const (
	nsMain = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPkg  = "http://schemas.openxmlformats.org/package/2006/relationships"

	relStyles    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	relNumbering = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	relHyperlink = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	relDocument  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"

	bulletNumID = 1
	maxListLvl  = 8
)

// DOCX renders Markdown as a Word document (.docx). It is built directly as zipped OOXML.
func DOCX(md []byte) ([]byte, error) {
	b := &docxBuilder{nextNum: bulletNumID + 1}
	b.blocks(NewParser().Parse(md).GetChildren(), blockCtx{})

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, data string }{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", rootRelsXML},
		{"word/document.xml", b.documentXML()},
		{"word/styles.xml", stylesXML},
		{"word/numbering.xml", b.numberingXML()},
		{"word/_rels/document.xml.rels", b.documentRelsXML()},
	}
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, fmt.Errorf("failed to write docx part %s: %w", p.name, err)
		}
		if _, err := w.Write([]byte(xml.Header + p.data)); err != nil {
			return nil, fmt.Errorf("failed to write docx part %s: %w", p.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finish docx: %w", err)
	}
	return buf.Bytes(), nil
}

// blockCtx carries the list and quote nesting while walking block nodes.
type blockCtx struct {
	listLevel int // 0 when not in a list, otherwise the nesting depth (1-based)
	numID     int // numbering instance of the enclosing list
	quote     bool
	// bullet holds a list item's marker until the first paragraph consumes it.
	bullet *bool
}

type docxBuilder struct {
	body    strings.Builder
	links   []string // hyperlink targets, relationship ids are rId10+index
	ordered []int    // numbering ids created for ordered lists
	nextNum int
}

func (b *docxBuilder) blocks(nodes []ast.Node, ctx blockCtx) {
	var inline []ast.Node
	flush := func() {
		if len(inline) > 0 {
			b.paragraph(b.runs(inline, runStyle{}), ctx)
			inline = nil
		}
	}

	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.Paragraph:
			flush()
			// gomarkdown nests fenced code blocks and lists inside paragraphs.
			b.blocks(n.Children, ctx)
		case *ast.Heading:
			flush()
			level := min(max(n.Level, 1), 6)
			fmt.Fprintf(&b.body, `<w:p><w:pPr><w:pStyle w:val="Heading%d"/></w:pPr>%s</w:p>`,
				level, b.runs(n.Children, runStyle{}))
		case *ast.List:
			flush()
			b.list(n, ctx)
		case *ast.CodeBlock:
			flush()
			b.codeBlock(string(n.Literal), ctx)
		case *ast.BlockQuote:
			flush()
			q := ctx
			q.quote = true
			b.blocks(n.Children, q)
		case *ast.HorizontalRule:
			flush()
			b.body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="CCCCCC"/></w:pBdr></w:pPr></w:p>`)
		case *ast.Table:
			flush()
			b.table(n)
		case *ast.HTMLBlock:
			flush()
		default:
			inline = append(inline, n)
		}
	}
	flush()
}

// paragraph writes runs as one paragraph, applying list and quote formatting from ctx.
func (b *docxBuilder) paragraph(runs string, ctx blockCtx) {
	if runs == "" {
		return
	}

	var ppr string
	switch {
	case ctx.listLevel > 0 && ctx.bullet != nil && *ctx.bullet:
		*ctx.bullet = false
		ppr = fmt.Sprintf(`<w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`,
			min(ctx.listLevel-1, maxListLvl), ctx.numID)
	case ctx.listLevel > 0:
		ppr = fmt.Sprintf(`<w:ind w:left="%d"/>`, 360*ctx.listLevel+360)
	case ctx.quote:
		ppr = `<w:pStyle w:val="Quote"/>`
	}
	if ppr != "" {
		ppr = "<w:pPr>" + ppr + "</w:pPr>"
	}
	b.body.WriteString("<w:p>" + ppr + runs + "</w:p>")
}

func (b *docxBuilder) list(l *ast.List, ctx blockCtx) {
	numID := bulletNumID
	if l.ListFlags&ast.ListTypeOrdered != 0 {
		numID = b.nextNum
		b.nextNum++
		b.ordered = append(b.ordered, numID)
	}

	for _, item := range l.Children {
		marker := true
		inner := ctx
		inner.listLevel++
		inner.numID = numID
		inner.bullet = &marker
		b.blocks(item.GetChildren(), inner)
	}
}

func (b *docxBuilder) codeBlock(code string, ctx blockCtx) {
	indent := ""
	if ctx.listLevel > 0 {
		indent = fmt.Sprintf(`<w:ind w:left="%d"/>`, 360*ctx.listLevel+360)
	}

	code = strings.Trim(code, "\n")
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		spacing := ""
		if i == len(lines)-1 {
			spacing = `<w:spacing w:after="160"/>`
		}
		b.body.WriteString(`<w:p><w:pPr><w:pStyle w:val="Code"/>` + spacing + indent + `</w:pPr>`)
		b.body.WriteString(`<w:r><w:t xml:space="preserve">` + escape(strings.ReplaceAll(line, "\t", "    ")) + `</w:t></w:r></w:p>`)
	}
}

func (b *docxBuilder) table(t *ast.Table) {
	b.body.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/><w:tblBorders>`)
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&b.body, `<w:%s w:val="single" w:sz="4" w:space="0" w:color="CCCCCC"/>`, edge)
	}
	b.body.WriteString(`</w:tblBorders><w:tblCellMar><w:left w:w="100" w:type="dxa"/><w:right w:w="100" w:type="dxa"/></w:tblCellMar></w:tblPr>`)

	var walk func(nodes []ast.Node, header bool)
	walk = func(nodes []ast.Node, header bool) {
		for _, n := range nodes {
			switch n := n.(type) {
			case *ast.TableHeader:
				walk(n.Children, true)
			case *ast.TableBody, *ast.TableFooter:
				walk(n.GetChildren(), false)
			case *ast.TableRow:
				b.tableRow(n, header)
			}
		}
	}
	walk(t.Children, false)

	// Word requires a paragraph after a table, and it also separates it from what follows.
	b.body.WriteString(`</w:tbl><w:p/>`)
}

func (b *docxBuilder) tableRow(row *ast.TableRow, header bool) {
	b.body.WriteString("<w:tr>")
	for _, cell := range row.Children {
		tcPr := ""
		if header {
			tcPr = `<w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="F4F4F4"/></w:tcPr>`
		}
		runs := b.runs(cell.GetChildren(), runStyle{bold: header})
		b.body.WriteString("<w:tc>" + tcPr + "<w:p>" + runs + "</w:p></w:tc>")
	}
	b.body.WriteString("</w:tr>")
}

// runStyle describes the inline formatting in effect.
type runStyle struct {
	bold, italic, code bool
	link               bool
}

func (b *docxBuilder) runs(nodes []ast.Node, st runStyle) string {
	var out strings.Builder
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.Text:
			out.WriteString(textRun(string(n.Literal), st))
		case *ast.Code:
			s := st
			s.code = true
			out.WriteString(textRun(string(n.Literal), s))
		case *ast.Emph:
			s := st
			s.italic = true
			out.WriteString(b.runs(n.Children, s))
		case *ast.Strong:
			s := st
			s.bold = true
			out.WriteString(b.runs(n.Children, s))
		case *ast.Del:
			out.WriteString(b.runs(n.Children, st))
		case *ast.Link:
			s := st
			s.link = true
			inner := b.runs(n.Children, s)
			if dest := string(n.Destination); dest != "" {
				b.links = append(b.links, dest)
				inner = fmt.Sprintf(`<w:hyperlink r:id="rId%d" w:history="1">%s</w:hyperlink>`, linkRelBase+len(b.links)-1, inner)
			}
			out.WriteString(inner)
		case *ast.Image:
			s := st
			s.italic = true
			out.WriteString(textRun("[image: ", s) + b.runs(n.Children, s) + textRun("]", s))
		case *ast.Softbreak:
			out.WriteString(textRun(" ", st))
		case *ast.Hardbreak:
			out.WriteString(`<w:r><w:br/></w:r>`)
		case *ast.HTMLSpan:
		default:
			out.WriteString(b.runs(n.GetChildren(), st))
		}
	}
	return out.String()
}

func textRun(text string, st runStyle) string {
	if text == "" {
		return ""
	}

	var props string
	if st.code {
		props += `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/>`
	}
	if st.bold {
		props += `<w:b/>`
	}
	if st.italic {
		props += `<w:i/>`
	}
	if st.link {
		props += `<w:color w:val="0000EE"/><w:u w:val="single"/>`
	}
	if props != "" {
		props = "<w:rPr>" + props + "</w:rPr>"
	}
	text = strings.NewReplacer("\r", "", "\n", " ").Replace(text)
	return `<w:r>` + props + `<w:t xml:space="preserve">` + escape(text) + `</w:t></w:r>`
}

func escape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// linkRelBase is the first relationship id used for hyperlinks; lower ids are fixed parts.
const linkRelBase = 10

func (b *docxBuilder) documentXML() string {
	return fmt.Sprintf(`<w:document xmlns:w="%s" xmlns:r="%s"><w:body>%s<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>`+
		`<w:pgMar w:top="851" w:right="851" w:bottom="851" w:left="851" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`,
		nsMain, nsRel, b.body.String())
}

func (b *docxBuilder) documentRelsXML() string {
	var s strings.Builder
	fmt.Fprintf(&s, `<Relationships xmlns="%s">`, nsPkg)
	fmt.Fprintf(&s, `<Relationship Id="rId1" Type="%s" Target="styles.xml"/>`, relStyles)
	fmt.Fprintf(&s, `<Relationship Id="rId2" Type="%s" Target="numbering.xml"/>`, relNumbering)
	for i, target := range b.links {
		fmt.Fprintf(&s, `<Relationship Id="rId%d" Type="%s" Target="%s" TargetMode="External"/>`,
			linkRelBase+i, relHyperlink, escape(target))
	}
	s.WriteString(`</Relationships>`)
	return s.String()
}

// numberingXML defines one bullet list and one decimal list, each with nine indent levels.
// Every ordered list gets its own instance so numbering restarts at 1.
func (b *docxBuilder) numberingXML() string {
	level := func(i int, format, text string) string {
		return fmt.Sprintf(`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/>`+
			`<w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
			i, format, text, 720*(i+1))
	}

	var bullets, decimals strings.Builder
	for i := 0; i <= maxListLvl; i++ {
		bullets.WriteString(level(i, "bullet", "•"))
		decimals.WriteString(level(i, "decimal", fmt.Sprintf("%%%d.", i+1)))
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<w:numbering xmlns:w="%s">`, nsMain)
	s.WriteString(`<w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="hybridMultilevel"/>` + bullets.String() + `</w:abstractNum>`)
	s.WriteString(`<w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="hybridMultilevel"/>` + decimals.String() + `</w:abstractNum>`)
	fmt.Fprintf(&s, `<w:num w:numId="%d"><w:abstractNumId w:val="0"/></w:num>`, bulletNumID)
	for _, id := range b.ordered {
		fmt.Fprintf(&s, `<w:num w:numId="%d"><w:abstractNumId w:val="1"/>`+
			`<w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, id)
	}
	s.WriteString(`</w:numbering>`)
	return s.String()
}

const contentTypesXML = `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
	`</Types>`

const rootRelsXML = `<Relationships xmlns="` + nsPkg + `">` +
	`<Relationship Id="rId1" Type="` + relDocument + `" Target="word/document.xml"/>` +
	`</Relationships>`

func headingStyle(level, halfPoints int) string {
	return fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/>`+
		`<w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="120"/><w:outlineLvl w:val="%d"/></w:pPr>`+
		`<w:rPr><w:b/><w:sz w:val="%d"/></w:rPr></w:style>`, level, level, level-1, halfPoints)
}

var stylesXML = `<w:styles xmlns:w="` + nsMain + `">` +
	`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
	headingStyle(1, 48) + headingStyle(2, 40) + headingStyle(3, 32) +
	headingStyle(4, 28) + headingStyle(5, 24) + headingStyle(6, 22) +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/>` +
	`<w:pPr><w:spacing w:after="60"/><w:ind w:left="720"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:qFormat/>` +
	`<w:pPr><w:pBdr><w:left w:val="single" w:sz="18" w:space="8" w:color="CCCCCC"/></w:pBdr><w:ind w:left="360"/></w:pPr>` +
	`<w:rPr><w:color w:val="555555"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:qFormat/>` +
	`<w:pPr><w:shd w:val="clear" w:color="auto" w:fill="F4F4F4"/><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>` +
	`<w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/><w:sz w:val="18"/></w:rPr></w:style>` +
	`</w:styles>`
