package compat_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/explainer"
)

// alwaysErrorExplainer is an Explainer that always returns an error.
type alwaysErrorExplainer struct{}

func (e *alwaysErrorExplainer) Name() string      { return "always-error" }
func (e *alwaysErrorExplainer) Available() bool   { return true }
func (e *alwaysErrorExplainer) Close() error      { return nil }

func (e *alwaysErrorExplainer) Explain(_ context.Context, _ analyzer.Segment) (string, error) {
	return "", errors.New("simulated explainer failure")
}

func (e *alwaysErrorExplainer) IntentMatch(_ context.Context, _, _ string) (explainer.IntentResult, error) {
	return explainer.IntentResult{}, errors.New("simulated intent-match failure")
}

// TestExplainer_FallbackOnAlwaysError verifies that EnrichSegments uses
// deterministic template text when the configured explainer always errors.
// The fallback must produce non-empty text that is NOT the raw error message.
func TestExplainer_FallbackOnAlwaysError(t *testing.T) {
	e := &alwaysErrorExplainer{}

	segments := []analyzer.Segment{
		{
			FilePath:    "main.go",
			ReasonCodes: []string{"NO_EXEC"},
			Tier:        1,
			Score:       0.9,
			AcceptedSec: 2,
		},
		{
			FilePath:    "auth.go",
			ReasonCodes: []string{"FAST_ACCEPT_SECURITY"},
			Tier:        1,
			Score:       0.95,
			AcceptedSec: 1,
		},
	}

	explainer.EnrichSegments(context.Background(), e, segments)

	for i, seg := range segments {
		if strings.TrimSpace(seg.Explanation) == "" {
			t.Errorf("segment[%d] (%s): Explanation is empty after fallback", i, seg.FilePath)
		}
		if strings.Contains(seg.Explanation, "simulated explainer failure") {
			t.Errorf("segment[%d] (%s): Explanation contains raw error message", i, seg.FilePath)
		}
		// Tier and Score must not be changed by EnrichSegments.
		if seg.Tier != segments[i].Tier {
			t.Errorf("segment[%d]: Tier changed from %d to %d", i, segments[i].Tier, seg.Tier)
		}
	}
}

// TestExplainer_FallbackIsDeterministic verifies that the fallback text for
// the same segment is identical across multiple EnrichSegments calls.
func TestExplainer_FallbackIsDeterministic(t *testing.T) {
	e := &alwaysErrorExplainer{}

	makeSegments := func() []analyzer.Segment {
		return []analyzer.Segment{
			{
				FilePath:    "service.go",
				ReasonCodes: []string{"HIGH_REGEN"},
				Tier:        2,
				Score:       0.6,
				RegenCount:  5,
			},
		}
	}

	segs1 := makeSegments()
	explainer.EnrichSegments(context.Background(), e, segs1)

	segs2 := makeSegments()
	explainer.EnrichSegments(context.Background(), e, segs2)

	if segs1[0].Explanation != segs2[0].Explanation {
		t.Errorf("fallback explanation is not deterministic:\n  call1: %q\n  call2: %q",
			segs1[0].Explanation, segs2[0].Explanation)
	}
}
