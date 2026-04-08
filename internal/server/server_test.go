package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer creates a Server backed by an in-memory SQLite database.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(":memory:", 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

// validPayload returns a minimal valid CGPF v0.3 JSON payload.
func validPayload() []byte {
	payload := map[string]interface{}{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions": []map[string]interface{}{
			{
				"id":     "sess-001",
				"source": "claude-code",
				"edits": []map[string]interface{}{
					{"file_path": "main.go", "tier": 1, "reason_codes": []string{"SEC_PATH"}},
					{"file_path": "README.md", "tier": 3, "reason_codes": []string{}},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

// TestIngest_ValidPayload sends a valid CGPF v0.3 payload and expects HTTP 200.
func TestIngest_ValidPayload(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(validPayload()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp["ok"] {
		t.Errorf("expected ok=true, got %v", resp)
	}
}

// TestIngest_InvalidVersion sends a payload with an unsupported CGPF version and expects HTTP 400.
func TestIngest_InvalidVersion(t *testing.T) {
	s := newTestServer(t)

	payload := map[string]interface{}{
		"cgpf_version": "0.2",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions":     []interface{}{},
	}
	data, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngest_TooLarge sends a body exceeding 10 MB and expects HTTP 413.
func TestIngest_TooLarge(t *testing.T) {
	s := newTestServer(t)

	// Build a body slightly over 10 MB.
	large := strings.Repeat("x", maxIngestBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(large))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

// TestStats_Empty calls GET /api/v1/stats on an empty database and expects valid JSON.
func TestStats_Empty(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp StatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}

	if resp.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", resp.TotalSessions)
	}
	if resp.TopFlaggedFiles == nil {
		t.Error("top_flagged_files must not be null")
	}
}

// TestDashboard_Returns200 checks that GET /api/v1/dashboard returns HTTP 200 with HTML content.
func TestDashboard_Returns200(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	s.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "barq-witness") {
		t.Error("expected dashboard HTML to contain 'barq-witness'")
	}
}
