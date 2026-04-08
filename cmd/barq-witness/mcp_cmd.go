package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yasserrmd/barq-witness/internal/mcp"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// runMCP implements `barq-witness mcp`.
// It opens the trace store and starts a stdio-based MCP server that
// exposes barq-witness trace data to AI tools that speak the Model Context
// Protocol (JSON-RPC 2.0 over stdin/stdout).
func runMCP(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(os.Stderr, "Usage: barq-witness mcp")
			fmt.Fprintln(os.Stderr, "Start a stdio-based MCP server for querying the barq-witness trace.")
			os.Exit(0)
		}
	}

	// Resolve the .witness directory and open the trace store.
	witnessDir := resolveWitnessDir()
	dbPath := filepath.Join(witnessDir, "trace.db")

	st, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace store: %v", err)
	}
	defer st.Close()

	// Resolve the repository root (parent of .witness/).
	repoPath := filepath.Dir(witnessDir)
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		repoPath = envBase
	}

	// Run the MCP server until stdin closes or a signal is received.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := mcp.New(st, repoPath)
	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		fatalf("mcp server: %v", err)
	}
}
