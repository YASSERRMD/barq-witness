package migration_test

import (
	"encoding/json"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/cgpf"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// TestCGPF_CurrentVersionIs1_0 verifies that the CGPF spec version is "1.0".
func TestCGPF_CurrentVersionIs1_0(t *testing.T) {
	if cgpf.Version != "1.0" {
		t.Errorf("expected cgpf.Version=1.0, got %q", cgpf.Version)
	}
}

// TestCGPF_ExportProducesValidJSON exports from a freshly migrated fixture
// store and verifies the output is valid JSON with expected top-level fields.
func TestCGPF_ExportProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := copyFixture(t, dir)

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	doc, err := cgpf.Export(st, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := cgpf.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Must parse as valid JSON.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Must contain the required top-level fields.
	for _, field := range []string{"cgpf_version", "generated_by", "generated_at", "repo", "sessions"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing top-level field %q in CGPF output", field)
		}
	}

	// cgpf_version must equal "1.0".
	if v, _ := raw["cgpf_version"].(string); v != "1.0" {
		t.Errorf("expected cgpf_version=1.0, got %q", v)
	}
}

// TestCGPF_ForwardCompatibility verifies that Unmarshal tolerates unknown
// fields added by a hypothetical future version.
func TestCGPF_ForwardCompatibility(t *testing.T) {
	// A CGPF JSON with an unknown top-level field and an unknown field inside
	// a session record.
	input := `{
		"cgpf_version": "1.0",
		"generated_by": "barq-witness vFUTURE",
		"generated_at": "2026-01-01T00:00:00Z",
		"unknown_future_field": "ignored",
		"repo": {
			"remote": null,
			"commit_range": {"from": "aaa", "to": "bbb"},
			"future_repo_field": 42
		},
		"sessions": [
			{
				"id": "s1",
				"started_at": "2026-01-01T00:00:00Z",
				"model": "claude-3",
				"source": "claude-code",
				"cwd": "/tmp",
				"git_head_start": "abc",
				"prompts": [],
				"edits": [],
				"executions": [],
				"future_session_field": true
			}
		]
	}`

	doc, err := cgpf.Unmarshal([]byte(input))
	if err != nil {
		t.Fatalf("Unmarshal with unknown fields: %v", err)
	}
	if doc.CGPFVersion != "1.0" {
		t.Errorf("expected version 1.0, got %q", doc.CGPFVersion)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(doc.Sessions))
	}
	if doc.Sessions[0].ID != "s1" {
		t.Errorf("unexpected session ID: %q", doc.Sessions[0].ID)
	}
}

// TestCGPF_RoundTrip verifies that Marshal then Unmarshal produces equivalent data.
func TestCGPF_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := copyFixture(t, dir)

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	original, err := cgpf.Export(st, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := cgpf.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	roundTripped, err := cgpf.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundTripped.CGPFVersion != original.CGPFVersion {
		t.Errorf("CGPFVersion mismatch: %q vs %q", roundTripped.CGPFVersion, original.CGPFVersion)
	}
	if len(roundTripped.Sessions) != len(original.Sessions) {
		t.Errorf("session count mismatch: %d vs %d", len(roundTripped.Sessions), len(original.Sessions))
	}
	if len(roundTripped.Sessions) > 0 {
		orig := original.Sessions[0]
		rt := roundTripped.Sessions[0]
		if rt.ID != orig.ID {
			t.Errorf("session ID mismatch: %q vs %q", rt.ID, orig.ID)
		}
		if rt.Model != orig.Model {
			t.Errorf("model mismatch: %q vs %q", rt.Model, orig.Model)
		}
		if len(rt.Prompts) != len(orig.Prompts) {
			t.Errorf("prompt count mismatch: %d vs %d", len(rt.Prompts), len(orig.Prompts))
		}
		if len(rt.Edits) != len(orig.Edits) {
			t.Errorf("edit count mismatch: %d vs %d", len(rt.Edits), len(orig.Edits))
		}
		if len(rt.Executions) != len(orig.Executions) {
			t.Errorf("execution count mismatch: %d vs %d", len(rt.Executions), len(orig.Executions))
		}
	}
}
