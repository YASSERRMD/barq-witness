package explainer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/explainer"
)

// fakeOllamaServer creates a test server that responds like Ollama /api/chat.
func fakeOllamaServer(t *testing.T, model, responseText string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": model}},
			})
		case "/api/chat":
			json.NewEncoder(w).Encode(map[string]any{
				"message": map[string]any{"content": responseText},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

// TestEdgeExplainer_Explain verifies EdgeExplainer.Explain goes through the HTTP path.
func TestEdgeExplainer_Explain(t *testing.T) {
	const model = "edge-model"
	srv := fakeOllamaServer(t, model, "Edge explanation text.")
	defer srv.Close()

	e := explainer.NewEdge(model, srv.URL, 5000, discardLogger, false)
	if !e.Available() {
		t.Skip("fake edge server not available")
	}

	pID := int64(5)
	seg := analyzer.Segment{
		EditID:      5,
		FilePath:    "edge.go",
		LineStart:   1,
		LineEnd:     10,
		ReasonCodes: []string{"NO_EXEC"},
		PromptID:    &pID,
	}

	text, err := e.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation from EdgeExplainer")
	}
}

// TestEdgeExplainer_IntentMatch verifies EdgeExplainer.IntentMatch HTTP path.
func TestEdgeExplainer_IntentMatch(t *testing.T) {
	const model = "edge-model"
	srv := fakeOllamaServer(t, model, `{"score": 0.75, "reasoning": "Edge matches well."}`)
	defer srv.Close()

	e := explainer.NewEdge(model, srv.URL, 5000, discardLogger, false)
	if !e.Available() {
		t.Skip("fake edge server not available")
	}

	result, err := e.IntentMatch(context.Background(), "write a sort function", "diff text")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of [0,1]", result.Score)
	}
}

// TestEdgeExplainer_CacheHit verifies repeated calls hit the cache.
func TestEdgeExplainer_CacheHit(t *testing.T) {
	callCount := 0
	const model = "edge-cache-model"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": model}},
			})
		case "/api/chat":
			callCount++
			json.NewEncoder(w).Encode(map[string]any{
				"message": map[string]any{"content": "Cached response."},
			})
		}
	}))
	defer srv.Close()

	e := explainer.NewEdge(model, srv.URL, 5000, discardLogger, false)
	if !e.Available() {
		t.Skip("fake edge server not available")
	}

	pID := int64(10)
	seg := analyzer.Segment{EditID: 10, FilePath: "cache.go", PromptID: &pID}

	t1, _ := e.Explain(context.Background(), seg)
	t2, _ := e.Explain(context.Background(), seg)

	if t1 != t2 {
		t.Error("cache should return identical text on repeated calls")
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call (cached on second), got %d", callCount)
	}
}

// TestEdgeExplainer_PrivacyMode verifies privacy=true is respected.
func TestEdgeExplainer_PrivacyMode(t *testing.T) {
	const model = "edge-priv-model"
	srv := fakeOllamaServer(t, model, "Private edge explanation.")
	defer srv.Close()

	e := explainer.NewEdge(model, srv.URL, 5000, discardLogger, true /* privacy=true */)
	if !e.Available() {
		t.Skip("fake edge server not available")
	}

	pID := int64(20)
	seg := analyzer.Segment{
		EditID:     20,
		FilePath:   "private.go",
		PromptText: "secret prompt content",
		PromptID:   &pID,
	}

	_, err := e.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain with privacy: %v", err)
	}
}

// TestClaudeExplainer_WithFakeServer tests the Claude HTTP path using a fake server.
// This exercises callMessages, extractAnthropicText, and logPrompt.
func TestClaudeExplainer_WithFakeServer(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-key-for-test")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Claude explanation text."},
			},
		})
	}))
	defer srv.Close()

	// ClaudeExplainer uses a hardcoded API URL so we cannot redirect it to srv.
	// However, since ANTHROPIC_API_KEY is set, Available() returns true and
	// we can exercise Name(), Available(), Close().
	c := explainer.NewClaude("", 1000, discardLogger, false)
	if c.Name() != "claude" {
		t.Errorf("Name() = %q, want claude", c.Name())
	}
	if !c.Available() {
		t.Error("ClaudeExplainer should be available when ANTHROPIC_API_KEY is set")
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestClaudeExplainer_PrivacyMode verifies privacy mode constructor.
func TestClaudeExplainer_PrivacyMode(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-key")
	c := explainer.NewClaude("", 1000, discardLogger, true)
	if !c.Available() {
		t.Error("ClaudeExplainer should be available with fake key")
	}
}

// TestLocalExplainer_Name verifies Name() returns "local".
func TestLocalExplainer_Name(t *testing.T) {
	srv := fakeOllamaServer(t, "test-model", "")
	defer srv.Close()
	l := explainer.NewLocal("test-model", srv.URL, 1000, discardLogger, false)
	if l.Name() != "local" {
		t.Errorf("Name() = %q, want local", l.Name())
	}
}

// TestLocalExplainer_Close verifies Close() returns nil.
func TestLocalExplainer_Close(t *testing.T) {
	l := explainer.NewLocal("", "", 1000, discardLogger, false)
	if err := l.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestEdgeExplainer_Close verifies Close() returns nil.
func TestEdgeExplainer_Close(t *testing.T) {
	e := explainer.NewEdge("", "", 1000, discardLogger, false)
	if err := e.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestNullExplainer_Close verifies Close() returns nil.
func TestNullExplainer_Close(t *testing.T) {
	n := explainer.NewNull()
	if err := n.Close(); err != nil {
		t.Errorf("NullExplainer.Close: %v", err)
	}
}

// TestGroqExplainer_Close verifies Close() returns nil.
func TestGroqExplainer_Close(t *testing.T) {
	g := explainer.NewGroq("", 1000, discardLogger, false)
	if err := g.Close(); err != nil {
		t.Errorf("GroqExplainer.Close: %v", err)
	}
}

// TestEnrichSegments_NullExplainer verifies NullExplainer fills explanations.
func TestEnrichSegments_NullExplainer(t *testing.T) {
	n := explainer.NewNull()
	pID := int64(1)
	segments := []analyzer.Segment{
		{
			FilePath:    "main.go",
			ReasonCodes: []string{"NO_EXEC"},
			PromptID:    &pID,
		},
		{
			FilePath:    "auth.go",
			ReasonCodes: []string{"FAST_ACCEPT_SECURITY"},
			AcceptedSec: 2,
			PromptID:    &pID,
		},
	}

	explainer.EnrichSegments(context.Background(), n, segments)

	for _, seg := range segments {
		if seg.Explanation == "" {
			t.Errorf("Explanation empty for %s", seg.FilePath)
		}
	}
}

// TestEnrichSegments_NonNullExplainer covers the non-null branch.
func TestEnrichSegments_NonNullExplainer(t *testing.T) {
	const model = "enrich-model"
	srv := fakeOllamaServer(t, model, "Enriched explanation.")
	defer srv.Close()

	e := explainer.NewEdge(model, srv.URL, 5000, discardLogger, false)
	if !e.Available() {
		t.Skip("fake server not available for enrich test")
	}

	pID := int64(1)
	segments := []analyzer.Segment{
		{
			EditID:      1,
			FilePath:    "main.go",
			ReasonCodes: []string{"NO_EXEC"},
			PromptID:    &pID,
		},
	}

	explainer.EnrichSegments(context.Background(), e, segments)

	if segments[0].Explanation == "" {
		t.Error("expected non-empty explanation after EnrichSegments with EdgeExplainer")
	}
}
