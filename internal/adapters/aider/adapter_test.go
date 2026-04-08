package aider_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/adapters/aider"
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

const sampleChat = `# aider chat started 2024-01-15 10:30:00

## User
Add error handling to the login function

## Assistant (gpt-4)
I'll add error handling...

> Modified login.py
> Modified auth/validators.py

## User
Run the tests

## Assistant (gpt-4)
` + "```bash" + `
python -m pytest tests/
` + "```" + `
Exit code: 0
`

// TestImportFromChat_Prompts verifies that ## User sections are stored as
// prompts.
func TestImportFromChat_Prompts(t *testing.T) {
	st := openStore(t)

	tmp := t.TempDir()
	chatPath := filepath.Join(tmp, "aider.md")
	if err := os.WriteFile(chatPath, []byte(sampleChat), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	n, err := aider.ImportFromChat(st, chatPath)
	if err != nil {
		t.Fatalf("ImportFromChat: %v", err)
	}
	// Two "## Modified" lines.
	if n != 2 {
		t.Errorf("edits = %d, want 2", n)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions))
	}

	sess := sessions[0]
	if sess.Source != "aider" {
		t.Errorf("Source = %q, want %q", sess.Source, "aider")
	}
	if sess.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", sess.Model, "gpt-4")
	}

	prompts, err := st.PromptsForSession(sess.ID)
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("prompts = %d, want 2", len(prompts))
	}
}

// TestImportFromChat_Edits verifies that "> Modified" lines are stored as
// edits with the correct file path and accepted_sec=-1.
func TestImportFromChat_Edits(t *testing.T) {
	st := openStore(t)

	tmp := t.TempDir()
	chatPath := filepath.Join(tmp, "aider.md")
	if err := os.WriteFile(chatPath, []byte(sampleChat), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := aider.ImportFromChat(st, chatPath); err != nil {
		t.Fatalf("ImportFromChat: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("no sessions stored")
	}

	edits, err := st.EditsForSession(sessions[0].ID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("edits = %d, want 2", len(edits))
	}

	wantFiles := []string{"login.py", "auth/validators.py"}
	for i, e := range edits {
		if e.FilePath != wantFiles[i] {
			t.Errorf("edits[%d].FilePath = %q, want %q", i, e.FilePath, wantFiles[i])
		}
		wantTool := "aider-edit(accepted_sec=-1)"
		if e.Tool != wantTool {
			t.Errorf("edits[%d].Tool = %q, want %q", i, e.Tool, wantTool)
		}
	}
}

// TestImportFromChat_Execution verifies that bash code blocks are stored as
// executions.
func TestImportFromChat_Execution(t *testing.T) {
	st := openStore(t)

	tmp := t.TempDir()
	chatPath := filepath.Join(tmp, "aider.md")
	if err := os.WriteFile(chatPath, []byte(sampleChat), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := aider.ImportFromChat(st, chatPath); err != nil {
		t.Fatalf("ImportFromChat: %v", err)
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("no sessions stored")
	}

	execs, err := st.ExecutionsForSession(sessions[0].ID)
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("executions = %d, want 1", len(execs))
	}
	if execs[0].Command != "python -m pytest tests/" {
		t.Errorf("Command = %q, want %q", execs[0].Command, "python -m pytest tests/")
	}
}

// TestImportFromChat_DuplicateSkipped verifies that re-importing the same
// chat file is a no-op.
func TestImportFromChat_DuplicateSkipped(t *testing.T) {
	st := openStore(t)

	tmp := t.TempDir()
	chatPath := filepath.Join(tmp, "aider.md")
	if err := os.WriteFile(chatPath, []byte(sampleChat), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := aider.ImportFromChat(st, chatPath); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if _, err := aider.ImportFromChat(st, chatPath); err != nil {
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
