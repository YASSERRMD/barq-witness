// Package codex is a stub adapter for the OpenAI Codex (codex CLI) tool.
//
// Expected Codex hook format (when available):
//   The Codex CLI is expected to expose hook events via stdin JSON payloads
//   covering session lifecycle, file edits, and shell executions.  When the
//   official hook schema is published this adapter will translate those events
//   into store writes.
//
// All methods are currently no-ops (return nil) pending official Codex hook
// documentation.
package codex

import (
	"github.com/yasserrmd/barq-witness/internal/adapters"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Adapter is the Codex stub implementation of adapters.Adapter.
type Adapter struct{}

// New returns a new Codex Adapter stub.
func New() *Adapter { return &Adapter{} }

// Source returns SourceCodex.
func (a *Adapter) Source() adapters.Source { return adapters.SourceCodex }

// RecordSession is a no-op stub.
// TODO: implement once Codex hook format is available.
func (a *Adapter) RecordSession(_ *store.Store, _, _, _, _ string) error { return nil }

// RecordEdit is a no-op stub.
// TODO: implement once Codex hook format is available.
func (a *Adapter) RecordEdit(_ *store.Store, _, _, _, _ string, _, _ int, _ int64) error {
	return nil
}

// RecordExecution is a no-op stub.
// TODO: implement once Codex hook format is available.
func (a *Adapter) RecordExecution(_ *store.Store, _, _, _ string, _ int, _ int64, _ int64) error {
	return nil
}

// RecordPrompt is a no-op stub.
// TODO: implement once Codex hook format is available.
func (a *Adapter) RecordPrompt(_ *store.Store, _, _ string, _ int64) error { return nil }
