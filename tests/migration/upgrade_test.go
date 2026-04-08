package migration_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

const fixtureV10 = "fixtures/v1.0/trace.db"

// copyFixture copies the v1.0 fixture DB into dir and returns the path.
func copyFixture(t *testing.T, dir string) string {
	t.Helper()
	src, err := os.Open(fixtureV10)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer src.Close()

	dst := filepath.Join(dir, "trace.db")
	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create dst: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, src); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

// TestUpgrade_V10ToCurrentClean copies the v1.0 fixture, opens it with
// store.Open (which runs all migrations), and checks that no data was lost
// and that migration-added columns have sensible defaults.
func TestUpgrade_V10ToCurrentClean(t *testing.T) {
	dir := t.TempDir()
	dbPath := copyFixture(t, dir)

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open after migration: %v", err)
	}
	defer st.Close()

	// Verify no data loss: AllSessions returns the one fixture session.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sess := sessions[0]
	if sess.ID != "fixture-session-001" {
		t.Errorf("unexpected session ID: %q", sess.ID)
	}

	// Verify the source column has the default value after migration.
	if sess.Source != "claude-code" {
		t.Errorf("expected source='claude-code', got %q", sess.Source)
	}

	// Verify prompts for session still work.
	prompts, err := st.PromptsForSession(sess.ID)
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Content != "Add a hello world function" {
		t.Errorf("unexpected prompt content: %q", prompts[0].Content)
	}

	// Verify intent_matches table exists by calling UpsertIntentMatch.
	edits, err := st.EditsForSession(sess.ID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) == 0 {
		t.Fatal("expected at least one edit after migration")
	}
	if err := st.UpsertIntentMatch(edits[0].ID, 0.9, "test reasoning", "test-model"); err != nil {
		t.Fatalf("UpsertIntentMatch (intent_matches table missing?): %v", err)
	}
}

// TestUpgrade_IsIdempotent opens the migrated DB a second time and verifies
// no error occurs and the data is unchanged.
func TestUpgrade_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := copyFixture(t, dir)

	// First open: runs all migrations.
	st1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("first store.Open: %v", err)
	}
	sessions1, err := st1.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions (first open): %v", err)
	}
	st1.Close()

	// Second open: migrations are idempotent.
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("second store.Open: %v", err)
	}
	defer st2.Close()

	sessions2, err := st2.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions (second open): %v", err)
	}

	if len(sessions1) != len(sessions2) {
		t.Errorf("session count changed between opens: %d -> %d", len(sessions1), len(sessions2))
	}
	if len(sessions2) > 0 && sessions2[0].ID != sessions1[0].ID {
		t.Errorf("session IDs changed between opens")
	}
}

// TestUpgrade_NewInsertsWork verifies that after migration, inserting new
// rows with current-schema fields (including source) works correctly.
func TestUpgrade_NewInsertsWork(t *testing.T) {
	dir := t.TempDir()
	dbPath := copyFixture(t, dir)

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	newSess := model.Session{
		ID:           "new-session-cursor",
		StartedAt:    time.Now().UnixMilli(),
		CWD:          "/tmp/new",
		GitHeadStart: "aaa000",
		Model:        "gpt-4",
		Source:       "cursor",
	}
	if err := st.InsertSession(newSess); err != nil {
		t.Fatalf("InsertSession with source=cursor: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}

	var found *model.Session
	for i := range sessions {
		if sessions[i].ID == "new-session-cursor" {
			found = &sessions[i]
			break
		}
	}
	if found == nil {
		t.Fatal("newly inserted session not found")
	}
	if found.Source != "cursor" {
		t.Errorf("expected source='cursor', got %q", found.Source)
	}
}
