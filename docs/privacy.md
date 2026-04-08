# barq-witness Privacy Guide

## What is stored locally

barq-witness writes all data to `.witness/trace.db`, a local SQLite file that
is gitignored by default and never leaves your machine unless you explicitly
share it. The database has four tables:

- `sessions` -- one row per Claude Code session. Fields include the session ID,
  working directory, git HEAD SHA at session start and end, and the model name
  if reported by Claude Code.
- `prompts` -- one row per user prompt. Fields include the prompt text, a
  SHA-256 hash of that text, and the timestamp.
- `edits` -- one row per file modification (Edit, MultiEdit, Write). Fields
  include the file path, unified diff, SHA-256 hashes of content before and
  after the edit, and the line range of the change.
- `executions` -- one row per Bash command. Fields include the command text,
  its classification (test/run/git/install/other), exit code, and duration in
  milliseconds.

## Privacy mode

When `[privacy] mode = true` is set in `.witness/config.toml`, barq-witness
replaces sensitive content before writing it to the trace:

- Prompt content in `prompts.content` is replaced with its SHA-256 hash.
- Command text in `executions.command` is replaced with its SHA-256 hash.
- Diffs in `edits.diff` are replaced with a byte-length placeholder
  (e.g. `<diff: 1842 bytes>`).

The hash is one-way: you can verify that two entries share the same content
by comparing hashes, but you cannot recover the original text from the hash.
Privacy mode is useful in shared or audited environments where prompt and
command text must not be stored in plaintext.

## What leaves your machine

Nothing leaves your machine by default. Three opt-in features send data
externally:

1. **Explainer API calls** -- when an explainer backend such as `claude` or
   `groq` is configured, the prompt text for a flagged segment is sent to that
   API to generate a short description. No code or diff is sent unless the
   explainer backend is explicitly configured to include it.
2. **Sync to team aggregator** -- a future enterprise feature will allow
   syncing aggregate signal counts (no code, no prompts) to a team dashboard.
   This is disabled by default and requires explicit configuration.
3. **VS Code extension** -- the companion VS Code extension reads the local
   trace file and renders it in the editor. It makes no network calls.

## Air-gapped environments

barq-witness works fully offline. The deterministic analyzer requires no
outbound connections. For human-readable descriptions in air-gapped
environments, use the local Ollama explainer backend:

```toml
[explainer]
backend = "local"
model = "liquid/lfm2.5-1.2b"
```

Or the edge backend (Phase 19), which is optimized for low-memory devices:

```toml
[explainer]
backend = "edge"
model = "qwen2.5-coder:1.5b"
```

Both backends run entirely on your machine via Ollama and make no outbound
connections.

## Gitignoring the trace

`barq-witness init` writes `.witness/` to `.gitignore` automatically. The
trace database should never be committed to version control. It contains
plaintext prompt and command history that may include sensitive information,
and committing it would expose that history to anyone with repository access.

If you accidentally committed the trace, remove it with:

```bash
git rm --cached .witness/trace.db
echo '.witness/' >> .gitignore
git commit -m "remove trace database from version control"
```

Then rotate any secrets that may have appeared in prompts or commands.
