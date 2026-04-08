// Package aider provides a read-only importer for Aider chat history files.
//
// It parses the Aider markdown chat history format and writes sessions,
// prompts, edits, and executions into the barq-witness trace store.
//
// The Adapter struct remains a no-op stub for the live hook interface.
// Use ImportFromChat for batch import from a saved chat history file.
package aider

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// Adapter is the Aider stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Aider Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceAider.
func (a *Adapter) Source() adapters.Source { return adapters.SourceAider }

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

// reStarted matches the aider header line, e.g.:
//
//	# aider chat started 2024-01-15 10:30:00
var reStarted = regexp.MustCompile(`^#\s+aider chat started\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)

// reModified matches lines produced by aider for modified files, e.g.:
//
//	> Modified login.py
var reModified = regexp.MustCompile(`^>\s+Modified\s+(.+)`)

// reAssistant matches "## Assistant (model)" headers.
var reAssistant = regexp.MustCompile(`^##\s+Assistant`)

// parsedRecord holds all extracted records before writing to the store.
type parsedRecord struct {
	prompts    []model.Prompt
	edits      []model.Edit
	executions []model.Execution
}

// ImportFromChat reads an Aider markdown chat history at chatPath, inserts
// the session, prompts, edits, and executions into st, and returns the number
// of edits (file modifications) imported. If a session with the same derived
// ID already exists the import is skipped gracefully.
//
// AcceptedSec is set to -1 for all edits because Aider does not track accept
// time.
func ImportFromChat(st *store.Store, chatPath string) (int, error) {
	f, err := os.Open(chatPath)
	if err != nil {
		return 0, fmt.Errorf("aider import: open chat: %w", err)
	}
	defer f.Close()

	// Derive a stable session ID from the file path.
	sessionID := "aider-" + util.SHA256HexString(chatPath)[:12]

	var (
		startedAt    int64
		currentModel string
	)

	// Parsing state.
	type section int
	const (
		secNone section = iota
		secUser
		secAssistant
	)

	var (
		sec       section
		userLines []string
		bashLines []string
		inBash    bool
	)

	// Monotonically increasing ms offset so events have distinct timestamps.
	var tsOffset int64
	nextTS := func() int64 {
		tsOffset += 1000
		return startedAt + tsOffset
	}

	var rec parsedRecord

	flushUser := func() {
		content := strings.TrimSpace(strings.Join(userLines, "\n"))
		if content == "" {
			return
		}
		rec.prompts = append(rec.prompts, model.Prompt{
			SessionID:   sessionID,
			Timestamp:   nextTS(),
			Content:     content,
			ContentHash: util.SHA256HexString(content),
		})
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse the session header line.
		if m := reStarted.FindStringSubmatch(line); m != nil {
			t, err2 := time.Parse("2006-01-02 15:04:05", m[1])
			if err2 == nil {
				startedAt = t.UnixMilli()
			}
			continue
		}

		// Section headers.
		if strings.HasPrefix(line, "## User") {
			if sec == secUser {
				flushUser()
			}
			sec = secUser
			userLines = nil
			inBash = false
			bashLines = nil
			continue
		}

		if reAssistant.MatchString(line) {
			if sec == secUser {
				flushUser()
			}
			// Extract model name if present, e.g. "## Assistant (gpt-4)".
			if idx := strings.Index(line, "("); idx >= 0 {
				end := strings.LastIndex(line, ")")
				if end > idx {
					currentModel = line[idx+1 : end]
				}
			}
			sec = secAssistant
			inBash = false
			bashLines = nil
			userLines = nil
			continue
		}

		switch sec {
		case secUser:
			userLines = append(userLines, line)

		case secAssistant:
			// Detect bash code fence.
			if strings.HasPrefix(line, "```") {
				lang := strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				if inBash {
					// Closing fence -- process the accumulated bash block.
					inBash = false
					cmd := strings.TrimSpace(strings.Join(bashLines, "\n"))
					if cmd != "" {
						rec.executions = append(rec.executions, model.Execution{
							SessionID: sessionID,
							Timestamp: nextTS(),
							Command:   cmd,
						})
					}
					bashLines = nil
				} else if lang == "bash" || lang == "sh" || lang == "" {
					inBash = true
					bashLines = nil
				}
				continue
			}

			if inBash {
				bashLines = append(bashLines, line)
				continue
			}

			// Check for modified file lines.
			if m := reModified.FindStringSubmatch(line); m != nil {
				filePath := strings.TrimSpace(m[1])
				rec.edits = append(rec.edits, model.Edit{
					SessionID: sessionID,
					Timestamp: nextTS(),
					FilePath:  filePath,
					Tool:      "aider-edit(accepted_sec=-1)",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("aider import: scan: %w", err)
	}

	// Flush any trailing user section.
	if sec == secUser {
		flushUser()
	}

	if startedAt == 0 {
		startedAt = time.Now().UnixMilli()
	}

	// Insert session first so foreign key constraints are satisfied.
	sess := model.Session{
		ID:        sessionID,
		StartedAt: startedAt,
		Model:     currentModel,
		Source:    string(adapters.SourceAider),
	}

	if err := st.InsertSession(sess); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, nil
		}
		return 0, fmt.Errorf("aider import: insert session: %w", err)
	}

	// Insert prompts.
	for _, p := range rec.prompts {
		if _, err := st.InsertPrompt(p); err != nil {
			return 0, fmt.Errorf("aider import: insert prompt: %w", err)
		}
	}

	// Insert edits.
	for _, e := range rec.edits {
		if err := st.InsertEdit(e); err != nil {
			return len(rec.edits), fmt.Errorf("aider import: insert edit: %w", err)
		}
	}

	// Insert executions.
	for _, x := range rec.executions {
		if err := st.InsertExecution(x); err != nil {
			return len(rec.edits), fmt.Errorf("aider import: insert execution: %w", err)
		}
	}

	return len(rec.edits), nil
}
