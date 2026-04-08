// Package analyzer implements the deterministic risk-scoring engine.
// No LLM, no network.  Every signal is computed from the local SQLite trace
// and the git diff.
package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	gitdiff "github.com/yasserrmd/barq-witness/internal/diff"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// PluginSignal is a risk signal returned by an external plugin.
// It mirrors plugin.Signal but is defined here to avoid a circular import.
type PluginSignal struct {
	Code    string
	Tier    int
	Message string
}

// AnalyzeOptions controls optional behaviour for AnalyzeWithOptions.
type AnalyzeOptions struct {
	// Matcher is the optional intent-matching backend. nil disables the signal.
	Matcher IntentMatcher
	// Threshold is the score below which PROMPT_DIFF_MISMATCH fires.
	// A zero value defaults to 0.5.
	Threshold float64
	// PluginRunner is an optional function that runs all configured plugins
	// against a segment and returns their signals.  nil means no plugins.
	// Plugin signal codes must already carry the "plugin:" prefix.
	PluginRunner func(ctx context.Context, seg Segment) []PluginSignal
}

// Segment represents one reviewed unit: a contiguous set of lines in a file
// that was written by Claude Code during a traced session.
type Segment struct {
	FilePath    string   `json:"file_path"`
	LineStart   int      `json:"line_start"`
	LineEnd     int      `json:"line_end"`
	EditID      int64    `json:"edit_id"`
	SessionID   string   `json:"session_id"`
	PromptID    *int64   `json:"prompt_id,omitempty"`
	PromptText  string   `json:"prompt_text,omitempty"` // resolved from prompts table
	GeneratedAt int64    `json:"generated_at"`          // unix ms
	AcceptedSec int      `json:"accepted_sec"`          // seconds from prompt to edit (-1 if unknown)
	Modified    bool     `json:"modified"`              // after_hash != current on-disk file hash
	Executed    bool     `json:"executed"`              // any execution touched this file after the edit
	Reopened    bool     `json:"reopened"`              // any subsequent tool call touched this file
	RegenCount  int      `json:"regen_count"`           // number of edits to same file within session
	SecurityHit bool     `json:"security_hit"`          // file matches a security-sensitive glob
	NewDep      bool     `json:"new_dep"`               // file is a dependency manifest
	Tier        int      `json:"tier"`                  // 1, 2, or 3 (0 = unscored)
	ReasonCodes []string `json:"reason_codes"`
	Score       float64  `json:"score"`
	// Explanation is populated by the optional explainer layer after analysis.
	// It is never set by the analyzer itself and never affects Tier or Score.
	Explanation string `json:"explanation,omitempty"`
}

// Report is the analysis output for a commit range.
type Report struct {
	CommitRange   string    `json:"commit_range"`
	GeneratedAt   int64     `json:"generated_at"`
	TotalSegments int       `json:"total_segments"`
	Tier1Count    int       `json:"tier1_count"`
	Tier2Count    int       `json:"tier2_count"`
	Tier3Count    int       `json:"tier3_count"`
	Segments      []Segment `json:"segments"` // ranked by Score descending, ties broken by FilePath
}

// Analyze computes a risk report for the diff between fromSHA and toSHA in
// the git repository at repoPath, cross-referenced against the trace store.
// If fromSHA is empty, toSHA is compared against its first parent.
// Intent matching is disabled (nil matcher).
func Analyze(st *store.Store, repoPath, fromSHA, toSHA string) (*Report, error) {
	return AnalyzeWithOptions(st, repoPath, fromSHA, toSHA, AnalyzeOptions{})
}

