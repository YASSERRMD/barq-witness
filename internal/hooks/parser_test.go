package hooks_test

import (
	"testing"

	"github.com/yasserrmd/barq-witness/internal/hooks"
)

// Real-world-representative Claude Code hook JSON payloads used throughout.

const sessionStartJSON = `{
    "session_id": "sess-abc123",
    "transcript_path": "/Users/dev/.claude/sessions/sess-abc123.json",
    "cwd": "/Users/dev/myrepo",
    "model": "claude-opus-4-5"
}`

const userPromptJSON = `{
    "session_id": "sess-abc123",
    "transcript_path": "/Users/dev/.claude/sessions/sess-abc123.json",
    "cwd": "/Users/dev/myrepo",
    "prompt": "Write a function that validates an email address"
}`

const postToolUseEditJSON = `{
    "session_id": "sess-abc123",
    "transcript_path": "/Users/dev/.claude/sessions/sess-abc123.json",
    "cwd": "/Users/dev/myrepo",
    "tool_name": "Edit",
    "tool_input": {
        "file_path": "internal/auth/validator.go",
        "old_string": "func Validate(s string) bool {\n\treturn true\n}",
        "new_string": "func Validate(s string) bool {\n\treturn strings.Contains(s, \"@\")\n}"
    },
    "tool_response": {
        "stdout": "The file internal/auth/validator.go has been updated successfully.",
        "stderr": "",
        "exit_code": 0
    }
}`

const postToolUseWriteJSON = `{
    "session_id": "sess-abc123",
    "tool_name": "Write",
    "tool_input": {
        "file_path": "internal/auth/validator_test.go",
        "content": "package auth_test\n\nimport \"testing\"\n\nfunc TestValidate(t *testing.T) {}\n"
    },
    "tool_response": {
        "stdout": "New file created successfully.",
        "exit_code": 0
    }
}`

const postToolUseMultiEditJSON = `{
    "session_id": "sess-abc123",
    "tool_name": "MultiEdit",
    "tool_input": {
        "file_path": "cmd/main.go",
        "edits": [
            {"old_string": "version = \"1.0\"", "new_string": "version = \"1.1\""},
            {"old_string": "debug = false", "new_string": "debug = true"}
        ]
    },
    "tool_response": {"exit_code": 0}
}`

const postToolUseBashJSON = `{
    "session_id": "sess-abc123",
    "transcript_path": "/Users/dev/.claude/sessions/sess-abc123.json",
    "cwd": "/Users/dev/myrepo",
    "tool_name": "Bash",
    "tool_input": {
        "command": "go test ./internal/auth/... -v",
        "description": "Run auth package tests"
    },
    "tool_response": {
        "stdout": "ok  \tgithub.com/example/myrepo/internal/auth\t0.312s",
        "stderr": "",
        "exit_code": 0,
        "duration_ms": 312
    }
}`

const sessionEndJSON = `{
    "session_id": "sess-abc123",
    "transcript_path": "/Users/dev/.claude/sessions/sess-abc123.json",
    "cwd": "/Users/dev/myrepo"
}`

// --- SessionStart -----------------------------------------------------------

func TestParseSessionStart_OK(t *testing.T) {
	p, err := hooks.ParseSessionStart([]byte(sessionStartJSON))
	if err != nil {
		t.Fatalf("ParseSessionStart: %v", err)
	}
	if p.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q, want %q", p.SessionID, "sess-abc123")
	}
	if p.CWD != "/Users/dev/myrepo" {
		t.Errorf("CWD = %q, want %q", p.CWD, "/Users/dev/myrepo")
	}
	if p.Model != "claude-opus-4-5" {
		t.Errorf("Model = %q, want %q", p.Model, "claude-opus-4-5")
	}
}

