package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/store"
)

// readFixture reads a hook-payload fixture file relative to the tests/fixtures directory.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "fixtures", "hook-payloads", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestRecord_SessionStart(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	// Override CWD in payload so the session resolves to our temp dir.
	payload := `{"session_id":"integ-session-001","cwd":"` + dir + `","model":"claude-sonnet-4-6"}`
	code := record(t, dir, "session-start", payload)
	if code != 0 {
		t.Fatalf("record session-start exited %d", code)
	}

	st, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "integ-session-001" {
		t.Errorf("session ID = %q, want integ-session-001", sessions[0].ID)
	}
}

func TestRecord_MalformedJSON_ExitsZero(t *testing.T) {
	// Malformed JSON must exit 0 -- hooks must never break Claude Code.
	dir := makeGitRepo(t)
	run(t, dir, "init")

	code := record(t, dir, "session-start", "{this is invalid json")
	if code != 0 {
		t.Errorf("malformed JSON should exit 0 (must not break Claude Code), got %d", code)
	}
}

func TestRecord_MalformedJSON_Fixture(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	payload := readFixture(t, "malformed.json")
	code := record(t, dir, "session-start", string(payload))
	if code != 0 {
		t.Errorf("malformed fixture should exit 0, got %d", code)
	}
}

func TestRecord_FullFlow(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	sessionID := "integ-full-001"

	record(t, dir, "session-start", `{"session_id":"`+sessionID+`","cwd":"`+dir+`","model":"claude-sonnet-4-6"}`)
	record(t, dir, "prompt", `{"session_id":"`+sessionID+`","prompt":"add a hello world function"}`)
	record(t, dir, "edit", `{"session_id":"`+sessionID+`","tool_name":"Write","tool_input":{"file_path":"main.go","content":"package main\n\nfunc hello() {}\n"}}`)
	record(t, dir, "exec", `{"session_id":"`+sessionID+`","tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_response":{"exit_code":0,"duration_ms":1234}}`)
	record(t, dir, "session-end", `{"session_id":"`+sessionID+`"}`)

	st, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].EndedAt == nil {
		t.Error("session EndedAt should be set after session-end")
	}

	prompts, err := st.PromptsForSession(sessionID)
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}

	execs, err := st.ExecutionsForSession(sessionID)
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Classification != "test" {
		t.Errorf("classification = %q, want test", execs[0].Classification)
	}
}

func TestRecord_MultipleSessionsIndependent(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	for i, id := range []string{"sess-a", "sess-b", "sess-c"} {
		_ = i
		record(t, dir, "session-start", `{"session_id":"`+id+`","cwd":"`+dir+`","model":"claude-sonnet-4-6"}`)
		record(t, dir, "prompt", `{"session_id":"`+id+`","prompt":"prompt for `+id+`"}`)
		record(t, dir, "session-end", `{"session_id":"`+id+`"}`)
	}

	st, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestRecord_ExecClassification(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	cases := []struct {
		cmd            string
		wantClassify   string
	}{
		{`go test ./...`, "test"},
		{`git status`, "git"},
		{`go get github.com/foo/bar`, "install"},
	}

	sessionID := "classify-test"
	record(t, dir, "session-start", `{"session_id":"`+sessionID+`","cwd":"`+dir+`","model":"m"}`)

	for _, tc := range cases {
		record(t, dir, "exec", `{"session_id":"`+sessionID+`","tool_name":"Bash","tool_input":{"command":"`+tc.cmd+`"},"tool_response":{"exit_code":0}}`)
	}

	st, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	execs, err := st.ExecutionsForSession(sessionID)
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != len(cases) {
		t.Fatalf("expected %d executions, got %d", len(cases), len(execs))
	}
	for i, tc := range cases {
		if execs[i].Classification != tc.wantClassify {
			t.Errorf("cmd %q: classification = %q, want %q", tc.cmd, execs[i].Classification, tc.wantClassify)
		}
	}
}
