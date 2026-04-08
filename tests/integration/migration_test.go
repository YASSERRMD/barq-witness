package integration

import (
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/store"
)

func TestMigration_FreshDBHasAllTables(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	// Verify key tables exist by performing operations that use them.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions on fresh DB: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions in fresh DB, got %d", len(sessions))
	}

	prompts, err := st.PromptsForSession("nonexistent")
	if err != nil {
		t.Fatalf("PromptsForSession on fresh DB: %v", err)
	}
	if len(prompts) != 0 {
		t.Errorf("expected 0 prompts in fresh DB, got %d", len(prompts))
	}
}

func TestMigration_IsIdempotent(t *testing.T) {
	dir := t.TempDir()

	// First open applies the schema.
	st1, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	st1.Close()

	// Second open should not fail -- migrations are idempotent.
	st2, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer st2.Close()

	sessions, err := st2.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions after second open: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("idempotent migration broke: unexpected sessions")
	}
}

func TestMigration_ThirdOpenStillWorks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trace.db")

	for i := 0; i < 3; i++ {
		st, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		sessions, err := st.AllSessions()
		if err != nil {
			t.Fatalf("AllSessions on open #%d: %v", i+1, err)
		}
		if len(sessions) != 0 {
			t.Errorf("open #%d: expected 0 sessions, got %d", i+1, len(sessions))
		}
		st.Close()
	}
}

func TestMigration_InsertAndReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trace.db")

	// Insert via first handle.
	st1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := st1.InsertSession(sessionFixture("migration-sess")); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	st1.Close()

	// Reopen and verify data is still there.
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer st2.Close()

	sessions, err := st2.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after reopen, got %d", len(sessions))
	}
	if sessions[0].ID != "migration-sess" {
		t.Errorf("session ID = %q, want migration-sess", sessions[0].ID)
	}
}
