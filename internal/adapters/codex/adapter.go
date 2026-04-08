// Package codex provides a read-only importer for Codex CLI session logs.
//
// It parses the Codex CLI JSON session format and writes sessions, edits,
// and executions into the barq-witness trace store.
//
// The Adapter struct remains a no-op stub for the live hook interface.
// Use ImportFromLog for batch import from a saved log file.
package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Adapter is the Codex stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Codex Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceCodex.
func (a *Adapter) Source() adapters.Source { return adapters.SourceCodex }

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

// codexLog is the JSON structure of a Codex CLI session log.
type codexLog struct {
	ID         string          `json:"id"`
	Model      string          `json:"model"`
	CWD        string          `json:"cwd"`
	CreatedAt  int64           `json:"created_at"`
	Patches    []codexPatch    `json:"patches"`
	Executions []codexExecution `json:"executions"`
}

type codexPatch struct {
	File       string `json:"file"`
	Patch      string `json:"patch"`
	Timestamp  int64  `json:"timestamp"`
	AcceptedMS int64  `json:"accepted_ms"`
}

type codexExecution struct {
	Cmd        string `json:"cmd"`
	ExitCode   *int   `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	Timestamp  int64  `json:"timestamp"`
}

// ImportFromLog reads a Codex CLI JSON session log at logPath, inserts the
// session, edits, and executions into st, and returns the number of edits
// imported. If a session with the same ID already exists the import is skipped
// gracefully.
func ImportFromLog(st *store.Store, logPath string) (int, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0, fmt.Errorf("codex import: read log: %w", err)
	}

	var log codexLog
	if err := json.Unmarshal(data, &log); err != nil {
		return 0, fmt.Errorf("codex import: parse JSON: %w", err)
	}

	if log.ID == "" {
		log.ID = "codex-imported"
	}

	startedAt := log.CreatedAt
	if startedAt == 0 {
		// Fall back to earliest patch or execution timestamp.
		for _, p := range log.Patches {
			if startedAt == 0 || p.Timestamp < startedAt {
				startedAt = p.Timestamp
			}
		}
		for _, e := range log.Executions {
			if startedAt == 0 || e.Timestamp < startedAt {
				startedAt = e.Timestamp
			}
		}
	}

	sess := model.Session{
		ID:        log.ID,
		StartedAt: startedAt,
		CWD:       log.CWD,
		Model:     log.Model,
		Source:    string(adapters.SourceCodex),
	}

	if err := st.InsertSession(sess); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, nil
		}
		return 0, fmt.Errorf("codex import: insert session: %w", err)
	}

	edits := 0
	for _, p := range log.Patches {
		acceptedSec := p.AcceptedMS / 1000
		edit := model.Edit{
			SessionID: log.ID,
			Timestamp: p.Timestamp,
			FilePath:  p.File,
			Tool:      fmt.Sprintf("codex-patch(accepted_sec=%d)", acceptedSec),
			Diff:      p.Patch,
		}
		if err := st.InsertEdit(edit); err != nil {
			return edits, fmt.Errorf("codex import: insert edit: %w", err)
		}
		edits++
	}

	for _, ex := range log.Executions {
		exitCode := -1
		if ex.ExitCode != nil {
			exitCode = *ex.ExitCode
		}
		var ec *int
		if exitCode >= 0 {
			ec = &exitCode
		}
		var dur *int64
		if ex.DurationMS > 0 {
			d := ex.DurationMS
			dur = &d
		}
		x := model.Execution{
			SessionID:  log.ID,
			Timestamp:  ex.Timestamp,
			Command:    ex.Cmd,
			ExitCode:   ec,
			DurationMS: dur,
		}
		if err := st.InsertExecution(x); err != nil {
			return edits, fmt.Errorf("codex import: insert execution: %w", err)
		}
	}

	return edits, nil
}
