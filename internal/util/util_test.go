package util_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/testutil"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// TestHeadSHA_ReturnsCorrectSHA verifies HeadSHA returns the HEAD commit.
func TestHeadSHA_ReturnsCorrectSHA(t *testing.T) {
	repoPath, _, headSHA := testutil.NewFixtureRepo(t)

	got, err := util.HeadSHA(repoPath)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty SHA")
	}
	if got != headSHA {
		t.Errorf("HeadSHA = %q, want %q", got, headSHA)
	}
}

// TestHeadSHA_NonExistentPath returns an error for a missing directory.
func TestHeadSHA_NonExistentPath(t *testing.T) {
	_, err := util.HeadSHA("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

// TestHeadSHA_NoCommits returns empty string (or error) for a repo with no commits.
func TestHeadSHA_NoCommits(t *testing.T) {
	dir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	got, err := util.HeadSHA(dir)
	if err != nil {
		// Some git versions return error on unborn HEAD -- acceptable.
		t.Logf("HeadSHA on unborn HEAD returned error (acceptable): %v", err)
		return
	}
	if got != "" {
		t.Errorf("expected empty SHA for no-commit repo, got %q", got)
	}
}

// TestSHA256Hex_KnownValue verifies SHA256Hex against a known hash.
func TestSHA256Hex_KnownValue(t *testing.T) {
	// SHA-256 of empty input is a well-known constant.
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got := util.SHA256Hex([]byte(""))
	if got != want {
		t.Errorf("SHA256Hex(\"\") = %q, want %q", got, want)
	}
}

// TestSHA256Hex_NonEmpty verifies SHA256Hex on a non-empty input.
func TestSHA256Hex_NonEmpty(t *testing.T) {
	got := util.SHA256Hex([]byte("hello"))
	if len(got) != 64 {
		t.Errorf("SHA256Hex: expected 64-char hex, got %d", len(got))
	}
	// Different inputs must produce different hashes.
	got2 := util.SHA256Hex([]byte("world"))
	if got == got2 {
		t.Error("SHA256Hex(hello) == SHA256Hex(world) -- collision!")
	}
}

// TestSHA256HexString_Deterministic verifies SHA256HexString is deterministic.
func TestSHA256HexString_Deterministic(t *testing.T) {
	got1 := util.SHA256HexString("test input")
	got2 := util.SHA256HexString("test input")
	if got1 != got2 {
		t.Error("SHA256HexString is not deterministic")
	}
	if len(got1) != 64 {
		t.Errorf("expected 64-char hex, got %d", len(got1))
	}
}

// TestSHA256HexString_EqualsHex verifies SHA256HexString matches SHA256Hex.
func TestSHA256HexString_EqualsHex(t *testing.T) {
	input := "barq-witness"
	fromString := util.SHA256HexString(input)
	fromBytes := util.SHA256Hex([]byte(input))
	if fromString != fromBytes {
		t.Errorf("SHA256HexString != SHA256Hex: %q vs %q", fromString, fromBytes)
	}
}

// TestOpenLogger_FallsBackToDiscard verifies OpenLogger does not panic on bad path.
func TestOpenLogger_FallsBackToDiscard(t *testing.T) {
	logger := util.OpenLogger("/nonexistent-dir-xyz-barq-witness-test")
	if logger == nil {
		t.Fatal("expected non-nil logger even on error")
	}
	// Should not panic on write.
	logger.Printf("test message")
}

// TestOpenLogger_ValidDir verifies OpenLogger creates and writes to a log file.
func TestOpenLogger_ValidDir(t *testing.T) {
	dir := t.TempDir()
	logger := util.OpenLogger(dir)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.Printf("test entry")

	// Verify the log file was created.
	logPath := filepath.Join(dir, "barq-witness.log")
	if !fileExists(logPath) {
		t.Errorf("expected log file at %s", logPath)
	}
}

func fileExists(path string) bool {
	cmd := exec.Command("test", "-f", path)
	return cmd.Run() == nil
}

// TestHeadSHA_AfterSecondCommit verifies HEAD advances after a second commit.
func TestHeadSHA_AfterSecondCommit(t *testing.T) {
	repoPath, parentSHA, headSHA := testutil.NewFixtureRepo(t)

	got, err := util.HeadSHA(repoPath)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	if got == parentSHA {
		t.Error("HEAD should be head commit, not parent commit")
	}
	if got != headSHA {
		t.Errorf("HeadSHA = %q, want headSHA %q", got, headSHA)
	}

	// Verify the two SHAs are 40-char hex strings.
	if !isHex40(parentSHA) {
		t.Errorf("parentSHA %q is not a 40-char hex string", parentSHA)
	}
	if !isHex40(headSHA) {
		t.Errorf("headSHA %q is not a 40-char hex string", headSHA)
	}
}

func isHex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	return strings.Trim(s, "0123456789abcdef") == ""
}
