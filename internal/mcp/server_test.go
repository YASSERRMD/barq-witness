package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/store"
)

// openTestStore creates a temporary SQLite trace store for testing.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// runServer sends lines to a fresh Server and returns the raw output.
func runServer(t *testing.T, st *store.Store, lines ...string) []Response {
	t.Helper()

	input := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var buf bytes.Buffer

	srv := newWithIO(st, ".", input, &buf)
	ctx := context.Background()
	if err := srv.Run(ctx); err != nil {
		t.Fatalf("server.Run: %v", err)
	}

	// Parse each output line as a Response.
	var responses []Response
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("unmarshal response %q: %v", line, err)
		}
		responses = append(responses, r)
	}
	return responses
}

// TestInitialize verifies that an initialize request receives the correct
// protocol version and server info.
func TestInitialize(t *testing.T) {
	st := openTestStore(t)

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resps := runServer(t, st, req)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	resp := resps[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	b, _ := json.Marshal(resp.Result)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got := result["protocolVersion"]; got != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", got)
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo missing or wrong type")
	}
	if serverInfo["name"] != "barq-witness" {
		t.Errorf("serverInfo.name = %v, want barq-witness", serverInfo["name"])
	}
}

// TestToolsList verifies that tools/list returns all four expected tools.
func TestToolsList(t *testing.T) {
	st := openTestStore(t)

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resps := runServer(t, st, req)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	resp := resps[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	b, _ := json.Marshal(resp.Result)
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal tools list: %v", err)
	}

	wantTools := map[string]bool{
		"barq_get_report":   false,
		"barq_get_segment":  false,
		"barq_list_sessions": false,
		"barq_get_stats":    false,
	}
	for _, tool := range result.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q not found in tools/list response", name)
		}
	}
	if len(result.Tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(result.Tools))
	}
}

// TestBarqGetStats_Empty verifies that barq_get_stats returns valid JSON on an
// empty store.
func TestBarqGetStats_Empty(t *testing.T) {
	st := openTestStore(t)

	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"barq_get_stats","arguments":{}}}`
	resps := runServer(t, st, req)

	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	resp := resps[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// Decode the toolCallResult, then decode the embedded JSON text.
	b, _ := json.Marshal(resp.Result)
	var tcr toolCallResult
	if err := json.Unmarshal(b, &tcr); err != nil {
		t.Fatalf("unmarshal toolCallResult: %v", err)
	}
	if len(tcr.Content) == 0 {
		t.Fatal("no content in tool result")
	}

	var stats map[string]any
	if err := json.Unmarshal([]byte(tcr.Content[0].Text), &stats); err != nil {
		t.Fatalf("unmarshal stats JSON: %v", err)
	}

	for _, key := range []string{"total_edits", "total_sessions", "tier1_count", "tier2_count", "tier3_count"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("stats missing key %q", key)
		}
	}
	// Empty store -- all counts must be zero.
	if v, _ := stats["total_edits"].(float64); v != 0 {
		t.Errorf("total_edits = %v, want 0", v)
	}
}

// TestBarqGetReport_NoCommits verifies that barq_get_report returns a graceful
// empty response when the store is empty and the repo has no history.
func TestBarqGetReport_NoCommits(t *testing.T) {
	st := openTestStore(t)

	// Use a temp directory that is not a real git repo so the git diff step
	// returns an error or an empty result gracefully.
	tmpRepo := t.TempDir()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"barq_get_report","arguments":{"from_sha":"","to_sha":"HEAD","top_n":5}}}` + "\n")
	var buf bytes.Buffer

	srv := newWithIO(st, tmpRepo, input, &buf)
	if err := srv.Run(context.Background()); err != nil {
		t.Fatalf("server.Run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("no output from server")
	}

	var resp Response
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Either an error (git not found) or a valid empty report are both acceptable.
	// The key assertion is that the server did NOT panic and returned valid JSON.
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}

	if resp.Result != nil {
		// Got a result -- verify it contains a parseable tool call response.
		b, _ := json.Marshal(resp.Result)
		var tcr toolCallResult
		if err := json.Unmarshal(b, &tcr); err != nil {
			t.Fatalf("unmarshal toolCallResult: %v", err)
		}
		if len(tcr.Content) > 0 {
			// Verify the embedded text is valid JSON.
			var report map[string]any
			if err := json.Unmarshal([]byte(tcr.Content[0].Text), &report); err != nil {
				t.Fatalf("embedded report JSON is invalid: %v", err)
			}
		}
	}
	// If resp.Error != nil, that is also acceptable (no git repo).
	_ = os.DevNull // silence unused import lint
}
