// Package watcher implements the live attention-map polling loop for
// barq-witness watch. It does not depend on any TUI library.
package watcher

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/render"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// Watcher polls the trace store on a fixed interval and streams a refreshed
// attention map to an io.Writer.
type Watcher struct {
	store    *store.Store
	repoPath string
	interval time.Duration
	topN     int
}

// New constructs a Watcher.
func New(st *store.Store, repoPath string, interval time.Duration, topN int) *Watcher {
	return &Watcher{
		store:    st,
		repoPath: repoPath,
		interval: interval,
		topN:     topN,
	}
}

// Run starts the polling loop. It writes to out and returns when ctx is
// cancelled or a fatal error occurs.
func (w *Watcher) Run(ctx context.Context, out io.Writer, format string) error {
	// Run once immediately before the first tick.
	if err := w.poll(out, format); err != nil {
		fmt.Fprintf(out, "barq-witness watch: poll error: %v\n", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.poll(out, format); err != nil {
				fmt.Fprintf(out, "barq-witness watch: poll error: %v\n", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// poll resolves HEAD, runs the analyzer, and renders one frame to out.
func (w *Watcher) poll(out io.Writer, format string) error {
	headSHA, err := util.HeadSHA(w.repoPath)
	if err != nil || headSHA == "" {
		// Clear screen then show a waiting message -- do not error out.
		fmt.Fprint(out, clearScreen)
		fmt.Fprintln(out, "barq-witness watch: waiting for first commit...")
		return nil
	}

	report, err := analyzer.Analyze(w.store, w.repoPath, "", headSHA)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	// Clear screen and reprint.
	fmt.Fprint(out, clearScreen)

	switch format {
	case "markdown":
		return render.Markdown(out, report, render.MarkdownOptions{TopN: w.topN})
	default: // "text"
		return render.Text(out, report, render.TextOptions{
			TopN:  w.topN,
			Color: true,
		})
	}
}

// clearScreen is the ANSI escape sequence to move cursor to top-left and clear
// the entire screen, giving a "refresh" effect without flicker.
const clearScreen = "\033[H\033[2J"
