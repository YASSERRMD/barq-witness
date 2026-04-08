// Package aider is a stub adapter for the Aider AI coding tool.
//
// Expected Aider hook format (when available):
//   Aider's --watch mode emits JSON lines to stdout describing file changes and
//   LLM interactions.  Each line is expected to be a JSON object with at least
//   a "type" field ("edit", "exec", "prompt", "session_start", "session_end")
//   and tool-specific fields.  When Aider formalises this schema this adapter
//   will parse those JSON lines and translate them into store writes.
//
// All methods are currently no-ops (return nil) pending a stable Aider JSON
// watch output format.
package aider

import (
	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Adapter is the Aider stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Aider Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceAider.
func (a *Adapter) Source() adapters.Source { return adapters.SourceAider }

// RecordSession is a no-op stub.
// TODO: implement once Aider --watch JSON format is stable.
func (a *Adapter) RecordSession(_ *store.Store, _, _, _, _ string) error { return nil }

// RecordEdit is a no-op stub.
// TODO: implement once Aider --watch JSON format is stable.
func (a *Adapter) RecordEdit(_ *store.Store, _, _, _, _ string, _, _ int, _ int64) error {
	return nil
}

// RecordExecution is a no-op stub.
// TODO: implement once Aider --watch JSON format is stable.
func (a *Adapter) RecordExecution(_ *store.Store, _, _, _ string, _ int, _ int64, _ int64) error {
	return nil
}

// RecordPrompt is a no-op stub.
// TODO: implement once Aider --watch JSON format is stable.
func (a *Adapter) RecordPrompt(_ *store.Store, _, _ string, _ int64) error { return nil }