// AnalyzeWithOptions is like Analyze but accepts optional analysis behaviour.
// When opts.Matcher is non-nil and a segment has prompt text and diff data,
// the matcher is called and PROMPT_DIFF_MISMATCH is added if score < threshold.
func AnalyzeWithOptions(st *store.Store, repoPath, fromSHA, toSHA string, opts AnalyzeOptions) (*Report, error) {
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	// 1. Get changed files from git diff.
	changes, err := gitdiff.ChangedFiles(repoPath, fromSHA, toSHA)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	if len(changes) == 0 {
		return emptyReport(fromSHA, toSHA), nil
	}

	// 2. Collect file paths and query the trace store.
	paths := make([]string, 0, len(changes))
	for _, c := range changes {
		if !c.IsDeleted {
			paths = append(paths, c.Path)
		}
	}

	edits, err := st.EditsForFiles(paths)
	if err != nil {
		return nil, fmt.Errorf("query edits: %w", err)
	}

	if len(edits) == 0 {
		return emptyReport(fromSHA, toSHA), nil
	}

	// 3. Build per-session caches (executions, all edits, distinct file counts).
	sessionExecs, err := buildSessionExecCache(st, edits)
	if err != nil {
		return nil, err
	}
	sessionEdits, err := buildSessionEditCache(st, edits)
	if err != nil {
		return nil, err
	}
	sessionFileCount := buildSessionFileCount(sessionEdits)
	sessionTestEdits := buildSessionTestEditFlag(sessionEdits)
	sessionTestExecs := buildSessionTestExecFlag(sessionExecs)

	// 4. Build a lookup from file path -> changed line ranges (from git diff).
	changedLines := buildChangedLineMap(changes)

	// 5. For each edit, build a Segment and compute signals.
	promptCache := make(map[int64]model.Prompt)
	var segments []Segment

	for _, edit := range edits {
		// Only include edits whose line range overlaps with the git diff.
		if !editOverlapsDiff(edit, changedLines[edit.FilePath]) {
			continue
		}

		seg := Segment{
			FilePath:    edit.FilePath,
			LineStart:   derefInt(edit.LineStart, 0),
			LineEnd:     derefInt(edit.LineEnd, 0),
			EditID:      edit.ID,
			SessionID:   edit.SessionID,
			PromptID:    edit.PromptID,
			GeneratedAt: edit.Timestamp,
			SecurityHit: IsSecurityPath(edit.FilePath),
			NewDep:      IsDependencyFile(edit.FilePath),
		}

		// Resolve prompt text.
		if edit.PromptID != nil {
			if p, ok := promptCache[*edit.PromptID]; ok {
				seg.PromptText = p.Content
			} else {
				if p, err := st.PromptByID(*edit.PromptID); err == nil && p != nil {
					promptCache[*edit.PromptID] = *p
					seg.PromptText = p.Content
				}
			}
		}

		// Regen count = number of edits to same file in the session.
		seg.RegenCount = countEditsToFile(sessionEdits[edit.SessionID], edit.FilePath)

		// Executed / Reopened flags.
		execs := sessionExecs[edit.SessionID]
		for _, x := range execs {
			if x.Timestamp > edit.Timestamp && execTouchesFile(x, edit.FilePath) {
				seg.Executed = true
				seg.Reopened = true
			}
		}
		for _, e := range sessionEdits[edit.SessionID] {
			if e.ID != edit.ID && e.Timestamp > edit.Timestamp && e.FilePath == edit.FilePath {
				seg.Reopened = true
			}
		}

		// Prompt timestamp for AcceptedSec.
		promptTS := int64(0)
		if edit.PromptID != nil {
			if p, ok := promptCache[*edit.PromptID]; ok {
				promptTS = p.Timestamp
			}
		}

		sigCtx := signalContext{
			edit:                edit,
			promptTS:            promptTS,
			promptText:          seg.PromptText,
			execsInSess:         execs,
			editsInSess:         sessionEdits[edit.SessionID],
			distinctFiles:       sessionFileCount[edit.SessionID],
			sessionHasTestEdits: sessionTestEdits[edit.SessionID],
			sessionHasTestExecs: sessionTestExecs[edit.SessionID],
		}

		computeSignals(sigCtx, &seg)

		// --- Optional Tier 2: PROMPT_DIFF_MISMATCH --------------------------
		// Only runs when a matcher is provided and the segment has prompt + diff.
		if opts.Matcher != nil && seg.PromptText != "" && edit.Diff != "" {
			score, _, matchErr := opts.Matcher.Match(context.Background(), seg.PromptText, edit.Diff)
			if matchErr == nil && score < threshold {
				seg.ReasonCodes = append(seg.ReasonCodes, ReasonPromptDiffMismatch)
				seg.Score += WeightTier2
				// Re-compute tier to account for the new reason code.
				seg.Tier = computeTier(seg.ReasonCodes)
			}
		}

		// --- Optional: plugin signals ---------------------------------------
		// Call each registered external plugin and merge their signals.
		if opts.PluginRunner != nil {
			pluginSignals := opts.PluginRunner(context.Background(), seg)
			for _, ps := range pluginSignals {
				seg.ReasonCodes = append(seg.ReasonCodes, ps.Code)
				// Map plugin tier to score weight.
				switch ps.Tier {
				case 1:
					seg.Score += WeightTier1
				case 2:
					seg.Score += WeightTier2
				default:
					seg.Score += WeightTier3
				}
				// Update tier: lower tier number = higher severity.
				if seg.Tier == 0 || ps.Tier < seg.Tier {
					seg.Tier = ps.Tier
				}
			}
		}

		if seg.Score == 0 {
			continue // exclude zero-score segments
		}

		segments = append(segments, seg)
	}

	// 6. Rank by Score descending, break ties by FilePath.
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].Score != segments[j].Score {
			return segments[i].Score > segments[j].Score
		}
		return segments[i].FilePath < segments[j].FilePath
	})

	// 7. Build the report.
	report := &Report{
		CommitRange:   commitRange(fromSHA, toSHA),
		GeneratedAt:   time.Now().UnixMilli(),
		TotalSegments: len(segments),
		Segments:      segments,
	}
	for _, seg := range segments {
		switch seg.Tier {
		case 1:
			report.Tier1Count++
		case 2:
			report.Tier2Count++
		case 3:
			report.Tier3Count++
		}
	}

	return report, nil
}

