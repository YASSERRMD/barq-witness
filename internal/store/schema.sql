-- barq-witness trace schema v1
-- All timestamps are stored as Unix milliseconds (INTEGER).

CREATE TABLE IF NOT EXISTS sessions (
    id             TEXT    PRIMARY KEY,
    started_at     INTEGER NOT NULL,
    ended_at       INTEGER,
    cwd            TEXT,
    git_head_start TEXT,
    git_head_end   TEXT,
    model          TEXT
);

CREATE TABLE IF NOT EXISTS prompts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT    NOT NULL,
    timestamp    INTEGER NOT NULL,
    content      TEXT,
    content_hash TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS edits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL,
    prompt_id   INTEGER,
    timestamp   INTEGER NOT NULL,
    file_path   TEXT    NOT NULL,
    tool        TEXT    NOT NULL,
    before_hash TEXT,
    after_hash  TEXT,
    line_start  INTEGER,
    line_end    INTEGER,
    diff        TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (prompt_id)  REFERENCES prompts(id)
);

CREATE TABLE IF NOT EXISTS executions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT    NOT NULL,
    timestamp      INTEGER NOT NULL,
    command        TEXT    NOT NULL,
    classification TEXT,
    files_touched  TEXT,
    exit_code      INTEGER,
    duration_ms    INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_prompts_session_id
    ON prompts (session_id);

CREATE INDEX IF NOT EXISTS idx_edits_session_id
    ON edits (session_id);

CREATE INDEX IF NOT EXISTS idx_edits_file_path
    ON edits (file_path);

CREATE INDEX IF NOT EXISTS idx_edits_timestamp
    ON edits (timestamp);

CREATE INDEX IF NOT EXISTS idx_executions_session_id
    ON executions (session_id);

CREATE INDEX IF NOT EXISTS idx_executions_timestamp
    ON executions (timestamp);
