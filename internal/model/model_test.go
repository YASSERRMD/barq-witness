package model_test

import (
	"testing"

	"github.com/yasserrmd/barq-witness/internal/model"
)

// TestSession_ZeroValues verifies default zero values for Session fields.
func TestSession_ZeroValues(t *testing.T) {
	var s model.Session
	if s.ID != "" {
		t.Errorf("ID zero = %q, want empty", s.ID)
	}
	if s.StartedAt != 0 {
		t.Errorf("StartedAt zero = %d, want 0", s.StartedAt)
	}
	if s.EndedAt != nil {
		t.Error("EndedAt zero should be nil")
	}
	if s.GitHeadEnd != nil {
		t.Error("GitHeadEnd zero should be nil")
	}
}

// TestSession_FieldRoundtrip verifies that fields set on Session can be read back.
func TestSession_FieldRoundtrip(t *testing.T) {
	endedAt := int64(9999)
	gitEnd := "def456"
	s := model.Session{
		ID:           "sess-1",
		StartedAt:    1000,
		EndedAt:      &endedAt,
		CWD:          "/home/user/project",
		GitHeadStart: "abc123",
		GitHeadEnd:   &gitEnd,
		Model:        "claude-sonnet-4-6",
		Source:       "cursor",
	}

	if s.ID != "sess-1" {
		t.Errorf("ID = %q, want sess-1", s.ID)
	}
	if s.StartedAt != 1000 {
		t.Errorf("StartedAt = %d, want 1000", s.StartedAt)
	}
	if s.EndedAt == nil || *s.EndedAt != endedAt {
		t.Errorf("EndedAt = %v, want &%d", s.EndedAt, endedAt)
	}
	if s.GitHeadEnd == nil || *s.GitHeadEnd != gitEnd {
		t.Errorf("GitHeadEnd = %v, want &%q", s.GitHeadEnd, gitEnd)
	}
	if s.Source != "cursor" {
		t.Errorf("Source = %q, want cursor", s.Source)
	}
}

// TestPrompt_ZeroValues verifies default zero values for Prompt fields.
func TestPrompt_ZeroValues(t *testing.T) {
	var p model.Prompt
	if p.ID != 0 {
		t.Errorf("ID zero = %d, want 0", p.ID)
	}
	if p.SessionID != "" {
		t.Errorf("SessionID zero = %q, want empty", p.SessionID)
	}
	if p.Content != "" {
		t.Errorf("Content zero = %q, want empty", p.Content)
	}
}

// TestPrompt_FieldRoundtrip verifies Prompt field assignment.
func TestPrompt_FieldRoundtrip(t *testing.T) {
	p := model.Prompt{
		ID:          42,
		SessionID:   "sess-2",
		Timestamp:   1234567890,
		Content:     "write a function",
		ContentHash: "cafebabe",
	}
	if p.ID != 42 {
		t.Errorf("ID = %d, want 42", p.ID)
	}
	if p.Content != "write a function" {
		t.Errorf("Content = %q", p.Content)
	}
	if p.ContentHash != "cafebabe" {
		t.Errorf("ContentHash = %q", p.ContentHash)
	}
}

// TestEdit_ZeroValues verifies default zero values for Edit fields.
func TestEdit_ZeroValues(t *testing.T) {
	var e model.Edit
	if e.ID != 0 {
		t.Errorf("ID zero = %d, want 0", e.ID)
	}
	if e.PromptID != nil {
		t.Error("PromptID zero should be nil")
	}
	if e.LineStart != nil {
		t.Error("LineStart zero should be nil")
	}
	if e.LineEnd != nil {
		t.Error("LineEnd zero should be nil")
	}
}

// TestEdit_PointerFields verifies optional pointer fields on Edit.
func TestEdit_PointerFields(t *testing.T) {
	promptID := int64(7)
	lineStart := 10
	lineEnd := 20

	e := model.Edit{
		ID:         1,
		SessionID:  "sess-3",
		PromptID:   &promptID,
		Timestamp:  100,
		FilePath:   "main.go",
		Tool:       "Edit",
		BeforeHash: "before",
		AfterHash:  "after",
		LineStart:  &lineStart,
		LineEnd:    &lineEnd,
		Diff:       "@@ -1 +1 @@\n",
	}

	if e.PromptID == nil || *e.PromptID != promptID {
		t.Errorf("PromptID = %v, want &%d", e.PromptID, promptID)
	}
	if e.LineStart == nil || *e.LineStart != lineStart {
		t.Errorf("LineStart = %v, want &%d", e.LineStart, lineStart)
	}
	if e.LineEnd == nil || *e.LineEnd != lineEnd {
		t.Errorf("LineEnd = %v, want &%d", e.LineEnd, lineEnd)
	}
}

// TestExecution_ZeroValues verifies default zero values for Execution fields.
func TestExecution_ZeroValues(t *testing.T) {
	var x model.Execution
	if x.ID != 0 {
		t.Errorf("ID zero = %d, want 0", x.ID)
	}
	if x.ExitCode != nil {
		t.Error("ExitCode zero should be nil")
	}
	if x.DurationMS != nil {
		t.Error("DurationMS zero should be nil")
	}
}

// TestExecution_PointerFields verifies optional pointer fields on Execution.
func TestExecution_PointerFields(t *testing.T) {
	exitCode := 0
	durMS := int64(500)

	x := model.Execution{
		ID:             5,
		SessionID:      "sess-4",
		Timestamp:      200,
		Command:        "go test ./...",
		Classification: "test",
		FilesTouched:   `["main.go"]`,
		ExitCode:       &exitCode,
		DurationMS:     &durMS,
	}

	if x.ExitCode == nil || *x.ExitCode != exitCode {
		t.Errorf("ExitCode = %v, want &%d", x.ExitCode, exitCode)
	}
	if x.DurationMS == nil || *x.DurationMS != durMS {
		t.Errorf("DurationMS = %v, want &%d", x.DurationMS, durMS)
	}
	if x.FilesTouched != `["main.go"]` {
		t.Errorf("FilesTouched = %q", x.FilesTouched)
	}
}
