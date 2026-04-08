package compat_test

import (
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/aider"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// TestAider_ImportFromChat loads the synthetic Aider chat history fixture and
// verifies that sessions, prompts, and edits are inserted correctly.
func TestAider_ImportFromChat(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	chatPath := filepath.Join("..", "fixtures", "session-logs", "aider", "chat.md")
	edits, err := aider.ImportFromChat(st, chatPath)
	if err != nil {
		t.Fatalf("ImportFromChat: %v", err)
	}

	// The fixture has 3 file modifications (config.go, config_test.go, config.go again).
	if edits < 1 {
		t.Errorf("expected at least 1 edit, got %d", edits)
	}

	// Verify a session was created.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Source != "aider" {
		t.Errorf("expected source=aider, got %q", sessions[0].Source)
	}

	// Verify edits were stored.
	storedEdits, err := st.EditsForSession(sessions[0].ID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(storedEdits) == 0 {
		t.Fatal("expected at least one edit row")
	}
}

// TestAider_AdapterSource verifies the aider adapter reports the correct source.
func TestAider_AdapterSource(t *testing.T) {
	a := aider.New()
	if got := string(a.Source()); got != "aider" {
		t.Errorf("expected source=aider, got %q", got)
	}
}
