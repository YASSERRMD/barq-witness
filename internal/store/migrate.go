package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// IntentMatch holds a stored intent-match result for a single edit.
type IntentMatch struct {
	EditID     int64
	Score      float64
	Reasoning  string
	Model      string
	ComputedAt int64 // Unix milliseconds
}

// Migrate applies all schema migrations to db idempotently.
// It is safe to call multiple times on the same database.
func Migrate(db *sql.DB) error {
	// Ensure the meta table exists first so we can read schema_version.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("migrate: create meta table: %w", err)
	}

	version := schemaVersion(db)

	// Migration 1: add intent_matches table.
	if version < 1 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS intent_matches (
				edit_id     INTEGER PRIMARY KEY,
				score       REAL    NOT NULL,
				reasoning   TEXT    NOT NULL,
				model       TEXT    NOT NULL,
				computed_at INTEGER NOT NULL
			)
		`); err != nil {
			return fmt.Errorf("migrate v1: create intent_matches: %w", err)
		}
		if err := setSchemaVersion(db, 1); err != nil {
			return err
		}
	}

	return nil
}

// schemaVersion reads the current schema version from the meta table.
// Returns 0 if the key is not present.
func schemaVersion(db *sql.DB) int {
	var v int
	err := db.QueryRow(`SELECT CAST(value AS INTEGER) FROM meta WHERE key = 'schema_version'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}
	if err != nil {
		return 0
	}
	return v
}

// setSchemaVersion persists the schema version in the meta table.
func setSchemaVersion(db *sql.DB, version int) error {
	_, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		fmt.Sprintf("%d", version),
	)
	if err != nil {
		return fmt.Errorf("migrate: set schema_version: %w", err)
	}
	return nil
}

// UpsertIntentMatch inserts or replaces an intent match record for the given edit.
func (s *Store) UpsertIntentMatch(editID int64, score float64, reasoning, model string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(
		`INSERT INTO intent_matches (edit_id, score, reasoning, model, computed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(edit_id) DO UPDATE SET
		   score       = excluded.score,
		   reasoning   = excluded.reasoning,
		   model       = excluded.model,
		   computed_at = excluded.computed_at`,
		editID, score, reasoning, model, now,
	)
	if err != nil {
		return fmt.Errorf("upsert intent match: %w", err)
	}
	return nil
}

// IntentMatchForEdit returns the stored intent match for the given edit ID.
// The second return value is false when no record exists.
func (s *Store) IntentMatchForEdit(editID int64) (*IntentMatch, bool, error) {
	row := s.db.QueryRow(
		`SELECT edit_id, score, reasoning, model, computed_at
		 FROM intent_matches WHERE edit_id = ?`,
		editID,
	)
	var m IntentMatch
	err := row.Scan(&m.EditID, &m.Score, &m.Reasoning, &m.Model, &m.ComputedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("intent match for edit: %w", err)
	}
	return &m, true, nil
}