// ---- helpers ----------------------------------------------------------------

func emptyReport(fromSHA, toSHA string) *Report {
	return &Report{
		CommitRange: commitRange(fromSHA, toSHA),
		GeneratedAt: time.Now().UnixMilli(),
	}
}

func commitRange(from, to string) string {
	if from == "" {
		return to
	}
	return from + ".." + to
}

// buildSessionExecCache loads executions for each unique session referenced
// by the edits and returns a map keyed by session ID.
func buildSessionExecCache(st *store.Store, edits []model.Edit) (map[string][]model.Execution, error) {
	seen := make(map[string]bool)
	result := make(map[string][]model.Execution)
	for _, e := range edits {
		if seen[e.SessionID] {
			continue
		}
		seen[e.SessionID] = true
		execs, err := st.ExecutionsForSession(e.SessionID)
		if err != nil {
			return nil, fmt.Errorf("executions for session %s: %w", e.SessionID, err)
		}
		result[e.SessionID] = execs
	}
	return result, nil
}

// buildSessionEditCache loads all edits for each session referenced by the input edits.
func buildSessionEditCache(st *store.Store, edits []model.Edit) (map[string][]model.Edit, error) {
	seen := make(map[string]bool)
	result := make(map[string][]model.Edit)
	for _, e := range edits {
		if seen[e.SessionID] {
			continue
		}
		seen[e.SessionID] = true
		all, err := st.EditsForSession(e.SessionID)
		if err != nil {
			return nil, fmt.Errorf("edits for session %s: %w", e.SessionID, err)
		}
		result[e.SessionID] = all
	}
	return result, nil
}

// buildSessionFileCount returns the number of distinct files touched per session.
func buildSessionFileCount(sessionEdits map[string][]model.Edit) map[string]int {
	result := make(map[string]int)
	for sessID, edits := range sessionEdits {
		files := make(map[string]bool)
		for _, e := range edits {
			files[e.FilePath] = true
		}
		result[sessID] = len(files)
	}
	return result
}

// buildChangedLineMap indexes the added line numbers per file path.
func buildChangedLineMap(changes []gitdiff.FileChange) map[string][]int {
	m := make(map[string][]int)
	for _, c := range changes {
		m[c.Path] = append(m[c.Path], c.AddedLines...)
	}
	return m
}

// editOverlapsDiff returns true if the edit's line range overlaps with any
// of the lines changed in the git diff.  If no line info is available on
// either side, we include the edit conservatively.
func editOverlapsDiff(edit model.Edit, diffLines []int) bool {
	if edit.LineStart == nil || edit.LineEnd == nil || len(diffLines) == 0 {
		return true // conservative inclusion
	}
	start := *edit.LineStart
	end := *edit.LineEnd
	for _, l := range diffLines {
		if l >= start && l <= end {
			return true
		}
	}
	return false
}

func countEditsToFile(edits []model.Edit, path string) int {
	n := 0
	for _, e := range edits {
		if e.FilePath == path {
			n++
		}
	}
	return n
}

func derefInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// buildSessionTestEditFlag returns a map from session ID to whether the session
// contains at least one edit to a *_test.go file.
func buildSessionTestEditFlag(sessionEdits map[string][]model.Edit) map[string]bool {
	result := make(map[string]bool)
	for sessID, edits := range sessionEdits {
		for _, e := range edits {
			if strings.HasSuffix(e.FilePath, "_test.go") {
				result[sessID] = true
				break
			}
		}
	}
	return result
}

// buildSessionTestExecFlag returns a map from session ID to whether the session
// contains at least one execution with classification="test".
func buildSessionTestExecFlag(sessionExecs map[string][]model.Execution) map[string]bool {
	result := make(map[string]bool)
	for sessID, execs := range sessionExecs {
		for _, x := range execs {
			if x.Classification == "test" {
				result[sessID] = true
				break
			}
		}
	}
	return result
}

