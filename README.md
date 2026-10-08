# go-mdtopdf-helper

A command-line tool that automatically converts Markdown files to PDF. Perfect for maintaining PDF documentation alongside your Markdown files in Git repositories.

## Features

- Converts Markdown files to PDF, HTML or DOCX (one or several at once)
- Choice of PDF rendering engines
- Works out of the box: the default engine is pure Go with no external dependencies
- Can run as a Git pre-commit hook
- Recursive directory scanning
- Parallel file processing
- Cross-platform support (Windows, Linux, macOS)

## Output Formats

Select formats with `-format`, for example `-format pdf,html,docx`. Each file is written next to its Markdown source with the matching extension (`README.md` → `README.pdf`, `README.html`, `README.docx`).

- **pdf**: rendered by the selected engine (see below).
- **html**: a standalone page with embedded CSS.
- **docx**: a Word document generated in pure Go (no Word or other tools needed). Supports headings, paragraphs, bold/italic/code, links, bulleted and numbered lists, code blocks, quotes, rules and tables. Images are written as `[image: alt]` text.

In pre-commit hook mode every generated file is staged.

## Engines

The engine only applies to PDF output.

Select an engine with `-engine`:

| Engine | Needs | Notes |
|--------|-------|-------|
| `gopdf` (default) | nothing | Pure Go. Supports headings, paragraphs, bold/italic/code, links, lists, code blocks, quotes, rules and tables. Images are shown as `[image: alt]`. |
| `chromedp` | Chrome or Chromium | Best rendering (full HTML/CSS). Runs headless Chrome; the sandbox is disabled automatically when running as root, as in most CI containers. |
| `wkhtmltopdf` | [wkhtmltopdf](https://wkhtmltopdf.org/) | Legacy engine. Looked up in the standard install locations (Windows: `C:\Program Files\wkhtmltopdf\bin`, Linux/macOS: `/usr/local/bin`, `/usr/bin`, Homebrew). |

## Installation

```bash
go get github.com/Warky-Devs/go-mdtopdf-helper
```

## Usage

### Basic Usage

Convert Markdown files in the current directory:

```bash
go-mdtopdf-helper
```

### Command Line Options

```bash
go-mdtopdf-helper [options]

Options:
  -dir string
        Directory to scan for markdown files (default ".")
  -recursive
        Scan directories recursively (default true)
  -parallel
        Convert files in parallel (default true)
  -hook
        Run as git pre-commit hook
  -engine string
        PDF engine: gopdf, wkhtmltopdf, chromedp (default "gopdf")
  -format string
        Output formats, comma separated: pdf, html, docx (default "pdf")
```

### Git Pre-commit Hook

To use as a Git pre-commit hook:

1. Create a file named `pre-commit` in your repository's `.git/hooks/` directory
2. Add the following content:

```bash
#!/bin/sh
go-mdtopdf-helper -hook
```

3. Make the hook executable:

```bash
chmod +x .git/hooks/pre-commit
```

When enabled as a pre-commit hook, the tool will:
1. Detect staged Markdown files
2. Ask for confirmation before conversion
3. Convert files to PDF
4. Automatically stage the generated PDFs

## PDF Output Configuration

Generated PDFs are A4 with 15mm margins on all sides. The `chromedp` and `wkhtmltopdf` engines render the full Markdown-to-HTML output, so they support everything the Markdown parser's common extensions produce.

## Project Layout

```
cmd/go-mdtopdf-helper/   CLI entry point (flags, hook mode)
internal/converter/      File discovery, parallel conversion, git hook
internal/document/       HTML and DOCX writers
internal/engine/         Engine interface and the gopdf, wkhtmltopdf and chromedp backends
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. Here's how you can contribute:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Setup

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Make your changes
4. Run tests:
   ```bash
   make test
   ```

`make` also provides `build`, `lint` (golangci-lint), `format` (gofmt) and `all`. Tests for the `chromedp` and `wkhtmltopdf` engines are skipped when the tool is not installed.

## Dependencies

- [gopdf](https://github.com/signintech/gopdf) - Pure Go PDF generation
- [chromedp](https://github.com/chromedp/chromedp) - Headless Chrome driver
- [go-wkhtmltopdf](https://github.com/SebastiaanKlippert/go-wkhtmltopdf) - Go wrapper for wkhtmltopdf
- [gomarkdown](https://github.com/gomarkdown/markdown) - Markdown parser and HTML renderer

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Thanks to [SebastiaanKlippert](https://github.com/SebastiaanKlippert) for the go-wkhtmltopdf library
- Thanks to the gomarkdown team for their Markdown parser