func TestParseSessionStart_BadJSON(t *testing.T) {
	_, err := hooks.ParseSessionStart([]byte("{bad json"))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// --- UserPromptSubmit -------------------------------------------------------

func TestParseUserPrompt_OK(t *testing.T) {
	p, err := hooks.ParseUserPrompt([]byte(userPromptJSON))
	if err != nil {
		t.Fatalf("ParseUserPrompt: %v", err)
	}
	if p.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q", p.SessionID)
	}
	if p.Prompt != "Write a function that validates an email address" {
		t.Errorf("Prompt = %q", p.Prompt)
	}
}

// --- PostToolUse (Edit) -----------------------------------------------------

func TestParsePostToolUse_Edit(t *testing.T) {
	p, err := hooks.ParsePostToolUse([]byte(postToolUseEditJSON))
	if err != nil {
		t.Fatalf("ParsePostToolUse(Edit): %v", err)
	}
	if p.ToolName != "Edit" {
		t.Errorf("ToolName = %q, want Edit", p.ToolName)
	}
	if p.ToolInput.FilePath != "internal/auth/validator.go" {
		t.Errorf("FilePath = %q", p.ToolInput.FilePath)
	}
	if p.ToolInput.OldString == "" {
		t.Error("OldString should not be empty")
	}
	if p.ToolInput.NewString == "" {
		t.Error("NewString should not be empty")
	}
	if p.ToolResponse.ExitCode == nil || *p.ToolResponse.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", p.ToolResponse.ExitCode)
	}
}

// --- PostToolUse (Write) ----------------------------------------------------

func TestParsePostToolUse_Write(t *testing.T) {
	p, err := hooks.ParsePostToolUse([]byte(postToolUseWriteJSON))
	if err != nil {
		t.Fatalf("ParsePostToolUse(Write): %v", err)
	}
	if p.ToolName != "Write" {
		t.Errorf("ToolName = %q, want Write", p.ToolName)
	}
	if p.ToolInput.Content == "" {
		t.Error("Content should not be empty")
	}
}

// --- PostToolUse (MultiEdit) ------------------------------------------------

func TestParsePostToolUse_MultiEdit(t *testing.T) {
	p, err := hooks.ParsePostToolUse([]byte(postToolUseMultiEditJSON))
	if err != nil {
		t.Fatalf("ParsePostToolUse(MultiEdit): %v", err)
	}
	if p.ToolName != "MultiEdit" {
		t.Errorf("ToolName = %q, want MultiEdit", p.ToolName)
	}
	if len(p.ToolInput.Edits) != 2 {
		t.Errorf("Edits count = %d, want 2", len(p.ToolInput.Edits))
	}
	if p.ToolInput.Edits[0].OldString != `version = "1.0"` {
		t.Errorf("Edits[0].OldString = %q", p.ToolInput.Edits[0].OldString)
	}
}

// --- PostToolUse (Bash) -----------------------------------------------------

func TestParsePostToolUse_Bash(t *testing.T) {
	p, err := hooks.ParsePostToolUse([]byte(postToolUseBashJSON))
	if err != nil {
		t.Fatalf("ParsePostToolUse(Bash): %v", err)
	}
	if p.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", p.ToolName)
	}
	if p.ToolInput.Command != "go test ./internal/auth/... -v" {
		t.Errorf("Command = %q", p.ToolInput.Command)
	}
	if p.ToolResponse.ExitCode == nil || *p.ToolResponse.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", p.ToolResponse.ExitCode)
	}
	if p.ToolResponse.DurationMS == nil || *p.ToolResponse.DurationMS != 312 {
		t.Errorf("DurationMS = %v, want 312", p.ToolResponse.DurationMS)
	}
}

// --- SessionEnd -------------------------------------------------------------

func TestParseSessionEnd_OK(t *testing.T) {
	p, err := hooks.ParseSessionEnd([]byte(sessionEndJSON))
	if err != nil {
		t.Fatalf("ParseSessionEnd: %v", err)
	}
	if p.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q", p.SessionID)
	}
}

func TestParseSessionEnd_BadJSON(t *testing.T) {
	_, err := hooks.ParseSessionEnd([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
