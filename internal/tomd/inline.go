package tomd

import (
	"regexp"
	"strings"
	"unicode"
)

// seg is a run of text with uniform inline formatting.
type seg struct {
	text                       string
	bold, italic, code, strike bool
	href                       string
}

func (s seg) sameFormat(o seg) bool {
	return s.bold == o.bold && s.italic == o.italic && s.code == o.code && s.strike == o.strike && s.href == o.href
}

// renderSegs turns formatted runs into inline Markdown. Adjacent runs with the same formatting
// are merged first, because sources such as Word split text into many runs.
func renderSegs(segs []seg) string {
	var merged []seg
	for _, s := range segs {
		if s.text == "" {
			continue
		}
		if n := len(merged); n > 0 && merged[n-1].sameFormat(s) {
			merged[n-1].text += s.text
			continue
		}
		merged = append(merged, s)
	}

	var out strings.Builder
	for i := 0; i < len(merged); {
		// Group consecutive runs of one link so the whole group sits inside a single [..](..).
		j := i + 1
		if merged[i].href != "" {
			for j < len(merged) && merged[j].href == merged[i].href {
				j++
			}
		}

		var inner strings.Builder
		for _, s := range merged[i:j] {
			inner.WriteString(renderSeg(s))
		}
		if href := merged[i].href; href != "" {
			lead, core, trail := splitSpace(inner.String())
			out.WriteString(lead + "[" + core + "](" + href + ")" + trail)
		} else {
			out.WriteString(inner.String())
		}
		i = j
	}
	return out.String()
}

func renderSeg(s seg) string {
	lead, core, trail := splitSpace(s.text)
	if core == "" {
		return s.text
	}

	if s.code {
		fence := "`"
		if strings.Contains(core, "`") {
			fence = "``"
		}
		return lead + fence + core + fence + trail
	}

	core = escapeInline(core)
	switch {
	case s.bold && s.italic:
		core = "***" + core + "***"
	case s.bold:
		core = "**" + core + "**"
	case s.italic:
		core = "*" + core + "*"
	}
	if s.strike {
		core = "~~" + core + "~~"
	}
	return lead + core + trail
}

// splitSpace separates leading and trailing whitespace, which must stay outside emphasis markers.
func splitSpace(s string) (lead, core, trail string) {
	core = strings.TrimLeftFunc(s, unicode.IsSpace)
	lead = s[:len(s)-len(core)]
	trimmed := strings.TrimRightFunc(core, unicode.IsSpace)
	return lead, trimmed, core[len(trimmed):]
}

// escapeInline backslash-escapes characters that would otherwise be read as Markdown.
func escapeInline(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		switch r {
		case '\\', '*', '`', '[', ']', '<':
			b.WriteByte('\\')
		case '_':
			// Underscores inside a word are literal in CommonMark.
			inWord := i > 0 && i < len(runes)-1 && isAlnum(runes[i-1]) && isAlnum(runes[i+1])
			if !inWord {
				b.WriteByte('\\')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

var blockStart = regexp.MustCompile(`^(#{1,6}\s|[-+*]\s|\d+[.)]\s|>)`)

// escapeBlockStart stops a paragraph from being read as a heading, list or quote.
func escapeBlockStart(s string) string {
	if blockStart.MatchString(s) {
		return `\` + s
	}
	return s
}

// collapseSpace squeezes runs of whitespace (including newlines) into single spaces.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
