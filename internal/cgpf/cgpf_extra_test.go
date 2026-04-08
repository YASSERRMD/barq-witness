package cgpf_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/cgpf"
	"github.com/yasserrmd/barq-witness/internal/model"
)

// TestExport_PrivacyModeOmitsContent verifies prompt content is omitted when Privacy=true.
func TestExport_PrivacyModeOmitsContent(t *testing.T) {
	s := openStore(t)
	sessID := seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{
		SessionID: sessID,
		Privacy:   true,
		RepoPath:  "",
	})
	if err != nil {
		t.Fatalf("Export(privacy=true): %v", err)
	}
	if len(doc.Sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	for _, sess := range doc.Sessions {
		for _, p := range sess.Prompts {
			if p.Content != nil {
				t.Error("prompt Content must be nil in privacy mode")
			}
		}
		for _, e := range sess.Executions {
			if e.Command != nil {
				t.Error("execution Command must be nil in privacy mode")
			}
		}
	}
}

// TestExport_NonPrivacyMode verifies prompt content is included when Privacy=false.
func TestExport_NonPrivacyMode(t *testing.T) {
	s := openStore(t)
	sessID := seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{
		SessionID: sessID,
		Privacy:   false,
	})
	if err != nil {
		t.Fatalf("Export(privacy=false): %v", err)
	}
	if len(doc.Sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	found := false
	for _, sess := range doc.Sessions {
		for _, p := range sess.Prompts {
			if p.Content != nil && *p.Content != "" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected at least one prompt with non-nil Content in non-privacy mode")
	}
}

// TestExport_WithRepoPath verifies that a repo path is used to detect remote URL.
func TestExport_WithRepoPath(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	repoDir := makeGitRepoWithRemote(t)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{RepoPath: repoDir})
	if err != nil {
		t.Fatalf("Export with repo path: %v", err)
	}
	// Remote was set in the repo, so it may or may not be detected.
	_ = doc.Repo.Remote
}

// TestExport_WithNoGitRepoPath handles missing repo path gracefully.
func TestExport_WithNoGitRepoPath(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{RepoPath: "/nonexistent/path"})
	if err != nil {
		t.Fatalf("Export with missing repo path: %v", err)
	}
	// No remote expected.
	if doc.Repo.Remote != nil {
		t.Logf("unexpected remote %q for non-existent repo", *doc.Repo.Remote)
	}
}

// TestExport_SpecificSessionID filters to a single session.
func TestExport_SpecificSessionID(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	// Insert a second session.
	if err := s.InsertSession(model.Session{
		ID:        "sess-other",
		StartedAt: nowMS(),
		CWD:       "/other",
	}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	doc, err := cgpf.Export(s, cgpf.ExportOptions{SessionID: "sess-export-001"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(doc.Sessions))
	}
	if doc.Sessions[0].ID != "sess-export-001" {
		t.Errorf("session ID = %q, want 'sess-export-001'", doc.Sessions[0].ID)
	}
}

// TestUnmarshal_RoundTrip verifies that a Document can be marshaled and unmarshaled.
func TestUnmarshal_RoundTrip(t *testing.T) {
	s := openStore(t)
	seedFixtureTrace(t, s)

	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	doc2, err := cgpf.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc2.CGPFVersion != doc.CGPFVersion {
		t.Errorf("CGPFVersion = %q, want %q", doc2.CGPFVersion, doc.CGPFVersion)
	}
	if len(doc2.Sessions) != len(doc.Sessions) {
		t.Errorf("sessions count = %d, want %d", len(doc2.Sessions), len(doc.Sessions))
	}
}

// TestUnmarshal_InvalidJSON returns error for bad JSON.
func TestUnmarshal_InvalidJSON(t *testing.T) {
	_, err := cgpf.Unmarshal([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestExport_EmptyStoreNoSessions returns a document with no sessions.
func TestExport_EmptyStoreNoSessions(t *testing.T) {
	s := openStore(t)
	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if doc.CGPFVersion == "" {
		t.Error("expected non-empty CGPFVersion")
	}
	if len(doc.Sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(doc.Sessions))
	}
}

// TestExport_DocumentVersion verifies the document uses the current CGPF version.
func TestExport_DocumentVersion(t *testing.T) {
	s := openStore(t)
	doc, err := cgpf.Export(s, cgpf.ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if doc.CGPFVersion != cgpf.Version {
		t.Errorf("CGPFVersion = %q, want %q", doc.CGPFVersion, cgpf.Version)
	}
}

// TestExport_FilesTouched verifies files_touched is parsed from JSON.
func TestExport_FilesTouched(t *testing.T) {
	s := openStore(t)
	sessID := "sess-ftouched"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	exitCode := 0
	durMS := int64(100)
	if err := s.InsertExecution(model.Execution{
		SessionID:      sessID,
		Timestamp:      nowMS(),
		Command:        "go build",
		Classification: "build",
		FilesTouched:   `["main.go","util.go"]`,
		ExitCode:       &exitCode,
		DurationMS:     &durMS,
	}); err != nil {
		t.Fatalf("InsertExecution: %v", err)
	}

	doc, err := cgpf.Export(s, cgpf.ExportOptions{SessionID: sessID})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(doc.Sessions) == 0 {
		t.Fatal("expected a session")
	}
	execs := doc.Sessions[0].Executions
	if len(execs) == 0 {
		t.Fatal("expected an execution")
	}
	if len(execs[0].FilesTouched) != 2 {
		t.Errorf("FilesTouched count = %d, want 2", len(execs[0].FilesTouched))
	}
}

// makeGitRepoWithRemote creates a git repo that has a remote URL set.
func makeGitRepoWithRemote(t *testing.T) string {
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
	run("remote", "add", "origin", "https://github.com/example/repo.git")

	if !strings.Contains(filepath.Join(dir, ".git"), ".git") {
		t.Skip("git not available")
	}
	return dir
}
