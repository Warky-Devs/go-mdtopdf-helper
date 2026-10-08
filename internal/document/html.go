// Package document renders Markdown into document formats that need no external tools.
package document

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
)

const htmlHead = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body { font-family: sans-serif; line-height: 1.5; color: #222; }
pre { background: #f4f4f4; padding: 8px; overflow-x: auto; }
code { font-family: monospace; }
blockquote { border-left: 3px solid #ccc; margin-left: 0; padding-left: 12px; color: #555; }
table { border-collapse: collapse; }
th, td { border: 1px solid #ccc; padding: 4px 8px; }
img { max-width: 100%; }
</style>
</head>
<body>
`

const htmlFoot = "\n</body>\n</html>\n"

// NewParser returns the Markdown parser used by every output format.
func NewParser() *parser.Parser {
	return parser.NewWithExtensions(parser.CommonExtensions | parser.AutoHeadingIDs)
}

// HTML renders a standalone HTML document with embedded styling.
func HTML(md []byte) []byte {
	body := markdown.ToHTML(md, NewParser(), nil)
	out := make([]byte, 0, len(htmlHead)+len(body)+len(htmlFoot))
	out = append(out, htmlHead...)
	out = append(out, body...)
	return append(out, htmlFoot...)
}
