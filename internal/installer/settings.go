// Package installer handles merging barq-witness hook entries into
// .claude/settings.json without clobbering existing user hooks.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// barqCmd is the command prefix used to detect existing barq-witness hooks.
const barqCmd = "barq-witness"

// hookEntry matches the inner hook object inside a hooks list.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookGroup is one element in the per-event array.
type hookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

// settingsFile is the top-level shape of .claude/settings.json.
type settingsFile struct {
	Hooks map[string][]hookGroup `json:"hooks,omitempty"`
	// Preserve any other top-level keys.
	Extra map[string]json.RawMessage `json:"-"`
}

// barqHooks returns the canonical set of barq-witness hook groups.
func barqHooks() map[string][]hookGroup {
	return map[string][]hookGroup{
		"SessionStart": {
			{Hooks: []hookEntry{{Type: "command", Command: "barq-witness record session-start"}}},
		},
		"UserPromptSubmit": {
			{Hooks: []hookEntry{{Type: "command", Command: "barq-witness record prompt"}}},
		},
		"PostToolUse": {
			{
				Matcher: "Edit|MultiEdit|Write",
				Hooks:   []hookEntry{{Type: "command", Command: "barq-witness record edit"}},
			},
			{
				Matcher: "Bash",
				Hooks:   []hookEntry{{Type: "command", Command: "barq-witness record exec"}},
			},
		},
		"SessionEnd": {
			{Hooks: []hookEntry{{Type: "command", Command: "barq-witness record session-end"}}},
		},
	}
}

// MergeResult summarises what Install did.
type MergeResult struct {
	Added   []string // event names where barq-witness hooks were added
	Skipped []string // event names where barq-witness hooks already existed
}

// Install reads (or creates) settingsPath and merges in the barq-witness hooks.
// If force is true, existing barq-witness hook entries are replaced.
// Returns the merge result and any I/O error.
func Install(settingsPath string, force bool) (*MergeResult, error) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read settings: %w", err)
	}

	// Start from an empty document if the file does not exist.
	if os.IsNotExist(err) || len(raw) == 0 {
		raw = []byte("{}")
	}

	// Unmarshal into a generic map to preserve unknown keys.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("parse settings.json: %w", err)
	}

	// Extract the existing hooks section (or create an empty one).
	var existingHooks map[string][]hookGroup
	if hooksRaw, ok := generic["hooks"]; ok {
		if err := json.Unmarshal(hooksRaw, &existingHooks); err != nil {
			return nil, fmt.Errorf("parse hooks section: %w", err)
		}
	}
	if existingHooks == nil {
		existingHooks = make(map[string][]hookGroup)
	}

	result := &MergeResult{}

	for event, barqGroups := range barqHooks() {
		if hasBarqHooks(existingHooks[event]) {
			if force {
				// Remove existing barq-witness groups, then re-add below.
				existingHooks[event] = removeBarqHooks(existingHooks[event])
			} else {
				result.Skipped = append(result.Skipped, event)
				continue
			}
		}
		existingHooks[event] = append(existingHooks[event], barqGroups...)
		result.Added = append(result.Added, event)
	}

	// Serialize the updated hooks back into the generic map.
	hooksJSON, err := json.Marshal(existingHooks)
	if err != nil {
		return nil, fmt.Errorf("marshal hooks: %w", err)
	}
	generic["hooks"] = hooksJSON

	// Write back with 2-space indentation.
	out, err := marshalIndented(generic)
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return nil, fmt.Errorf("create .claude dir: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return nil, fmt.Errorf("write settings: %w", err)
	}

	return result, nil
}

// HasBarqHooks returns true if any hook group in the event's list is a
// barq-witness hook.  Used by tests to inspect the installed settings.
func HasBarqHooks(groups []hookGroup) bool {
	return hasBarqHooks(groups)
}

// ReadHookGroups reads and returns the hookGroups for one event from a settings file.
func ReadHookGroups(settingsPath, event string) ([]hookGroup, error) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	hooksRaw, ok := doc["hooks"]
	if !ok {
		return nil, nil
	}
	var hooks map[string][]hookGroup
	if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
		return nil, err
	}
	return hooks[event], nil
}

// ---- internal helpers -------------------------------------------------------

func hasBarqHooks(groups []hookGroup) bool {
	for _, g := range groups {
		for _, h := range g.Hooks {
			if strings.Contains(h.Command, barqCmd) {
				return true
			}
		}
	}
	return false
}

func removeBarqHooks(groups []hookGroup) []hookGroup {
	var out []hookGroup
	for _, g := range groups {
		var hooks []hookEntry
		for _, h := range g.Hooks {
			if !strings.Contains(h.Command, barqCmd) {
				hooks = append(hooks, h)
			}
		}
		if len(hooks) > 0 {
			out = append(out, hookGroup{Matcher: g.Matcher, Hooks: hooks})
		}
	}
	return out
}

// marshalIndented encodes a map[string]json.RawMessage with 2-space indent,
// preserving the raw values verbatim (re-indent them for readability).
func marshalIndented(m map[string]json.RawMessage) ([]byte, error) {
	// Re-encode each value through the generic encoder so that inner objects
	// are indented consistently.
	cooked := make(map[string]any, len(m))
	for k, v := range m {
		var x any
		if err := json.Unmarshal(v, &x); err != nil {
			return nil, err
		}
		cooked[k] = x
	}
	return json.MarshalIndent(cooked, "", "  ")
}
