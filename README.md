# go-mdtopdf-helper

A command-line tool that automatically converts Markdown files to PDF using wkhtmltopdf. Perfect for maintaining PDF documentation alongside your Markdown files in Git repositories.

## Features

- Converts Markdown files to high-quality PDFs
- Can run as a Git pre-commit hook
- Recursive directory scanning
- Parallel file processing
- Cross-platform support (Windows, Linux, macOS)
- Automatic detection of wkhtmltopdf installation

## Prerequisites

This tool requires [wkhtmltopdf](https://wkhtmltopdf.org/) to be installed on your system. The application will check for its presence in standard installation locations:

- Windows: `C:\Program Files\wkhtmltopdf\bin`
- Linux: `/usr/local/bin/wkhtmltopdf` or `/usr/bin/wkhtmltopdf`
- macOS: `/usr/local/bin/wkhtmltopdf` or via Homebrew

If wkhtmltopdf is not found, you will be prompted to install it.

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

The generated PDFs are configured with:
- 300 DPI resolution
- 15mm margins on all sides
- Local file access enabled for images
- Support for common Markdown extensions

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
   go test ./...
   ```

## Dependencies

- [go-wkhtmltopdf](https://github.com/SebastiaanKlippert/go-wkhtmltopdf) - Go wrapper for wkhtmltopdf
- [gomarkdown](https://github.com/gomarkdown/markdown) - Markdown parser and HTML renderer

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Thanks to [SebastiaanKlippert](https://github.com/SebastiaanKlippert) for the go-wkhtmltopdf library
- Thanks to the gomarkdown team for their Markdown parser