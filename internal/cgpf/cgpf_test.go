package cgpf_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/cgpf"
	"github.com/yasserrmd/barq-witness/internal/model"
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

func nowMS() int64 { return time.Now().UnixMilli() }

// seedFixtureTrace inserts a realistic trace into s and returns the session ID.
func seedFixtureTrace(t *testing.T, s *store.Store) string {
	t.Helper()
	sessID := "sess-export-001"
	if err := s.InsertSession(model.Session{
		ID:           sessID,
		StartedAt:    nowMS(),
		CWD:          "/home/user/repo",
		GitHeadStart: "aaabbb111",
		Model:        "claude-opus-4-5",
	}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	pID, err := s.InsertPrompt(model.Prompt{
		SessionID:   sessID,
		Timestamp:   nowMS(),
		Content:     "write an HTTP handler for /health",
		ContentHash: "sha256abc",
	})
	if err != nil {
		t.Fatalf("InsertPrompt: %v", err)
	}

	ls, le := 1, 15
	if err := s.InsertEdit(model.Edit{
		SessionID:  sessID,
		PromptID:   &pID,
		Timestamp:  nowMS(),
		FilePath:   "cmd/server/handler.go",
		Tool:       "Write",
		BeforeHash: "before001",
		AfterHash:  "after001",
		LineStart:  &ls,
		LineEnd:    &le,
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	exitCode := 0
	dur := int64(412)
	if err := s.InsertExecution(model.Execution{
		SessionID:      sessID,
		Timestamp:      nowMS(),
		Command:        "go test ./cmd/server/...",
		Classification: "test",
		FilesTouched:   `["cmd/server/handler.go"]`,
		ExitCode:       &exitCode,
		DurationMS:     &dur,
	}); err != nil {
		t.Fatalf("InsertExecution: %v", err)
	}

	return sessID
}

// ---- tests -----------------------------------------------------------------

// TestExport_FullTrace verifies all fields are present in a full (non-privacy) export.
func TestExport_FullTrace(t *testing.T) {
	s := openStore(t)
	sessID := seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if doc.CGPFVersion != cgpf.Version {
		t.Errorf("CGPFVersion = %q, want %q", doc.CGPFVersion, cgpf.Version)
	}
	if !strings.Contains(doc.GeneratedBy, "barq-witness") {
		t.Errorf("GeneratedBy = %q, want to contain barq-witness", doc.GeneratedBy)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(doc.Sessions))
	}

	sess := doc.Sessions[0]
	if sess.ID != sessID {
		t.Errorf("session ID = %q, want %q", sess.ID, sessID)
	}
	if sess.Model != "claude-opus-4-5" {
		t.Errorf("Model = %q", sess.Model)
	}
	if len(sess.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(sess.Prompts))
	}
	if sess.Prompts[0].Content == nil || *sess.Prompts[0].Content == "" {
		t.Error("expected non-nil, non-empty prompt content in full mode")
	}
	if len(sess.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(sess.Edits))
	}
	if sess.Edits[0].FilePath != "cmd/server/handler.go" {
		t.Errorf("edit file_path = %q", sess.Edits[0].FilePath)
	}
	if len(sess.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(sess.Executions))
	}
	if sess.Executions[0].Command == nil || !strings.Contains(*sess.Executions[0].Command, "go test") {
		t.Errorf("execution command = %v", sess.Executions[0].Command)
	}
}

// TestExport_PrivacyMode verifies that content and command are omitted.
func TestExport_PrivacyMode(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{Privacy: true})
	if err != nil {
		t.Fatalf("Export privacy: %v", err)
	}

	if len(doc.Sessions) != 1 {
		t.Fatalf("expected 1 session")
	}
	sess := doc.Sessions[0]

	// Prompt content must be nil.
	for _, p := range sess.Prompts {
		if p.Content != nil {
			t.Error("privacy mode: prompt content should be nil")
		}
		// Hash must still be present.
		if p.ContentHash == "" {
			t.Error("privacy mode: content_hash should not be empty")
		}
	}

	// Execution command must be nil.
	for _, x := range sess.Executions {
		if x.Command != nil {
			t.Error("privacy mode: command should be nil")
		}
		// Classification must still be present.
		if x.Classification == "" {
			t.Error("privacy mode: classification should not be empty")
		}
	}
}

// TestExport_SessionFilter verifies the session ID filter works.
func TestExport_SessionFilter(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	// Insert a second session.
	if err := s.InsertSession(model.Session{
		ID: "sess-other", StartedAt: nowMS(), CWD: "/tmp",
	}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	doc, err := cgpf.Export(s, cgpf.ExportOptions{SessionID: "sess-export-001"})
	if err != nil {
		t.Fatalf("Export filter: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Errorf("expected 1 session after filter, got %d", len(doc.Sessions))
	}
}

// TestMarshalUnmarshal verifies round-trip JSON stability.
func TestMarshalUnmarshal(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := cgpf.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Must be valid JSON.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Round-trip.
	doc2, err := cgpf.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc2.CGPFVersion != doc.CGPFVersion {
		t.Errorf("version mismatch after round-trip: %q vs %q", doc2.CGPFVersion, doc.CGPFVersion)
	}
	if len(doc2.Sessions) != len(doc.Sessions) {
		t.Errorf("session count mismatch: %d vs %d", len(doc2.Sessions), len(doc.Sessions))
	}
}

// TestMarshal_GoldenFields verifies key fields appear literally in the JSON.
func TestMarshal_GoldenFields(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	doc, _ := cgpf.Export(s, cgpf.ExportOptions{})
	data, _ := cgpf.Marshal(doc)
	out := string(data)

	mustContain := func(field string) {
		t.Helper()
		if !strings.Contains(out, field) {
			t.Errorf("JSON output does not contain %q", field)
		}
	}

	mustContain(`"cgpf_version"`)
	mustContain(`"0.2"`)
	mustContain(`"generated_by"`)
	mustContain(`"generated_at"`)
	mustContain(`"sessions"`)
	mustContain(`"prompts"`)
	mustContain(`"edits"`)
	mustContain(`"executions"`)
	mustContain(`"content_hash"`)
	mustContain(`"classification"`)
	mustContain(`"files_touched"`)
}

// TestExport_EmptyStore verifies a valid empty document when trace is empty.
func TestExport_EmptyStore(t *testing.T) {
	s := openStore(t)
	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export empty: %v", err)
	}
	if doc.CGPFVersion != "0.2" {
		t.Errorf("CGPFVersion = %q", doc.CGPFVersion)
	}
	if doc.Sessions != nil && len(doc.Sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(doc.Sessions))
	}
}
