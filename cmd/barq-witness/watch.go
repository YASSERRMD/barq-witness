package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/watcher"
)

// runWatch implements `barq-witness watch [flags]`.
//
// Flags:
//
//	--interval N   poll interval in seconds (default 2)
//	--top N        show top N segments (default 10)
//	--format text|markdown  output format (default text)
func runWatch(args []string) {
	var (
		intervalSec = 2
		topN        = 10
		format      = "text"
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interval":
			i++
			if i < len(args) {
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					intervalSec = n
				}
			}
		case "--top":
			i++
			if i < len(args) {
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					topN = n
				}
			}
		case "--format":
			i++
			if i < len(args) {
				format = args[i]
			}
		default:
			if strings.HasPrefix(args[i], "--interval=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(args[i], "--interval=")); err == nil && n > 0 {
					intervalSec = n
				}
			} else if strings.HasPrefix(args[i], "--top=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(args[i], "--top=")); err == nil && n > 0 {
					topN = n
				}
			} else if strings.HasPrefix(args[i], "--format=") {
				format = strings.TrimPrefix(args[i], "--format=")
			}
		}
	}

	// Resolve paths.
	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cannot determine working directory: %v", err)
	}
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		cwd = envBase
	}
	witnessDir := filepath.Join(cwd, ".witness")
	dbPath := filepath.Join(witnessDir, "trace.db")

	st, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace store: %v", err)
	}
	defer st.Close()

	interval := time.Duration(intervalSec) * time.Second
	w := watcher.New(st, cwd, interval, topN)

	// Handle Ctrl+C / SIGINT gracefully.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	fmt.Fprintf(os.Stderr, "barq-witness watch: polling every %ds  (Ctrl+C to quit)\n", intervalSec)

	if err := w.Run(ctx, os.Stdout, format); err != nil {
		fatalf("watch: %v", err)
	}
}
