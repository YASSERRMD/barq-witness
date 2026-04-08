package bench_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

func seedStore(b *testing.B, st *store.Store, sessions, editsPerSession int) {
	b.Helper()
	for i := 0; i < sessions; i++ {
		sid := fmt.Sprintf("bench-session-%d", i)
		if err := st.InsertSession(model.Session{
			ID:           sid,
			StartedAt:    time.Now().UnixMilli(),
			CWD:          "/bench",
			GitHeadStart: "abc123",
			Model:        "claude-test",
			Source:       "claude-code",
		}); err != nil {
			b.Fatalf("InsertSession: %v", err)
		}
		for j := 0; j < editsPerSession; j++ {
			if err := st.InsertEdit(model.Edit{
				SessionID: sid,
				Timestamp: time.Now().UnixMilli(),
				FilePath:  fmt.Sprintf("file%d.go", j),
				Tool:      "Write",
				Diff:      "+func f() {}",
			}); err != nil {
				b.Fatalf("InsertEdit: %v", err)
			}
		}
	}
}

// BenchmarkAnalyzeSmall benchmarks analysis over 10 sessions x 100 edits = 1000 edits.
func BenchmarkAnalyzeSmall(b *testing.B) {
	dir := b.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	seedStore(b, st, 10, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.Analyze(st, dir, "", "HEAD")
	}
}

// BenchmarkAnalyzeMedium benchmarks analysis over 100 sessions x 100 edits = 10000 edits.
func BenchmarkAnalyzeMedium(b *testing.B) {
	dir := b.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	seedStore(b, st, 100, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.Analyze(st, dir, "", "HEAD")
	}
}
