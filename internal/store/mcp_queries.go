package store

import (
	"database/sql"
	"errors"

	"github.com/yasserrmd/barq-witness/internal/model"
)

// EditByID returns the edit with the given id, or nil if not found.
func (s *Store) EditByID(id int64) (*model.Edit, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, prompt_id, timestamp, file_path, tool,
		        before_hash, after_hash, line_start, line_end, diff
		 FROM edits WHERE id = ?`, id,
	)
	e := &model.Edit{}
	err := row.Scan(
		&e.ID, &e.SessionID, &e.PromptID, &e.Timestamp,
		&e.FilePath, &e.Tool,
		&e.BeforeHash, &e.AfterHash,
		&e.LineStart, &e.LineEnd, &e.Diff,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Stats holds aggregate counts from the trace store.
type Stats struct {
	TotalEdits    int `json:"total_edits"`
	TotalSessions int `json:"total_sessions"`
}

// GetStats returns aggregate counts for edits and sessions.
func (s *Store) GetStats() (Stats, error) {
	var st Stats
	row := s.db.QueryRow(`SELECT COUNT(*) FROM edits`)
	if err := row.Scan(&st.TotalEdits); err != nil {
		return st, err
	}
	row = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`)
	if err := row.Scan(&st.TotalSessions); err != nil {
		return st, err
	}
	return st, nil
}

// RecentSessions returns the most recent n sessions ordered by started_at
// descending. If limit <= 0, all sessions are returned.
func (s *Store) RecentSessions(limit int) ([]model.Session, error) {
	q := `SELECT id, started_at, ended_at, cwd, git_head_start, git_head_end, model,
	             COALESCE(source, 'claude-code')
	      FROM sessions ORDER BY started_at DESC`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		q += " LIMIT ?"
		rows, err = s.db.Query(q, limit)
	} else {
		rows, err = s.db.Query(q)
	}
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
