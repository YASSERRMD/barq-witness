package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func nowMS() int64 { return time.Now().UnixMilli() }

// TestOpenClose verifies the store opens on a fresh path and closes cleanly.
func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".witness", "trace.db")

	// File must not exist before open.
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("expected db file to not exist yet")
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// File must exist after open.
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSchemaIdempotent verifies that opening the same path twice does not fail.
func TestSchemaIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".witness", "trace.db")

	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("second Open (idempotent): %v", err)
	}
	s2.Close()
}

// TestInsertAndReadSession covers InsertSession and EndSession.
func TestInsertAndReadSession(t *testing.T) {
	s := newTestStore(t)

	sess := model.Session{
		ID:           "sess-001",
		StartedAt:    nowMS(),
		CWD:          "/tmp/repo",
		GitHeadStart: "abc123",
		Model:        "claude-3-5-sonnet",
	}
	if err := s.InsertSession(sess); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	endedAt := nowMS()
	if err := s.EndSession("sess-001", endedAt, "def456"); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

// TestEndSessionNotFound verifies EndSession returns error for unknown ID.
func TestEndSessionNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.EndSession("no-such-session", nowMS(), ""); err == nil {
		t.Fatal("expected error for missing session")
	}
}

// TestInsertAndReadPrompt covers InsertPrompt and LatestPromptForSession.
func TestInsertAndReadPrompt(t *testing.T) {
	s := newTestStore(t)

	// Insert parent session first (FK constraint).
	if err := s.InsertSession(model.Session{ID: "sess-p1", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	p1 := model.Prompt{
		SessionID:   "sess-p1",
		Timestamp:   nowMS(),
		Content:     "write a hello world",
		ContentHash: "aabbcc",
	}
	id1, err := s.InsertPrompt(p1)
	if err != nil {
		t.Fatalf("InsertPrompt: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("expected positive id, got %d", id1)
	}

	// Insert a second, later prompt.
	p2 := model.Prompt{
		SessionID:   "sess-p1",
		Timestamp:   nowMS() + 1000,
		Content:     "add error handling",
		ContentHash: "ddeeff",
	}
	id2, err := s.InsertPrompt(p2)
	if err != nil {
		t.Fatalf("InsertPrompt p2: %v", err)
	}

	latest, err := s.LatestPromptForSession("sess-p1")
	if err != nil {
		t.Fatalf("LatestPromptForSession: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a prompt, got nil")
	}
	if latest.ID != id2 {
		t.Errorf("expected latest id=%d, got %d", id2, latest.ID)
	}
}

// TestLatestPromptNoneExists verifies nil is returned when no prompts exist.
func TestLatestPromptNoneExists(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-empty", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	got, err := s.LatestPromptForSession("sess-empty")
	if err != nil {
		t.Fatalf("LatestPromptForSession: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// TestInsertAndReadEdit covers InsertEdit and EditsForFiles.
func TestInsertAndReadEdit(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertSession(model.Session{ID: "sess-e1", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	lineStart, lineEnd := 10, 20
	e := model.Edit{
		SessionID:  "sess-e1",
		Timestamp:  nowMS(),
		FilePath:   "internal/foo/bar.go",
		Tool:       "Edit",
		BeforeHash: "hash-before",
		AfterHash:  "hash-after",
		LineStart:  &lineStart,
		LineEnd:    &lineEnd,
		Diff:       "@@ -10,3 +10,5 @@\n+new line\n",
	}
	if err := s.InsertEdit(e); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	edits, err := s.EditsForFiles([]string{"internal/foo/bar.go"})
	if err != nil {
		t.Fatalf("EditsForFiles: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].FilePath != "internal/foo/bar.go" {
		t.Errorf("wrong file path: %s", edits[0].FilePath)
	}
	if edits[0].BeforeHash != "hash-before" {
		t.Errorf("wrong before_hash: %s", edits[0].BeforeHash)
	}
}

// TestEditsForFilesEmpty verifies an empty slice is returned for no matches.
func TestEditsForFilesEmpty(t *testing.T) {
	s := newTestStore(t)
	edits, err := s.EditsForFiles([]string{"nonexistent.go"})
	if err != nil {
		t.Fatalf("EditsForFiles: %v", err)
	}
	if len(edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(edits))
	}
}

// TestEditsForFilesNilInput verifies nil input is safe.
func TestEditsForFilesNilInput(t *testing.T) {
	s := newTestStore(t)
	edits, err := s.EditsForFiles(nil)
	if err != nil {
		t.Fatalf("EditsForFiles(nil): %v", err)
	}
	if edits != nil {
		t.Fatalf("expected nil, got %v", edits)
	}
}

// TestInsertAndReadExecution covers InsertExecution and ExecutionsForSession.
func TestInsertAndReadExecution(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertSession(model.Session{ID: "sess-x1", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	exitCode := 0
	durMS := int64(123)
	x := model.Execution{
		SessionID:      "sess-x1",
		Timestamp:      nowMS(),
		Command:        "go test ./...",
		Classification: "test",
		FilesTouched:   `["internal/store/store_test.go"]`,
		ExitCode:       &exitCode,
		DurationMS:     &durMS,
	}
	if err := s.InsertExecution(x); err != nil {
		t.Fatalf("InsertExecution: %v", err)
	}

	execs, err := s.ExecutionsForSession("sess-x1")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Command != "go test ./..." {
		t.Errorf("wrong command: %s", execs[0].Command)
	}
	if execs[0].Classification != "test" {
		t.Errorf("wrong classification: %s", execs[0].Classification)
	}
}

// TestForeignKeyLinkage verifies that an edit can be linked to a prompt via prompt_id.
func TestForeignKeyLinkage(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertSession(model.Session{ID: "sess-fk", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	promptID, err := s.InsertPrompt(model.Prompt{
		SessionID:   "sess-fk",
		Timestamp:   nowMS(),
		Content:     "generate auth middleware",
		ContentHash: "abc",
	})
	if err != nil {
		t.Fatalf("InsertPrompt: %v", err)
	}

	if err := s.InsertEdit(model.Edit{
		SessionID: "sess-fk",
		PromptID:  &promptID,
		Timestamp: nowMS(),
		FilePath:  "middleware/auth.go",
		Tool:      "Write",
	}); err != nil {
		t.Fatalf("InsertEdit with prompt_id: %v", err)
	}

	edits, err := s.EditsForFiles([]string{"middleware/auth.go"})
	if err != nil {
		t.Fatalf("EditsForFiles: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].PromptID == nil || *edits[0].PromptID != promptID {
		t.Errorf("expected prompt_id=%d, got %v", promptID, edits[0].PromptID)
	}
}
