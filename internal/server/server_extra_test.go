package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHandleIngest_MethodNotAllowed returns 405 for GET requests.
func TestHandleIngest_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingest", nil)
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleIngest_InvalidJSON returns 400 for malformed JSON.
func TestHandleIngest_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestHandleIngest_MissingUUID returns 400 for missing author_uuid.
func TestHandleIngest_MissingUUID(t *testing.T) {
	s := newTestServer(t)
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "",
		"sessions":     []any{},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing UUID, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleIngest_InvalidUUID returns 400 for a malformed UUID.
func TestHandleIngest_InvalidUUID(t *testing.T) {
	s := newTestServer(t)
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "not-a-uuid",
		"sessions":     []any{},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", w.Code)
	}
}

// TestHandleIngest_TooManyEdits returns 400 when edit count exceeds limit.
func TestHandleIngest_TooManyEdits(t *testing.T) {
	s := newTestServer(t)
	edits := make([]map[string]any, maxEditsPerPayload+1)
	for i := range edits {
		edits[i] = map[string]any{"file_path": "f.go", "tier": 1, "reason_codes": []string{}}
	}
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions": []map[string]any{
			{"id": "big-sess", "source": "claude-code", "edits": edits},
		},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for too many edits, got %d", w.Code)
	}
}

// TestHandleIngest_EmptySessions returns 200 for a valid payload with no sessions.
func TestHandleIngest_EmptySessions(t *testing.T) {
	s := newTestServer(t)
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions":     []any{},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty sessions, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleIngest_SessionEmptyID skips sessions with empty ID.
func TestHandleIngest_SessionEmptyID(t *testing.T) {
	s := newTestServer(t)
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions": []map[string]any{
			{"id": "", "source": "claude-code", "edits": []any{}},
		},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (empty ID sessions are skipped), got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleStats_MethodNotAllowed returns 405 for POST requests.
func TestHandleStats_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.handleStats(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleStats_WithData returns non-zero counts after ingest.
func TestHandleStats_WithData(t *testing.T) {
	s := newTestServer(t)

	// Ingest some data first.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(validPayload()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest failed: %d %s", w.Code, w.Body.String())
	}

	// Now check stats.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w = httptest.NewRecorder()
	s.handleStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stats failed: %d %s", w.Code, w.Body.String())
	}

	var resp StatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if resp.TotalSessions == 0 {
		t.Error("expected TotalSessions > 0 after ingest")
	}
	if resp.TotalEdits == 0 {
		t.Error("expected TotalEdits > 0 after ingest")
	}
}

// TestHandleDashboard_MethodNotAllowed returns 405 for POST requests.
func TestHandleDashboard_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	s.handleDashboard(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleDashboard_WithData verifies dashboard renders after ingest.
func TestHandleDashboard_WithData(t *testing.T) {
	s := newTestServer(t)

	// Ingest data.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(validPayload()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}

	// Request dashboard.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	w = httptest.NewRecorder()
	s.handleDashboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "barq-witness") {
		t.Error("expected HTML to contain 'barq-witness'")
	}
}

// TestStop_NilServer verifies Stop handles nil server gracefully.
func TestStop_NilServer(t *testing.T) {
	s := newTestServer(t)
	// srv is nil because Start was never called.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Errorf("Stop with nil srv: %v", err)
	}
}

// TestQueryStats_TopFlaggedFiles verifies top flagged files are returned.
func TestQueryStats_TopFlaggedFiles(t *testing.T) {
	s := newTestServer(t)

	// Ingest a payload where one file appears in a high-tier edit.
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440000",
		"sessions": []map[string]any{
			{
				"id":     "sess-flagged",
				"source": "claude-code",
				"edits": []map[string]any{
					{"file_path": "auth/login.go", "tier": 1, "reason_codes": []string{"SEC_PATH"}},
					{"file_path": "auth/login.go", "tier": 1, "reason_codes": []string{"NO_EXEC"}},
					{"file_path": "other.go", "tier": 2, "reason_codes": []string{}},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}

	stats, err := s.queryStats()
	if err != nil {
		t.Fatalf("queryStats: %v", err)
	}
	if len(stats.TopFlaggedFiles) == 0 {
		t.Fatal("expected at least one flagged file")
	}
	// auth/login.go has 2 tier-1 edits, so it should be first.
	if stats.TopFlaggedFiles[0].File != "auth/login.go" {
		t.Errorf("top flagged file = %q, want auth/login.go", stats.TopFlaggedFiles[0].File)
	}
}

// TestIngest_MultipleTiers verifies tier counts are correctly aggregated.
func TestIngest_MultipleTiers(t *testing.T) {
	s := newTestServer(t)
	payload := map[string]any{
		"cgpf_version": "0.3",
		"author_uuid":  "550e8400-e29b-41d4-a716-446655440001",
		"sessions": []map[string]any{
			{
				"id":     "sess-tiers",
				"source": "cursor",
				"edits": []map[string]any{
					{"file_path": "a.go", "tier": 1, "reason_codes": []string{}},
					{"file_path": "b.go", "tier": 2, "reason_codes": []string{}},
					{"file_path": "c.go", "tier": 3, "reason_codes": []string{}},
					{"file_path": "d.go", "tier": 0, "reason_codes": []string{}},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIngest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}

	stats, err := s.queryStats()
	if err != nil {
		t.Fatalf("queryStats: %v", err)
	}
	if stats.Tier1Count != 1 {
		t.Errorf("tier1_count = %d, want 1", stats.Tier1Count)
	}
	if stats.Tier2Count != 1 {
		t.Errorf("tier2_count = %d, want 1", stats.Tier2Count)
	}
	if stats.Tier3Count != 1 {
		t.Errorf("tier3_count = %d, want 1", stats.Tier3Count)
	}
}
