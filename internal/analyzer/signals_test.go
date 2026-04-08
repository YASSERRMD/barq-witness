package analyzer_test

// signals_test.go contains focused end-to-end signal tests that drive Analyze
// with controlled trace + git repo data to verify each signal fires and stays
// quiet as expected.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// initGitRepoWithFiles creates a git repo, commits a first version of the
// named files (empty content), then commits a second version (with "line N"
// content) so that each file appears in the HEAD..parent diff.
func initGitRepoWithFiles(t *testing.T, fileNames ...string) (repoPath, parentSHA, headSHA string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@t.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@t.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	run("config", "user.email", "t@t.com")
	run("config", "user.name", "Test")

	// First commit: create all files with placeholder content.
	for _, name := range fileNames {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("// placeholder\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		run("add", name)
	}
	run("commit", "-m", "initial")
	pSHA := run("rev-parse", "HEAD")

	// Second commit: update each file with content so lines differ.
	for i, name := range fileNames {
		full := filepath.Join(dir, name)
		content := fmt.Sprintf("// updated %s\nline %d\n", name, i+1)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		run("add", name)
	}
	run("commit", "-m", "update")
	hSHA := run("rev-parse", "HEAD")

	return dir, pSHA, hSHA
}

func openFreshStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ms(d time.Duration) int64 { return time.Now().Add(-d).UnixMilli() }

// hasReason returns true if the segment contains the given reason code.
func hasReason(seg analyzer.Segment, code string) bool {
	for _, r := range seg.ReasonCodes {
		if r == code {
			return true
		}
	}
	return false
}

// firstSegmentForFile returns the first segment whose FilePath matches.
func firstSegmentForFile(report *analyzer.Report, path string) (analyzer.Segment, bool) {
	for _, seg := range report.Segments {
		if seg.FilePath == path {
			return seg, true
		}
	}
	return analyzer.Segment{}, false
}

// --- NO_EXEC fires ----------------------------------------------------------

func TestSignal_NoExec_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-noexec-fire"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: ms(5 * time.Minute),
		Content: "write main", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: ms(4 * time.Minute), FilePath: "main.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// No execution inserted.

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "NO_EXEC") {
		t.Errorf("expected NO_EXEC, got %v", seg.ReasonCodes)
	}
}

// --- NO_EXEC quiet when executed -------------------------------------------

func TestSignal_NoExec_Quiet(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-noexec-quiet"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: ms(5 * time.Minute),
		Content: "write main", ContentHash: "h",
	})
	editTS := ms(4 * time.Minute)
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: editTS, FilePath: "main.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	exitCode := 0
	if err := s.InsertExecution(model.Execution{
		SessionID: sessID, Timestamp: editTS + 1000,
		Command: "go run main.go", Classification: "run",
		FilesTouched: `["main.go"]`, ExitCode: &exitCode,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if seg, ok := firstSegmentForFile(report, "main.go"); ok {
		if hasReason(seg, "NO_EXEC") {
			t.Errorf("NO_EXEC should not fire when execution exists, got %v", seg.ReasonCodes)
		}
	}
}

// --- FAST_ACCEPT_SECURITY fires ---------------------------------------------

func TestSignal_FastAcceptSecurity_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "internal/auth/handler.go")

	sessID := "sess-fast-sec"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	// Prompt 3 seconds before edit.
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 3000,
		Content: "add auth check", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: now, FilePath: "internal/auth/handler.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "internal/auth/handler.go")
	if !ok {
		t.Fatal("expected segment for auth/handler.go")
	}
	if !hasReason(seg, "FAST_ACCEPT_SECURITY") {
		t.Errorf("expected FAST_ACCEPT_SECURITY, got %v", seg.ReasonCodes)
	}
	if seg.Tier != 1 {
		t.Errorf("expected tier 1, got %d", seg.Tier)
	}
}

// --- FAST_ACCEPT_GENERIC fires -----------------------------------------------

