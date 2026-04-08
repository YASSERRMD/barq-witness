CREATE TABLE IF NOT EXISTS ingested_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    author_uuid TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'claude-code',
    session_id TEXT NOT NULL,
    ingested_at INTEGER NOT NULL,
    edit_count INTEGER NOT NULL DEFAULT 0,
    tier1_count INTEGER NOT NULL DEFAULT 0,
    tier2_count INTEGER NOT NULL DEFAULT 0,
    tier3_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS ingested_edits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    tier INTEGER NOT NULL DEFAULT 0,
    reason_codes TEXT NOT NULL DEFAULT '',
    ingested_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_edits_file ON ingested_edits(file_path);
CREATE INDEX IF NOT EXISTS idx_sessions_author ON ingested_sessions(author_uuid);
