package compat_test

import (
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/cursor"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// TestCursor_ImportFromLog loads the synthetic Cursor session fixture and
// verifies that sessions, edits, and executions are inserted correctly.
func TestCursor_ImportFromLog(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	logPath := filepath.Join("..", "fixtures", "session-logs", "cursor", "session.json")
	edits, err := cursor.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	// The fixture has 2 accepted edits and 1 rejected edit.
	if edits != 2 {
		t.Errorf("expected 2 accepted edits, got %d", edits)
	}

	// Verify session was created.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "cursor-compat-session-001" {
		t.Errorf("unexpected session ID: %q", sessions[0].ID)
	}
	if sessions[0].Source != "cursor" {
		t.Errorf("expected source=cursor, got %q", sessions[0].Source)
	}

	// Verify edits.
	storedEdits, err := st.EditsForSession("cursor-compat-session-001")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(storedEdits) != 2 {
		t.Fatalf("expected 2 edit rows, got %d", len(storedEdits))
	}

	// Verify execution was created.
	execs, err := st.ExecutionsForSession("cursor-compat-session-001")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Command != "go build ./..." {
		t.Errorf("unexpected command: %q", execs[0].Command)
	}
}

// TestCursor_ImportIdempotent verifies that importing the same log twice does
// not insert duplicate rows (the second import is a no-op).
func TestCursor_ImportIdempotent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	logPath := filepath.Join("..", "fixtures", "session-logs", "cursor", "session.json")

	if _, err := cursor.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("first import: %v", err)
	}
	// Second import should be a no-op (session already exists).
	n, err := cursor.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 edits on second import, got %d", n)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session after two imports, got %d", len(sessions))
	}
}
