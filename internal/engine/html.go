package engine

import "github.com/Warky-Devs/go-mdtopdf-helper.git/internal/document"

// markdownToHTML renders a complete HTML document for the browser based engines.
func markdownToHTML(md []byte) []byte {
	return document.HTML(md)
}
