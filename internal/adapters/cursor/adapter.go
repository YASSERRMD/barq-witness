// Package cursor provides a read-only importer for Cursor AI session logs.
//
// It parses the Cursor JSON session export format and writes sessions, edits,
// and executions into the barq-witness trace store.
//
// The Adapter struct remains a no-op stub for the live hook interface.
// Use ImportFromLog for batch import from a saved log file.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Adapter is the Cursor stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Cursor Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceCursor.
func (a *Adapter) Source() adapters.Source { return adapters.SourceCursor }

// RecordSession is a no-op stub.
func (a *Adapter) RecordSession(_ *store.Store, _, _, _, _ string) error { return nil }

// RecordEdit is a no-op stub.
func (a *Adapter) RecordEdit(_ *store.Store, _, _, _, _ string, _, _ int, _ int64) error {
	return nil
}

// RecordExecution is a no-op stub.
func (a *Adapter) RecordExecution(_ *store.Store, _, _, _ string, _ int, _ int64, _ int64) error {
	return nil
}

// RecordPrompt is a no-op stub.
func (a *Adapter) RecordPrompt(_ *store.Store, _, _ string, _ int64) error { return nil }

// --- read-only import -------------------------------------------------------

// cursorLog is the JSON structure of a Cursor session export.
type cursorLog struct {
	SessionID string       `json:"session_id"`
	Model     string       `json:"model"`
	CWD       string       `json:"cwd"`
	Events    []cursorEvent `json:"events"`
}

type cursorEvent struct {
	Type          string `json:"type"`
	// edit fields
	File          string `json:"file"`
	Diff          string `json:"diff"`
	Timestamp     int64  `json:"timestamp"`
	Accepted      bool   `json:"accepted"`
	AcceptDelayMS int64  `json:"accept_delay_ms"`
	// command fields
	Command    string `json:"command"`
	ExitCode   *int   `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}

// ImportFromLog reads a Cursor JSON session log at logPath, inserts the
// session, edits, and executions into st, and returns the number of edits
// imported. If a session with the same ID already exists the import is skipped
// gracefully.
func ImportFromLog(st *store.Store, logPath string) (int, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0, fmt.Errorf("cursor import: read log: %w", err)
	}

	var log cursorLog
	if err := json.Unmarshal(data, &log); err != nil {
		return 0, fmt.Errorf("cursor import: parse JSON: %w", err)
	}

	if log.SessionID == "" {
		log.SessionID = "cursor-imported"
	}

	// Determine session start time from the earliest event timestamp.
	var startedAt int64
	for _, e := range log.Events {
		if startedAt == 0 || e.Timestamp < startedAt {
			startedAt = e.Timestamp
		}
	}
	if startedAt == 0 {
		startedAt = 0
	}

	sess := model.Session{
		ID:        log.SessionID,
		StartedAt: startedAt,
		CWD:       log.CWD,
		Model:     log.Model,
		Source:    string(adapters.SourceCursor),
	}

	if err := st.InsertSession(sess); err != nil {
		// "UNIQUE constraint failed" means the session was already imported.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, nil
		}
		return 0, fmt.Errorf("cursor import: insert session: %w", err)
	}

	edits := 0
	for _, ev := range log.Events {
		switch ev.Type {
		case "edit":
			// Only import accepted edits.
			if !ev.Accepted {
				continue
			}
			acceptedSec := ev.AcceptDelayMS / 1000
			edit := model.Edit{
				SessionID: log.SessionID,
				Timestamp: ev.Timestamp,
				FilePath:  ev.File,
				Tool:      fmt.Sprintf("cursor-edit(accepted_sec=%d)", acceptedSec),
				Diff:      ev.Diff,
			}
			if err := st.InsertEdit(edit); err != nil {
				return edits, fmt.Errorf("cursor import: insert edit: %w", err)
			}
			edits++

		case "command":
			exitCode := -1
			if ev.ExitCode != nil {
				exitCode = *ev.ExitCode
			}
			var ec *int
			if exitCode >= 0 {
				ec = &exitCode
			}
			var dur *int64
			if ev.DurationMS > 0 {
				d := ev.DurationMS
				dur = &d
			}
			x := model.Execution{
				SessionID:  log.SessionID,
				Timestamp:  ev.Timestamp,
				Command:    ev.Command,
				ExitCode:   ec,
				DurationMS: dur,
			}
			if err := st.InsertExecution(x); err != nil {
				return edits, fmt.Errorf("cursor import: insert execution: %w", err)
			}
		}
	}

	return edits, nil
}
