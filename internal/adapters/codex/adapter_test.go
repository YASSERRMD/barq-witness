package codex_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/codex"
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
	p := filepath.Join(dir, "codex-log.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// TestImportFromLog_BasicSession verifies that ImportFromLog creates a session
// and imports patches and executions from a Codex CLI JSON log.
func TestImportFromLog_BasicSession(t *testing.T) {
	st := openStore(t)

	exitCode0 := 0
	log := map[string]any{
		"id":         "sess-123",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(1234567890000),
		"patches": []map[string]any{
			{
				"file":        "main.py",
				"patch":       "--- a\n+++ b\n@@ -1 +1 @@\n+print('hi')",
				"timestamp":   int64(1234567891000),
				"accepted_ms": int64(2000),
			},
		},
		"executions": []map[string]any{
			{
				"cmd":         "python -m pytest",
				"exit_code":   exitCode0,
				"duration_ms": int64(3000),
				"timestamp":   int64(1234567892000),
			},
		},
	}

	logPath := writeJSON(t, t.TempDir(), log)

	n, err := codex.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}
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
	if sess.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", sess.ID, "sess-123")
	}
	if sess.Source != "codex" {
		t.Errorf("Source = %q, want %q", sess.Source, "codex")
	}
	if sess.Model != "codex-mini" {
		t.Errorf("Model = %q, want %q", sess.Model, "codex-mini")
	}
	if sess.StartedAt != int64(1234567890000) {
		t.Errorf("StartedAt = %d, want %d", sess.StartedAt, int64(1234567890000))
	}

	edits, err := st.EditsForSession("sess-123")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edit rows = %d, want 1", len(edits))
	}
	if edits[0].FilePath != "main.py" {
		t.Errorf("FilePath = %q, want %q", edits[0].FilePath, "main.py")
	}

	execs, err := st.ExecutionsForSession("sess-123")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("execution rows = %d, want 1", len(execs))
	}
	if execs[0].Command != "python -m pytest" {
		t.Errorf("Command = %q, want %q", execs[0].Command, "python -m pytest")
	}
	if execs[0].ExitCode == nil || *execs[0].ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", execs[0].ExitCode)
	}
	if execs[0].DurationMS == nil || *execs[0].DurationMS != 3000 {
		t.Errorf("DurationMS = %v, want 3000", execs[0].DurationMS)
	}
}

// TestImportFromLog_DuplicateSkipped verifies that re-importing the same log
// is a no-op and does not return an error.
func TestImportFromLog_DuplicateSkipped(t *testing.T) {
	st := openStore(t)

	log := map[string]any{
		"id":         "sess-dup",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(1234567890000),
		"patches":    []map[string]any{},
		"executions": []map[string]any{},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := codex.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if _, err := codex.ImportFromLog(st, logPath); err != nil {
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

// TestImportFromLog_AcceptedSec verifies that accepted_ms is converted into
// accepted seconds encoded in the Tool field.
func TestImportFromLog_AcceptedSec(t *testing.T) {
	st := openStore(t)

	log := map[string]any{
		"id":         "sess-acc",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(1000000000000),
		"patches": []map[string]any{
			{
				"file":        "foo.py",
				"patch":       "+line",
				"timestamp":   int64(1000000001000),
				"accepted_ms": int64(5500),
			},
		},
		"executions": []map[string]any{},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := codex.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	edits, err := st.EditsForSession("sess-acc")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	// accepted_ms=5500 -> accepted_sec=5
	want := "codex-patch(accepted_sec=5)"
	if edits[0].Tool != want {
		t.Errorf("Tool = %q, want %q", edits[0].Tool, want)
	}
}
