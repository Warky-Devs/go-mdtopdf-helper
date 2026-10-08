package tomd

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// pdfLine is one visual line of text on a page.
type pdfLine struct {
	x, y, size float64
	segs       []seg
	plain      string
	raw        string // like plain but keeps leading indentation, for code
	mono       bool
}

// PDF converts a PDF to Markdown on a best-effort basis. PDFs carry no document structure, so
// headings are inferred from font size, code from monospaced fonts, and lists from bullet or
// number prefixes. Tables and images are not recovered.
func PDF(data []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to read PDF: %v", r)
		}
	}()

	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}

	var pages [][]pdfLine
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		if lines := groupLines(page.Content().Text); len(lines) > 0 {
			pages = append(pages, lines)
		}
	}

	return []byte(buildMarkdown(pages)), nil
}

func fontFlags(font string) (bold, italic, mono bool) {
	f := strings.ToLower(font)
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(f, s) {
				return true
			}
		}
		return false
	}
	return has("bold", "black", "heavy"), has("italic", "oblique"), isMono(f)
}

// groupLines clusters text items into lines and turns them into formatted runs.
func groupLines(items []pdf.Text) []pdfLine {
	items = append([]pdf.Text(nil), items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Y > items[j].Y })

	var clusters [][]pdf.Text
	for _, it := range items {
		n := len(clusters)
		if n > 0 && math.Abs(clusters[n-1][0].Y-it.Y) <= math.Max(2, 0.35*it.FontSize) {
			clusters[n-1] = append(clusters[n-1], it)
			continue
		}
		clusters = append(clusters, []pdf.Text{it})
	}

	var lines []pdfLine
	for _, c := range clusters {
		if l, ok := buildLine(c); ok {
			lines = append(lines, l)
		}
	}
	return lines
}

func buildLine(items []pdf.Text) (pdfLine, bool) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].X < items[j].X })

	line := pdfLine{x: items[0].X, y: items[0].Y}
	sizeChars := map[float64]int{}
	monoChars, chars := 0, 0

	var prev *pdf.Text
	for i := range items {
		it := items[i]
		if it.S == "�" || it.S == "" {
			continue
		}

		bold, italic, mono := fontFlags(it.Font)
		s := seg{text: it.S, bold: bold, italic: italic, code: mono}
		if prev != nil && needsSpace(*prev, it) {
			s.text = " " + s.text
		}
		prev = &items[i]

		if n := len(line.segs); n > 0 && line.segs[n-1].sameFormat(s) {
			line.segs[n-1].text += s.text
		} else {
			line.segs = append(line.segs, s)
		}

		if !unicode.IsSpace([]rune(it.S)[0]) {
			sizeChars[math.Round(it.FontSize*2)/2]++
			chars++
			if mono {
				monoChars++
			}
		}
	}

	line.raw = strings.TrimRight(plainText(line.segs), " \t")
	line.plain = strings.TrimSpace(line.raw)
	if line.plain == "" {
		return line, false
	}

	best := 0
	for size, n := range sizeChars {
		if n > best {
			best, line.size = n, size
		}
	}
	line.mono = monoChars == chars
	return line, true
}

// needsSpace decides whether a space separates two consecutive text items. When the extractor knows
// glyph widths the real gap is used; otherwise (positions only advance per text-show operator) any
// new position starts a new word, except before closing punctuation.
func needsSpace(prev, next pdf.Text) bool {
	if unicode.IsSpace([]rune(prev.S)[0]) || unicode.IsSpace([]rune(next.S)[0]) {
		return false
	}
	if prev.W > 0 {
		return next.X-(prev.X+prev.W) > 0.2*next.FontSize
	}
	if next.X-prev.X < 0.01 {
		return false // same show operator: part of the same word
	}

	first := []rune(next.S)[0]
	last := []rune(prev.S)[len([]rune(prev.S))-1]
	return !strings.ContainsRune(",.;:!?)]}%’”", first) && !strings.ContainsRune("([{‘“", last)
}

var listMarker = regexp.MustCompile(`^([•·▪‣◦●○▫–\-*]|\d{1,3}[.)])\s+`)

// trimPrefix removes the first n runes from the runs.
func trimPrefix(segs []seg, n int) []seg {
	out := make([]seg, 0, len(segs))
	for _, s := range segs {
		r := []rune(s.text)
		switch {
		case n >= len(r):
			n -= len(r)
			continue
		case n > 0:
			s.text = string(r[n:])
			n = 0
		}
		out = append(out, s)
	}
	return out
}

