// Package render converts a Report into human-readable output.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

// ANSI escape codes -- only emitted when writing to a tty.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
)

// TextOptions controls the plain-text renderer.
type TextOptions struct {
	// TopN limits the number of segments shown (0 = all).
	TopN int
	// Color enables ANSI colour codes.
	Color bool
}

// Text writes a plain-text attention map for report to w.
func Text(w io.Writer, report *analyzer.Report, opts TextOptions) error {
	segments := report.Segments
	if opts.TopN > 0 && len(segments) > opts.TopN {
		segments = segments[:opts.TopN]
	}

	// --- Header ---
	tf := func(s string) string {
		if opts.Color {
			return ansiBold + s + ansiReset
		}
		return s
	}
	fmt.Fprintf(w, "%s\n", tf("barq-witness attention map"))
	fmt.Fprintf(w, "commit range : %s\n", report.CommitRange)
	fmt.Fprintf(w, "generated at : %s\n", msToTime(report.GeneratedAt))
	fmt.Fprintf(w, "segments     : %d total  (tier1=%d  tier2=%d  tier3=%d)\n\n",
		report.TotalSegments, report.Tier1Count, report.Tier2Count, report.Tier3Count)

	if len(segments) == 0 {
		fmt.Fprintln(w, "No flagged segments found.")
		writeReasonGlossary(w, opts.Color, nil)
		return nil
	}

	seenReasons := map[string]bool{}

	for i, seg := range segments {
		tierLabel, tierColor := tierInfo(seg.Tier, opts.Color)
		fmt.Fprintf(w, "%s[%d] %s  score=%.0f\n%s\n",
			tierColor, i+1, tierLabel, seg.Score, ansiReset)

		// File + line range
		lineRange := ""
		if seg.LineStart > 0 || seg.LineEnd > 0 {
			lineRange = fmt.Sprintf(":%d-%d", seg.LineStart, seg.LineEnd)
		}
		fmt.Fprintf(w, "  file    : %s%s\n", seg.FilePath, lineRange)

		// One-line summary from reason codes.
		summaries := reasonSummaries(seg)
		for _, s := range summaries {
			fmt.Fprintf(w, "  flag    : %s\n", s)
			for _, r := range seg.ReasonCodes {
				seenReasons[r] = true
			}
		}

		// Prompt snippet.
		if seg.PromptText != "" {
			truncated := truncate(seg.PromptText, 200)
			fmt.Fprintf(w, "  prompt  : %q\n", truncated)
		}

		// Explanation (from explainer layer; falls back to deterministic template).
		if seg.Explanation != "" {
			fmt.Fprintf(w, "  explain : %s\n", seg.Explanation)
		}

		// Timing facts.
		genTime := msToTime(seg.GeneratedAt)
		acceptStr := "unknown"
		if seg.AcceptedSec >= 0 {
			acceptStr = fmt.Sprintf("%ds", seg.AcceptedSec)
		}
		execStr := "executed"
		if !seg.Executed {
			execStr = "never executed"
		}
		fmt.Fprintf(w, "  timing  : generated %s  accepted in %s  %s\n",
			genTime, acceptStr, execStr)

		fmt.Fprintln(w, "")
	}

	// Reason glossary for reasons that appeared.
	writeReasonGlossary(w, opts.Color, seenReasons)
	return nil
}

// ---- helpers ----------------------------------------------------------------

func tierInfo(tier int, color bool) (label, ansiCode string) {
	switch tier {
	case 1:
		label = "TIER 1"
		if color {
			ansiCode = ansiRed
		}
	case 2:
		label = "TIER 2"
		if color {
			ansiCode = ansiYellow
		}
	case 3:
		label = "TIER 3"
		if color {
			ansiCode = ansiDim
		}
	default:
		label = "TIER ?"
	}
	return
}

func reasonSummaries(seg analyzer.Segment) []string {
	var out []string
	for _, r := range seg.ReasonCodes {
		out = append(out, expandTemplate(r, seg))
	}
	return out
}

// expandTemplate fills in the human-readable description for a reason code.
func expandTemplate(code string, seg analyzer.Segment) string {
	switch code {
	case "NO_EXEC":
		return "Generated but never executed locally before commit"
	case "FAST_ACCEPT_SECURITY":
		return fmt.Sprintf("Accepted in %ds in a security-sensitive path", seg.AcceptedSec)
	case "TEST_FAIL_NO_RETEST":
		return "Test failed, code was regenerated, never re-tested"
	case "HIGH_REGEN":
		return fmt.Sprintf("Regenerated %d times in this session -- author may have stopped reading carefully", seg.RegenCount)
	case "NEVER_REOPENED":
		return "File was never opened again after generation"
	case "LARGE_MULTIFILE":
		return fmt.Sprintf("Part of a session that touched %d distinct files", seg.RegenCount)
	case "NEW_DEPENDENCY":
		return fmt.Sprintf("Introduces a new dependency in %s", seg.FilePath)
	case "FAST_ACCEPT_GENERIC":
		return fmt.Sprintf("Accepted in %ds without modification", seg.AcceptedSec)
	case "LONG_GENERATED_BLOCK":
		lines := 0
		for _, line := range strings.Split(seg.PromptText, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				lines++
			}
		}
		return "Single edit added many lines"
	default:
		return code
	}
}

func msToTime(ms int64) string {
	if ms == 0 {
		return "unknown"
	}
	return time.UnixMilli(ms).UTC().Format("15:04:05 UTC")
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "..."
}

func writeReasonGlossary(w io.Writer, color bool, seen map[string]bool) {
	fmt.Fprintln(w, "-- Why was this flagged? --")
	allCodes := []struct{ code, desc string }{
		{"NO_EXEC", "Edit recorded in trace but no execution touched the file before the commit."},
		{"FAST_ACCEPT_SECURITY", "Edit in a security-sensitive path was accepted very quickly (< 5s)."},
		{"TEST_FAIL_NO_RETEST", "A test run failed, code was re-generated, but tests were never re-run."},
		{"HIGH_REGEN", "The same file was edited 4+ times in a 10-minute window."},
		{"NEVER_REOPENED", "The file was never accessed by any subsequent tool after generation."},
		{"LARGE_MULTIFILE", "The session touched more than 10 distinct files -- high cognitive load."},
		{"NEW_DEPENDENCY", "The edit modified a dependency manifest (package.json, go.mod, etc)."},
		{"FAST_ACCEPT_GENERIC", "Edit was accepted in under 3 seconds."},
		{"LONG_GENERATED_BLOCK", "A single tool call added more than 100 lines."},
	}
	for _, item := range allCodes {
		if seen != nil && !seen[item.code] {
			continue
		}
		fmt.Fprintf(w, "  %-26s %s\n", item.code, item.desc)
	}
}
