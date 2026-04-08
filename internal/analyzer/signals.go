package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/model"
)

// Signal weight constants.
const (
	WeightTier1 = 100
	WeightTier2 = 50
	WeightTier3 = 20
)

// Reason codes (machine-readable, stable identifiers).
const (
	ReasonNoExec              = "NO_EXEC"
	ReasonFastAcceptSecurity  = "FAST_ACCEPT_SECURITY"
	ReasonTestFailNoRetest    = "TEST_FAIL_NO_RETEST"
	ReasonHighRegen           = "HIGH_REGEN"
	ReasonNeverReopened       = "NEVER_REOPENED"
	ReasonLargeMultifile      = "LARGE_MULTIFILE"
	ReasonNewDependency       = "NEW_DEPENDENCY"
	ReasonFastAcceptGeneric   = "FAST_ACCEPT_GENERIC"
	ReasonLongGeneratedBlock  = "LONG_GENERATED_BLOCK"
	ReasonPromptDiffMismatch  = "PROMPT_DIFF_MISMATCH"
)

// signalContext bundles all trace data needed to evaluate signals for one
// segment.  It is precomputed once per session to avoid repeated DB queries.
type signalContext struct {
	edit         model.Edit
	promptTS     int64  // unix-ms of the linked prompt (0 if none)
	promptText   string
	execsInSess  []model.Execution // all executions in the session, sorted by timestamp
	editsInSess  []model.Edit      // all edits in the session, sorted by timestamp
	distinctFiles int              // distinct files touched in this session
}

// computeSignals runs all signal checks against ctx and populates seg.
// It returns the set of matched reason codes and the total score.
func computeSignals(ctx signalContext, seg *Segment) {
	var reasons []string
	var score float64

	acceptedSec := acceptedSeconds(ctx)

	// --- TIER 1 ---------------------------------------------------------

	if checkNoExec(ctx) {
		reasons = append(reasons, ReasonNoExec)
		score += WeightTier1
	}

	if acceptedSec >= 0 && acceptedSec < 5 && seg.SecurityHit {
		reasons = append(reasons, ReasonFastAcceptSecurity)
		score += WeightTier1
	}

	if checkTestFailNoRetest(ctx) {
		reasons = append(reasons, ReasonTestFailNoRetest)
		score += WeightTier1
	}

	// --- TIER 2 ---------------------------------------------------------

	if checkHighRegen(ctx) {
		reasons = append(reasons, ReasonHighRegen)
		score += WeightTier2
	}

	if checkNeverReopened(ctx) {
		reasons = append(reasons, ReasonNeverReopened)
		score += WeightTier2
	}

	if ctx.distinctFiles > 10 {
		reasons = append(reasons, ReasonLargeMultifile)
		score += WeightTier2
	}

	// --- TIER 3 ---------------------------------------------------------

	if seg.NewDep {
		reasons = append(reasons, ReasonNewDependency)
		score += WeightTier3
	}

	fastAcceptAlreadyCovered := containsReason(reasons, ReasonFastAcceptSecurity)
	if acceptedSec >= 0 && acceptedSec < 3 && !fastAcceptAlreadyCovered {
		reasons = append(reasons, ReasonFastAcceptGeneric)
		score += WeightTier3
	}

	if checkLongGeneratedBlock(ctx) {
		reasons = append(reasons, ReasonLongGeneratedBlock)
		score += WeightTier3
	}

	seg.ReasonCodes = reasons
	seg.Score = score
	seg.AcceptedSec = int(acceptedSec)

	// Compute tier: lowest tier number among matched signals.
	seg.Tier = computeTier(reasons)
}

// ---- signal check functions ------------------------------------------------

// checkNoExec returns true if there is no execution row in the same session
// that references this file after the edit timestamp.
func checkNoExec(ctx signalContext) bool {
	for _, x := range ctx.execsInSess {
		if x.Timestamp <= ctx.edit.Timestamp {
			continue
		}
		if execTouchesFile(x, ctx.edit.FilePath) {
			return false
		}
	}
	return true
}

