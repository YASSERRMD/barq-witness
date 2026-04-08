// Package model defines the core event types persisted in the trace store.
// All timestamps are Unix milliseconds (int64).
package model

// Session represents a single AI coding session.
type Session struct {
	ID           string
	StartedAt    int64
	EndedAt      *int64
	CWD          string
	GitHeadStart string
	GitHeadEnd   *string
	Model        string
	// Source identifies the AI coding tool that produced this session.
	// Defaults to "claude-code" for backwards compatibility.
	Source string
}

// Prompt represents a user prompt submitted during a session.
type Prompt struct {
	ID          int64
	SessionID   string
	Timestamp   int64
	Content     string
	ContentHash string
}

// Edit represents a file modification made by a Claude Code tool
// (Edit, MultiEdit, or Write).
type Edit struct {
	ID         int64
	SessionID  string
	PromptID   *int64
	Timestamp  int64
	FilePath   string
	Tool       string
	BeforeHash string
	AfterHash  string
	LineStart  *int
	LineEnd    *int
	Diff       string
}

// Execution represents a Bash command executed during a session.
type Execution struct {
	ID             int64
	SessionID      string
	Timestamp      int64
	Command        string
	Classification string
	FilesTouched   string // JSON array string
	ExitCode       *int
	DurationMS     *int64
}
