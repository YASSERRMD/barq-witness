package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newShortDaemon starts a daemon with a socket path short enough for macOS
// (Unix socket paths are limited to ~104 characters on macOS).
// It creates a directory in /tmp with a short random suffix.
func newShortDaemon(t *testing.T) (*Daemon, string, func()) {
	t.Helper()
	// Use os.MkdirTemp with a short prefix so the socket path stays well under 104 chars.
	dir, err := os.MkdirTemp("", "bw")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	socketPath := filepath.Join(dir, "d.sock")
	dbPath := filepath.Join(dir, "t.db")

	d, err := New(socketPath, dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	cleanup := func() {
		d.Stop()
		os.RemoveAll(dir)
	}
	return d, socketPath, cleanup
}

// sendMsg dials the socket, sends msg, returns the response, and closes.
func sendMsg(t *testing.T, socketPath string, msg map[string]any) map[string]any {
	t.Helper()
	c, err := Dial(socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	resp, err := c.Send(msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	return resp
}

// TestHandlePrompt_Happy verifies that a prompt message is stored correctly.
func TestHandlePrompt_Happy(t *testing.T) {
	d, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{
		"op": "session_start", "session_id": "sess-p1", "cwd": "/tmp",
	})
	assertOK(t, resp, "session_start")

	resp = sendMsg(t, socketPath, map[string]any{
		"op":         "prompt",
		"session_id": "sess-p1",
		"content":    "Write a function to reverse a string",
		"timestamp":  time.Now().UnixMilli(),
	})
	assertOK(t, resp, "prompt")

	time.Sleep(20 * time.Millisecond)

	prompts, err := d.store.PromptsForSession("sess-p1")
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Content != "Write a function to reverse a string" {
		t.Errorf("prompt content = %q", prompts[0].Content)
	}
	if prompts[0].ContentHash == "" {
		t.Error("expected non-empty ContentHash")
	}
}

// TestHandlePrompt_MissingSessionID returns error when session_id is missing.
func TestHandlePrompt_MissingSessionID(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{
		"op": "prompt", "content": "some prompt",
	})
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false when session_id missing from prompt op")
	}
}

// TestHandleExecution_Happy verifies that an execution message is stored correctly.
func TestHandleExecution_Happy(t *testing.T) {
	d, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{
		"op": "session_start", "session_id": "sess-ex1", "cwd": "/tmp",
	})
	assertOK(t, resp, "session_start")

	resp = sendMsg(t, socketPath, map[string]any{
		"op":             "execution",
		"session_id":     "sess-ex1",
		"command":        "go build ./...",
		"classification": "build",
		"exit_code":      float64(0),
		"duration_ms":    float64(1234),
		"timestamp":      time.Now().UnixMilli(),
	})
	assertOK(t, resp, "execution")

	time.Sleep(20 * time.Millisecond)

	execs, err := d.store.ExecutionsForSession("sess-ex1")
	if err != nil {
		t.Fatalf("ExecutionsForSession: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].Command != "go build ./..." {
		t.Errorf("command = %q", execs[0].Command)
	}
	if execs[0].Classification != "build" {
		t.Errorf("classification = %q", execs[0].Classification)
	}
}

// TestHandleExecution_MissingSessionID returns error when session_id is missing.
func TestHandleExecution_MissingSessionID(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{
		"op": "execution", "command": "go test",
	})
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false when session_id missing from execution op")
	}
}

// TestUnknownOp returns error JSON for an unknown op field.
func TestUnknownOp(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{"op": "totally_unknown_operation"})
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false for unknown op")
	}
	if errMsg, _ := resp["error"].(string); errMsg == "" {
		t.Error("expected non-empty error message for unknown op")
	}
}

// TestInvalidJSON returns error JSON for invalid JSON input.
func TestInvalidJSON(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	c, err := Dial(socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if _, err := c.conn.Write([]byte("{this is not valid json}\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var resp map[string]any
	if err := json.NewDecoder(c.conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

// TestHandleSessionStart_MissingID returns error when session_id is missing.
func TestHandleSessionStart_MissingID(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{"op": "session_start"})
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false when session_id missing")
	}
}

// TestHandleSessionEnd_MissingID returns error when session_id is missing.
func TestHandleSessionEnd_MissingID(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{"op": "session_end"})
	if ok, _ := resp["ok"].(bool); ok {
		t.Error("expected ok=false when session_id missing from session_end")
	}
}

// TestHandlePrompt_NoTimestamp uses current time when timestamp is absent.
func TestHandlePrompt_NoTimestamp(t *testing.T) {
	d, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{"op": "session_start", "session_id": "sess-nots"})
	assertOK(t, resp, "session_start")

	resp = sendMsg(t, socketPath, map[string]any{
		"op": "prompt", "session_id": "sess-nots", "content": "hello",
		// timestamp intentionally omitted
	})
	assertOK(t, resp, "prompt")

	time.Sleep(20 * time.Millisecond)

	prompts, err := d.store.PromptsForSession("sess-nots")
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("expected prompt to be stored without explicit timestamp")
	}
	if prompts[0].Timestamp <= 0 {
		t.Error("expected positive timestamp set by daemon")
	}
}