func buildMarkdown(pages [][]pdfLine) string {
	body := bodySize(pages)
	headings := headingSizes(pages, body)

	w := &mdWriter{}
	var (
		para     []seg // paragraph or list item being accumulated
		paraKind paraKind
		paraMark string // list marker line prefix
		textX    float64
		lastY    float64
		codeX    float64
	)

	flush := func() {
		if len(para) == 0 {
			return
		}
		text := collapseSpace(renderSegs(para))
		if paraKind == kindList {
			w.extend(kindList, paraMark+text, "\n")
		} else {
			w.add(kindParagraph, escapeBlockStart(text))
		}
		para = nil
	}

	for _, page := range pages {
		if paraKind != kindCode {
			flush()
		}

		left := page[0].x
		for _, l := range page {
			left = math.Min(left, l.x)
		}

		for _, l := range page {
			switch level := headingLevel(l, headings); {
			case l.mono:
				flush()
				if paraKind != kindCode {
					codeX = l.x
				}
				if blanks := int(math.Round((lastY-l.y)/(l.size*1.4))) - 1; paraKind == kindCode && blanks > 0 && lastY > l.y {
					for range blanks {
						w.extend(kindCode, "", "\n")
					}
				}
				indent := strings.Repeat(" ", max(0, int(math.Round((l.x-codeX)/(l.size*0.6)))))
				w.extend(kindCode, indent+l.raw, "\n")
				paraKind = kindCode

			case level > 0:
				flush()
				w.add(kindHeading, strings.Repeat("#", level)+" "+collapseSpace(renderSegs(stripCode(l.segs))))
				paraKind = kindHeading

			default:
				segs := l.segs
				if m := listMarker.FindString(l.plain); m != "" {
					flush()
					depth := int(math.Round((l.x - left) / (l.size * 1.6)))
					paraKind = kindList
					paraMark = strings.Repeat("    ", max(depth, 0)) + markdownMarker(strings.TrimSpace(m))
					segs = trimPrefix(trimLeadingSpace(segs), len([]rune(strings.TrimSpace(m))))
					textX = l.x + l.size
				} else if len(para) > 0 && paraKind != kindCode && lastY-l.y > 0 && lastY-l.y <= l.size*1.7 &&
					(paraKind != kindList || l.x >= textX-2) {
					segs = append([]seg{{text: " "}}, segs...)
				} else {
					flush()
					paraKind = kindParagraph
				}
				para = append(para, segs...)
			}
			lastY = l.y
		}
	}
	flush()

	return w.String()
}

func trimLeadingSpace(segs []seg) []seg {
	if len(segs) > 0 {
		segs = append([]seg(nil), segs...)
		segs[0].text = strings.TrimLeftFunc(segs[0].text, unicode.IsSpace)
	}
	return segs
}

func markdownMarker(m string) string {
	if n, err := strconv.Atoi(strings.TrimRight(m, ".)")); err == nil {
		return strconv.Itoa(n) + ". "
	}
	return "- "
}

// stripCode removes inline-code styling, which is meaningless inside a heading.
func stripCode(segs []seg) []seg {
	out := append([]seg(nil), segs...)
	for i := range out {
		out[i].code, out[i].bold = false, false
	}
	return out
}

// bodySize is the font size that carries the most (non-code) text.
func bodySize(pages [][]pdfLine) float64 {
	counts := map[float64]int{}
	for _, p := range pages {
		for _, l := range p {
			if !l.mono {
				counts[l.size] += len([]rune(l.plain))
			}
		}
	}

	var body float64
	best := 0
	for size, n := range counts {
		if n > best || (n == best && size < body) {
			body, best = size, n
		}
	}
	return body
}

// headingSizes lists the distinct sizes clearly larger than body text, largest first.
func headingSizes(pages [][]pdfLine, body float64) []float64 {
	seen := map[float64]bool{}
	for _, p := range pages {
		for _, l := range p {
			if !l.mono && l.size > body*1.12 {
				seen[l.size] = true
			}
		}
	}

	sizes := make([]float64, 0, len(seen))
	for s := range seen {
		sizes = append(sizes, s)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	return sizes
}

func headingLevel(l pdfLine, headings []float64) int {
	if l.mono {
		return 0
	}
	for i, s := range headings {
		if l.size == s {
			return min(i+1, 6)
		}
	}
	return 0
}
