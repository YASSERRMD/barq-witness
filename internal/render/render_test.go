package render_test

import (
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/render"
)

// fixtureReport returns a deterministic report for golden-file tests.
func fixtureReport() *analyzer.Report {
	pID := int64(1)
	return &analyzer.Report{
		CommitRange:   "abc123..def456",
		GeneratedAt:   1700000000000,
		TotalSegments: 2,
		Tier1Count:    1,
		Tier2Count:    1,
		Tier3Count:    0,
		Segments: []analyzer.Segment{
			{
				FilePath:    "internal/auth/handler.go",
				LineStart:   10,
				LineEnd:     25,
				EditID:      1,
				SessionID:   "sess-001",
				PromptID:    &pID,
				PromptText:  "rewrite auth middleware to use JWT",
				GeneratedAt: 1700000000000,
				AcceptedSec: 3,
				Executed:    false,
				Reopened:    false,
				SecurityHit: true,
				NewDep:      false,
				Tier:        1,
				ReasonCodes: []string{"NO_EXEC", "FAST_ACCEPT_SECURITY"},
				Score:       200,
			},
			{
				FilePath:    "pkg/service/user.go",
				LineStart:   50,
				LineEnd:     80,
				EditID:      2,
				SessionID:   "sess-001",
				PromptID:    nil,
				PromptText:  "",
				GeneratedAt: 1700000000000,
				AcceptedSec: -1,
				Executed:    false,
				Reopened:    false,
				SecurityHit: false,
				NewDep:      false,
				RegenCount:  5,
				Tier:        2,
				ReasonCodes: []string{"HIGH_REGEN"},
				Score:       50,
			},
		},
	}
}

// --- Text renderer tests ----------------------------------------------------

func TestText_ContainsHeader(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	if err := render.Text(&buf, report, render.TextOptions{}); err != nil {
		t.Fatalf("Text: %v", err)
	}
	out := buf.String()
	mustContain(t, out, "barq-witness attention map")
	mustContain(t, out, "abc123..def456")
	mustContain(t, out, "2 total")
}

func TestText_ContainsTierLabels(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{})
	out := buf.String()
	mustContain(t, out, "TIER 1")
	mustContain(t, out, "TIER 2")
}

func TestText_ContainsFilePath(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{})
	out := buf.String()
	mustContain(t, out, "internal/auth/handler.go")
	mustContain(t, out, "pkg/service/user.go")
}

func TestText_ContainsReasonSummary(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{})
	out := buf.String()
	mustContain(t, out, "never executed")
	mustContain(t, out, "security-sensitive")
}

func TestText_ContainsPromptSnippet(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{})
	out := buf.String()
	mustContain(t, out, "rewrite auth middleware")
}

func TestText_ContainsGlossary(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{})
	out := buf.String()
	mustContain(t, out, "Why was this flagged?")
	mustContain(t, out, "NO_EXEC")
	mustContain(t, out, "HIGH_REGEN")
}

func TestText_TopNLimitsOutput(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Text(&buf, report, render.TextOptions{TopN: 1})
	out := buf.String()
	mustContain(t, out, "internal/auth/handler.go")
	if strings.Contains(out, "pkg/service/user.go") {
		t.Error("TopN=1 should exclude the second segment")
	}
}

func TestText_EmptyReport(t *testing.T) {
	report := &analyzer.Report{
		CommitRange: "a..b",
		GeneratedAt: 1700000000000,
	}
	var buf strings.Builder
	if err := render.Text(&buf, report, render.TextOptions{}); err != nil {
		t.Fatalf("Text empty: %v", err)
	}
	mustContain(t, buf.String(), "No flagged segments found.")
}

func TestText_Deterministic(t *testing.T) {
	report := fixtureReport()
	var b1, b2 strings.Builder
	render.Text(&b1, report, render.TextOptions{})
	render.Text(&b2, report, render.TextOptions{})
	if b1.String() != b2.String() {
		t.Error("Text output is not deterministic")
	}
}

// --- Markdown renderer tests ------------------------------------------------

func TestMarkdown_ContainsMarker(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	if err := render.Markdown(&buf, report, render.MarkdownOptions{}); err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	mustContain(t, buf.String(), render.CommentMarker)
}

func TestMarkdown_ContainsHeading(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Markdown(&buf, report, render.MarkdownOptions{})
	mustContain(t, buf.String(), "# barq-witness attention map")
}

func TestMarkdown_ContainsSummaryTable(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Markdown(&buf, report, render.MarkdownOptions{})
	out := buf.String()
	mustContain(t, out, "abc123..def456")
	mustContain(t, out, "Tier 1 (critical)")
}

func TestMarkdown_ContainsSegmentHeadings(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Markdown(&buf, report, render.MarkdownOptions{})
	out := buf.String()
	mustContain(t, out, "###")
	mustContain(t, out, "internal/auth/handler.go")
}

func TestMarkdown_ContainsGlossary(t *testing.T) {
	report := fixtureReport()
	var buf strings.Builder
	render.Markdown(&buf, report, render.MarkdownOptions{})
	out := buf.String()
	mustContain(t, out, "Why was this flagged?")
	mustContain(t, out, "NO_EXEC")
}

func TestMarkdown_Deterministic(t *testing.T) {
	report := fixtureReport()
	var b1, b2 strings.Builder
	render.Markdown(&b1, report, render.MarkdownOptions{})
	render.Markdown(&b2, report, render.MarkdownOptions{})
	if b1.String() != b2.String() {
		t.Error("Markdown output is not deterministic")
	}
}

func TestMarkdown_EmptyReport(t *testing.T) {
	report := &analyzer.Report{
		CommitRange: "a..b",
		GeneratedAt: 1700000000000,
	}
	var buf strings.Builder
	if err := render.Markdown(&buf, report, render.MarkdownOptions{}); err != nil {
		t.Fatalf("Markdown empty: %v", err)
	}
	mustContain(t, buf.String(), "No flagged segments found")
}

// ---- helpers ----------------------------------------------------------------

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("output does not contain %q\n--- output ---\n%s", sub, s)
	}
}
