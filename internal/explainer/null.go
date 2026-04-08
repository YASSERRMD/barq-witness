package explainer

import (
	"context"
	"fmt"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

// NullExplainer is the default explainer used when no LLM is configured.
// It produces deterministic, template-based explanations identical to the
// renderer's built-in text.
type NullExplainer struct{}

// NewNull returns a NullExplainer.
func NewNull() *NullExplainer { return &NullExplainer{} }

func (n *NullExplainer) Name() string      { return "null" }
func (n *NullExplainer) Available() bool   { return true }
func (n *NullExplainer) Close() error      { return nil }

// Explain returns the deterministic template sentences for the segment's
// reason codes.  No LLM is called.
func (n *NullExplainer) Explain(_ context.Context, seg analyzer.Segment) (string, error) {
	if len(seg.ReasonCodes) == 0 {
		return "", nil
	}
	var parts []string
	for _, code := range seg.ReasonCodes {
		parts = append(parts, nullTemplate(code, seg))
	}
	return strings.Join(parts, " "), nil
}

// IntentMatch always returns 1.0 with a fixed explanation.
func (n *NullExplainer) IntentMatch(_ context.Context, _, _ string) (IntentResult, error) {
	return IntentResult{
		Score:     1.0,
		Reasoning: "intent match not computed (no explainer configured)",
	}, nil
}

// nullTemplate maps a reason code to a one-sentence deterministic description.
func nullTemplate(code string, seg analyzer.Segment) string {
	switch code {
	case "NO_EXEC":
		return "Generated but never executed locally before commit."
	case "FAST_ACCEPT_SECURITY":
		return fmt.Sprintf("Accepted in %ds in a security-sensitive path.", seg.AcceptedSec)
	case "TEST_FAIL_NO_RETEST":
		return "Test failed, code was regenerated, never re-tested."
	case "HIGH_REGEN":
		return fmt.Sprintf("Regenerated %d times in this session.", seg.RegenCount)
	case "NEVER_REOPENED":
		return "File was never opened again after generation."
	case "LARGE_MULTIFILE":
		return "Part of a session that touched many distinct files."
	case "NEW_DEPENDENCY":
		return fmt.Sprintf("Introduces a new dependency in %s.", seg.FilePath)
	case "FAST_ACCEPT_GENERIC":
		return fmt.Sprintf("Accepted in %ds without modification.", seg.AcceptedSec)
	case "LONG_GENERATED_BLOCK":
		return "Single edit added many lines."
	default:
		return code + "."
	}
}
