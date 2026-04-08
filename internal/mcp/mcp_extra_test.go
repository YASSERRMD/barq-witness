package mcp

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// newPopulatedStore creates a store with one session and one edit for testing.
func newPopulatedStore(t *testing.T) (*store.Store, int64) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	if err := st.InsertSession(model.Session{
		ID:        "sess-mcp-1",
		StartedAt: time.Now().UnixMilli(),
		CWD:       "/home/user/project",
		Model:     "claude-sonnet-4-6",
		Source:    "claude-code",
	}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	lineStart, lineEnd := 1, 10
	if err := st.InsertEdit(model.Edit{
		SessionID:  "sess-mcp-1",
		Timestamp:  time.Now().UnixMilli(),
		FilePath:   "main.go",
		Tool:       "Edit",
		BeforeHash: "before",
		AfterHash:  "after",
		LineStart:  &lineStart,
		LineEnd:    &lineEnd,
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	edits, err := st.EditsForFiles([]string{"main.go"})
	if err != nil || len(edits) == 0 {
		t.Fatalf("EditsForFiles: err=%v, len=%d", err, len(edits))
	}
	return st, edits[0].ID
}

// TestBarqGetSegment_Found verifies barq_get_segment returns a segment for a known edit.
func TestBarqGetSegment_Found(t *testing.T) {
	st, editID := newPopulatedStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_get_segment",
			"arguments": map[string]any{"edit_id": editID},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error != nil {
		t.Fatalf("unexpected error: %v", resps[0].Error)
	}

	b, _ := json.Marshal(resps[0].Result)
	var tcr toolCallResult
	if err := json.Unmarshal(b, &tcr); err != nil {
		t.Fatalf("unmarshal toolCallResult: %v", err)
	}
	if len(tcr.Content) == 0 {
		t.Fatal("expected content in tool result")
	}

	var seg map[string]any
	if err := json.Unmarshal([]byte(tcr.Content[0].Text), &seg); err != nil {
		t.Fatalf("unmarshal segment: %v", err)
	}
	if seg["file_path"] != "main.go" {
		t.Errorf("file_path = %v, want main.go", seg["file_path"])
	}
}

// TestBarqGetSegment_NotFound returns an error for a missing edit_id.
func TestBarqGetSegment_NotFound(t *testing.T) {
	st := openTestStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_get_segment",
			"arguments": map[string]any{"edit_id": 999999},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected error for missing edit_id")
	}
	if resps[0].Error.Code != errNotFound {
		t.Errorf("error code = %d, want %d (errNotFound)", resps[0].Error.Code, errNotFound)
	}
}

// TestBarqListSessions_WithData verifies barq_list_sessions returns inserted sessions.
func TestBarqListSessions_WithData(t *testing.T) {
	st, _ := newPopulatedStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_list_sessions",
			"arguments": map[string]any{"limit": 10},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error != nil {
		t.Fatalf("unexpected error: %v", resps[0].Error)
	}

	b, _ := json.Marshal(resps[0].Result)
	var tcr toolCallResult
	if err := json.Unmarshal(b, &tcr); err != nil {
		t.Fatalf("unmarshal toolCallResult: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(tcr.Content[0].Text), &result); err != nil {
		t.Fatalf("unmarshal sessions: %v", err)
	}

	sessions, ok := result["sessions"].([]any)
	if !ok {
		t.Fatalf("sessions field missing or wrong type")
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session")
	}
}

// TestBarqListSessions_Empty returns an empty sessions list on an empty store.
func TestBarqListSessions_Empty(t *testing.T) {
	st := openTestStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      13,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_list_sessions",
			"arguments": map[string]any{},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error != nil {
		t.Fatalf("unexpected error: %v", resps[0].Error)
	}
}

// TestBarqGetStats_WithData verifies barq_get_stats returns non-zero counts.
func TestBarqGetStats_WithData(t *testing.T) {
	st, _ := newPopulatedStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      14,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_get_stats",
			"arguments": map[string]any{},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error != nil {
		t.Fatalf("unexpected error: %v", resps[0].Error)
	}

	b, _ := json.Marshal(resps[0].Result)
	var tcr toolCallResult
	if err := json.Unmarshal(b, &tcr); err != nil {
		t.Fatalf("unmarshal toolCallResult: %v", err)
	}

	var stats map[string]any
	if err := json.Unmarshal([]byte(tcr.Content[0].Text), &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}

	if v, _ := stats["total_sessions"].(float64); v < 1 {
		t.Errorf("total_sessions = %v, want >= 1", v)
	}
	if v, _ := stats["total_edits"].(float64); v < 1 {
		t.Errorf("total_edits = %v, want >= 1", v)
	}
}

// TestUnknownMethod returns errNotFound for an unknown method.
func TestUnknownMethod(t *testing.T) {
	st := openTestStore(t)

	req := `{"jsonrpc":"2.0","id":20,"method":"nonexistent/method","params":{}}`
	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resps[0].Error.Code != errNotFound {
		t.Errorf("error code = %d, want %d (errNotFound)", resps[0].Error.Code, errNotFound)
	}
}

// TestMalformedJSON returns errParse for invalid JSON input.
func TestMalformedJSON(t *testing.T) {
	st := openTestStore(t)

	req := `{this is not valid json`
	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if resps[0].Error.Code != errParse {
		t.Errorf("error code = %d, want %d (errParse)", resps[0].Error.Code, errParse)
	}
}

// TestUnknownTool returns errNotFound for an unknown tool name.
func TestUnknownTool(t *testing.T) {
	st := openTestStore(t)

	req := mustMarshal(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      21,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "barq_nonexistent_tool",
			"arguments": map[string]any{},
		},
	})

	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resps[0].Error.Code != errNotFound {
		t.Errorf("error code = %d, want %d (errNotFound)", resps[0].Error.Code, errNotFound)
	}
}

// TestToolsCallInvalidParams returns errInvalid when params cannot be parsed.
func TestToolsCallInvalidParams(t *testing.T) {
	st := openTestStore(t)

	// Send tools/call with non-object params to trigger parse error.
	req := `{"jsonrpc":"2.0","id":22,"method":"tools/call","params":null}`
	resps := runServer(t, st, req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected error for null params in tools/call")
	}
}

// TestEmptyLines skips empty lines without producing output.
func TestEmptyLines(t *testing.T) {
	st := openTestStore(t)

	// Two empty lines + one valid request.
	req := `{"jsonrpc":"2.0","id":30,"method":"initialize","params":{}}`
	resps := runServer(t, st, "", "", req)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response (empty lines should be skipped), got %d", len(resps))
	}
}

// mustMarshal marshals v to a JSON string or fails the test.
func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}
