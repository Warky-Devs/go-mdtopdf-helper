package engine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	renderTimeout = 60 * time.Second
	marginInches  = 15 / 25.4 // 15mm
)

// chromedpEngine drives a single headless Chrome shared by all renders; each render gets its own tab.
type chromedpEngine struct {
	once          sync.Once
	browserCtx    context.Context
	cancelBrowser context.CancelFunc
	cancelAlloc   context.CancelFunc
	startErr      error
}

func (e *chromedpEngine) Name() string { return Chromedp }

func (e *chromedpEngine) start() {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.Flag("disable-dev-shm-usage", true))
	// Containers and CI commonly run as root, where Chrome refuses to start with its sandbox.
	if os.Geteuid() == 0 {
		opts = append(opts, chromedp.NoSandbox)
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	e.browserCtx, e.cancelBrowser, e.cancelAlloc = browserCtx, cancelBrowser, cancelAlloc

	// The browser lives as long as the context that first runs it, so start it here
	// instead of from a per-render context that is cancelled after each file.
	e.startErr = chromedp.Do(browserCtx)
}

func (e *chromedpEngine) Render(md []byte, outputFile string) error {
	e.once.Do(e.start)
	if e.startErr != nil {
		return fmt.Errorf("failed to start Chrome (is Chrome/Chromium installed?): %w", e.startErr)
	}

	tabCtx, cancelTab := chromedp.NewContext(e.browserCtx)
	defer cancelTab()
	// Run once so the tab owns no timeout, then bound the actual work.
	if err := chromedp.Do(tabCtx); err != nil {
		return fmt.Errorf("failed to open Chrome tab: %w", err)
	}
	ctx, cancel := context.WithTimeout(tabCtx, renderTimeout)
	defer cancel()

	html := string(markdownToHTML(md))
	setContent := chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		tree, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return err
		}
		_, err = cdp.Call(ctx, t, page.SetDocumentContent, page.SetDocumentContentParams{
			FrameID: tree.FrameTree.Frame.ID,
			HTML:    html,
		})
		return err
	})
	if err := chromedp.Do(ctx, chromedp.Navigate("about:blank"), setContent); err != nil {
		return fmt.Errorf("failed to load document in Chrome: %w", err)
	}

	pdf, err := chromedp.Run(ctx, chromedp.PrintToPDF(
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFMargin(marginInches),
		chromedp.PDFPrintBackground(),
	))
	if err != nil {
		return fmt.Errorf("chrome failed to render PDF: %w", err)
	}

	if err := os.WriteFile(outputFile, pdf, 0o644); err != nil {
		return fmt.Errorf("failed to write PDF file: %w", err)
	}
	return nil
}

func (e *chromedpEngine) Close() error {
	if e.cancelBrowser != nil {
		e.cancelBrowser()
		e.cancelAlloc()
	}
	return nil
}
