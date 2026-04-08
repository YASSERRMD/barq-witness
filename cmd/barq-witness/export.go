package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yasserrmd/barq-witness/internal/cgpf"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// runExport implements `barq-witness export [flags]`.
func runExport(args []string) {
	var (
		sessionID  string
		fromCommit string
		toCommit   string
		outFile    string
		privacy    bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session":
			i++
			if i < len(args) {
				sessionID = args[i]
			}
		case "--from-commit":
			i++
			if i < len(args) {
				fromCommit = args[i]
			}
		case "--to-commit":
			i++
			if i < len(args) {
				toCommit = args[i]
			}
		case "--out":
			i++
			if i < len(args) {
				outFile = args[i]
			}
		case "--privacy":
			privacy = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cannot determine working directory: %v", err)
	}
	dbPath := filepath.Join(cwd, ".witness", "trace.db")
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		dbPath = filepath.Join(envBase, ".witness", "trace.db")
		cwd = envBase
	}

	s, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace store: %v", err)
	}
	defer s.Close()

	cgpf.BinaryVersion = version // set by main.go

	doc, err := cgpf.Export(s, cgpf.ExportOptions{
		SessionID:  sessionID,
		FromCommit: fromCommit,
		ToCommit:   toCommit,
		Privacy:    privacy,
		RepoPath:   cwd,
	})
	if err != nil {
		fatalf("export: %v", err)
	}

	data, err := cgpf.Marshal(doc)
	if err != nil {
		fatalf("marshal: %v", err)
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, data, 0o644); err != nil {
			fatalf("write %s: %v", outFile, err)
		}
		fmt.Fprintf(os.Stdout, "CGPF v%s export written to %s (%d sessions)\n",
			cgpf.Version, outFile, len(doc.Sessions))
	} else {
		fmt.Printf("%s\n", data)
	}
}
