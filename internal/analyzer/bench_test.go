package analyzer_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// initBenchRepo sets up a minimal two-commit git repo that changes a single
// file and returns the repo path, the first commit SHA, and the second (HEAD).
func initBenchRepo(b *testing.B) (repoPath, fromSHA, toSHA string) {
	b.Helper()
	dir := b.TempDir()

	run := func(args ...string) {
		b.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Bench",
			"GIT_AUTHOR_EMAIL=bench@bench.com",
			"GIT_COMMITTER_NAME=Bench",
			"GIT_COMMITTER_EMAIL=bench@bench.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			b.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	sha := func(ref string) string {
		out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
		if err != nil {
			b.Fatalf("rev-parse %s: %v", ref, err)
		}
		s := string(out)
		for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
			s = s[:len(s)-1]
		}
		return s
	}

	run("init")
	run("config", "user.email", "bench@bench.com")
	run("config", "user.name", "Bench")

	fp := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fp, []byte("package main\n"), 0o644); err != nil {
		b.Fatalf("write: %v", err)
	}
	run("add", "main.go")
	run("commit", "-m", "initial")
	from := sha("HEAD")

	// Write 100 lines to create meaningful diff coverage.
	content := "package main\n\n"
	for i := 0; i < 100; i++ {
		content += fmt.Sprintf("// line %d\n", i)
	}
	if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
		b.Fatalf("write: %v", err)
	}
	run("add", "main.go")
	run("commit", "-m", "expand")
	to := sha("HEAD")

	return dir, from, to
}

// openBenchStore opens a fresh SQLite store in b.TempDir.
func openBenchStore(b *testing.B) *store.Store {
	b.Helper()
	s, err := store.Open(filepath.Join(b.TempDir(), "trace.db"))
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

// seedEditsForBench inserts n edits against "main.go" in a single session.
func seedEditsForBench(b *testing.B, s *store.Store, sessID string, n int) {
	b.Helper()
	if err := s.InsertSession(model.Session{
		ID:        sessID,
		StartedAt: time.Now().UnixMilli(),
	}); err != nil {
		b.Fatalf("InsertSession: %v", err)
	}
	ts := time.Now().UnixMilli()
	for i := 0; i < n; i++ {
		ls := i + 1
		le := i + 1
		if err := s.InsertEdit(model.Edit{
			SessionID:  sessID,
			Timestamp:  ts + int64(i),
			FilePath:   "main.go",
			Tool:       "Write",
			BeforeHash: fmt.Sprintf("before%d", i),
			AfterHash:  fmt.Sprintf("after%d", i),
			LineStart:  &ls,
			LineEnd:    &le,
		}); err != nil {
			b.Fatalf("InsertEdit: %v", err)
		}
	}
}

// BenchmarkAnalyze_100Edits benchmarks Analyze against a store with 100 edits.
func BenchmarkAnalyze_100Edits(b *testing.B) {
	s := openBenchStore(b)
	seedEditsForBench(b, s, "sess-bench-100", 100)
	repoPath, fromSHA, toSHA := initBenchRepo(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(s, repoPath, fromSHA, toSHA)
		if err != nil {
			b.Fatalf("Analyze: %v", err)
		}
	}
}

// BenchmarkAnalyze_1000Edits benchmarks Analyze against a store with 1000 edits.
func BenchmarkAnalyze_1000Edits(b *testing.B) {
	s := openBenchStore(b)
	seedEditsForBench(b, s, "sess-bench-1000", 1000)
	repoPath, fromSHA, toSHA := initBenchRepo(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(s, repoPath, fromSHA, toSHA)
		if err != nil {
			b.Fatalf("Analyze: %v", err)
		}
	}
}
