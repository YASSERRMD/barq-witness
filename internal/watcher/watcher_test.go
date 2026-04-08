package watcher_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/watcher"
)

// openTempStore creates a temporary SQLite store for testing.
func openTempStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trace.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open temp store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// tempRepo returns the path to a temp directory that acts as the repo root.
// We use the OS temp dir itself -- no commits means HeadSHA returns "" which
// the watcher handles gracefully.
func tempRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "barq-witness-test-repo-*")
	if err != nil {
		t.Fatalf("create temp repo dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestWatcher_RunsWithoutError starts the watcher for 3 seconds and checks
// that it writes output and returns nil when the context is cancelled.
func TestWatcher_RunsWithoutError(t *testing.T) {
	st := openTempStore(t)
	repo := tempRepo(t)

	w := watcher.New(st, repo, 1*time.Second, 10)

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := w.Run(ctx, &buf, "text")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected output written to buffer, got none")
	}
}

// TestWatcher_CancelStops verifies that cancelling the context causes Run to
// return promptly (well within 2 seconds).
func TestWatcher_CancelStops(t *testing.T) {
	st := openTempStore(t)
	repo := tempRepo(t)

	w := watcher.New(st, repo, 30*time.Second, 10) // very long interval

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- w.Run(ctx, &bytes.Buffer{}, "text")
	}()

	// Cancel almost immediately.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2 seconds after context cancel")
	}
}
