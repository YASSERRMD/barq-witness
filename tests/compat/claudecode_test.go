package compat_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/adapters/claudecode"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// TestClaudeCodeAdapter_RecordPipeline exercises the claudecode adapter's
// record pipeline (RecordSession, RecordPrompt, RecordEdit, RecordExecution)
// using the same event types as the hook-payload fixtures, and verifies that
// the resulting DB rows are correct.
func TestClaudeCodeAdapter_RecordPipeline(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	a := claudecode.New()

	sessionID := "compat-cc-session-001"
	now := time.Now().UnixMilli()

	// Record a session start.
	if err := a.RecordSession(st, sessionID, "/project", "claude-sonnet-4-6", "abc123"); err != nil {
		t.Fatalf("RecordSession: %v", err)
	}

	// Record a prompt.
	if err := a.RecordPrompt(st, sessionID, "Add a helper function", now); err != nil {
		t.Fatalf("RecordPrompt: %v", err)
	}

	// Record an edit (simulating a Write tool call).
	if err := a.RecordEdit(st, sessionID, "helper.go", "Write", "+func helper() {}", 1, 1, now+1000); err != nil {
		t.Fatalf("RecordEdit: %v", err)
	}

	// Record a bash execution.
	if err := a.RecordExecution(st, sessionID, "go build ./...", "build", 0, 500, now+2000); err != nil {
		t.Fatalf("RecordExecution: %v", err)
	}

	// Verify the session row.
	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != sessionID {
		t.Errorf("unexpected session ID: %q", sessions[0].ID)
	}
	if sessions[0].Source != "claude-code" {
		t.Errorf("expected source=claude-code, got %q", sessions[0].Source)
	}

	// Verify the prompt row.
	prompts, err := st.PromptsForSession(sessionID)
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Content != "Add a helper function" {
		t.Errorf("unexpected prompt content: %q", prompts[0].Content)
	}

	// Verify the edit row.
	edits, err := st.EditsForSession(sessionID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].FilePath != "helper.go" {
		t.Errorf("unexpected file path: %q", edits[0].FilePath)
	}
	if edits[0].PromptID == nil {
		t.Error("expected edit.PromptID to be set (prompt linkage)")
	}
}
