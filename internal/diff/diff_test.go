package diff_test

import (
	"testing"

	"github.com/yasserrmd/barq-witness/internal/diff"
	"github.com/yasserrmd/barq-witness/internal/testutil"
)

// TestChangedFiles_TwoCommits verifies ChangedFiles returns the modified file.
func TestChangedFiles_TwoCommits(t *testing.T) {
	repoPath, parentSHA, headSHA := testutil.NewFixtureRepo(t)

	changes, err := diff.ChangedFiles(repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one changed file")
	}

	found := false
	for _, c := range changes {
		if c.Path == "main.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected main.go in changes, got: %v", changes)
	}
}

// TestChangedFiles_EmptyFromSHA uses implicit parent detection.
func TestChangedFiles_EmptyFromSHA(t *testing.T) {
	repoPath, _, headSHA := testutil.NewFixtureRepo(t)

	changes, err := diff.ChangedFiles(repoPath, "", headSHA)
	if err != nil {
		t.Fatalf("ChangedFiles with empty fromSHA: %v", err)
	}

	// The second commit modified main.go relative to its parent.
	if len(changes) == 0 {
		t.Fatal("expected at least one changed file when fromSHA is empty")
	}
}

// TestChangedFiles_InvalidRepo returns an error for a non-git directory.
func TestChangedFiles_InvalidRepo(t *testing.T) {
	_, err := diff.ChangedFiles("/nonexistent/path/xyz", "abc", "def")
	if err == nil {
		t.Fatal("expected error for invalid repo path")
	}
}

// TestChangedFiles_InvalidToSHA returns an error for a bad SHA.
func TestChangedFiles_InvalidToSHA(t *testing.T) {
	repoPath, _, _ := testutil.NewFixtureRepo(t)
	_, err := diff.ChangedFiles(repoPath, "", "0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for nonexistent commit SHA")
	}
}

// TestChangedFilePaths_ReturnsPaths verifies the convenience wrapper.
func TestChangedFilePaths_ReturnsPaths(t *testing.T) {
	repoPath, parentSHA, headSHA := testutil.NewFixtureRepo(t)

	paths, err := diff.ChangedFilePaths(repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("ChangedFilePaths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("expected at least one path")
	}

	found := false
	for _, p := range paths {
		if p == "main.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected main.go in paths, got: %v", paths)
	}
}

// TestChangedFilePaths_InvalidRepo returns an error for a bad path.
func TestChangedFilePaths_InvalidRepo(t *testing.T) {
	_, err := diff.ChangedFilePaths("/nonexistent/path/xyz", "abc", "def")
	if err == nil {
		t.Fatal("expected error for invalid repo path")
	}
}

// TestFileChange_Fields verifies that FileChange fields are populated.
func TestFileChange_Fields(t *testing.T) {
	repoPath, parentSHA, headSHA := testutil.NewFixtureRepo(t)

	changes, err := diff.ChangedFiles(repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}

	for _, c := range changes {
		if c.Path == "" {
			t.Error("FileChange.Path should not be empty")
		}
		// Modified files should have added lines (we added a function).
		if len(c.AddedLines) == 0 && len(c.DeletedLines) == 0 && !c.IsNew && !c.IsDeleted {
			t.Logf("note: no line-level changes for %s (may be binary or rename)", c.Path)
		}
	}
}

// TestChangedFiles_InitialCommit uses the first commit against an empty tree.
func TestChangedFiles_InitialCommit(t *testing.T) {
	repoPath, parentSHA, _ := testutil.NewFixtureRepo(t)

	// The first commit (parentSHA) introduced main.go from nothing.
	changes, err := diff.ChangedFiles(repoPath, "", parentSHA)
	if err != nil {
		t.Fatalf("ChangedFiles for initial commit: %v", err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one change in initial commit")
	}

	for _, c := range changes {
		if c.Path == "main.go" {
			if !c.IsNew {
				t.Error("expected IsNew=true for file in initial commit")
			}
		}
	}
}
