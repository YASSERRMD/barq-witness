package analyzer

import "context"

// IntentMatcher checks whether a diff matches what the prompt asked for.
// Implementations may call an LLM, a heuristic, or any other mechanism.
// The analyzer treats the matcher as a black box: only the returned score
// determines whether PROMPT_DIFF_MISMATCH fires.
type IntentMatcher interface {
	// Match returns a score in [0, 1] and an optional reasoning string.
	// A score below the configured threshold causes PROMPT_DIFF_MISMATCH to fire.
	// err signals a backend failure; the signal is skipped on error.
	Match(ctx context.Context, prompt string, diff string) (score float64, reasoning string, err error)
}
