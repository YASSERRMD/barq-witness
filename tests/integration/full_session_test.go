package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFullSession_EndToEnd(t *testing.T) {
	dir := makeGitRepo(t)

	// Init.
	_, _, code := run(t, dir, "init")
	if code != 0 {
		t.Fatalf("init failed with code %d", code)
	}

	// Simulate a full session.
	sessionID := "full-session-001"
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC).UnixMilli()

	// Session start.
	record(t, dir, "session-start", fmt.Sprintf(
		`{"session_id":%q,"cwd":%q,"model":"claude-sonnet-4-6"}`,
		sessionID, dir))

	// Prompt.
	record(t, dir, "prompt", fmt.Sprintf(
		`{"session_id":%q,"prompt":"add auth middleware","timestamp":%d}`,
		sessionID, ts+1000))

	// Create the auth directory and file so the edit has a real file to reference.
	authDir := filepath.Join(dir, "internal", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("mkdir auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "handler.go"), []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write handler.go: %v", err)
	}

	// Edit a security-sensitive file.
	record(t, dir, "edit", fmt.Sprintf(
		`{"session_id":%q,"tool_name":"Write","tool_input":{"file_path":%q,"content":"package auth\n\nfunc Auth() {}\n"},"timestamp":%d}`,
		sessionID, filepath.Join(authDir, "handler.go"), ts+2000))

	// No execution after the edit (triggers NO_EXEC signal).

	// Session end.
	record(t, dir, "session-end", fmt.Sprintf(
		`{"session_id":%q}`,
		sessionID))

	// Commit the changed file so the analyzer has a git range to examine.
	gitRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Run()
	}
	gitRun("add", ".")
	gitRun("-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "add auth middleware")

	// Run report.
	stdout, stderr, code := run(t, dir, "report", "--format", "text")
	t.Logf("report stdout: %s", stdout)
	t.Logf("report stderr: %s", stderr)
	if code != 0 {
		t.Errorf("report exited %d", code)
	}

	// Run export.
	stdout, _, code = run(t, dir, "export")
	if code != 0 {
		t.Errorf("export exited %d", code)
	}
	if !strings.Contains(stdout, "\"cgpf_version\"") && !strings.Contains(stdout, "cgpf_version") {
		t.Errorf("export output missing cgpf_version field: %s", stdout)
	}

	// Run version.
	stdout, _, code = run(t, dir, "version")
	if code != 0 {
		t.Errorf("version exited %d", code)
	}
	if !strings.Contains(stdout, "v1") && !strings.Contains(stdout, "barq-witness") {
		t.Errorf("version output = %q, want something with v1 or barq-witness", stdout)
	}
}

func TestFullSession_VersionCommand(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := run(t, dir, "version")
	if code != 0 {
		t.Fatalf("version exited %d", code)
	}
	if !strings.Contains(stdout, "barq-witness") {
		t.Errorf("version output missing barq-witness: %s", stdout)
	}
}

func TestFullSession_UnknownCommandFails(t *testing.T) {
	dir := t.TempDir()
	_, _, code := run(t, dir, "unknowncmd123")
	if code == 0 {
		t.Error("unknown command should exit non-zero")
	}
}

func TestFullSession_ExportWithData(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	// Seed a session.
	record(t, dir, "session-start", `{"session_id":"export-test","cwd":"`+dir+`","model":"claude-sonnet-4-6"}`)
	record(t, dir, "prompt", `{"session_id":"export-test","prompt":"write tests"}`)
	record(t, dir, "session-end", `{"session_id":"export-test"}`)

	stdout, _, code := run(t, dir, "export")
	if code != 0 {
		t.Fatalf("export exited %d", code)
	}
	if !strings.Contains(stdout, "export-test") {
		t.Errorf("export output does not contain session id: %s", stdout)
	}
}
