package integration

import (
	"os"
	"testing"
)

func TestExplainer_NullAlwaysWorks(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	_, _, code := run(t, dir, "report", "--explainer", "null", "--format", "text")
	if code != 0 {
		t.Errorf("report with null explainer failed with code %d", code)
	}
}

func TestExplainer_ClaudeSkippedWithoutKey(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		t.Skip("ANTHROPIC_API_KEY is set")
	}
	dir := makeGitRepo(t)
	run(t, dir, "init")

	// Should fall back gracefully to null without crashing.
	_, _, code := run(t, dir, "report", "--explainer", "claude", "--format", "text")
	if code != 0 {
		t.Errorf("report with claude explainer (no key) should fall back, got code %d", code)
	}
}

func TestExplainer_LocalFallsBackGracefully(t *testing.T) {
	// Verify that when Ollama is not running the binary still exits 0.
	dir := makeGitRepo(t)
	run(t, dir, "init")

	_, _, code := run(t, dir, "report", "--explainer", "local", "--format", "text")
	if code != 0 {
		t.Errorf("report with local explainer should fall back gracefully, got code %d", code)
	}
}

func TestExplainer_NullWithMarkdownFormat(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	_, _, code := run(t, dir, "report", "--explainer", "null", "--format", "markdown")
	if code != 0 {
		t.Errorf("report with null explainer + markdown format failed with code %d", code)
	}
}

func TestExplainer_NullWithJSONFormat(t *testing.T) {
	dir := makeGitRepo(t)
	run(t, dir, "init")

	_, _, code := run(t, dir, "report", "--explainer", "null", "--format", "json")
	if code != 0 {
		t.Errorf("report with null explainer + json format failed with code %d", code)
	}
}
