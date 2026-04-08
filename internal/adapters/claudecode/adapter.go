// Package claudecode implements the barq-witness Adapter interface for
// Claude Code hook events.  It translates the Claude Code JSON hook payloads
// into store writes, carrying the business logic previously in
// cmd/barq-witness/record.go.
package claudecode

import (
	"time"

	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// Adapter is the Claude Code implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Claude Code Adapter.
func New() *Adapter { return &Adapter{} }

// Source returns SourceClaudeCode.
func (a *Adapter) Source() adapters.Source { return adapters.SourceClaudeCode }

// RecordSession inserts a session row with source="claude-code".
func (a *Adapter) RecordSession(st *store.Store, sessionID, cwd, modelName, gitHead string) error {
	sess := model.Session{
		ID:           sessionID,
		StartedAt:    time.Now().UnixMilli(),
		CWD:          cwd,
		GitHeadStart: gitHead,
		Model:        modelName,
		Source:       string(adapters.SourceClaudeCode),
	}
	return st.InsertSession(sess)
}

// RecordEdit inserts an edit row.  lineStart and lineEnd of 0 are treated as
// "not available" and stored as nil.
func (a *Adapter) RecordEdit(
	st *store.Store,
	sessionID, filePath, tool, diff string,
	lineStart, lineEnd int,
	timestamp int64,
) error {
	latest, err := st.LatestPromptForSession(sessionID)
	if err != nil {
		// Non-fatal -- proceed without prompt linkage.
		latest = nil
	}
	var promptID *int64
	if latest != nil {
		promptID = &latest.ID
	}

	var ls, le *int
	if lineStart > 0 {
		ls = &lineStart
	}
	if lineEnd > 0 {
		le = &lineEnd
	}

	edit := model.Edit{
		SessionID: sessionID,
		PromptID:  promptID,
		Timestamp: timestamp,
		FilePath:  filePath,
		Tool:      tool,
		Diff:      diff,
		LineStart: ls,
		LineEnd:   le,
	}
	return st.InsertEdit(edit)
}

// RecordExecution inserts an execution row.  exitCode of -1 and durationMS of
// -1 are treated as "not available" and stored as nil.
func (a *Adapter) RecordExecution(
	st *store.Store,
	sessionID, command, classification string,
	exitCode int,
	durationMS int64,
	timestamp int64,
) error {
	var ec *int
	if exitCode >= 0 {
		ec = &exitCode
	}
	var dur *int64
	if durationMS >= 0 {
		dur = &durationMS
	}

	x := model.Execution{
		SessionID:      sessionID,
		Timestamp:      timestamp,
		Command:        command,
		Classification: classification,
		ExitCode:       ec,
		DurationMS:     dur,
	}
	return st.InsertExecution(x)
}

// RecordPrompt inserts a prompt row.
func (a *Adapter) RecordPrompt(st *store.Store, sessionID, content string, timestamp int64) error {
	prompt := model.Prompt{
		SessionID:   sessionID,
		Timestamp:   timestamp,
		Content:     content,
		ContentHash: util.SHA256HexString(content),
	}
	_, err := st.InsertPrompt(prompt)
	return err
}
