package store_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// openBenchStore opens a fresh SQLite store for benchmarks.
func openBenchStore(b *testing.B) *store.Store {
	b.Helper()
	s, err := store.Open(filepath.Join(b.TempDir(), "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

// BenchmarkInsertEdit benchmarks inserting 1000 edits sequentially.
func BenchmarkInsertEdit(b *testing.B) {
	s := openBenchStore(b)

	// Insert the session once (outside the timed region).
	sessID := "sess-bench-insert"
	if err := s.InsertSession(model.Session{
		ID:        sessID,
		StartedAt: time.Now().UnixMilli(),
	}); err != nil {
		b.Fatalf("InsertSession: %v", err)
	}

	ts := time.Now().UnixMilli()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % 1000
		ls := idx + 1
		le := idx + 1
		if err := s.InsertEdit(model.Edit{
			SessionID:  sessID,
			Timestamp:  ts + int64(i),
			FilePath:   fmt.Sprintf("file%d.go", idx),
			Tool:       "Write",
			BeforeHash: fmt.Sprintf("before%d", idx),
			AfterHash:  fmt.Sprintf("after%d", idx),
			LineStart:  &ls,
			LineEnd:    &le,
		}); err != nil {
			b.Fatalf("InsertEdit: %v", err)
		}
	}
}

// BenchmarkEditsForSession benchmarks querying edits for a session with 1000 rows.
func BenchmarkEditsForSession(b *testing.B) {
	s := openBenchStore(b)

	// Seed the session and 1000 edits before timing starts.
	sessID := "sess-bench-query"
	if err := s.InsertSession(model.Session{
		ID:        sessID,
		StartedAt: time.Now().UnixMilli(),
	}); err != nil {
		b.Fatalf("InsertSession: %v", err)
	}
	ts := time.Now().UnixMilli()
	for i := 0; i < 1000; i++ {
		ls := i + 1
		le := i + 1
		if err := s.InsertEdit(model.Edit{
			SessionID:  sessID,
			Timestamp:  ts + int64(i),
			FilePath:   fmt.Sprintf("file%d.go", i),
			Tool:       "Write",
			BeforeHash: fmt.Sprintf("before%d", i),
			AfterHash:  fmt.Sprintf("after%d", i),
			LineStart:  &ls,
			LineEnd:    &le,
		}); err != nil {
			b.Fatalf("InsertEdit setup: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edits, err := s.EditsForSession(sessID)
		if err != nil {
			b.Fatalf("EditsForSession: %v", err)
		}
		_ = edits
	}
}
