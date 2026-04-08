package bench_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// BenchmarkStoreInsertEdit measures the throughput of inserting 1000 edits.
func BenchmarkStoreInsertEdit(b *testing.B) {
	dir := b.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	// Pre-insert the session so edit foreign references are valid.
	if err := st.InsertSession(model.Session{
		ID:           "bench-edit-session",
		StartedAt:    time.Now().UnixMilli(),
		CWD:          "/bench",
		GitHeadStart: "abc",
		Model:        "test",
		Source:       "claude-code",
	}); err != nil {
		b.Fatalf("InsertSession: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_ = st.InsertEdit(model.Edit{
				SessionID: "bench-edit-session",
				Timestamp: time.Now().UnixMilli(),
				FilePath:  fmt.Sprintf("file%d.go", j),
				Tool:      "Write",
				Diff:      "+line",
			})
		}
	}
}

// BenchmarkStoreQuerySessions measures AllSessions on a 1000-session DB.
func BenchmarkStoreQuerySessions(b *testing.B) {
	dir := b.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	// Seed 1000 sessions.
	for i := 0; i < 1000; i++ {
		_ = st.InsertSession(model.Session{
			ID:           fmt.Sprintf("session-%d", i),
			StartedAt:    time.Now().UnixMilli(),
			CWD:          "/bench",
			GitHeadStart: "abc",
			Model:        "test",
			Source:       "claude-code",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = st.AllSessions()
	}
}

// BenchmarkStoreEditsForSession measures EditsForSession on a session with 1000 edits.
func BenchmarkStoreEditsForSession(b *testing.B) {
	dir := b.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	const sid = "bench-heavy-session"
	_ = st.InsertSession(model.Session{
		ID:           sid,
		StartedAt:    time.Now().UnixMilli(),
		CWD:          "/bench",
		GitHeadStart: "abc",
		Model:        "test",
		Source:       "claude-code",
	})

	// Insert 1000 edits for this session.
	for j := 0; j < 1000; j++ {
		_ = st.InsertEdit(model.Edit{
			SessionID: sid,
			Timestamp: time.Now().UnixMilli(),
			FilePath:  fmt.Sprintf("file%d.go", j),
			Tool:      "Write",
			Diff:      "+line",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = st.EditsForSession(sid)
	}
}
