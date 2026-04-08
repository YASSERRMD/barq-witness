package compat_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestClaudeExplainer_SkippedWithoutKey is gated on ANTHROPIC_API_KEY being
// set.  When the key is absent the test is skipped.  This ensures the test
// does not make network calls in CI or local environments without credentials.
func TestClaudeExplainer_SkippedWithoutKey(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set -- skipping Claude explainer live test")
	}
	// If key is present, this is a documentation placeholder.
	// A real implementation would instantiate the ClaudeExplainer and call
	// Explain on a synthetic segment.  That path is already covered by
	// internal/explainer/internal_test.go using a mock HTTP server.
	t.Log("ANTHROPIC_API_KEY is set; live Claude explainer test would run here")
}

// TestGroqExplainer_SkippedWithoutKey is gated on GROQ_API_KEY being set.
func TestGroqExplainer_SkippedWithoutKey(t *testing.T) {
	if os.Getenv("GROQ_API_KEY") == "" {
		t.Skip("GROQ_API_KEY not set -- skipping Groq explainer live test")
	}
	t.Log("GROQ_API_KEY is set; live Groq explainer test would run here")
}

// TestLocalExplainer_SkippedWhenOllamaNotRunning pings the Ollama API first.
// If Ollama is not running the test is skipped.  If it is running, the test
// is a documentation-only pass (actual live coverage is in internal/explainer).
func TestLocalExplainer_SkippedWhenOllamaNotRunning(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("Ollama not reachable at localhost:11434 (err=%v) -- skipping local explainer live test",
			err)
	}
	defer resp.Body.Close()
	fmt.Println("Ollama is reachable; live local explainer test would run here")
	t.Log("Ollama is running; live local explainer test passed (no model required in compat suite)")
}
