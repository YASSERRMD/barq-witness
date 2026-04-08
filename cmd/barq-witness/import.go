// barq-witness import -- read-only import subcommands.
//
// Usage:
//
//	barq-witness import cursor --log <path>
//	barq-witness import codex  --log <path>
//	barq-witness import aider  --chat <path>
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	aideradapter "github.com/yasserrmd/barq-witness/internal/adapters/aider"
	codexadapter "github.com/yasserrmd/barq-witness/internal/adapters/codex"
	cursoradapter "github.com/yasserrmd/barq-witness/internal/adapters/cursor"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// runImport dispatches to the appropriate read-only importer based on the
// first positional argument (cursor, codex, aider).
func runImport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: barq-witness import <cursor|codex|aider> [flags]")
		os.Exit(1)
	}

	tool := args[0]
	rest := args[1:]

	switch tool {
	case "cursor":
		runImportCursor(rest)
	case "codex":
		runImportCodex(rest)
	case "aider":
		runImportAider(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown import tool: %s (want cursor, codex, or aider)\n", tool)
		os.Exit(1)
	}
}

func openStoreForImport() *store.Store {
	dir := resolveWitnessDir()
	dbPath := filepath.Join(dir, "trace.db")
	st, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "barq-witness import: open store: %v\n", err)
		os.Exit(1)
	}
	return st
}

func runImportCursor(args []string) {
	fs := flag.NewFlagSet("import cursor", flag.ExitOnError)
	logPath := fs.String("log", "", "path to Cursor session log (JSON)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *logPath == "" {
		fmt.Fprintln(os.Stderr, "usage: barq-witness import cursor --log <path>")
		os.Exit(1)
	}

	st := openStoreForImport()
	defer st.Close()

	n, err := cursoradapter.ImportFromLog(st, *logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cursor import error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("cursor: imported %d edit(s)\n", n)
}

func runImportCodex(args []string) {
	fs := flag.NewFlagSet("import codex", flag.ExitOnError)
	logPath := fs.String("log", "", "path to Codex CLI session log (JSON)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *logPath == "" {
		fmt.Fprintln(os.Stderr, "usage: barq-witness import codex --log <path>")
		os.Exit(1)
	}

	st := openStoreForImport()
	defer st.Close()

	n, err := codexadapter.ImportFromLog(st, *logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex import error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("codex: imported %d edit(s)\n", n)
}

func runImportAider(args []string) {
	fs := flag.NewFlagSet("import aider", flag.ExitOnError)
	chatPath := fs.String("chat", "", "path to Aider chat history markdown file")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *chatPath == "" {
		fmt.Fprintln(os.Stderr, "usage: barq-witness import aider --chat <path>")
		os.Exit(1)
	}

	st := openStoreForImport()
	defer st.Close()

	n, err := aideradapter.ImportFromChat(st, *chatPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aider import error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("aider: imported %d edit(s)\n", n)
}
