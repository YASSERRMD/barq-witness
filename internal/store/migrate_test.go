package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
	_ "modernc.org/sqlite"
)

// openRawDB opens a bare SQLite database (no schema, no migrate) for
// testing the Migrate function directly.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "raw.db"))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestMigrate_CleanDB verifies Migrate applies cleanly to an empty database.
func TestMigrate_CleanDB(t *testing.T) {
	db := openRawDB(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate on clean db: %v", err)
	}

	// Verify meta table exists and has schema_version = 2.
	var ver string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&ver)
	if err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if ver != "2" {
		t.Errorf("schema_version = %q, want %q", ver, "2")
	}

	// Verify intent_matches table exists.
	var name string
	err = db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='intent_matches'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("check intent_matches table: %v", err)
	}
	if name != "intent_matches" {
		t.Errorf("intent_matches table not created")
	}
}

// TestMigrate_Idempotent verifies that calling Migrate twice is safe.
func TestMigrate_Idempotent(t *testing.T) {
	db := openRawDB(t)

	if err := store.Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("second Migrate (idempotent): %v", err)
	}
}

// TestUpsertAndReadIntentMatch verifies UpsertIntentMatch and IntentMatchForEdit.
func TestUpsertAndReadIntentMatch(t *testing.T) {
	s := newTestStore(t)

	// Must insert a session and edit first due to FK on edits table.
	// The intent_matches table only requires edit_id to exist in edits.
	// Insert the parent session.
	if err := s.InsertSession(model.Session{ID: "sess-im1", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	// Insert an edit.
	if err := s.InsertEdit(model.Edit{
		SessionID:  "sess-im1",
		Timestamp:  nowMS(),
		FilePath:   "main.go",
		Tool:       "Write",
		BeforeHash: "bh",
		AfterHash:  "ah",
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	// Retrieve the edit ID.
	edits, err := s.EditsForFiles([]string{"main.go"})
	if err != nil || len(edits) == 0 {
		t.Fatalf("EditsForFiles: %v (len=%d)", err, len(edits))
	}
	editID := edits[0].ID

	// No record yet.
	_, ok, err := s.IntentMatchForEdit(editID)
	if err != nil {
		t.Fatalf("IntentMatchForEdit (none): %v", err)
	}
	if ok {
		t.Fatal("expected no record before upsert")
	}

	// Upsert.
	if err := s.UpsertIntentMatch(editID, 0.8, "matches the prompt well", "claude-3-5-sonnet"); err != nil {
		t.Fatalf("UpsertIntentMatch: %v", err)
	}

	// Read back.
	im, ok, err := s.IntentMatchForEdit(editID)
	if err != nil {
		t.Fatalf("IntentMatchForEdit (after upsert): %v", err)
	}
	if !ok {
		t.Fatal("expected record after upsert")
	}
	if im.EditID != editID {
		t.Errorf("EditID = %d, want %d", im.EditID, editID)
	}
	if im.Score != 0.8 {
		t.Errorf("Score = %v, want 0.8", im.Score)
	}
	if im.Reasoning != "matches the prompt well" {
		t.Errorf("Reasoning = %q, want %q", im.Reasoning, "matches the prompt well")
	}
	if im.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = %q, want %q", im.Model, "claude-3-5-sonnet")
	}
	if im.ComputedAt <= 0 {
		t.Errorf("ComputedAt should be positive, got %d", im.ComputedAt)
	}
}

// TestUpsertIntentMatch_Update verifies that a second upsert overwrites the record.
func TestUpsertIntentMatch_Update(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertSession(model.Session{ID: "sess-im2", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	if err := s.InsertEdit(model.Edit{
		SessionID:  "sess-im2",
		Timestamp:  nowMS(),
		FilePath:   "util.go",
		Tool:       "Write",
		BeforeHash: "bh",
		AfterHash:  "ah",
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	edits, err := s.EditsForFiles([]string{"util.go"})
	if err != nil || len(edits) == 0 {
		t.Fatalf("EditsForFiles: %v", err)
	}
	editID := edits[0].ID

	if err := s.UpsertIntentMatch(editID, 0.3, "first reasoning", "model-a"); err != nil {
		t.Fatalf("first UpsertIntentMatch: %v", err)
	}

	// Sleep briefly so computed_at changes.
	time.Sleep(2 * time.Millisecond)

	if err := s.UpsertIntentMatch(editID, 0.7, "updated reasoning", "model-b"); err != nil {
		t.Fatalf("second UpsertIntentMatch: %v", err)
	}

	im, ok, err := s.IntentMatchForEdit(editID)
	if err != nil || !ok {
		t.Fatalf("IntentMatchForEdit after second upsert: %v ok=%v", err, ok)
	}
	if im.Score != 0.7 {
		t.Errorf("Score after update = %v, want 0.7", im.Score)
	}
	if im.Model != "model-b" {
		t.Errorf("Model after update = %q, want %q", im.Model, "model-b")
	}
}

// TestIntentMatchForEdit_NotFound verifies that a missing record returns ok=false.
func TestIntentMatchForEdit_NotFound(t *testing.T) {
	s := newTestStore(t)

	im, ok, err := s.IntentMatchForEdit(99999)
	if err != nil {
		t.Fatalf("IntentMatchForEdit (not found): %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing edit, got record: %+v", im)
	}
}
