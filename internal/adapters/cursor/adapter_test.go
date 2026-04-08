package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/cursor"
	"github.com/yasserrmd/barq-witness/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func writeJSON(t *testing.T, dir string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	p := filepath.Join(dir, "cursor-log.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// TestImportFromLog_BasicSession verifies that ImportFromLog creates a session
// and imports accepted edits from a Cursor JSON log.
func TestImportFromLog_BasicSession(t *testing.T) {
	st := openStore(t)

	exitCode0 := 0
	log := map[string]any{
		"session_id": "cursor-sess-001",
		"model":      "gpt-4",
		"cwd":        "/project",
		"events": []map[string]any{
			{
				"type":            "edit",
				"file":            "src/main.ts",
				"diff":            "--- a\n+++ b\n@@ -1 +1 @@\n+export {};",
				"timestamp":       int64(1234567890000),
				"accepted":        true,
				"accept_delay_ms": int64(3000),
			},
			{
				"type":            "edit",
				"file":            "src/other.ts",
				"diff":            "--- a\n+++ b",
				"timestamp":       int64(1234567891000),
				"accepted":        false,
				"accept_delay_ms": int64(0),
			},
			{
				"type":        "command",
				"command":     "npm test",
				"exit_code":   exitCode0,
				"duration_ms": int64(5000),
				"timestamp":   int64(1234567892000),
			},
		},
	}

	logPath := writeJSON(t, t.TempDir(), log)

	n, err := cursor.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}
	// Only one accepted edit; the rejected edit should be skipped.
	if n != 1 {
		t.Errorf("edits imported = %d, want 1", n)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions))
	}

	sess := sessions[0]
	if sess.ID != "cursor-sess-001" {
		t.Errorf("session ID = %q, want %q", sess.ID, "cursor-sess-001")
	}
	if sess.Source != "cursor" {
		t.Errorf("Source = %q, want %q", sess.Source, "cursor")
	}
	if sess.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", sess.Model, "gpt-4")
	}
	if sess.CWD != "/project" {
		t.Errorf("CWD = %q, want %q", sess.CWD, "/project")
	}

	edits, err := st.EditsForSession("cursor-sess-001")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edit rows = %d, want 1", len(edits))
	}
	if edits[0].FilePath != "src/main.ts" {
		t.Errorf("FilePath = %q, want %q", edits[0].FilePath, "src/main.ts")
	}

	execs, err := st.ExecutionsForSession("cursor-sess-001")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("execution rows = %d, want 1", len(execs))
	}
	if execs[0].Command != "npm test" {
		t.Errorf("Command = %q, want %q", execs[0].Command, "npm test")
	}
}

// TestImportFromLog_DuplicateSkipped verifies that re-importing the same log
// is a no-op and does not return an error.
func TestImportFromLog_DuplicateSkipped(t *testing.T) {
	st := openStore(t)

	log := map[string]any{
		"session_id": "cursor-dup-001",
		"model":      "gpt-4",
		"cwd":        "/project",
		"events":     []map[string]any{},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := cursor.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("first import: %v", err)
	}
	// Second import of the same session_id must not error.
	if _, err := cursor.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("second import (duplicate): %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("sessions = %d, want 1 (duplicate skipped)", len(sessions))
	}
}

// TestImportFromLog_AcceptedSec verifies that accept_delay_ms is converted
// into accepted seconds encoded in the Tool field.
func TestImportFromLog_AcceptedSec(t *testing.T) {
	st := openStore(t)

	log := map[string]any{
		"session_id": "cursor-acc-001",
		"model":      "gpt-4",
		"cwd":        "/project",
		"events": []map[string]any{
			{
				"type":            "edit",
				"file":            "app.ts",
				"diff":            "+line",
				"timestamp":       int64(1000000000000),
				"accepted":        true,
				"accept_delay_ms": int64(7500),
			},
		},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	n, err := cursor.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}
	if n != 1 {
		t.Fatalf("edits = %d, want 1", n)
	}

	edits, err := st.EditsForSession("cursor-acc-001")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	// accept_delay_ms=7500 -> accepted_sec=7
	want := "cursor-edit(accepted_sec=7)"
	if edits[0].Tool != want {
		t.Errorf("Tool = %q, want %q", edits[0].Tool, want)
	}
}
