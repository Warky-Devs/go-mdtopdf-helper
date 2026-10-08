package engine

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/Warky-Devs/go-mdtopdf-helper.git/internal/document"
	"github.com/gomarkdown/markdown/ast"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

// gopdfEngine renders Markdown directly into PDF drawing commands. It has no external dependencies.
type gopdfEngine struct{}

func (e *gopdfEngine) Name() string { return GoPDF }

func (e *gopdfEngine) Close() error { return nil }

func (e *gopdfEngine) Render(md []byte, outputFile string) error {
	r, err := newPDFRenderer()
	if err != nil {
		return err
	}

	doc := document.NewParser().Parse(md)
	r.blocks(doc.GetChildren(), r.left, r.right-r.left)
	if r.err != nil {
		return r.err
	}
	return r.pdf.WritePdf(outputFile)
}

const (
	pageWidth     = 595.28 // A4 in points
	pageHeight    = 841.89
	pageMargin    = 42.5 // 15mm
	bodySize      = 11.0
	codeSize      = 9.0
	lineSpacing   = 1.4
	blockGap      = 6.0
	listIndent    = 18.0
	quoteIndent   = 14.0
	cellPadding   = 4.0
	tabWidth      = "    "
	baselineShift = 0.8
	fontRegular   = "go"
	fontBold      = "go-bold"
	fontItalic    = "go-italic"
	fontBoldItal  = "go-bold-italic"
	fontMono      = "go-mono"
)

type rgb struct{ r, g, b uint8 }

var (
	colorText = rgb{34, 34, 34}
	colorLink = rgb{0, 0, 238}
	colorRule = rgb{204, 204, 204}
	colorCode = rgb{244, 244, 244}
)

// style describes the inline formatting in effect while collecting tokens.
type style struct {
	bold, italic, mono, link bool
}

// token is one word with its formatting. Width fields are filled in during layout.
type token struct {
	text        string
	family      string
	size        float64
	color       rgb
	spaceBefore bool
	lineBreak   bool
	width       float64
	space       float64
}

type line struct {
	tokens []token
	height float64
}

type pdfRenderer struct {
	pdf         *gopdf.GoPdf
	left, right float64
	y           float64
	page        int
	err         error
}

func newPDFRenderer() (*pdfRenderer, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: pageWidth, H: pageHeight}})

	fonts := map[string][]byte{
		fontRegular:  goregular.TTF,
		fontBold:     gobold.TTF,
		fontItalic:   goitalic.TTF,
		fontBoldItal: gobolditalic.TTF,
		fontMono:     gomono.TTF,
	}
	for name, data := range fonts {
		if err := pdf.AddTTFFontData(name, data); err != nil {
			return nil, fmt.Errorf("failed to load font %s: %w", name, err)
		}
	}

	pdf.AddPage()
	return &pdfRenderer{pdf: pdf, left: pageMargin, right: pageWidth - pageMargin, y: pageMargin, page: 1}, nil
}

func (r *pdfRenderer) fail(err error) {
	if err != nil && r.err == nil {
		r.err = err
	}
}

// ensureSpace starts a new page when h points do not fit below the current position.
func (r *pdfRenderer) ensureSpace(h float64) {
	if r.y+h > pageHeight-pageMargin {
		r.pdf.AddPage()
		r.page++
		r.y = pageMargin
	}
}

func (r *pdfRenderer) setFont(family string, size float64) {
	r.fail(r.pdf.SetFont(family, "", size))
}

func (r *pdfRenderer) measure(family string, size float64, text string) float64 {
	r.setFont(family, size)
	w, err := r.pdf.MeasureTextWidth(text)
	r.fail(err)
	return w
}

// blocks renders block level nodes. Runs of adjacent inline nodes (as found in tight list items)
// are treated as one paragraph.
func (r *pdfRenderer) blocks(nodes []ast.Node, x, width float64) {
	var inline []ast.Node
	flush := func() {
		if len(inline) > 0 {
			r.paragraph(r.collect(inline, style{}, bodySize, colorText), x, width)
			inline = nil
		}
	}

	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.Paragraph:
			flush()
			r.blocks(n.Children, x, width)
		case *ast.Heading:
			flush()
			r.heading(n, x, width)
		case *ast.List:
			flush()
			r.list(n, x, width)
		case *ast.CodeBlock:
			flush()
			r.codeBlock(string(n.Literal), x, width)
		case *ast.BlockQuote:
			flush()
			r.blockQuote(n, x, width)
		case *ast.HorizontalRule:
			flush()
			r.ensureSpace(blockGap * 2)
			r.y += blockGap
			r.rule(x, x+width, r.y)
			r.y += blockGap
		case *ast.Table:
			flush()
			r.table(n, x, width)
		case *ast.HTMLBlock:
			flush()
		default:
			inline = append(inline, n)
		}
	}
	flush()
}

