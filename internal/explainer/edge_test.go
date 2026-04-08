package explainer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/explainer"
)

func TestEdgeExplainer_Name(t *testing.T) {
	e := explainer.NewEdge("", "", 0, discardLogger, false)
	if e.Name() != "edge" {
		t.Errorf("Name() = %q, want %q", e.Name(), "edge")
	}
}

func TestEdgeExplainer_SkippedWhenOllamaNotRunning(t *testing.T) {
	// Use a short timeout so the test is fast when Ollama is not running.
	e := explainer.NewEdge("", "", 500, discardLogger, false)
	if e.Available() {
		t.Skip("Ollama is running with the edge model present; skipping unavailability test")
	}
	if e.Available() {
		t.Error("EdgeExplainer should not be available when Ollama is not running")
	}
}

func TestEdgeExplainer_LiveExplain(t *testing.T) {
	e := explainer.NewEdge("", "", 3000, discardLogger, false)
	if !e.Available() {
		t.Skip("Ollama not running or edge model not present at localhost:11434")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, err := e.Explain(ctx, fixtureSegment())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			t.Skipf("edge model not available on this Ollama instance: %v", err)
		}
		t.Fatalf("EdgeExplainer.Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation from EdgeExplainer")
	}
	t.Logf("Edge explanation: %s", text)
}
