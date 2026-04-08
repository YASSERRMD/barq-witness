# How barq-witness Works

## Architecture overview

```
  Claude Code session
  +-------------------------------------------------+
  |                                                 |
  |  User prompt --> [UserPromptSubmit hook]         |
  |                         |                       |
  |                         v                       |
  |              barq-witness record prompt          |
  |                         |                       |
  |                  writes to SQLite               |
  |                                                 |
  |  Edit / Write --> [PostToolUse hook]             |
  |                         |                       |
  |                         v                       |
  |              barq-witness record edit            |
  |                         |                       |
  |                  writes to SQLite               |
  |                                                 |
  |  Bash command --> [PostToolUse hook]             |
  |                         |                       |
  |                         v                       |
  |              barq-witness record exec            |
  |                         |                       |
  |                  writes to SQLite               |
  +-------------------------------------------------+

          .witness/trace.db  (local SQLite file, gitignored)
                    |
                    v
         barq-witness report --from A --to B
                    |
         +----------+----------+
         |                     |
    git diff A..B        trace query
    (changed files       (edits, prompts,
    and line ranges)      executions)
         |                     |
         +----------+----------+
                    |
             risk scorer
          (9 deterministic
            signal checks)
                    |
            ranked Report
                    |
          text / markdown
           attention map
```

## The five components

### 1. Hooks (read-only observers)

barq-witness installs four Claude Code hooks via `.claude/settings.json`:

| Event | Hook command |
|---|---|
| SessionStart | `barq-witness record session-start` |
| UserPromptSubmit | `barq-witness record prompt` |
| PostToolUse (Edit/MultiEdit/Write) | `barq-witness record edit` |
| PostToolUse (Bash) | `barq-witness record exec` |
| SessionEnd | `barq-witness record session-end` |

Each hook reads a JSON payload from stdin and writes one row to the trace
database. Hooks always exit 0 and complete in under 50ms in the common case.
They never modify files, never send data anywhere, and never slow Claude Code.

### 2. Trace store (.witness/trace.db)

A local SQLite database with four tables:

- `sessions` -- one row per Claude Code session
- `prompts` -- one row per user prompt
- `edits` -- one row per file modification (Edit, MultiEdit, Write)
- `executions` -- one row per Bash command

The database is created automatically on `barq-witness init` and updated
silently by the hooks during every session.

### 3. Diff reader (go-git)

When you run `barq-witness report`, the tool uses go-git to read the actual
line-level diff between two commits. This tells the risk scorer which lines
changed and therefore which trace edits are relevant to the commit.

### 4. Risk scorer (deterministic signals)

The scorer cross-references the git diff with the trace and evaluates nine
signal checks -- all pure functions with no LLM, no randomness, and no
network calls. Each signal has a weight; the total weight is the segment
score. Segments are ranked by score descending.

See [signals-reference.md](signals-reference.md) for the full list.

### 5. Renderer

Two output formats:

- **text** (default when stdout is a tty): ANSI-coloured, human-readable,
  with a reason glossary at the end.
- **markdown** (default when piped or in CI): GitHub-flavoured markdown
  suitable for PR comments. Starts with a hidden HTML marker so the GitHub
  Action can update rather than create new comments.

## Data flow summary

1. `barq-witness init` installs hooks and creates the database.
2. During a Claude Code session, hooks write rows to `trace.db`.
3. At review time, `barq-witness report` reads `trace.db` + the git diff.
4. The risk scorer builds a ranked list of flagged segments.
5. The renderer prints the attention map.

No step involves a network call, an LLM, or any external service.
