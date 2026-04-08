// Package cgpf implements the Code Generation Provenance Format (CGPF) v0.1.
// CGPF is the stable, portable JSON export format for barq-witness traces.
package cgpf

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Version is the CGPF spec version this package implements.
const Version = "0.2"

// BinaryVersion is set by the main package at build time.
var BinaryVersion = "vDEV"

// ---- CGPF document types ---------------------------------------------------

// Document is the top-level CGPF export document.
type Document struct {
	CGPFVersion string    `json:"cgpf_version"`
	GeneratedBy string    `json:"generated_by"`
	GeneratedAt string    `json:"generated_at"`
	Repo        RepoMeta  `json:"repo"`
	Sessions    []Session `json:"sessions"`
}

// RepoMeta contains repository-level metadata.
type RepoMeta struct {
	Remote      *string     `json:"remote"`
	CommitRange CommitRange `json:"commit_range"`
}

// CommitRange is the from/to SHA pair used to scope the export.
type CommitRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Session is the per-session export structure.
type Session struct {
	ID             string               `json:"id"`
	StartedAt      string               `json:"started_at"`
	EndedAt        *string              `json:"ended_at"`
	Model          string               `json:"model"`
	CWD            string               `json:"cwd"`
	GitHeadStart   string               `json:"git_head_start"`
	GitHeadEnd     *string              `json:"git_head_end"`
	Prompts        []Prompt             `json:"prompts"`
	Edits          []Edit               `json:"edits"`
	Executions     []Execution          `json:"executions"`
	IntentMatches  []IntentMatchRecord  `json:"intent_matches,omitempty"`
}

// IntentMatchRecord is the CGPF intent match record per edit.
type IntentMatchRecord struct {
	EditID     int64   `json:"edit_id"`
	Score      float64 `json:"score"`
	Reasoning  *string `json:"reasoning,omitempty"` // omitted in privacy mode
	Model      string  `json:"model"`
	ComputedAt string  `json:"computed_at"`
}

// Prompt is the CGPF prompt record.
type Prompt struct {
	ID          int64   `json:"id"`
	Timestamp   string  `json:"timestamp"`
	ContentHash string  `json:"content_hash"`
	Content     *string `json:"content,omitempty"` // omitted in privacy mode
}

// Edit is the CGPF edit record.
type Edit struct {
	ID         int64   `json:"id"`
	PromptID   *int64  `json:"prompt_id"`
	Timestamp  string  `json:"timestamp"`
	FilePath   string  `json:"file_path"`
	Tool       string  `json:"tool"`
	BeforeHash string  `json:"before_hash"`
	AfterHash  string  `json:"after_hash"`
	LineStart  *int    `json:"line_start"`
	LineEnd    *int    `json:"line_end"`
}

// Execution is the CGPF execution record.
type Execution struct {
	ID             int64    `json:"id"`
	Timestamp      string   `json:"timestamp"`
	Command        *string  `json:"command,omitempty"` // omitted in privacy mode
	Classification string   `json:"classification"`
	FilesTouched   []string `json:"files_touched"`
	ExitCode       *int     `json:"exit_code"`
	DurationMS     *int64   `json:"duration_ms"`
}

// ExportOptions controls what is included in the export.
type ExportOptions struct {
	// SessionID restricts export to a single session (empty = all sessions).
	SessionID string
	// FromCommit and ToCommit restrict to sessions whose git range overlaps.
	// When both are empty all sessions are included.
	FromCommit string
	ToCommit   string
	// Privacy omits prompt content and bash command text.
	Privacy bool
	// RepoPath is used to read the git remote URL.
	RepoPath string
}

// Export builds a CGPF Document from the trace store according to opts.
func Export(st *store.Store, opts ExportOptions) (*Document, error) {
	sessions, err := st.AllSessions()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// Filter by session ID if requested.
	if opts.SessionID != "" {
		var filtered []model.Session
		for _, s := range sessions {
			if s.ID == opts.SessionID {
				filtered = append(filtered, s)
			}
		}
		sessions = filtered
	}

	remote := detectRemote(opts.RepoPath)

	doc := &Document{
		CGPFVersion: Version,
		GeneratedBy: fmt.Sprintf("barq-witness %s", BinaryVersion),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Repo: RepoMeta{
			Remote: remote,
			CommitRange: CommitRange{
				From: opts.FromCommit,
				To:   opts.ToCommit,
			},
		},
	}

	for _, sess := range sessions {
		exported, err := exportSession(st, sess, opts)
		if err != nil {
			return nil, fmt.Errorf("export session %s: %w", sess.ID, err)
		}
		doc.Sessions = append(doc.Sessions, exported)
	}

	return doc, nil
}

