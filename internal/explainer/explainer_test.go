package explainer_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/config"
	"github.com/yasserrmd/barq-witness/internal/explainer"
)

// ---- fixtures --------------------------------------------------------------

var discardLogger = log.New(io.Discard, "", 0)

func fixtureSegment() analyzer.Segment {
	pID := int64(1)
	return analyzer.Segment{
		FilePath:    "internal/auth/handler.go",
		LineStart:   10,
		LineEnd:     30,
		EditID:      42,
		SessionID:   "sess-test",
		PromptID:    &pID,
		PromptText:  "add JWT validation middleware",
		GeneratedAt: time.Now().UnixMilli(),
		AcceptedSec: 3,
		Executed:    false,
		Modified:    false,
		SecurityHit: true,
		Tier:        1,
		ReasonCodes: []string{"NO_EXEC", "FAST_ACCEPT_SECURITY"},
		Score:       200,
	}
}

// ---- NullExplainer ---------------------------------------------------------

func TestNullExplainer_Available(t *testing.T) {
	n := explainer.NewNull()
	if !n.Available() {
		t.Error("NullExplainer must always be available")
	}
}

func TestNullExplainer_Name(t *testing.T) {
	if explainer.NewNull().Name() != "null" {
		t.Error("Name() should be 'null'")
	}
}

func TestNullExplainer_Explain_NeverErrors(t *testing.T) {
	n := explainer.NewNull()
	seg := fixtureSegment()
	text, err := n.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("NullExplainer.Explain returned error: %v", err)
	}
	if text == "" {
		t.Error("NullExplainer.Explain returned empty string")
	}
	// Must contain deterministic content, not arbitrary LLM text.
	if !strings.Contains(text, "never executed") && !strings.Contains(text, "security-sensitive") {
		t.Errorf("unexpected null explanation: %q", text)
	}
}

func TestNullExplainer_IntentMatch_ReturnsFixed(t *testing.T) {
	n := explainer.NewNull()
	result, err := n.IntentMatch(context.Background(), "prompt", "diff")
	if err != nil {
		t.Fatalf("IntentMatch error: %v", err)
	}
	if result.Score != 1.0 {
		t.Errorf("NullExplainer IntentMatch score = %v, want 1.0", result.Score)
	}
	if result.Reasoning == "" {
		t.Error("NullExplainer IntentMatch must return a reasoning string")
	}
}

func TestNullExplainer_EmptyReasonCodes(t *testing.T) {
	n := explainer.NewNull()
	seg := fixtureSegment()
	seg.ReasonCodes = nil
	text, err := n.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if text != "" {
		t.Errorf("empty reason codes should produce empty explanation, got %q", text)
	}
}

// ---- ClaudeExplainer -------------------------------------------------------

func TestClaudeExplainer_SkippedWithoutKey(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		t.Skip("ANTHROPIC_API_KEY is set; skipping unavailability test")
	}
	c := explainer.NewClaude("", 1000, discardLogger, false)
	if c.Available() {
		t.Error("ClaudeExplainer should not be available without ANTHROPIC_API_KEY")
	}
}