func (r *pdfRenderer) heading(h *ast.Heading, x, width float64) {
	sizes := [...]float64{24, 20, 16, 14, 12, 11}
	size := sizes[min(max(h.Level, 1), len(sizes))-1]

	r.y += size * 0.5
	toks := r.collect(h.Children, style{bold: true}, size, colorText)
	lines := r.layout(toks, width)
	r.ensureSpace(lines[0].height + blockGap)
	r.drawLines(lines, x)
	if h.Level <= 2 {
		r.rule(x, x+width, r.y)
		r.y += 2
	}
	r.y += blockGap
}

func (r *pdfRenderer) paragraph(toks []token, x, width float64) {
	if len(toks) == 0 {
		return
	}
	r.drawLines(r.layout(toks, width), x)
	r.y += blockGap
}

func (r *pdfRenderer) list(l *ast.List, x, width float64) {
	ordered := l.ListFlags&ast.ListTypeOrdered != 0
	for i, item := range l.Children {
		marker := "•"
		if ordered {
			marker = strconv.Itoa(i+1) + "."
		}

		r.ensureSpace(bodySize * lineSpacing * 2)
		r.drawText(marker, fontRegular, bodySize, colorText, x, r.y+bodySize*(lineSpacing-1)/2)
		r.blocks(item.GetChildren(), x+listIndent, width-listIndent)
	}
}

func (r *pdfRenderer) blockQuote(q *ast.BlockQuote, x, width float64) {
	startPage, startY := r.page, r.y
	r.blocks(q.Children, x+quoteIndent, width-quoteIndent)
	if r.page == startPage {
		r.pdf.SetLineWidth(2)
		r.pdf.SetStrokeColor(colorRule.r, colorRule.g, colorRule.b)
		r.pdf.Line(x+2, startY, x+2, r.y-blockGap)
	}
}

func (r *pdfRenderer) codeBlock(code string, x, width float64) {
	const pad = 4.0
	lineH := codeSize * lineSpacing
	textW := width - 2*pad

	code = strings.ReplaceAll(strings.Trim(code, "\n"), "\t", tabWidth)
	var rows []string
	for _, src := range strings.Split(code, "\n") {
		rows = append(rows, r.wrapChars(src, fontMono, codeSize, textW)...)
	}

	r.pdf.SetFillColor(colorCode.r, colorCode.g, colorCode.b)
	r.ensureSpace(pad + lineH)
	r.fillRect(x, r.y, width, pad)
	r.y += pad
	for i, row := range rows {
		extra := 0.0
		if i == len(rows)-1 {
			extra = pad
		}
		r.ensureSpace(lineH + extra)
		r.pdf.SetFillColor(colorCode.r, colorCode.g, colorCode.b)
		r.fillRect(x, r.y, width, lineH+extra)
		r.drawText(row, fontMono, codeSize, colorText, x+pad, r.y+(lineH-codeSize)/2)
		r.y += lineH
	}
	r.y += pad + blockGap
}

func (r *pdfRenderer) fillRect(x, y, w, h float64) {
	r.pdf.RectFromUpperLeftWithStyle(x, y, w, h, "F")
}

