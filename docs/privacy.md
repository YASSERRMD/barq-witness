# Privacy and Data Handling

## What barq-witness records

barq-witness records the following data locally in `.witness/trace.db`:

| Data | Stored | Where |
|---|---|---|
| Session ID (opaque string from Claude Code) | Yes | `sessions.id` |
| Working directory | Yes | `sessions.cwd` |
| Git HEAD SHA at session start and end | Yes | `sessions.git_head_*` |
| Model name (if reported by Claude Code) | Yes | `sessions.model` |
| User prompt text | Yes | `prompts.content` |
| SHA-256 hash of prompt text | Yes | `prompts.content_hash` |
| File paths of edited files | Yes | `edits.file_path` |
| SHA-256 hash of file content before/after each edit | Yes | `edits.before_hash`, `edits.after_hash` |
| Line range of each edit (best-effort) | Yes | `edits.line_start`, `edits.line_end` |
| Unified diff of each edit | Yes | `edits.diff` |
| Shell command text | Yes | `executions.command` |
| Command classification (test/run/git/install/other) | Yes | `executions.classification` |
| Exit code and duration of each command | Yes | `executions.exit_code`, `executions.duration_ms` |

**What is NOT recorded:**

- File contents (only hashes are stored)
- Claude Code's responses or generated output (only the diff is stored)
- Environment variables
- Credentials or secrets (unless they appear in a prompt or command -- see below)

---

## Where your data lives

**Everything stays on your machine by default.**

The trace database is at `.witness/trace.db` inside your repository. It is
gitignored by `barq-witness init` and is never committed or pushed.

barq-witness has no network code. There is no telemetry, no analytics, no
home-phone-back. The binary makes zero outbound network calls during normal
operation.

You can verify this with a packet sniffer or by inspecting the source:

```bash
grep -r "http\|net\." internal/ cmd/
# Only go-git network code for reading local git objects -- no outbound calls.
```

---

## The GitHub Action

The GitHub Action (`YASSERRMD/barq-witness@main`) runs entirely within your
own GitHub Actions runner, inside your own CI pipeline. It:

1. Downloads the barq-witness binary from GitHub Releases (this is the only
   external download).
2. Reads `.witness/trace.db` from the PR head (which must be committed to the
   repo for the Action to see it -- see below).
3. Runs `barq-witness report` locally on the runner.
4. Posts the output as a PR comment using your `secrets.GITHUB_TOKEN`.

**Important**: for the GitHub Action to work, the trace database must be
present in the PR. This means either:

- Your CI must generate the trace as part of the build (not typical for this
  tool), or
- You commit `.witness/trace.db` to the repository (not recommended for
  privacy reasons), or
- You use the local `barq-witness report` workflow only and skip the Action.

The recommended pattern is to use `barq-witness report` locally before
committing and let the Action serve as a secondary reviewer.

The Action does not send your trace data to any third-party service. The
barq-witness project has no server infrastructure.

---

## Privacy mode

If you need to share a trace with a third party (security auditor, open-source
collaborator), use `--privacy` to omit the sensitive fields:

```bash
barq-witness export --privacy --out trace-redacted.json
```

In privacy mode:

- `prompts[].content` is **omitted**
- `executions[].command` is **omitted**
- All hashes, classifications, file paths, and timestamps are **retained**

The resulting file allows a reviewer to compute risk signals without seeing
the raw prompt text or shell commands.

---

## Credentials and secrets in prompts or commands

If a user types a secret into a Claude Code prompt (e.g. "here is my AWS key:
AKIA..."), that secret will appear in `prompts.content` in the trace. Similarly,
if Claude Code runs a command containing a secret (e.g. `curl -H "Authorization:
Bearer $TOKEN"`), the expanded command may appear in `executions.command`.

**Mitigations:**

1. Never type secrets into Claude Code prompts.
2. Use environment variables for secrets in shell commands (the env var name
   is recorded, not the value, as long as the shell expands it before the hook
   sees it).
3. Use `barq-witness export --privacy` when sharing traces.
4. The `.witness/trace.db` file is gitignored by default and should not be
   committed.

---

## Deleting the trace

To delete the trace for a repository:

```bash
rm -rf .witness/
```

This removes the database and the log file. The hooks in `.claude/settings.json`
will continue to try to write to the database but will create a new one on the
next session.

To fully remove barq-witness from a repository, also remove the hook entries
from `.claude/settings.json`.