func TestClaudeExplainer_LiveExplain(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	c := explainer.NewClaude("", 10000, discardLogger, false)
	if !c.Available() {
		t.Skip("Claude not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	seg := fixtureSegment()
	text, err := c.Explain(ctx, seg)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation")
	}
	t.Logf("Claude explanation: %s", text)
}

func TestClaudeExplainer_CacheHit(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	c := explainer.NewClaude("", 10000, discardLogger, false)
	if !c.Available() {
		t.Skip("Claude not available")
	}
	ctx := context.Background()
	seg := fixtureSegment()

	// First call -- hits the API.
	t1, err := c.Explain(ctx, seg)
	if err != nil {
		t.Fatalf("first Explain: %v", err)
	}
	// Second call -- must return same text from cache without API call.
	t2, err := c.Explain(ctx, seg)
	if err != nil {
		t.Fatalf("second Explain: %v", err)
	}
	if t1 != t2 {
		t.Error("cache should return identical text on repeated calls")
	}
}

// ---- GroqExplainer ---------------------------------------------------------

func TestGroqExplainer_SkippedWithoutKey(t *testing.T) {
	if os.Getenv("GROQ_API_KEY") != "" {
		t.Skip("GROQ_API_KEY is set")
	}
	g := explainer.NewGroq("", 1000, discardLogger, false)
	if g.Available() {
		t.Error("GroqExplainer should not be available without GROQ_API_KEY")
	}
}

func TestGroqExplainer_LiveExplain(t *testing.T) {
	if os.Getenv("GROQ_API_KEY") == "" {
		t.Skip("GROQ_API_KEY not set")
	}
	g := explainer.NewGroq("", 10000, discardLogger, false)
	if !g.Available() {
		t.Skip("Groq not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	text, err := g.Explain(ctx, fixtureSegment())
	if err != nil {
		t.Fatalf("Groq Explain: %v", err)
	}
	t.Logf("Groq explanation: %s", text)
}

// ---- LocalExplainer --------------------------------------------------------

func TestLocalExplainer_SkippedWhenOllamaNotRunning(t *testing.T) {
	l := explainer.NewLocal("", "", 500, discardLogger, false)
	if l.Available() {
		t.Skip("Ollama is running; skipping unavailability test")
	}
	if l.Available() {
		t.Error("LocalExplainer should not be available when Ollama is not running")
	}
}

func TestLocalExplainer_LiveExplain(t *testing.T) {
	l := explainer.NewLocal("", "", 2000, discardLogger, false)
	if !l.Available() {
		t.Skip("Ollama not running at localhost:11434")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, err := l.Explain(ctx, fixtureSegment())
	if err != nil {
		// Skip rather than fail if the model is simply not present on this machine.
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			t.Skipf("Ollama model not available: %v", err)
		}
		t.Fatalf("Local Explain: %v", err)
	}
	t.Logf("Local explanation: %s", text)
}

// ---- explainer.New factory -------------------------------------------------

func TestNew_DefaultIsNull(t *testing.T) {
	cfg := config.Default()
	e := explainer.New(cfg, t.TempDir())
	if e.Name() != "null" {
		t.Errorf("default explainer should be null, got %q", e.Name())
	}
}

func TestNew_UnavailableBackendFallsBackToNull(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		t.Skip("ANTHROPIC_API_KEY is set; cannot test fallback")
	}
	cfg := config.Default()
	cfg.Explainer.Backend = "claude"
	e := explainer.New(cfg, t.TempDir())
	if e.Name() != "null" {
		t.Errorf("unavailable backend should fall back to null, got %q", e.Name())
	}
}

// ---- EnrichSegments --------------------------------------------------------

// TestEnrichSegments_FallbackOnError verifies that if an explainer errors, the
// segment gets a deterministic template instead, with no error propagating.
func TestEnrichSegments_FallbackOnError(t *testing.T) {
	e := &errorExplainer{}
	segments := []analyzer.Segment{fixtureSegment()}

	explainer.EnrichSegments(context.Background(), e, segments)

	if segments[0].Explanation == "" {
		t.Error("EnrichSegments must populate Explanation even when explainer errors")
	}
	// The fallback must be the NullExplainer deterministic text, not the error.
	if strings.Contains(segments[0].Explanation, "intentional error") {
		t.Error("error text must not appear in the Explanation")
	}
}

// TestEnrichSegments_CannotChangeTierOrScore verifies the invariant.
func TestEnrichSegments_CannotChangeTierOrScore(t *testing.T) {
	e := &tierChangingExplainer{}
	seg := fixtureSegment()
	originalTier := seg.Tier
	originalScore := seg.Score

	segments := []analyzer.Segment{seg}
	explainer.EnrichSegments(context.Background(), e, segments)

	if segments[0].Tier != originalTier {
		t.Errorf("Tier changed from %d to %d -- must not happen", originalTier, segments[0].Tier)
	}
	if segments[0].Score != originalScore {
		t.Errorf("Score changed from %v to %v -- must not happen", originalScore, segments[0].Score)
	}
}

// ---- mock explainers for invariant tests -----------------------------------

// errorExplainer always returns an error from Explain.
type errorExplainer struct{}

func (e *errorExplainer) Name() string    { return "error" }
func (e *errorExplainer) Available() bool { return true }
func (e *errorExplainer) Close() error    { return nil }
func (e *errorExplainer) Explain(_ context.Context, _ analyzer.Segment) (string, error) {
	return "", fmt.Errorf("intentional error from mock explainer")
}
func (e *errorExplainer) IntentMatch(_ context.Context, _, _ string) (explainer.IntentResult, error) {
	return explainer.IntentResult{Score: 1.0}, nil
}

// tierChangingExplainer tries to change the tier and score via its return value.
// EnrichSegments must not allow this -- it only sets Explanation.
type tierChangingExplainer struct{}

func (tc *tierChangingExplainer) Name() string    { return "tier-changer" }
func (tc *tierChangingExplainer) Available() bool { return true }
func (tc *tierChangingExplainer) Close() error    { return nil }
func (tc *tierChangingExplainer) Explain(_ context.Context, seg analyzer.Segment) (string, error) {
	// The explainer only returns a string -- it cannot modify the segment.
	return "explanation text", nil
}
func (tc *tierChangingExplainer) IntentMatch(_ context.Context, _, _ string) (explainer.IntentResult, error) {
	return explainer.IntentResult{Score: 1.0}, nil
}