// Marshal serialises a CGPF Document to pretty-printed JSON.
func Marshal(doc *Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// Unmarshal parses a CGPF JSON document.
func Unmarshal(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ---- internal helpers -------------------------------------------------------

func exportSession(st *store.Store, sess model.Session, opts ExportOptions) (Session, error) {
	out := Session{
		ID:           sess.ID,
		StartedAt:    msToISO(sess.StartedAt),
		Model:        sess.Model,
		CWD:          sess.CWD,
		GitHeadStart: sess.GitHeadStart,
	}
	if sess.EndedAt != nil {
		s := msToISO(*sess.EndedAt)
		out.EndedAt = &s
	}
	if sess.GitHeadEnd != nil {
		out.GitHeadEnd = sess.GitHeadEnd
	}

	// Prompts.
	prompts, err := st.PromptsForSession(sess.ID)
	if err != nil {
		return out, fmt.Errorf("prompts: %w", err)
	}
	for _, p := range prompts {
		ep := Prompt{
			ID:          p.ID,
			Timestamp:   msToISO(p.Timestamp),
			ContentHash: p.ContentHash,
		}
		if !opts.Privacy {
			ep.Content = &p.Content
		}
		out.Prompts = append(out.Prompts, ep)
	}

	// Edits.
	edits, err := st.EditsForSession(sess.ID)
	if err != nil {
		return out, fmt.Errorf("edits: %w", err)
	}
	for _, e := range edits {
		out.Edits = append(out.Edits, Edit{
			ID:         e.ID,
			PromptID:   e.PromptID,
			Timestamp:  msToISO(e.Timestamp),
			FilePath:   e.FilePath,
			Tool:       e.Tool,
			BeforeHash: e.BeforeHash,
			AfterHash:  e.AfterHash,
			LineStart:  e.LineStart,
			LineEnd:    e.LineEnd,
		})
	}

	// Executions.
	execs, err := st.ExecutionsForSession(sess.ID)
	if err != nil {
		return out, fmt.Errorf("executions: %w", err)
	}
	for _, x := range execs {
		ex := Execution{
			ID:             x.ID,
			Timestamp:      msToISO(x.Timestamp),
			Classification: x.Classification,
			FilesTouched:   parseFilesTouched(x.FilesTouched),
			ExitCode:       x.ExitCode,
			DurationMS:     x.DurationMS,
		}
		if !opts.Privacy {
			ex.Command = &x.Command
		}
		out.Executions = append(out.Executions, ex)
	}

	// Intent matches -- one per edit that has a stored match.
	for _, e := range edits {
		im, ok, err := st.IntentMatchForEdit(e.ID)
		if err != nil || !ok {
			continue
		}
		rec := IntentMatchRecord{
			EditID:     im.EditID,
			Score:      im.Score,
			Model:      im.Model,
			ComputedAt: msToISO(im.ComputedAt),
		}
		if !opts.Privacy {
			rec.Reasoning = &im.Reasoning
		}
		out.IntentMatches = append(out.IntentMatches, rec)
	}

	return out, nil
}

func msToISO(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}

func parseFilesTouched(raw string) []string {
	if raw == "" || raw == "null" {
		return []string{}
	}
	var files []string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return []string{}
	}
	return files
}

// detectRemote tries to read the "origin" remote URL from the git repo.
func detectRemote(repoPath string) *string {
	if repoPath == "" {
		return nil
	}
	repo, err := gogit.PlainOpenWithOptions(repoPath, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return nil
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil
	}
	u := urls[0]
	return &u
}

// nullString converts a sql.NullString to *string.
func nullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

// trimSHA trims whitespace from a SHA string.
func trimSHA(s string) string {
	return strings.TrimSpace(s)
}
