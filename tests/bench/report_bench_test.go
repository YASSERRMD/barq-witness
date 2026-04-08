package bench_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/render"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// buildReport creates a synthetic analyzer.Report with n segments.
func buildReport(n int) *analyzer.Report {
	segs := make([]analyzer.Segment, n)
	for i := 0; i < n; i++ {
		segs[i] = analyzer.Segment{
			FilePath:    fmt.Sprintf("file%d.go", i),
			LineStart:   1,
			LineEnd:     20,
			EditID:      int64(i + 1),
			SessionID:   fmt.Sprintf("session-%d", i%10),
			GeneratedAt: time.Now().UnixMilli(),
			AcceptedSec: 3,
			Tier:        (i % 3) + 1,
			ReasonCodes: []string{"NO_EXEC"},
			Score:       float64(i) / float64(n),
		}
	}
	return &analyzer.Report{
		CommitRange:   "abc...def",
		GeneratedAt:   time.Now().UnixMilli(),
		TotalSegments: n,
		Tier1Count:    n / 3,
		Tier2Count:    n / 3,
		Tier3Count:    n - (n/3)*2,
		Segments:      segs,
	}
}

// seedStoreForReport seeds the store but is not used by these benchmarks
// (we build a synthetic report directly to avoid git dependency).
func seedStoreForReport(b *testing.B, dir string, sessions int) *store.Store {
	b.Helper()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	for i := 0; i < sessions; i++ {
		_ = st.InsertSession(model.Session{
			ID:           fmt.Sprintf("report-session-%d", i),
			StartedAt:    time.Now().UnixMilli(),
			CWD:          "/bench",
			GitHeadStart: "abc",
			Model:        "test",
			Source:       "claude-code",
		})
	}
	return st
}

// BenchmarkReportSmall benchmarks text and markdown rendering for 10 segments.
func BenchmarkReportSmall(b *testing.B) {
	report := buildReport(10)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = render.Text(&buf, report, render.TextOptions{})
		buf.Reset()
		_ = render.Markdown(&buf, report, render.MarkdownOptions{})
	}
}

// BenchmarkReportMedium benchmarks text and markdown rendering for 100 segments.
func BenchmarkReportMedium(b *testing.B) {
	report := buildReport(100)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = render.Text(&buf, report, render.TextOptions{})
		buf.Reset()
		_ = render.Markdown(&buf, report, render.MarkdownOptions{})
	}
}