func TestSignal_FastAcceptGeneric_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-fast-gen"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	// Prompt 2 seconds before edit, non-security path.
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 2000,
		Content: "add func", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: now, FilePath: "main.go", Tool: "Edit",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "FAST_ACCEPT_GENERIC") {
		t.Errorf("expected FAST_ACCEPT_GENERIC, got %v", seg.ReasonCodes)
	}
}

// --- TEST_FAIL_NO_RETEST fires -----------------------------------------------

func TestSignal_TestFailNoRetest_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-testfail"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := ms(20 * time.Minute)

	// Failed test execution BEFORE the edit.
	failCode := 1
	if err := s.InsertExecution(model.Execution{
		SessionID: sessID, Timestamp: base,
		Command: "go test ./...", Classification: "test",
		ExitCode: &failCode,
	}); err != nil {
		t.Fatal(err)
	}

	// Edit to main.go after the test failure.
	editTS := base + 5000
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: editTS - 10000,
		Content: "fix bug", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: editTS, FilePath: "main.go", Tool: "Edit",
	}); err != nil {
		t.Fatal(err)
	}
	// No subsequent test execution.

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "TEST_FAIL_NO_RETEST") {
		t.Errorf("expected TEST_FAIL_NO_RETEST, got %v", seg.ReasonCodes)
	}
}

// --- NEVER_REOPENED fires ---------------------------------------------------

func TestSignal_NeverReopened_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-reopened"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, Timestamp: ms(5 * time.Minute),
		FilePath: "main.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// Nothing touches main.go afterwards.

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "NEVER_REOPENED") {
		t.Errorf("expected NEVER_REOPENED, got %v", seg.ReasonCodes)
	}
}

// --- HIGH_REGEN fires -------------------------------------------------------

func TestSignal_HighRegen_FiresViaAnalyze(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-hregen"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := ms(20 * time.Minute)
	// 4 edits within 5 minutes.
	for i := 0; i < 4; i++ {
		if err := s.InsertEdit(model.Edit{
			SessionID: sessID,
			Timestamp: base + int64(i)*60*1000,
			FilePath:  "main.go",
			Tool:      "Edit",
		}); err != nil {
			t.Fatal(err)
		}
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := false
	for _, seg := range report.Segments {
		if seg.FilePath == "main.go" && hasReason(seg, "HIGH_REGEN") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one main.go segment with HIGH_REGEN; report=%+v", report)
	}
}

// --- LARGE_MULTIFILE fires --------------------------------------------------

func TestSignal_LargeMultifile_Fires(t *testing.T) {
	// Create a repo with >10 files changed, plus a session touching all of them.
	files := make([]string, 11)
	for i := range files {
		files[i] = fmt.Sprintf("pkg/file%d.go", i)
	}
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, files...)

	sessID := "sess-multifile"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := ms(20 * time.Minute)
	for i, f := range files {
		if err := s.InsertEdit(model.Edit{
			SessionID: sessID,
			Timestamp: base + int64(i)*1000,
			FilePath:  f,
			Tool:      "Write",
		}); err != nil {
			t.Fatal(err)
		}
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := false
	for _, seg := range report.Segments {
		if hasReason(seg, "LARGE_MULTIFILE") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LARGE_MULTIFILE; segments=%v", report.Segments)
	}
}

// --- LONG_GENERATED_BLOCK fires ---------------------------------------------

func TestSignal_LongGeneratedBlock_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go")

	sessID := "sess-longblock"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}

	// Build a diff string with 101 added lines.
	var diffLines []string
	diffLines = append(diffLines, "@@ -1,1 +1,102 @@")
	for i := 0; i < 101; i++ {
		diffLines = append(diffLines, fmt.Sprintf("+line%d", i))
	}
	bigDiff := strings.Join(diffLines, "\n")

	if err := s.InsertEdit(model.Edit{
		SessionID: sessID,
		Timestamp: ms(5 * time.Minute),
		FilePath:  "main.go",
		Tool:      "Write",
		Diff:      bigDiff,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "LONG_GENERATED_BLOCK") {
		t.Errorf("expected LONG_GENERATED_BLOCK, got %v", seg.ReasonCodes)
	}
}

// --- NEW_DEPENDENCY fires ---------------------------------------------------