// checkHighRegen returns true if the same file was edited >= 4 times within
// a 10-minute window inside this session.
func checkHighRegen(ctx signalContext) bool {
	const windowMS = 10 * 60 * 1000 // 10 minutes in milliseconds
	const threshold = 4

	var fileedits []int64
	for _, e := range ctx.editsInSess {
		if e.FilePath == ctx.edit.FilePath {
			fileedits = append(fileedits, e.Timestamp)
		}
	}

	// Sliding window count.
	for i, start := range fileedits {
		count := 0
		for _, ts := range fileedits[i:] {
			if ts-start <= windowMS {
				count++
			}
		}
		if count >= threshold {
			return true
		}
	}
	return false
}

// checkNeverReopened returns true if no tool accessed the file after the edit.
func checkNeverReopened(ctx signalContext) bool {
	for _, e := range ctx.editsInSess {
		if e.ID == ctx.edit.ID {
			continue
		}
		if e.Timestamp > ctx.edit.Timestamp && e.FilePath == ctx.edit.FilePath {
			return false
		}
	}
	for _, x := range ctx.execsInSess {
		if x.Timestamp > ctx.edit.Timestamp && execTouchesFile(x, ctx.edit.FilePath) {
			return false
		}
	}
	return true
}

// checkTestFailNoRetest returns true when the session shows the pattern:
// test execution -> non-zero exit -> edit on same file -> no subsequent test.
func checkTestFailNoRetest(ctx signalContext) bool {
	for i, x := range ctx.execsInSess {
		if x.Classification != "test" {
			continue
		}
		if x.ExitCode == nil || *x.ExitCode == 0 {
			continue
		}
		if x.Timestamp > ctx.edit.Timestamp {
			continue
		}
		// Found a failed test before this edit -- look for a subsequent test.
		hasRetest := false
		for _, x2 := range ctx.execsInSess[i+1:] {
			if x2.Classification == "test" && x2.Timestamp > ctx.edit.Timestamp {
				hasRetest = true
				break
			}
		}
		if !hasRetest {
			return true
		}
	}
	return false
}

// checkLongGeneratedBlock returns true if the edit's diff adds > 100 lines.
func checkLongGeneratedBlock(ctx signalContext) bool {
	diff := ctx.edit.Diff
	if diff == "" {
		return false
	}
	added := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		}
	}
	return added > 100
}

// ---- helpers ----------------------------------------------------------------

func acceptedSeconds(ctx signalContext) int64 {
	if ctx.promptTS == 0 {
		return -1
	}
	delta := ctx.edit.Timestamp - ctx.promptTS
	if delta < 0 {
		return 0
	}
	return delta / 1000
}

func execTouchesFile(x model.Execution, filePath string) bool {
	if x.FilesTouched == "" || x.FilesTouched == "null" {
		return false
	}
	var files []string
	if err := json.Unmarshal([]byte(x.FilesTouched), &files); err != nil {
		return false
	}
	for _, f := range files {
		if f == filePath {
			return true
		}
	}
	return false
}

func containsReason(reasons []string, r string) bool {
	for _, x := range reasons {
		if x == r {
			return true
		}
	}
	return false
}

// computeTier returns the lowest tier number among the matched reason codes,
// or 0 if no reasons are present.
func computeTier(reasons []string) int {
	if len(reasons) == 0 {
		return 0
	}
	tier1 := map[string]bool{ReasonNoExec: true, ReasonFastAcceptSecurity: true, ReasonTestFailNoRetest: true}
	tier2 := map[string]bool{ReasonHighRegen: true, ReasonNeverReopened: true, ReasonLargeMultifile: true, ReasonPromptDiffMismatch: true}

	best := 3
	for _, r := range reasons {
		if tier1[r] {
			return 1
		}
		if tier2[r] {
			best = 2
		}
	}
	return best
}