// TestHandleEdit_WithPromptLinkage verifies edit is linked to latest prompt.
func TestHandleEdit_WithPromptLinkage(t *testing.T) {
	d, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	resp := sendMsg(t, socketPath, map[string]any{"op": "session_start", "session_id": "sess-lnk"})
	assertOK(t, resp, "session_start")

	resp = sendMsg(t, socketPath, map[string]any{
		"op": "prompt", "session_id": "sess-lnk", "content": "add error handling",
	})
	assertOK(t, resp, "prompt")

	resp = sendMsg(t, socketPath, map[string]any{
		"op": "edit", "session_id": "sess-lnk",
		"file_path": "handler.go", "tool": "Edit",
		"diff": "@@ -1 +1 @@\n+if err != nil { return err }\n",
		"line_start": float64(5), "line_end": float64(5),
	})
	assertOK(t, resp, "edit")

	time.Sleep(30 * time.Millisecond)

	edits, err := d.store.EditsForSession("sess-lnk")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].PromptID == nil {
		t.Error("expected edit to be linked to prompt, PromptID is nil")
	}
}

// TestDerivePIDPath verifies the PID path derivation logic.
func TestDerivePIDPath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/tmp/barq/daemon.sock", "/tmp/barq/daemon.pid"},
		{"/var/run/daemon.sock", "/var/run/daemon.pid"},
	}
	for _, tc := range cases {
		got := derivePIDPath(tc.input)
		if got != tc.want {
			t.Errorf("derivePIDPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestJSONInt64_Types verifies jsonInt64 handles all numeric type variants.
func TestJSONInt64_Types(t *testing.T) {
	cases := []struct {
		name string
		msg  map[string]any
		want int64
	}{
		{"float64", map[string]any{"v": float64(42)}, 42},
		{"int64", map[string]any{"v": int64(99)}, 99},
		{"int", map[string]any{"v": int(7)}, 7},
		{"missing", map[string]any{}, 0},
		{"string_ignored", map[string]any{"v": "hello"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonInt64(tc.msg, "v")
			if got != tc.want {
				t.Errorf("jsonInt64 = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestJSONInt64Ptr_Zero returns nil for zero value.
func TestJSONInt64Ptr_Zero(t *testing.T) {
	msg := map[string]any{"v": float64(0)}
	ptr := jsonInt64Ptr(msg, "v")
	if ptr != nil {
		t.Errorf("expected nil for zero value, got %v", ptr)
	}
}

// TestJSONInt64Ptr_NonZero returns a pointer for non-zero value.
func TestJSONInt64Ptr_NonZero(t *testing.T) {
	msg := map[string]any{"v": float64(500)}
	ptr := jsonInt64Ptr(msg, "v")
	if ptr == nil {
		t.Fatal("expected non-nil pointer for value 500")
	}
	if *ptr != 500 {
		t.Errorf("*ptr = %d, want 500", *ptr)
	}
}

// TestJSONIntPtr_Types verifies jsonIntPtr returns correct values.
func TestJSONIntPtr_Types(t *testing.T) {
	msg := map[string]any{"exit": float64(1)}
	ptr := jsonIntPtr(msg, "exit")
	if ptr == nil {
		t.Fatal("expected non-nil pointer for exit=1")
	}
	if *ptr != 1 {
		t.Errorf("*ptr = %d, want 1", *ptr)
	}

	missing := jsonIntPtr(map[string]any{}, "exit")
	if missing != nil {
		t.Error("expected nil for missing key")
	}
}

// TestIsDaemonRunning_False verifies IsDaemonRunning returns false for a non-existent socket.
func TestIsDaemonRunning_False(t *testing.T) {
	if IsDaemonRunning("/tmp/nonexistent-bw-test.sock") {
		t.Error("expected IsDaemonRunning to return false for non-existent socket")
	}
}

// TestIsDaemonRunning_True verifies IsDaemonRunning returns true for a running daemon.
func TestIsDaemonRunning_True(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	if !IsDaemonRunning(socketPath) {
		t.Error("expected IsDaemonRunning to return true for running daemon")
	}
}

// TestAlreadyRunning verifies New returns error if daemon already running on socket.
func TestAlreadyRunning(t *testing.T) {
	_, socketPath, cleanup := newShortDaemon(t)
	defer cleanup()

	// Try to create a second daemon on the same socket -- should fail.
	dir, _ := os.MkdirTemp("", "bw2")
	defer os.RemoveAll(dir)
	_, err := New(socketPath, filepath.Join(dir, "t.db"))
	if err == nil {
		t.Fatal("expected error when daemon already running on socket")
	}
}

// Compile-time check: fmt is used via Sprintf in test helper.
var _ = fmt.Sprintf
