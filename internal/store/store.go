// Package store provides a SQLite-backed trace store for barq-witness.
// It is the sole writer to the .witness/trace.db file.
package store

import (
	"database/sql"
	"errors"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/model"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store wraps a SQLite database and exposes typed trace operations.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the trace database at path.
// The directory is created if it does not exist.
// The schema is applied idempotently on every open.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create trace dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer; WAL mode allows concurrent reads.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// Apply schema (all statements are IF NOT EXISTS).
	for _, stmt := range splitStatements(schemaSQL) {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply schema: %w", err)
		}
	}

	// Apply incremental migrations.
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// AllSessions returns all sessions ordered by started_at ascending.
func (s *Store) AllSessions() ([]model.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, started_at, ended_at, cwd, git_head_start, git_head_end, model,
		        COALESCE(source, 'claude-code')
		 FROM sessions ORDER BY started_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []model.Session
	for rows.Next() {
		var sess model.Session
		if err := rows.Scan(
			&sess.ID, &sess.StartedAt, &sess.EndedAt,
			&sess.CWD, &sess.GitHeadStart, &sess.GitHeadEnd, &sess.Model,
			&sess.Source,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// PromptsForSession returns all prompts for a session ordered by timestamp.
func (s *Store) PromptsForSession(sessionID string) ([]model.Prompt, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, timestamp, content, content_hash
		 FROM prompts WHERE session_id = ? ORDER BY timestamp ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prompts []model.Prompt
	for rows.Next() {
		var p model.Prompt
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Timestamp, &p.Content, &p.ContentHash); err != nil {
			return nil, err
		}
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}

// InsertSession inserts a new session row.
func (s *Store) InsertSession(sess model.Session) error {
	src := sess.Source
	if src == "" {
		src = "claude-code"
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, started_at, ended_at, cwd, git_head_start, git_head_end, model, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.StartedAt, sess.EndedAt,
		sess.CWD, sess.GitHeadStart, sess.GitHeadEnd, sess.Model, src,
	)
	return err
}

// EndSession sets ended_at and git_head_end for the given session.
func (s *Store) EndSession(id string, endedAt int64, gitHeadEnd string) error {
	res, err := s.db.Exec(
		`UPDATE sessions SET ended_at = ?, git_head_end = ? WHERE id = ?`,
		endedAt, gitHeadEnd, id,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session %q not found", id)
	}
	return nil
}

// InsertPrompt inserts a prompt and returns its auto-assigned ID.
func (s *Store) InsertPrompt(p model.Prompt) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO prompts (session_id, timestamp, content, content_hash)
		 VALUES (?, ?, ?, ?)`,
		p.SessionID, p.Timestamp, p.Content, p.ContentHash,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertEdit inserts an edit row.
func (s *Store) InsertEdit(e model.Edit) error {
	_, err := s.db.Exec(
		`INSERT INTO edits
		 (session_id, prompt_id, timestamp, file_path, tool,
		  before_hash, after_hash, line_start, line_end, diff)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.SessionID, e.PromptID, e.Timestamp, e.FilePath, e.Tool,
		e.BeforeHash, e.AfterHash, e.LineStart, e.LineEnd, e.Diff,
	)
	return err
}

// InsertExecution inserts an execution row.
func (s *Store) InsertExecution(x model.Execution) error {
	_, err := s.db.Exec(
		`INSERT INTO executions
		 (session_id, timestamp, command, classification, files_touched, exit_code, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		x.SessionID, x.Timestamp, x.Command, x.Classification,
		x.FilesTouched, x.ExitCode, x.DurationMS,
	)
	return err
}

// LatestPromptForSession returns the most recent prompt for the given session,
// or nil if none exists.
func (s *Store) LatestPromptForSession(sessionID string) (*model.Prompt, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, timestamp, content, content_hash
		 FROM prompts
		 WHERE session_id = ?
		 ORDER BY timestamp DESC, id DESC
		 LIMIT 1`,
		sessionID,
	)
	p := &model.Prompt{}
	err := row.Scan(&p.ID, &p.SessionID, &p.Timestamp, &p.Content, &p.ContentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// EditsForFiles returns all edit rows whose file_path is in the given list.
func (s *Store) EditsForFiles(files []string) ([]model.Edit, error) {
	if len(files) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(files))
	args := make([]any, len(files))
	for i, f := range files {
		placeholders[i] = "?"
		args[i] = f
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, prompt_id, timestamp, file_path, tool,
		        before_hash, after_hash, line_start, line_end, diff
		 FROM edits
		 WHERE file_path IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY timestamp ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdits(rows)
}

// PromptByID returns the prompt with the given id, or nil if not found.
func (s *Store) PromptByID(id int64) (*model.Prompt, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, timestamp, content, content_hash
		 FROM prompts WHERE id = ?`, id,
	)
	p := &model.Prompt{}
	err := row.Scan(&p.ID, &p.SessionID, &p.Timestamp, &p.Content, &p.ContentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// EditsForSession returns all edit rows for the given session,
// ordered by timestamp ascending.
func (s *Store) EditsForSession(sessionID string) ([]model.Edit, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, prompt_id, timestamp, file_path, tool,
		        before_hash, after_hash, line_start, line_end, diff
		 FROM edits
		 WHERE session_id = ?
		 ORDER BY timestamp ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdits(rows)
}

// ExecutionsForSession returns all execution rows for the given session,
// ordered by timestamp ascending.
func (s *Store) ExecutionsForSession(sessionID string) ([]model.Execution, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, timestamp, command, classification,
		        files_touched, exit_code, duration_ms
		 FROM executions
		 WHERE session_id = ?
		 ORDER BY timestamp ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExecutions(rows)
}

// --- helpers ----------------------------------------------------------------

func splitStatements(sql string) []string {
	var stmts []string
	for _, s := range strings.Split(sql, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

func scanEdits(rows *sql.Rows) ([]model.Edit, error) {
	var edits []model.Edit
	for rows.Next() {
		var e model.Edit
		if err := rows.Scan(
			&e.ID, &e.SessionID, &e.PromptID, &e.Timestamp,
			&e.FilePath, &e.Tool,
			&e.BeforeHash, &e.AfterHash,
			&e.LineStart, &e.LineEnd, &e.Diff,
		); err != nil {
			return nil, err
		}
		edits = append(edits, e)
	}
	return edits, rows.Err()
}

func scanExecutions(rows *sql.Rows) ([]model.Execution, error) {
	var execs []model.Execution
	for rows.Next() {
		var x model.Execution
		if err := rows.Scan(
			&x.ID, &x.SessionID, &x.Timestamp,
			&x.Command, &x.Classification,
			&x.FilesTouched, &x.ExitCode, &x.DurationMS,
		); err != nil {
			return nil, err
		}
		execs = append(execs, x)
	}
	return execs, rows.Err()
}
