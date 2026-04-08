package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/store"
	inttui "github.com/yasserrmd/barq-witness/internal/tui"
)

// runTUI implements `barq-witness tui [flags]`.
//
// Flags:
//
//	--top N   show top N segments (default 10)
func runTUI(args []string) {
	topN := 10

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--top":
			i++
			if i < len(args) {
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					topN = n
				}
			}
		default:
			if strings.HasPrefix(args[i], "--top=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(args[i], "--top=")); err == nil && n > 0 {
					topN = n
				}
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

	if err := inttui.Run(st, cwd, topN); err != nil {
		fatalf("tui: %v", err)
	}
}