func TestSignal_NewDependency_FiresViaAnalyze(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "go.mod")

	sessID := "sess-dep"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID,
		Timestamp: ms(5 * time.Minute),
		FilePath:  "go.mod",
		Tool:      "Edit",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "go.mod")
	if !ok {
		t.Fatal("expected segment for go.mod")
	}
	if !hasReason(seg, "NEW_DEPENDENCY") {
		t.Errorf("expected NEW_DEPENDENCY, got %v", seg.ReasonCodes)
	}
}

// --- FAST_ACCEPT_SECURITY_V2 fires (5-9 seconds) ----------------------------

func TestFastAcceptSecurityV2_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "internal/auth/handler.go")

	sessID := "sess-fastsec-v2-fires"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	// Prompt 7 seconds before edit -- should fire V2, not V1.
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 7000,
		Content: "add auth check", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: now, FilePath: "internal/auth/handler.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// Add an execution so NO_EXEC does not fire (which would elevate tier to 1).
	exitCode := 0
	if err := s.InsertExecution(model.Execution{
		SessionID: sessID, Timestamp: now + 1000,
		Command: "go build ./...", Classification: "build",
		FilesTouched: `["internal/auth/handler.go"]`, ExitCode: &exitCode,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "internal/auth/handler.go")
	if !ok {
		t.Fatal("expected segment for auth/handler.go")
	}
	if !hasReason(seg, "FAST_ACCEPT_SECURITY_V2") {
		t.Errorf("expected FAST_ACCEPT_SECURITY_V2, got %v", seg.ReasonCodes)
	}
	if hasReason(seg, "FAST_ACCEPT_SECURITY") {
		t.Errorf("FAST_ACCEPT_SECURITY should not fire when accepted in 7s, got %v", seg.ReasonCodes)
	}
	if seg.Tier != 2 {
		t.Errorf("expected tier 2, got %d", seg.Tier)
	}
}

// --- FAST_ACCEPT_SECURITY_V2 does not fire when < 5s (FAST_ACCEPT_SECURITY fires instead) ---

func TestFastAcceptSecurityV2_NoFireBelowFive(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "internal/auth/handler.go")

	sessID := "sess-fastsec-v2-below5"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	// Prompt 3 seconds before edit -- FAST_ACCEPT_SECURITY fires, V2 must not.
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 3000,
		Content: "add auth check", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: now, FilePath: "internal/auth/handler.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "internal/auth/handler.go")
	if !ok {
		t.Fatal("expected segment for auth/handler.go")
	}
	if !hasReason(seg, "FAST_ACCEPT_SECURITY") {
		t.Errorf("expected FAST_ACCEPT_SECURITY to fire for 3s accept, got %v", seg.ReasonCodes)
	}
	if hasReason(seg, "FAST_ACCEPT_SECURITY_V2") {
		t.Errorf("FAST_ACCEPT_SECURITY_V2 must not fire when FAST_ACCEPT_SECURITY already fired, got %v", seg.ReasonCodes)
	}
}

// --- FAST_ACCEPT_SECURITY_V2 does not fire when >= 10s ----------------------

func TestFastAcceptSecurityV2_NoFireAboveTen(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "internal/auth/handler.go")

	sessID := "sess-fastsec-v2-above10"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	// Prompt 12 seconds before edit -- neither signal should fire.
	pID, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 12000,
		Content: "add auth check", ContentHash: "h",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pID,
		Timestamp: now, FilePath: "internal/auth/handler.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "internal/auth/handler.go")
	if !ok {
		// Zero-score segments are excluded from the report -- that is fine.
		return
	}
	if hasReason(seg, "FAST_ACCEPT_SECURITY") {
		t.Errorf("FAST_ACCEPT_SECURITY must not fire for 12s accept, got %v", seg.ReasonCodes)
	}
	if hasReason(seg, "FAST_ACCEPT_SECURITY_V2") {
		t.Errorf("FAST_ACCEPT_SECURITY_V2 must not fire for 12s accept, got %v", seg.ReasonCodes)
	}
}

