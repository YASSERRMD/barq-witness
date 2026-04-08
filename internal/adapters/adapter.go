// Package adapters defines the Adapter interface and Source identifiers for
// normalising hook events from multiple AI coding tools into barq-witness
// store writes.
package adapters

import "github.com/yasserrmd/barq-witness/internal/store"

// Source identifies the AI coding tool that generated a trace event.
type Source string

const (
	SourceClaudeCode Source = "claude-code"
	SourceCursor     Source = "cursor"
	SourceCodex      Source = "codex"
	SourceAider      Source = "aider"
	SourceUnknown    Source = "unknown"
)

// Adapter normalizes hook events from an AI coding tool into store writes.
type Adapter interface {
	Source() Source
	// RecordSession records session start; returns session ID
	RecordSession(st *store.Store, sessionID, cwd, model, gitHead string) error
	// RecordEdit records a file edit event
	RecordEdit(st *store.Store, sessionID, filePath, tool, diff string, lineStart, lineEnd int, timestamp int64) error
	// RecordExecution records a command execution event
	RecordExecution(st *store.Store, sessionID, command, classification string, exitCode int, durationMS int64, timestamp int64) error
	// RecordPrompt records a prompt event
	RecordPrompt(st *store.Store, sessionID, content string, timestamp int64) error
}
