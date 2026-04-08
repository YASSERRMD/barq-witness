// Package hooks parses the JSON payloads that Claude Code delivers to
// hook commands via stdin.  All structs mirror the documented Claude Code
// hook schema; unknown fields are silently ignored.
package hooks

import "encoding/json"

// --- SessionStart ------------------------------------------------------------

// SessionStartPayload is the stdin payload for the SessionStart hook.
type SessionStartPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Model          string `json:"model"`
}

// ParseSessionStart decodes a SessionStart hook payload.
func ParseSessionStart(data []byte) (*SessionStartPayload, error) {
	var p SessionStartPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// --- UserPromptSubmit --------------------------------------------------------

// UserPromptPayload is the stdin payload for the UserPromptSubmit hook.
type UserPromptPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	// Claude Code delivers the user message in the "prompt" field.
	Prompt string `json:"prompt"`
}

// ParseUserPrompt decodes a UserPromptSubmit hook payload.
func ParseUserPrompt(data []byte) (*UserPromptPayload, error) {
	var p UserPromptPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// --- PostToolUse -------------------------------------------------------------

// EditEntry is one entry inside a MultiEdit tool_input.edits array.
type EditEntry struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// ToolInput represents the tool_input field for PostToolUse hook payloads.
type ToolInput struct {
	// Edit / Write / MultiEdit fields
	FilePath  string      `json:"file_path"`
	OldString string      `json:"old_string"`
	NewString string      `json:"new_string"`
	Content   string      `json:"content"`
	Edits     []EditEntry `json:"edits"`
	// Bash fields
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ToolResponse represents the tool_response field for PostToolUse payloads.
type ToolResponse struct {
	ExitCode   *int   `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMS *int64 `json:"duration_ms"`
}

// PostToolUsePayload is the stdin payload for the PostToolUse hook.
type PostToolUsePayload struct {
	SessionID      string       `json:"session_id"`
	TranscriptPath string       `json:"transcript_path"`
	CWD            string       `json:"cwd"`
	ToolName       string       `json:"tool_name"`
	ToolInput      ToolInput    `json:"tool_input"`
	ToolResponse   ToolResponse `json:"tool_response"`
}

// ParsePostToolUse decodes a PostToolUse hook payload.
func ParsePostToolUse(data []byte) (*PostToolUsePayload, error) {
	var p PostToolUsePayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// --- SessionEnd / Stop -------------------------------------------------------

// SessionEndPayload is the stdin payload for the SessionEnd / Stop hook.
type SessionEndPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

// ParseSessionEnd decodes a SessionEnd or Stop hook payload.
func ParseSessionEnd(data []byte) (*SessionEndPayload, error) {
	var p SessionEndPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
