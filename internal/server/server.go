// Package server implements the barq-witness self-hosted team aggregator HTTP server.
// It accepts anonymized CGPF v0.3 payloads, stores summary data in SQLite,
// and serves a simple HTML dashboard plus a JSON stats endpoint.
package server

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// maxIngestBytes is the maximum accepted request body size (10 MB).
const maxIngestBytes = 10 * 1024 * 1024

// maxEditsPerPayload is the maximum number of edits per ingested document.
const maxEditsPerPayload = 10_000

// uuidRE is a loose UUID format validator.
var uuidRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>barq-witness team dashboard</title>
<style>
  body { font-family: monospace; background: #0d1117; color: #c9d1d9; margin: 0; padding: 24px; }
  h1 { color: #58a6ff; margin-bottom: 4px; }
  .subtitle { color: #8b949e; font-size: 0.85em; margin-bottom: 32px; }
  .cards { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 32px; }
  .card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 20px 28px; min-width: 140px; }
  .card-label { color: #8b949e; font-size: 0.8em; text-transform: uppercase; letter-spacing: 0.05em; }
  .card-value { font-size: 2em; font-weight: bold; color: #f0f6fc; margin-top: 4px; }
  table { border-collapse: collapse; width: 100%; max-width: 800px; margin-bottom: 32px; }
  th { background: #161b22; color: #8b949e; font-size: 0.8em; text-transform: uppercase; padding: 8px 12px; text-align: left; border-bottom: 1px solid #30363d; }
  td { padding: 8px 12px; border-bottom: 1px solid #21262d; font-size: 0.9em; }
  tr:last-child td { border-bottom: none; }
  .tier1 { color: #f85149; }
  .tier2 { color: #d29922; }
  .tier3 { color: #3fb950; }
  h2 { color: #e6edf3; margin-top: 0; }
  .section { margin-bottom: 32px; }
  .updated { color: #8b949e; font-size: 0.78em; margin-top: 24px; }
</style>
</head>
<body>
<h1>barq-witness</h1>
<p class="subtitle">self-hosted team aggregator dashboard</p>

<div class="cards">
  <div class="card"><div class="card-label">Sessions</div><div class="card-value">{{.TotalSessions}}</div></div>
  <div class="card"><div class="card-label">Total Edits</div><div class="card-value">{{.TotalEdits}}</div></div>
  <div class="card"><div class="card-label">Contributors</div><div class="card-value">{{.Contributors}}</div></div>
</div>

<div class="section">
<h2>Tier breakdown</h2>
<table>
  <thead><tr><th>Tier</th><th>Description</th><th>Count</th></tr></thead>
  <tbody>
    <tr><td class="tier1">Tier 1</td><td>Immediate review</td><td>{{.Tier1Count}}</td></tr>
    <tr><td class="tier2">Tier 2</td><td>Review before merge</td><td>{{.Tier2Count}}</td></tr>
    <tr><td class="tier3">Tier 3</td><td>Low risk</td><td>{{.Tier3Count}}</td></tr>
  </tbody>
</table>
</div>

{{if .TopFlaggedFiles}}
<div class="section">
<h2>Top flagged files</h2>
<table>
  <thead><tr><th>#</th><th>File</th><th>Flag count</th></tr></thead>
  <tbody>
    {{range $i, $f := .TopFlaggedFiles}}
    <tr><td>{{inc $i}}</td><td>{{$f.File}}</td><td>{{$f.Count}}</td></tr>
    {{end}}
  </tbody>
</table>
</div>
{{end}}

<p class="updated">last updated: {{.LastUpdated}}</p>
</body>
</html>`

var dashboardTmpl = template.Must(
	template.New("dashboard").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).Parse(dashboardHTML),
)

// Server is the aggregator HTTP server.
type Server struct {
	db   *sql.DB
	port int
	srv  *http.Server
}

// New opens (or creates) the SQLite database at dbPath and initialises the schema.
func New(dbPath string, port int) (*Server, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Server{db: db, port: port}, nil
}

// Start registers routes and blocks until the server exits.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", s.handleIngest)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/dashboard", s.handleDashboard)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// applySchema runs the embedded DDL.
func applySchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}

// ---- ingest endpoint -------------------------------------------------------

// cgpfPayload is the subset of a CGPF document that we parse for ingest.
type cgpfPayload struct {
	CGPFVersion string        `json:"cgpf_version"`
	AuthorUUID  string        `json:"author_uuid"`
	Sessions    []cgpfSession `json:"sessions"`
}

type cgpfSession struct {
	ID     string     `json:"id"`
	Source string     `json:"source"`
	Edits  []cgpfEdit `json:"edits"`
}

type cgpfEdit struct {
	FilePath    string   `json:"file_path"`
	Tier        int      `json:"tier"`
	ReasonCodes []string `json:"reason_codes"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce 10 MB limit.
	r.Body = http.MaxBytesReader(w, r.Body, maxIngestBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var payload cgpfPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Validate CGPF version.
	if payload.CGPFVersion != "0.3" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported cgpf_version; expected 0.3"})
		return
	}

	// Validate author_uuid.
	if payload.AuthorUUID == "" || !uuidRE.MatchString(payload.AuthorUUID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "author_uuid is missing or not a valid UUID"})
		return
	}

	// Count total edits across all sessions for DoS protection.
	totalEdits := 0
	for _, sess := range payload.Sessions {
		totalEdits += len(sess.Edits)
	}
	if totalEdits > maxEditsPerPayload {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload contains too many edits (max 10000)"})
		return
	}

	// Persist each session.
	now := time.Now().UnixMilli()
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db begin: " + err.Error()})
		return
	}
	defer tx.Rollback() //nolint:errcheck

	for _, sess := range payload.Sessions {
		if sess.ID == "" {
			continue
		}
		src := sess.Source
		if src == "" {
			src = "claude-code"
		}

		tier1, tier2, tier3 := 0, 0, 0
		for _, e := range sess.Edits {
			switch e.Tier {
			case 1:
				tier1++
			case 2:
				tier2++
			case 3:
				tier3++
			}
		}

		_, err := tx.Exec(
			`INSERT INTO ingested_sessions
             (author_uuid, source, session_id, ingested_at, edit_count, tier1_count, tier2_count, tier3_count)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			payload.AuthorUUID, src, sess.ID, now, len(sess.Edits), tier1, tier2, tier3,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert session: " + err.Error()})
			return
		}

		for _, e := range sess.Edits {
			rc := strings.Join(e.ReasonCodes, ",")
			_, err := tx.Exec(
				`INSERT INTO ingested_edits (session_id, file_path, tier, reason_codes, ingested_at)
                 VALUES (?, ?, ?, ?, ?)`,
				sess.ID, e.FilePath, e.Tier, rc, now,
			)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert edit: " + err.Error()})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db commit: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- stats endpoint --------------------------------------------------------

// StatsResponse is the JSON payload returned by GET /api/v1/stats.
type StatsResponse struct {
	TotalSessions   int           `json:"total_sessions"`
	TotalEdits      int           `json:"total_edits"`
	Tier1Count      int           `json:"tier1_count"`
	Tier2Count      int           `json:"tier2_count"`
	Tier3Count      int           `json:"tier3_count"`
	TopFlaggedFiles []FlaggedFile `json:"top_flagged_files"`
	Contributors    int           `json:"contributors"`
	LastUpdated     string        `json:"last_updated"`
}

// FlaggedFile is one entry in the top-flagged-files list.
type FlaggedFile struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.queryStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) queryStats() (*StatsResponse, error) {
	resp := &StatsResponse{
		TopFlaggedFiles: []FlaggedFile{},
		LastUpdated:     time.Now().UTC().Format(time.RFC3339),
	}

	// Total sessions and edit/tier counts.
	row := s.db.QueryRow(`
        SELECT COUNT(*),
               COALESCE(SUM(edit_count),0),
               COALESCE(SUM(tier1_count),0),
               COALESCE(SUM(tier2_count),0),
               COALESCE(SUM(tier3_count),0)
        FROM ingested_sessions`)
	if err := row.Scan(
		&resp.TotalSessions, &resp.TotalEdits,
		&resp.Tier1Count, &resp.Tier2Count, &resp.Tier3Count,
	); err != nil {
		return nil, fmt.Errorf("query session counts: %w", err)
	}

	// Distinct contributors.
	row = s.db.QueryRow(`SELECT COUNT(DISTINCT author_uuid) FROM ingested_sessions`)
	if err := row.Scan(&resp.Contributors); err != nil {
		return nil, fmt.Errorf("query contributors: %w", err)
	}

	// Top 10 flagged files (tier > 0).
	rows, err := s.db.Query(`
        SELECT file_path, COUNT(*) AS cnt
        FROM ingested_edits
        WHERE tier > 0
        GROUP BY file_path
        ORDER BY cnt DESC
        LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("query top files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ff FlaggedFile
		if err := rows.Scan(&ff.File, &ff.Count); err != nil {
			return nil, fmt.Errorf("scan top files: %w", err)
		}
		resp.TopFlaggedFiles = append(resp.TopFlaggedFiles, ff)
	}

	// Last updated from most recent ingestion.
	var lastAt sql.NullInt64
	row = s.db.QueryRow(`SELECT MAX(ingested_at) FROM ingested_sessions`)
	if err := row.Scan(&lastAt); err == nil && lastAt.Valid {
		resp.LastUpdated = time.UnixMilli(lastAt.Int64).UTC().Format(time.RFC3339)
	}

	return resp, nil
}

// ---- dashboard endpoint ----------------------------------------------------

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.queryStats()
	if err != nil {
		http.Error(w, "internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, stats); err != nil {
		_ = err
	}
}

// ---- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
