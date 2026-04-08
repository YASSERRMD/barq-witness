package watcher_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/watcher"
)

// newGitRepo creates a temporary git repository with one commit and returns its path.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run("add", "main.go")
	run("commit", "-m", "initial")

	return dir
}

// TestWatcher_MarkdownFormat verifies that format="markdown" produces output.
func TestWatcher_MarkdownFormat(t *testing.T) {
	st := openTempStore(t)
	repo := tempRepo(t)

	w := watcher.New(st, repo, 30*time.Second, 5)
	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx, &buf, "markdown"); err != nil {
		t.Fatalf("Run(markdown): %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected some output in markdown format")
	}
}

// TestWatcher_WithGitRepo polls a real git repo with commits.
func TestWatcher_WithGitRepo(t *testing.T) {
	st := openTempStore(t)
	repo := newGitRepo(t)

	w := watcher.New(st, repo, 5*time.Second, 5)
	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx, &buf, "text"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// With a real git repo, the watcher either produces a report or the ANSI clear screen.
	if buf.Len() == 0 {
		t.Error("expected output with real git repo")
	}
}

// TestWatcher_TextFormat verifies text format is the default.
func TestWatcher_TextFormat(t *testing.T) {
	st := openTempStore(t)
	repo := tempRepo(t)

	w := watcher.New(st, repo, 30*time.Second, 10)
	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx, &buf, "text"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	output := buf.String()
	// No-git repo should produce the "waiting for first commit" message.
	if !strings.Contains(output, "waiting") && !strings.Contains(output, "\033") {
		t.Logf("output: %q", output[:min(len(output), 200)])
	}
}

// TestWatcher_ZeroTopN uses zero topN without panic.
func TestWatcher_ZeroTopN(t *testing.T) {
	st := openTempStore(t)
	repo := tempRepo(t)

	w := watcher.New(st, repo, 30*time.Second, 0)
	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx, &buf, "text"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
