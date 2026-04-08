// Command gen creates a synthetic v1.0 fixture database for migration tests.
//
// The fixture DB intentionally uses the original schema without the newer
// columns (source) or tables (intent_matches, meta) so that the migration
// test suite can verify that store.Open migrates them cleanly.
//
// Usage:
//
//	go run ./tests/migration/gen/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	outDir := filepath.Join("tests", "migration", "fixtures", "v1.0")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	dbPath := filepath.Join(outDir, "trace.db")
	// Remove existing file so we always produce a fresh v1.0 fixture.
	_ = os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := createV1Schema(db); err != nil {
		log.Fatalf("schema: %v", err)
	}
	if err := insertFixtureData(db); err != nil {
		log.Fatalf("insert: %v", err)
	}

	fmt.Printf("wrote %s\n", dbPath)
}

func createV1Schema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE sessions (
			id              TEXT PRIMARY KEY,
			started_at      INTEGER NOT NULL,
			ended_at        INTEGER,
			cwd             TEXT    NOT NULL DEFAULT '',
			git_head_start  TEXT    NOT NULL DEFAULT '',
			git_head_end    TEXT,
			model           TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE prompts (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id   TEXT    NOT NULL,
			timestamp    INTEGER NOT NULL,
			content      TEXT    NOT NULL DEFAULT '',
			content_hash TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE edits (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id   TEXT    NOT NULL,
			prompt_id    INTEGER,
			timestamp    INTEGER NOT NULL,
			file_path    TEXT    NOT NULL DEFAULT '',
			tool         TEXT    NOT NULL DEFAULT '',
			before_hash  TEXT,
			after_hash   TEXT,
			line_start   INTEGER,
			line_end     INTEGER,
			diff         TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE executions (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id      TEXT    NOT NULL,
			timestamp       INTEGER NOT NULL,
			command         TEXT    NOT NULL DEFAULT '',
			classification  TEXT    NOT NULL DEFAULT '',
			files_touched   TEXT,
			exit_code       INTEGER,
			duration_ms     INTEGER
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

func insertFixtureData(db *sql.DB) error {
	// Insert a test session (no source column in v1.0).
	_, err := db.Exec(
		`INSERT INTO sessions (id, started_at, ended_at, cwd, git_head_start, git_head_end, model)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"fixture-session-001",
		int64(1700000000000),
		int64(1700003600000),
		"/home/user/project",
		"abc1234abc1234abc1234abc1234abc1234abc1234",
		"def5678def5678def5678def5678def5678def5678",
		"claude-3-opus",
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	// Insert a prompt.
	_, err = db.Exec(
		`INSERT INTO prompts (session_id, timestamp, content, content_hash)
		 VALUES (?, ?, ?, ?)`,
		"fixture-session-001",
		int64(1700001000000),
		"Add a hello world function",
		"sha256:aabbccdd",
	)
	if err != nil {
		return fmt.Errorf("insert prompt: %w", err)
	}

	// Insert an edit.  Use empty string for before_hash to avoid NULL scan
	// errors when scanning into model.Edit.BeforeHash (type string).
	_, err = db.Exec(
		`INSERT INTO edits (session_id, prompt_id, timestamp, file_path, tool, before_hash, after_hash, line_start, line_end, diff)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"fixture-session-001",
		1,
		int64(1700001500000),
		"main.go",
		"Write",
		"",
		"sha256:11223344",
		1,
		5,
		"+func hello() { fmt.Println(\"hello\") }",
	)
	if err != nil {
		return fmt.Errorf("insert edit: %w", err)
	}

	// Insert an execution.
	_, err = db.Exec(
		`INSERT INTO executions (session_id, timestamp, command, classification, files_touched, exit_code, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"fixture-session-001",
		int64(1700002000000),
		"go build ./...",
		"build",
		`["main.go"]`,
		0,
		1234,
	)
	if err != nil {
		return fmt.Errorf("insert execution: %w", err)
	}

	return nil
}
