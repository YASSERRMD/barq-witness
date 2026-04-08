package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_CreatesDotWitness(t *testing.T) {
	dir := makeGitRepo(t)
	stdout, stderr, code := run(t, dir, "init")
	_ = stdout
	_ = stderr
	if code != 0 {
		t.Fatalf("init exited %d: stderr=%s", code, stderr)
	}
	// Check .witness/ exists.
	if _, err := os.Stat(filepath.Join(dir, ".witness")); err != nil {
		t.Errorf(".witness dir not created: %v", err)
	}
	// Check trace.db exists inside .witness/.
	if _, err := os.Stat(filepath.Join(dir, ".witness", "trace.db")); err != nil {
		t.Errorf(".witness/trace.db not created: %v", err)
	}
	// Check .claude/settings.json exists.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Errorf(".claude/settings.json not created: %v", err)
	}
	// Check .gitignore contains .witness.
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".witness") {
		t.Errorf(".gitignore does not mention .witness; got: %s", string(data))
	}
}

func TestInit_IsIdempotent(t *testing.T) {
	dir := makeGitRepo(t)
	_, _, code1 := run(t, dir, "init")
	_, _, code2 := run(t, dir, "init")
	if code1 != 0 || code2 != 0 {
		t.Fatalf("init failed: first=%d second=%d", code1, code2)
	}
}

func TestInit_NonGitDirFails(t *testing.T) {
	// A plain temp dir with no .git must cause init to fail.
	dir := t.TempDir()
	_, _, code := run(t, dir, "init")
	if code == 0 {
		t.Error("expected non-zero exit for non-git dir, got 0")
	}
}

func TestInit_OutputMentionsDatabase(t *testing.T) {
	dir := makeGitRepo(t)
	stdout, _, code := run(t, dir, "init")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if !strings.Contains(stdout, "trace.db") && !strings.Contains(stdout, "Trace database") {
		t.Errorf("init output does not mention database: %s", stdout)
	}
}
