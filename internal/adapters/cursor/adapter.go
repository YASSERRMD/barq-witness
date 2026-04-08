// Package cursor is a stub adapter for the Cursor AI coding tool.
//
// Expected Cursor hook format (when available):
//   Cursor is expected to expose project-level hooks via a JSON IPC mechanism
//   similar to Claude Code.  When the Cursor hook format is published, this
//   adapter will parse events such as file saves, terminal commands, and prompt
//   submissions and translate them into store writes.
//
// All methods are currently no-ops (return nil) pending official Cursor hook
// documentation.
package cursor

import (
	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Adapter is the Cursor stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Cursor Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceCursor.
func (a *Adapter) Source() adapters.Source { return adapters.SourceCursor }

// RecordSession is a no-op stub.
// TODO: implement once Cursor hook format is available.
func (a *Adapter) RecordSession(_ *store.Store, _, _, _, _ string) error { return nil }

// RecordEdit is a no-op stub.
// TODO: implement once Cursor hook format is available.
func (a *Adapter) RecordEdit(_ *store.Store, _, _, _, _ string, _, _ int, _ int64) error {
	return nil
}

// RecordExecution is a no-op stub.
// TODO: implement once Cursor hook format is available.
func (a *Adapter) RecordExecution(_ *store.Store, _, _, _ string, _ int, _ int64, _ int64) error {
	return nil
}

// RecordPrompt is a no-op stub.
// TODO: implement once Cursor hook format is available.
func (a *Adapter) RecordPrompt(_ *store.Store, _, _ string, _ int64) error { return nil }