func (r *pdfRenderer) table(t *ast.Table, x, width float64) {
	type row struct {
		toks   [][]token
		cells  [][]line
		header bool
		height float64
	}

	var rows []row
	cols := 0
	var walk func(nodes []ast.Node, header bool)
	walk = func(nodes []ast.Node, header bool) {
		for _, n := range nodes {
			switch n := n.(type) {
			case *ast.TableHeader:
				walk(n.Children, true)
			case *ast.TableBody, *ast.TableFooter:
				walk(n.GetChildren(), false)
			case *ast.TableRow:
				rw := row{header: header}
				for _, cell := range n.Children {
					rw.toks = append(rw.toks, r.collect(cell.GetChildren(), style{bold: header}, bodySize, colorText))
				}
				rows = append(rows, rw)
				cols = max(cols, len(n.Children))
			}
		}
	}
	walk(t.Children, false)
	if cols == 0 {
		return
	}

	// Size columns from their content: natural is the width on one line, minimum the longest word.
	natural := make([]float64, cols)
	minimum := make([]float64, cols)
	for i := range natural {
		natural[i], minimum[i] = 2*cellPadding, 2*cellPadding
	}
	for _, rw := range rows {
		for ci, toks := range rw.toks {
			var one float64
			for _, l := range r.layout(toks, 1e9) {
				for _, tk := range l.tokens {
					one += tk.space + tk.width
					minimum[ci] = max(minimum[ci], tk.width+2*cellPadding)
				}
			}
			natural[ci] = max(natural[ci], one+2*cellPadding)
		}
	}
	colW := columnWidths(natural, minimum, width)

	for i := range rows {
		rw := &rows[i]
		rw.cells = make([][]line, len(rw.toks))
		for ci, toks := range rw.toks {
			lines := r.layout(toks, colW[ci]-2*cellPadding)
			rw.cells[ci] = lines
			h := 0.0
			for _, l := range lines {
				h += l.height
			}
			rw.height = max(rw.height, h+2*cellPadding)
		}
	}

	for _, rw := range rows {
		r.ensureSpace(rw.height)
		cx := x
		for ci := 0; ci < cols; ci++ {
			w := colW[ci]
			if rw.header {
				r.pdf.SetFillColor(colorCode.r, colorCode.g, colorCode.b)
				r.fillRect(cx, r.y, w, rw.height)
			}
			r.pdf.SetStrokeColor(colorRule.r, colorRule.g, colorRule.b)
			r.pdf.SetLineWidth(0.5)
			r.pdf.RectFromUpperLeftWithStyle(cx, r.y, w, rw.height, "D")

			if ci < len(rw.cells) {
				cellY := r.y + cellPadding
				for _, l := range rw.cells[ci] {
					r.drawLine(l, cx+cellPadding, cellY)
					cellY += l.height
				}
			}
			cx += w
		}
		r.y += rw.height
	}
	r.y += blockGap
}

// columnWidths splits width between columns: each gets at least its minimum, and any
// remaining space goes to the columns that would otherwise wrap, in proportion to need.
func columnWidths(natural, minimum []float64, width float64) []float64 {
	n := len(natural)
	out := make([]float64, n)

	var sumNat, sumMin float64
	for i := range natural {
		sumNat += natural[i]
		sumMin += minimum[i]
	}

	switch {
	case sumNat <= width:
		for i := range out {
			out[i] = natural[i] * width / sumNat
		}
	case sumMin >= width:
		for i := range out {
			out[i] = width / float64(n)
		}
	default:
		spare, need := width-sumMin, sumNat-sumMin
		for i := range out {
			out[i] = minimum[i] + spare*(natural[i]-minimum[i])/need
		}
	}
	return out
}

func (r *pdfRenderer) rule(x1, x2, y float64) {
	r.pdf.SetLineWidth(0.5)
	r.pdf.SetStrokeColor(colorRule.r, colorRule.g, colorRule.b)
	r.pdf.Line(x1, y, x2, y)
}

// collect flattens inline nodes into styled word tokens.
func (r *pdfRenderer) collect(nodes []ast.Node, st style, size float64, color rgb) []token {
	c := &collector{size: size, color: color}
	c.walk(nodes, st)
	return c.tokens
}

type collector struct {
	tokens       []token
	size         float64
	color        rgb
	pendingSpace bool
}

func (c *collector) walk(nodes []ast.Node, st style) {
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.Text:
			c.text(string(n.Literal), st)
		case *ast.Code:
			s := st
			s.mono = true
			c.text(string(n.Literal), s)
		case *ast.Emph:
			s := st
			s.italic = true
			c.walk(n.Children, s)
		case *ast.Strong:
			s := st
			s.bold = true
			c.walk(n.Children, s)
		case *ast.Del:
			c.walk(n.Children, st)
		case *ast.Link:
			s := st
			s.link = true
			c.walk(n.Children, s)
		case *ast.Image:
			s := st
			s.italic = true
			c.text("[image:", s)
			c.walk(n.Children, s)
			c.text("]", s)
			c.pendingSpace = false
		case *ast.Softbreak:
			c.pendingSpace = true
		case *ast.Hardbreak:
			c.tokens = append(c.tokens, token{lineBreak: true})
			c.pendingSpace = false
		case *ast.HTMLSpan:
		default:
			c.walk(n.GetChildren(), st)
		}
	}
}

