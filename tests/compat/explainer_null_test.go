package compat_test

import (
	"context"
	"strings"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/explainer"
)

// allReasonCodes lists every reason code that NullExplainer must handle.
var allReasonCodes = []string{
	"NO_EXEC",
	"FAST_ACCEPT_SECURITY",
	"TEST_FAIL_NO_RETEST",
	"PROMPT_DIFF_MISMATCH",
	"HIGH_REGEN",
	"NEVER_REOPENED",
	"LARGE_MULTIFILE",
	"NEW_DEPENDENCY",
	"FAST_ACCEPT_GENERIC",
	"LONG_GENERATED_BLOCK",
	"FAST_ACCEPT_SECURITY_V2",
	"COMMIT_WITHOUT_TEST",
}

// TestNullExplainer_AllReasonCodes verifies that NullExplainer.Explain
// produces non-empty, deterministic output for every known reason code.
func TestNullExplainer_AllReasonCodes(t *testing.T) {
	null := explainer.NewNull()
	ctx := context.Background()

	for _, code := range allReasonCodes {
		code := code
		t.Run(code, func(t *testing.T) {
			seg := analyzer.Segment{
				FilePath:    "pkg/auth/token.go",
				LineStart:   1,
				LineEnd:     50,
				AcceptedSec: 2,
				RegenCount:  3,
				ReasonCodes: []string{code},
				Tier:        1,
				Score:       0.8,
			}

			result1, err := null.Explain(ctx, seg)
			if err != nil {
				t.Fatalf("Explain(%q): unexpected error: %v", code, err)
			}
			if strings.TrimSpace(result1) == "" {
				t.Errorf("Explain(%q): got empty string", code)
			}

			// Determinism check: calling again must produce identical output.
			result2, err := null.Explain(ctx, seg)
			if err != nil {
				t.Fatalf("Explain(%q) second call: unexpected error: %v", code, err)
			}
			if result1 != result2 {
				t.Errorf("Explain(%q) not deterministic: %q != %q", code, result1, result2)
			}
		})
	}
}

// TestNullExplainer_IntentMatchDeterministic verifies IntentMatch always
// returns 1.0 and a non-empty reasoning string.
func TestNullExplainer_IntentMatchDeterministic(t *testing.T) {
	null := explainer.NewNull()
	ctx := context.Background()

	r1, err := null.IntentMatch(ctx, "add logging", "+log.Printf(...)")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if r1.Score != 1.0 {
		t.Errorf("expected score=1.0, got %f", r1.Score)
	}
	if r1.Reasoning == "" {
		t.Error("expected non-empty reasoning")
	}

	r2, err := null.IntentMatch(ctx, "add logging", "+log.Printf(...)")
	if err != nil {
		t.Fatalf("IntentMatch second call: %v", err)
	}
	if r1.Score != r2.Score || r1.Reasoning != r2.Reasoning {
		t.Error("IntentMatch is not deterministic")
	}
}

// TestNullExplainer_EmptyReasonCodes verifies that an empty reason code list
// produces an empty string (no crash).
func TestNullExplainer_EmptyReasonCodes(t *testing.T) {
	null := explainer.NewNull()
	seg := analyzer.Segment{FilePath: "main.go", ReasonCodes: nil}
	result, err := null.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain with empty codes: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for no reason codes, got %q", result)
	}
}
