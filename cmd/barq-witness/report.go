package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/render"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// runReport implements `barq-witness report [flags]`.
func runReport(args []string) {
	var (
		fromSHA   string
		toSHA     string
		commitSHA string
		format    string // "text" or "markdown"
		topN      = 10
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			i++
			if i < len(args) {
				fromSHA = args[i]
			}
		case "--to":
			i++
			if i < len(args) {
				toSHA = args[i]
			}
		case "--commit":
			i++
			if i < len(args) {
				commitSHA = args[i]
			}
		case "--format":
			i++
			if i < len(args) {
				format = args[i]
			}
		case "--top":
			i++
			if i < len(args) {
				n, err := strconv.Atoi(args[i])
				if err == nil && n > 0 {
					topN = n
				}
			}
		default:
			if strings.HasPrefix(args[i], "--format=") {
				format = strings.TrimPrefix(args[i], "--format=")
			}
		}
	}

	// Resolve commit range.
	if commitSHA != "" {
		toSHA = commitSHA
		fromSHA = ""
	}
	if toSHA == "" {
		toSHA = "HEAD"
	}

	// Resolve repo and DB paths.
	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cannot determine working directory: %v", err)
	}
	dbPath := filepath.Join(cwd, ".witness", "trace.db")
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		dbPath = filepath.Join(envBase, ".witness", "trace.db")
		cwd = envBase
	}

	// Resolve HEAD reference to a real SHA when the user passes "HEAD".
	resolvedTo, err := resolveRef(cwd, toSHA)
	if err != nil || resolvedTo == "" {
		resolvedTo = toSHA
	}
	resolvedFrom := fromSHA
	if fromSHA != "" {
		if r, err := resolveRef(cwd, fromSHA); err == nil && r != "" {
			resolvedFrom = r
		}
	}

	// Open trace store.
	s, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace store: %v", err)
	}
	defer s.Close()

	// Run the analyzer.
	report, err := analyzer.Analyze(s, cwd, resolvedFrom, resolvedTo)
	if err != nil {
		fatalf("analyze: %v", err)
	}

	// Choose format.
	if format == "" {
		if isTTY() {
			format = "text"
		} else {
			format = "markdown"
		}
	}

	switch format {
	case "text":
		if err := render.Text(os.Stdout, report, render.TextOptions{
			TopN:  topN,
			Color: isTTY(),
		}); err != nil {
			fatalf("render text: %v", err)
		}
	case "markdown":
		if err := render.Markdown(os.Stdout, report, render.MarkdownOptions{
			TopN: topN,
		}); err != nil {
			fatalf("render markdown: %v", err)
		}
	default:
		fatalf("unknown format %q (use text or markdown)", format)
	}
}

// resolveRef resolves a git ref (e.g. HEAD, branch name) to a SHA using go-git.
func resolveRef(repoPath, ref string) (string, error) {
	return util.HeadSHA(repoPath)
}

// isTTY returns true when stdout is attached to a terminal.
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// resolveCommitRange prints a user-friendly error when the range is unusable.
func validateRange(from, to string) error {
	if to == "" {
		return fmt.Errorf("could not determine target commit (use --commit or --to)")
	}
	return nil
}
