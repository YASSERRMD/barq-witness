package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestDaemon starts a daemon in a temporary directory and returns the
// daemon, the socket path, and a cleanup function.
func newTestDaemon(t *testing.T) (*Daemon, string, string, func()) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "daemon.sock")
	dbPath := filepath.Join(dir, "trace.db")

	d, err := New(socketPath, dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Give the accept loop a moment to be ready.
	time.Sleep(20 * time.Millisecond)

	cleanup := func() {
		d.Stop()
	}
	return d, socketPath, dbPath, cleanup
}

// TestPing verifies that a running daemon responds to a ping message.
func TestPing(t *testing.T) {
	_, socketPath, _, cleanup := newTestDaemon(t)
	defer cleanup()

	c, err := Dial(socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if !c.Ping() {
		t.Fatal("expected Ping to return true")
	}
}

// TestSessionFlow sends session_start, edit, and session_end messages to the
// daemon and verifies the records appear in the store.
func TestSessionFlow(t *testing.T) {
	d, socketPath, _, cleanup := newTestDaemon(t)
	defer cleanup()

	c, err := Dial(socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	sessionID := "test-session-42"
	now := time.Now().UnixMilli()

	// session_start
	resp, err := c.Send(map[string]any{
		"op":         "session_start",
		"session_id": sessionID,
		"cwd":        "/tmp/test",
		"model":      "claude-opus-4",
		"git_head":   "abc123",
	})
	if err != nil {
		t.Fatalf("Send session_start: %v", err)
	}
	assertOK(t, resp, "session_start")

	// edit
	resp, err = c.Send(map[string]any{
		"op":         "edit",
		"session_id": sessionID,
		"file_path":  "main.go",
		"tool":       "Edit",
		"diff":       "@@ -1,1 +1,1 @@\n-old\n+new\n",
		"line_start": 1,
		"line_end":   1,
		"timestamp":  now,
	})
	if err != nil {
		t.Fatalf("Send edit: %v", err)
	}
	assertOK(t, resp, "edit")

	// session_end
	resp, err = c.Send(map[string]any{
		"op":         "session_end",
		"session_id": sessionID,
	})
	if err != nil {
		t.Fatalf("Send session_end: %v", err)
	}
	assertOK(t, resp, "session_end")

	// Allow the store writes to complete before querying.
	time.Sleep(20 * time.Millisecond)

	// Verify session in store.
	sessions, err := d.store.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("session %q not found in store", sessionID)
	}

	// Verify edit in store.
	edits, err := d.store.EditsForSession(sessionID)
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].FilePath != "main.go" {
		t.Errorf("expected file_path main.go, got %q", edits[0].FilePath)
	}
}

// TestStopCleansUpSocket verifies that Stop removes the socket file.
func TestStopCleansUpSocket(t *testing.T) {
	_, socketPath, _, cleanup := newTestDaemon(t)

	// Verify socket exists before stop.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("socket should exist after Start")
	}

	cleanup() // calls d.Stop()
	time.Sleep(20 * time.Millisecond)

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatal("socket should be removed after Stop")
	}
}

// TestDialNotRunning verifies that Dial returns an error when no daemon is
// listening.
func TestDialNotRunning(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "nonexistent.sock")
	_, err := Dial(socketPath)
	if err == nil {
		t.Fatal("expected error dialing non-existent socket")
	}
}

// assertOK fails the test if the response does not have "ok": true.
func assertOK(t *testing.T, resp map[string]any, op string) {
	t.Helper()
	ok, _ := resp["ok"].(bool)
	if !ok {
		errMsg, _ := resp["error"].(string)
		t.Fatalf("%s: expected ok=true, got error=%q", op, errMsg)
	}
}
