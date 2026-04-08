package compat_test

import (
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/codex"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// TestCodex_ImportFromLog loads the synthetic Codex session fixture and
// verifies that sessions, edits, and executions are inserted correctly.
func TestCodex_ImportFromLog(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	logPath := filepath.Join("..", "fixtures", "session-logs", "codex", "session.json")
	edits, err := codex.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	// The fixture has 2 patches.
	if edits != 2 {
		t.Errorf("expected 2 edits, got %d", edits)
	}

	// Verify session was created.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "codex-compat-session-001" {
		t.Errorf("unexpected session ID: %q", sessions[0].ID)
	}
	if sessions[0].Source != "codex" {
		t.Errorf("expected source=codex, got %q", sessions[0].Source)
	}

	// Verify edits.
	storedEdits, err := st.EditsForSession("codex-compat-session-001")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(storedEdits) != 2 {
		t.Fatalf("expected 2 edit rows, got %d", len(storedEdits))
	}

	// Verify execution.
	execs, err := st.ExecutionsForSession("codex-compat-session-001")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
}

// TestCodex_AdapterSource verifies the codex adapter reports the correct source.
func TestCodex_AdapterSource(t *testing.T) {
	a := codex.New()
	if got := string(a.Source()); got != "codex" {
		t.Errorf("expected source=codex, got %q", got)
	}
}
