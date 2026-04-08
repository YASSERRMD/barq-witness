package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/config"
	"github.com/yasserrmd/barq-witness/internal/explainer"
	"github.com/yasserrmd/barq-witness/internal/render"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// runReport implements `barq-witness report [flags]`.
func runReport(args []string) {
	var (
		fromSHA       string
		toSHA         string
		commitSHA     string
		format        string // "text" or "markdown"
		topN          = 10
		explainerName string // "" | null | claude | groq | local
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
		case "--explainer":
			i++
			if i < len(args) {
				explainerName = args[i]
			}
		default:
			if strings.HasPrefix(args[i], "--format=") {
				format = strings.TrimPrefix(args[i], "--format=")
			} else if strings.HasPrefix(args[i], "--explainer=") {
				explainerName = strings.TrimPrefix(args[i], "--explainer=")
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
	witnessDir := filepath.Join(cwd, ".witness")
	dbPath := filepath.Join(witnessDir, "trace.db")
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		cwd = envBase
		witnessDir = filepath.Join(cwd, ".witness")
		dbPath = filepath.Join(witnessDir, "trace.db")
	}

	// Load config.
	cfg, err := config.Load(witnessDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config.toml: %v\n", err)
		cfg = config.Default()
	}

	// CLI flag overrides config file.
	if explainerName != "" {
		cfg.Explainer.Backend = explainerName
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

	// Build the explainer (always constructed; may be the null backend).
	exp := explainer.New(cfg, witnessDir)
	defer exp.Close()

	// Wire the optional intent matcher: a thin adapter around the explainer's
	// IntentMatch method.  Only active when EnableIntentMatching is true.
	analyzeOpts := analyzer.AnalyzeOptions{
		Threshold: cfg.Analyzer.IntentMatchThreshold,
	}
	if cfg.Analyzer.EnableIntentMatching && exp.Name() != "null" {
		analyzeOpts.Matcher = &explainerIntentAdapter{exp: exp}
	}

	// Run the analyzer.
	report, err := analyzer.AnalyzeWithOptions(s, cwd, resolvedFrom, resolvedTo, analyzeOpts)
	if err != nil {
		fatalf("analyze: %v", err)
	}

	// Run the explainer over the segments (always completes even on error).
	explainer.EnrichSegments(context.Background(), exp, report.Segments)

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

// resolveRef resolves a git ref to a SHA using go-git.
func resolveRef(repoPath, _ string) (string, error) {
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

// validateRange prints a user-friendly error when the range is unusable.
func validateRange(from, to string) error {
	if to == "" {
		return fmt.Errorf("could not determine target commit (use --commit or --to)")
	}
	return nil
}

// explainerIntentAdapter wraps an explainer.Explainer to satisfy the
// analyzer.IntentMatcher interface.  It delegates to IntentMatch and translates
// the result so the analyzer stays free of any explainer dependency.
type explainerIntentAdapter struct {
	exp explainer.Explainer
}

func (a *explainerIntentAdapter) Match(ctx context.Context, prompt string, diff string) (float64, string, error) {
	result, err := a.exp.IntentMatch(ctx, prompt, diff)
	if err != nil {
		return 0, "", err
	}
	return result.Score, result.Reasoning, nil
}
