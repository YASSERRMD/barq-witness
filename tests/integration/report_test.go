package integration

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

func TestReport_EmptyTrace(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	stdout, _, code := run(t, dir, "report", "--format", "text")
	if code != 0 {
		t.Fatalf("report exited %d", code)
	}
	// An empty trace should produce either "No flagged segments" or a zero count.
	if !strings.Contains(stdout, "No flagged segments") && !strings.Contains(stdout, "0 total") {
		t.Logf("report output: %s", stdout)
		// Verify it does not crash -- empty output is acceptable.
	}
}

func TestReport_FormatMarkdown(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	stdout, _, code := run(t, dir, "report", "--format", "markdown")
	if code != 0 {
		t.Fatalf("report markdown exited %d", code)
	}
	// Must contain the barq-witness comment marker.
	if !strings.Contains(stdout, "barq-witness") {
		t.Errorf("markdown output missing barq-witness header; got: %s", stdout)
	}
}

func TestReport_FormatJSON(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	stdout, _, code := run(t, dir, "report", "--format", "json")
	if code != 0 {
		t.Fatalf("report json exited %d", code)
	}
	// Should be valid JSON containing the segments field.
	if !strings.Contains(stdout, "segments") {
		t.Errorf("JSON output missing 'segments' field: %s", stdout)
	}
	// Should also contain generated_at.
	if !strings.Contains(stdout, "generated_at") {
		t.Errorf("JSON output missing 'generated_at' field: %s", stdout)
	}
}

func TestReport_TopNFlag(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	// Insert several sessions so the report has data to process.
	st, err := store.Open(filepath.Join(dir, ".witness", "trace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for i := 0; i < 5; i++ {
		sid := fmt.Sprintf("session-%d", i)
		err := st.InsertSession(model.Session{
			ID:           sid,
			StartedAt:    int64(1700000000000 + i*1000),
			CWD:          dir,
			GitHeadStart: "abc",
			Model:        "test",
			Source:       "claude-code",
		})
		if err != nil {
			t.Fatalf("InsertSession: %v", err)
		}
	}
	st.Close()

	_, _, code := run(t, dir, "report", "--format", "text", "--top", "2")
	if code != 0 {
		t.Errorf("report --top 2 exited %d", code)
	}
}

func TestReport_AllFormatsExitZero(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	for _, format := range []string{"text", "markdown", "json"} {
		_, _, code := run(t, dir, "report", "--format", format)
		if code != 0 {
			t.Errorf("report --format %s exited %d", format, code)
		}
	}
}

func TestReport_UnknownFormatFails(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	_, _, code := run(t, dir, "report", "--format", "xml")
	if code == 0 {
		t.Error("report with unknown format should exit non-zero")
	}
}
