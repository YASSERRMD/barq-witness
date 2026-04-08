// Package testutil provides shared test helpers for barq-witness tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// NewFixtureStore creates an in-memory SQLite store with known data.
func NewFixtureStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "trace.db"))
	if err != nil {
		t.Fatalf("open fixture store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// NewFixtureRepo creates a git repo in t.TempDir() with two commits.
// Returns the repo path and two commit SHAs (parent, head).
func NewFixtureRepo(t *testing.T) (repoPath, parentSHA, headSHA string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// First commit
	f := filepath.Join(dir, "main.go")
	os.WriteFile(f, []byte("package main\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "initial")
	parentSHA = run("rev-parse", "HEAD")

	// Second commit
	os.WriteFile(f, []byte("package main\nfunc main() {}\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "add main")
	headSHA = run("rev-parse", "HEAD")

	return dir, parentSHA, headSHA
}

// NewFixtureSession inserts a complete session with edits and executions.
// Returns the session ID.
func NewFixtureSession(t *testing.T, st *store.Store) string {
	t.Helper()
	// Sanitize t.Name() to avoid slashes and other illegal chars in the session ID.
	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	sessionID := "test-session-" + name

	err := st.InsertSession(model.Session{
		ID:           sessionID,
		StartedAt:    time.Now().Add(-10 * time.Minute).UnixMilli(),
		CWD:          "/test",
		GitHeadStart: "abc123",
		Model:        "claude-sonnet-4-6",
		Source:       "claude-code",
	})
	if err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	return sessionID
}

// LoadFixture reads a file from the testdata directory relative to the test.
func LoadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}