func (c *collector) text(s string, st style) {
	if s == "" {
		return
	}
	family := fontRegular
	switch {
	case st.mono:
		family = fontMono
	case st.bold && st.italic:
		family = fontBoldItal
	case st.bold:
		family = fontBold
	case st.italic:
		family = fontItalic
	}
	color := c.color
	if st.link {
		color = colorLink
	}

	if unicode.IsSpace([]rune(s)[0]) {
		c.pendingSpace = true
	}
	words := strings.Fields(s)
	for i, w := range words {
		c.tokens = append(c.tokens, token{
			text:        w,
			family:      family,
			size:        c.size,
			color:       color,
			spaceBefore: c.pendingSpace && len(c.tokens) > 0,
		})
		// Words within one text run are always separated by whitespace.
		c.pendingSpace = i < len(words)-1
	}
	runes := []rune(s)
	if unicode.IsSpace(runes[len(runes)-1]) {
		c.pendingSpace = true
	}
}

// layout greedily wraps tokens into lines no wider than width.
func (r *pdfRenderer) layout(toks []token, width float64) []line {
	var lines []line
	var cur line
	curW := 0.0

	endLine := func() {
		if cur.height == 0 {
			cur.height = bodySize * lineSpacing
		}
		lines = append(lines, cur)
		cur, curW = line{}, 0
	}

	for _, t := range toks {
		if t.lineBreak {
			endLine()
			continue
		}

		for _, piece := range r.fit(t, width) {
			piece.width = r.measure(piece.family, piece.size, piece.text)
			piece.space = 0
			if piece.spaceBefore && len(cur.tokens) > 0 {
				piece.space = r.measure(piece.family, piece.size, " ")
			}
			if len(cur.tokens) > 0 && curW+piece.space+piece.width > width {
				endLine()
				piece.space = 0
			}
			curW += piece.space + piece.width
			cur.tokens = append(cur.tokens, piece)
			cur.height = max(cur.height, piece.size*lineSpacing)
		}
	}
	if len(cur.tokens) > 0 || len(lines) == 0 {
		endLine()
	}
	return lines
}

// fit splits a token wider than width into pieces that each fit.
func (r *pdfRenderer) fit(t token, width float64) []token {
	if r.measure(t.family, t.size, t.text) <= width {
		return []token{t}
	}

	var out []token
	for i, chunk := range r.wrapChars(t.text, t.family, t.size, width) {
		piece := t
		piece.text = chunk
		piece.spaceBefore = t.spaceBefore && i == 0
		out = append(out, piece)
	}
	return out
}

// wrapChars breaks text on character boundaries so each piece fits within width.
func (r *pdfRenderer) wrapChars(text, family string, size, width float64) []string {
	if text == "" {
		return []string{""}
	}

	var out []string
	var cur []rune
	for _, ch := range text {
		next := string(append(cur, ch))
		if len(cur) > 0 && r.measure(family, size, next) > width {
			out = append(out, string(cur))
			cur = nil
		}
		cur = append(cur, ch)
	}
	return append(out, string(cur))
}

func (r *pdfRenderer) drawLines(lines []line, x float64) {
	for _, l := range lines {
		r.ensureSpace(l.height)
		r.drawLine(l, x, r.y)
		r.y += l.height
	}
}

func (r *pdfRenderer) drawLine(l line, x, y float64) {
	for _, t := range l.tokens {
		x += t.space
		r.drawText(t.text, t.family, t.size, t.color, x, y+(l.height-t.size)/2)
		x += t.width
	}
}

func (r *pdfRenderer) drawText(text, family string, size float64, color rgb, x, y float64) {
	r.setFont(family, size)
	r.pdf.SetTextColor(color.r, color.g, color.b)
	// gopdf positions text by a point near the baseline rather than the top of the glyphs,
	// so shift down to make y the top of a size-high box.
	r.pdf.SetXY(x, y+size*baselineShift)
	r.fail(r.pdf.Text(text))
}
