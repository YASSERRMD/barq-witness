// Package plugin implements the external plugin runner for barq-witness.
// Plugins are external executables that communicate via stdin/stdout using
// newline-delimited JSON (CGPF format). This allows users to write custom
// risk signals in any language.
//
// Protocol:
//   - Host sends one JSON line to the plugin's stdin: {"version":"0.3","segment":{...}}
//   - Plugin writes one JSON line to stdout: {"signals":[...],"error":null}
//   - Timeout is 5 seconds per plugin call.
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

// Signal is a risk signal returned by a plugin.
type Signal struct {
	Code    string `json:"code"`
	Tier    int    `json:"tier"`
	Message string `json:"message"`
}

// PluginResult is the structured response from a plugin executable.
type PluginResult struct {
	Signals []Signal `json:"signals"`
	Error   *string  `json:"error"`
}

// Plugin represents a configured external plugin.
type Plugin struct {
	Name string // plugin name from config
	Path string // executable path
}

// hostRequest is the JSON line sent to the plugin on stdin.
type hostRequest struct {
	Version string          `json:"version"`
	Segment segmentPayload  `json:"segment"`
}

// segmentPayload is the serialised form of analyzer.Segment for the plugin protocol.
type segmentPayload struct {
	FilePath    string   `json:"file_path"`
	Tier        int      `json:"tier"`
	ReasonCodes []string `json:"reason_codes"`
	Score       float64  `json:"score"`
	PromptText  string   `json:"prompt_text"`
	EditID      int64    `json:"edit_id"`
	SessionID   string   `json:"session_id"`
	LineStart   int      `json:"line_start"`
	LineEnd     int      `json:"line_end"`
	Modified    bool     `json:"modified"`
	Executed    bool     `json:"executed"`
	Reopened    bool     `json:"reopened"`
	RegenCount  int      `json:"regen_count"`
	SecurityHit bool     `json:"security_hit"`
	NewDep      bool     `json:"new_dep"`
}

// pluginTimeout is the maximum time a single plugin invocation may take.
const pluginTimeout = 5 * time.Second

// Run invokes the plugin executable, sends segment JSON on stdin, and reads
// the response from stdout.
// Timeout: 5 seconds per plugin call.
// Returns empty PluginResult on any error (never propagates plugin errors to the caller).
func (p *Plugin) Run(ctx context.Context, seg analyzer.Segment) (PluginResult, error) {
	// Build the request payload.
	req := hostRequest{
		Version: "0.3",
		Segment: segmentPayload{
			FilePath:    seg.FilePath,
			Tier:        seg.Tier,
			ReasonCodes: seg.ReasonCodes,
			Score:       seg.Score,
			PromptText:  seg.PromptText,
			EditID:      seg.EditID,
			SessionID:   seg.SessionID,
			LineStart:   seg.LineStart,
			LineEnd:     seg.LineEnd,
			Modified:    seg.Modified,
			Executed:    seg.Executed,
			Reopened:    seg.Reopened,
			RegenCount:  seg.RegenCount,
			SecurityHit: seg.SecurityHit,
			NewDep:      seg.NewDep,
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return PluginResult{}, fmt.Errorf("marshal request: %w", err)
	}

	// Enforce timeout.
	callCtx, cancel := context.WithTimeout(ctx, pluginTimeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, p.Path) // #nosec G204 -- path is from user config
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return PluginResult{}, fmt.Errorf("plugin %s exec: %w", p.Name, err)
	}

	// Parse response -- take the first non-empty line.
	line := ""
	for _, l := range strings.Split(stdout.String(), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			line = l
			break
		}
	}
	if line == "" {
		return PluginResult{}, fmt.Errorf("plugin %s returned empty output", p.Name)
	}

	var result PluginResult
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return PluginResult{}, fmt.Errorf("plugin %s response parse: %w", p.Name, err)
	}

	// Enforce the plugin: namespace prefix on all signal codes.
	for i := range result.Signals {
		if !strings.HasPrefix(result.Signals[i].Code, "plugin:") {
			result.Signals[i].Code = "plugin:" + result.Signals[i].Code
		}
	}

	return result, nil
}

// RunAll runs all configured plugins against seg and returns the merged slice
// of signals.  Errors from individual plugins are silently discarded; the
// caller always receives a (possibly empty) slice -- never an error.
func RunAll(ctx context.Context, plugins []Plugin, seg analyzer.Segment) []Signal {
	var all []Signal
	for _, p := range plugins {
		result, err := p.Run(ctx, seg)
		if err != nil {
			// Silent discard: plugin errors must not affect the host.
			continue
		}
		all = append(all, result.Signals...)
	}
	return all
}
