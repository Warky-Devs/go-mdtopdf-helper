package tomd

import (
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
)

// HTML converts an HTML document or fragment to Markdown.
func HTML(data []byte) ([]byte, error) {
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		strikethrough.NewStrikethroughPlugin(),
		table.NewTablePlugin(),
	))

	md, err := conv.ConvertString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML: %w", err)
	}
	return []byte(strings.TrimSpace(md) + "\n"), nil
}
