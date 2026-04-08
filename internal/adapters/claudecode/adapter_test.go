package claudecode_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/adapters/claudecode"
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

// TestRecordSession verifies that RecordSession inserts a session with
// source="claude-code".
func TestRecordSession(t *testing.T) {
	st := openStore(t)
	a := claudecode.New()

	if got := a.Source(); got != "claude-code" {
		t.Errorf("Source() = %q, want %q", got, "claude-code")
	}

	sessID := "test-sess-001"
	err := a.RecordSession(st, sessID, "/home/user/project", "claude-opus-4-5", "abc123")
	if err != nil {
		t.Fatalf("RecordSession: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	sess := sessions[0]
	if sess.ID != sessID {
		t.Errorf("ID = %q, want %q", sess.ID, sessID)
	}
	if sess.Source != "claude-code" {
		t.Errorf("Source = %q, want %q", sess.Source, "claude-code")
	}
	if sess.Model != "claude-opus-4-5" {
		t.Errorf("Model = %q, want %q", sess.Model, "claude-opus-4-5")
	}
	if sess.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want %q", sess.CWD, "/home/user/project")
	}
	if sess.GitHeadStart != "abc123" {
		t.Errorf("GitHeadStart = %q, want %q", sess.GitHeadStart, "abc123")
	}
}

// TestRecordEdit verifies that RecordEdit inserts an edit row.
func TestRecordEdit(t *testing.T) {
	st := openStore(t)
	a := claudecode.New()

	sessID := "test-sess-002"
	if err := a.RecordSession(st, sessID, "/tmp/proj", "claude-opus-4-5", ""); err != nil {
		t.Fatalf("RecordSession: %v", err)
	}

	now := time.Now().UnixMilli()
	err := a.RecordEdit(st, sessID, "main.go", "Write", "@@ -1 +1 @@\n+package main\n", 1, 5, now)
	if err != nil {
		t.Fatalf("RecordEdit: %v", err)
	}

	edits, err := st.EditsForSession(sessID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}

	e := edits[0]
	if e.FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q", e.FilePath, "main.go")
	}
	if e.Tool != "Write" {
		t.Errorf("Tool = %q, want %q", e.Tool, "Write")
	}
	if e.SessionID != sessID {
		t.Errorf("SessionID = %q, want %q", e.SessionID, sessID)
	}
	if e.LineStart == nil || *e.LineStart != 1 {
		t.Errorf("LineStart = %v, want 1", e.LineStart)
	}
	if e.LineEnd == nil || *e.LineEnd != 5 {
		t.Errorf("LineEnd = %v, want 5", e.LineEnd)
	}
}

// TestRecordExecution verifies that RecordExecution inserts an execution row.
func TestRecordExecution(t *testing.T) {
	st := openStore(t)
	a := claudecode.New()

	sessID := "test-sess-003"
	if err := a.RecordSession(st, sessID, "/tmp/proj", "claude-opus-4-5", ""); err != nil {
		t.Fatalf("RecordSession: %v", err)
	}

	now := time.Now().UnixMilli()
	err := a.RecordExecution(st, sessID, "go test ./...", "test", 0, 1200, now)
	if err != nil {
		t.Fatalf("RecordExecution: %v", err)
	}

	execs, err := st.ExecutionsForSession(sessID)
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}

	x := execs[0]
	if x.Command != "go test ./..." {
		t.Errorf("Command = %q, want %q", x.Command, "go test ./...")
	}
	if x.Classification != "test" {
		t.Errorf("Classification = %q, want %q", x.Classification, "test")
	}
	if x.ExitCode == nil || *x.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", x.ExitCode)
	}
	if x.DurationMS == nil || *x.DurationMS != 1200 {
		t.Errorf("DurationMS = %v, want 1200", x.DurationMS)
	}
}
