package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests.
	tmp, err := os.MkdirTemp("", "barq-witness-integration-*")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "barq-witness")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/barq-witness")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}

	os.Exit(m.Run())
}

// run executes the binary with args in dir and returns stdout, stderr, and exit code.
func run(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// record pipes JSON to the record subcommand and returns the exit code.
func record(t *testing.T, dir string, subcmd string, payload string) (exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, "record", subcmd)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(payload)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	if err != nil {
		t.Logf("record %s stderr: %s", subcmd, errBuf.String())
	}
	return 0
}

// makeGitRepo creates a temporary directory, initialises a git repository,
// and makes an initial commit so that barq-witness init can succeed.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	gitRun("init")
	gitRun("config", "user.email", "test@test.com")
	gitRun("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	gitRun("add", ".")
	gitRun("commit", "-m", "initial")
	return dir
}