// --- COMMIT_WITHOUT_TEST fires -----------------------------------------------

func TestCommitWithoutTest_Fires(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go", "main_test.go")

	sessID := "sess-commit-no-test-fires"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := ms(20 * time.Minute)

	// Edit to the production file.
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, Timestamp: base,
		FilePath: "main.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// Edit to the test file (establishes sessionHasTestEdits).
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, Timestamp: base + 1000,
		FilePath: "main_test.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// No test execution inserted.

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	seg, ok := firstSegmentForFile(report, "main.go")
	if !ok {
		t.Fatal("expected segment for main.go")
	}
	if !hasReason(seg, "COMMIT_WITHOUT_TEST") {
		t.Errorf("expected COMMIT_WITHOUT_TEST, got %v", seg.ReasonCodes)
	}
}

// --- COMMIT_WITHOUT_TEST does not fire when tests ran -----------------------

func TestCommitWithoutTest_NoFireWhenTestsRan(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t, "main.go", "main_test.go")

	sessID := "sess-commit-no-test-quiet"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := ms(20 * time.Minute)

	// Edit to the production file.
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, Timestamp: base,
		FilePath: "main.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// Edit to the test file (establishes sessionHasTestEdits).
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, Timestamp: base + 1000,
		FilePath: "main_test.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}
	// Test execution IS present -- signal must not fire.
	exitCode := 0
	if err := s.InsertExecution(model.Execution{
		SessionID: sessID, Timestamp: base + 2000,
		Command: "go test ./...", Classification: "test",
		ExitCode: &exitCode,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// Check any segment for main.go -- signal must not fire.
	for _, seg := range report.Segments {
		if seg.FilePath == "main.go" && hasReason(seg, "COMMIT_WITHOUT_TEST") {
			t.Errorf("COMMIT_WITHOUT_TEST must not fire when test execution exists, got %v", seg.ReasonCodes)
		}
	}
}

// --- Multi-segment ranking end-to-end ---------------------------------------

// TestAnalyze_MultiSegmentRanking verifies that the segment with higher score
// appears first in the ranked output.
func TestAnalyze_MultiSegmentRanking(t *testing.T) {
	s := openFreshStore(t)
	repoPath, parentSHA, headSHA := initGitRepoWithFiles(t,
		"main.go",
		"internal/auth/handler.go",
	)

	sessID := "sess-multirank"
	if err := s.InsertSession(model.Session{ID: sessID, StartedAt: ms(30 * time.Minute)}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UnixMilli()
	// Security file accepted in 2s -- should score very high.
	pSec, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 2000,
		Content: "rewrite auth", ContentHash: "h1",
	})
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pSec,
		Timestamp: now, FilePath: "internal/auth/handler.go", Tool: "Write",
	}); err != nil {
		t.Fatal(err)
	}

	// Normal file accepted in 2s (FAST_ACCEPT_GENERIC fires) -- lower score than security path.
	pNorm, _ := s.InsertPrompt(model.Prompt{
		SessionID: sessID, Timestamp: now - 2000,
		Content: "add func", ContentHash: "h2",
	})
	editTS := now + 100
	if err := s.InsertEdit(model.Edit{
		SessionID: sessID, PromptID: &pNorm,
		Timestamp: editTS, FilePath: "main.go", Tool: "Edit",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := analyzer.Analyze(s, repoPath, parentSHA, headSHA)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(report.Segments) < 2 {
		t.Fatalf("expected >=2 segments, got %d", len(report.Segments))
	}

	// The first segment must have score >= second.
	if report.Segments[0].Score < report.Segments[1].Score {
		t.Errorf("ranking wrong: seg[0].score=%v < seg[1].score=%v",
			report.Segments[0].Score, report.Segments[1].Score)
	}

	// The high-risk security segment should be first.
	if report.Segments[0].FilePath != "internal/auth/handler.go" {
		t.Logf("segments: %v %v (scores %.0f %.0f)",
			report.Segments[0].FilePath, report.Segments[1].FilePath,
			report.Segments[0].Score, report.Segments[1].Score)
	}
}
