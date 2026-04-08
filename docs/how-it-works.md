# How barq-witness Works

barq-witness installs as Claude Code hooks in `.claude/settings.json`. During
`barq-witness init`, the tool appends hook entries for the PreToolUse,
PostToolUse, and Stop events so that Claude Code calls the witness binary
automatically -- without any changes to the developer's workflow.

On every edit, prompt, and execution, the hook fires and writes a row to
`.witness/trace.db`, a local SQLite database operating in WAL mode. WAL mode
ensures that concurrent reads and writes do not block each other, which means
reporting can run while a session is still in progress. The schema has four
tables -- sessions, prompts, edits, and executions -- and the database is
created automatically on init and gitignored immediately.

At report time, the deterministic analyzer reads the trace, computes signals,
scores segments, and outputs text, markdown, or JSON. The analyzer is a set of
pure functions: given the same trace and the same git diff, it always produces
the same output. There is no randomness, no sampling, and no network call in
the critical path. Signals are evaluated in a fixed order and the result is a
ranked list of flagged segments sorted by score descending.

LLMs are never in the critical path. They are an optional description layer
only. After the deterministic analyzer completes and assigns scores, an
explainer backend may add a short human-readable sentence describing why a
segment was flagged. The explainer never changes tier or score -- it only
annotates. If no explainer is configured, the null backend runs silently and
the report is complete without any API call.

```
Claude Code
    |
    | (hook events: PreToolUse, PostToolUse, Stop)
    v
barq-witness record --> .witness/trace.db (SQLite WAL)
                               |
                    (optional daemon.sock shortcut)
                               |
                    barq-witness report / tui / watch
                               |
                    +----------+-----------+
                    |                      |
               Analyzer               Explainer
          (deterministic,           (LLM, optional,
           always runs)              describe-only)
                    |
              Report / JSON / Markdown
                    |
          +---------+----------+
          |                    |
    Terminal output      GitHub Action
     (text/TUI)        (PR comment update)
```

## Data flow step by step

1. `barq-witness init` writes hook entries into `.claude/settings.json` and
   creates `.witness/trace.db` with the four-table schema.
2. The developer opens a Claude Code session. SessionStart fires and barq-witness
   records a new row in the `sessions` table.
3. When the developer submits a prompt, the UserPromptSubmit hook fires and
   barq-witness writes the prompt text and timestamp to `prompts`.
4. When Claude Code edits or writes a file, the PostToolUse hook fires and
   barq-witness writes the file path, diff, line range, and hashes to `edits`.
5. When Claude Code runs a Bash command, the PostToolUse hook fires and
   barq-witness writes the command text, classification, exit code, and
   duration to `executions`.
6. When the session ends, the Stop hook fires and barq-witness records the
   closing git HEAD SHA in `sessions`.
7. The developer runs `barq-witness report`. The analyzer reads the trace,
   cross-references it with the git diff, evaluates all signals, scores every
   changed segment, and renders the ranked attention map as text, markdown, or
   JSON.
