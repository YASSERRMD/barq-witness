package codex_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/adapters/codex"
)

// TestNew_Source verifies the Adapter stubs return correct values.
func TestNew_Source(t *testing.T) {
	a := codex.New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
	if a.Source() != adapters.SourceCodex {
		t.Errorf("Source() = %v, want SourceCodex", a.Source())
	}
}

// TestAdapter_RecordSession_NoOp verifies RecordSession returns nil.
func TestAdapter_RecordSession_NoOp(t *testing.T) {
	a := codex.New()
	st := openStore(t)
	if err := a.RecordSession(st, "sess", "/cwd", "model", "head"); err != nil {
		t.Errorf("RecordSession: %v", err)
	}
}

// TestAdapter_RecordEdit_NoOp verifies RecordEdit returns nil.
func TestAdapter_RecordEdit_NoOp(t *testing.T) {
	a := codex.New()
	st := openStore(t)
	if err := a.RecordEdit(st, "sess", "file.go", "Edit", "diff", 1, 10, 1000); err != nil {
		t.Errorf("RecordEdit: %v", err)
	}
}

// TestAdapter_RecordExecution_NoOp verifies RecordExecution returns nil.
func TestAdapter_RecordExecution_NoOp(t *testing.T) {
	a := codex.New()
	st := openStore(t)
	if err := a.RecordExecution(st, "sess", "go test", "test", 0, 1000, 500); err != nil {
		t.Errorf("RecordExecution: %v", err)
	}
}

// TestAdapter_RecordPrompt_NoOp verifies RecordPrompt returns nil.
func TestAdapter_RecordPrompt_NoOp(t *testing.T) {
	a := codex.New()
	st := openStore(t)
	if err := a.RecordPrompt(st, "sess", "test prompt", 1000); err != nil {
		t.Errorf("RecordPrompt: %v", err)
	}
}

// TestImportFromLog_NotFound returns error for missing file.
func TestImportFromLog_NotFound(t *testing.T) {
	st := openStore(t)
	_, err := codex.ImportFromLog(st, "/nonexistent/path/log.json")
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
}

// TestImportFromLog_InvalidJSON returns error for malformed JSON.
func TestImportFromLog_InvalidJSON(t *testing.T) {
	st := openStore(t)
	f := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(f, []byte("{bad json}"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := codex.ImportFromLog(st, f)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestImportFromLog_NoIDGeneratesDefault uses "codex-imported" as session ID.
func TestImportFromLog_NoIDGeneratesDefault(t *testing.T) {
	st := openStore(t)
	log := map[string]any{
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(1000),
		"patches":    []any{},
		"executions": []any{},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	n, err := codex.ImportFromLog(st, logPath)
	if err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}
	if n != 0 {
		t.Errorf("edits = %d, want 0", n)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "codex-imported" {
		t.Errorf("ID = %q, want 'codex-imported'", sessions[0].ID)
	}
}

// TestImportFromLog_FallbackTimestamp derives startedAt from patches when created_at=0.
func TestImportFromLog_FallbackTimestamp(t *testing.T) {
	st := openStore(t)
	log := map[string]any{
		"id":         "sess-ts",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(0),
		"patches": []map[string]any{
			{"file": "a.py", "patch": "+x", "timestamp": int64(5000), "accepted_ms": int64(0)},
			{"file": "b.py", "patch": "+y", "timestamp": int64(3000), "accepted_ms": int64(0)},
		},
		"executions": []any{},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := codex.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if sessions[0].StartedAt != 3000 {
		t.Errorf("StartedAt = %d, want 3000 (earliest patch)", sessions[0].StartedAt)
	}
}

// TestImportFromLog_FallbackTimestampFromExec derives startedAt from executions when no patches.
func TestImportFromLog_FallbackTimestampFromExec(t *testing.T) {
	st := openStore(t)
	log := map[string]any{
		"id":         "sess-exects",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(0),
		"patches":    []any{},
		"executions": []map[string]any{
			{"cmd": "pytest", "timestamp": int64(9000), "duration_ms": int64(500)},
			{"cmd": "lint", "timestamp": int64(7000), "duration_ms": int64(100)},
		},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := codex.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if sessions[0].StartedAt != 7000 {
		t.Errorf("StartedAt = %d, want 7000 (earliest exec)", sessions[0].StartedAt)
	}
}

// TestImportFromLog_NilExitCode handles nil exit code gracefully.
func TestImportFromLog_NilExitCode(t *testing.T) {
	st := openStore(t)
	log := map[string]any{
		"id":         "sess-nilexit",
		"model":      "codex-mini",
		"cwd":        "/project",
		"created_at": int64(1000),
		"patches":    []any{},
		"executions": []map[string]any{
			// exit_code intentionally absent (nil in JSON).
			{"cmd": "pytest", "timestamp": int64(2000), "duration_ms": int64(0)},
		},
	}
	logPath := writeJSON(t, t.TempDir(), log)

	if _, err := codex.ImportFromLog(st, logPath); err != nil {
		t.Fatalf("ImportFromLog: %v", err)
	}

	execs, err := st.ExecutionsForSession("sess-nilexit")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) == 0 {
		t.Fatal("expected execution to be stored")
	}
	// When exit_code is nil and negative, ExitCode pointer should be nil.
	if execs[0].ExitCode != nil {
		t.Errorf("expected nil ExitCode for absent exit_code, got %v", execs[0].ExitCode)
	}
}
