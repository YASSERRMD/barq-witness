# Code Generation Provenance Format (CGPF) v0.1

## Overview

CGPF (Code Generation Provenance Format) is an open, stable JSON format for
recording the provenance of AI-generated code changes. It answers three
questions that code reviewers need to know:

1. **What** did the AI generate? (edits, with before/after hashes)
2. **Why** did it generate it? (the prompt that triggered each edit)
3. **What happened next?** (executions run after the edit, exit codes)

CGPF is designed to be:

- **Tool-agnostic**: any AI coding assistant that can write hook scripts can
  produce a CGPF trace.
- **Deterministic**: every field is derived from observable facts, not model
  output. No inferred or probabilistic fields.
- **Privacy-respecting**: the `--privacy` flag omits prompt text and command
  strings, retaining only hashes and classifications.
- **Stable**: the `cgpf_version` field lets consumers detect schema changes.
  A minor version bump is backwards-compatible; a major version bump is not.

---

## Document Structure

A CGPF document is a single UTF-8 JSON object.

```json
{
  "cgpf_version": "0.1",
  "generated_by": "barq-witness vX.Y.Z",
  "generated_at": "2026-04-08T12:00:00Z",
  "repo": {
    "remote": "https://github.com/owner/repo",
    "commit_range": {
      "from": "abc123",
      "to":   "def456"
    }
  },
  "sessions": [ ... ]
}
```

### Top-level fields

| Field | Type | Description |
|---|---|---|
| `cgpf_version` | string | Spec version, currently `"0.1"` |
| `generated_by` | string | Tool name and version that produced the document |
| `generated_at` | ISO-8601 UTC | Document generation timestamp |
| `repo.remote` | string or null | Git remote URL (`origin`), null if not detectable |
| `repo.commit_range.from` | SHA string | Start of the commit range (may be empty) |
| `repo.commit_range.to` | SHA string | End of the commit range (may be empty) |
| `sessions` | array of Session | All exported sessions, ordered by `started_at` |

---

## Session Object

```json
{
  "id": "sess-abc123",
  "started_at": "2026-04-08T10:00:00Z",
  "ended_at": "2026-04-08T10:45:00Z",
  "model": "claude-opus-4-5",
  "cwd": "/home/user/myrepo",
  "git_head_start": "aaabbb111",
  "git_head_end": "cccddd222",
  "prompts": [ ... ],
  "edits": [ ... ],
  "executions": [ ... ]
}
```

| Field | Type | Description |
|---|---|---|
| `id` | string | Opaque session identifier assigned by the AI tool |
| `started_at` | ISO-8601 UTC | When the session started |
| `ended_at` | ISO-8601 UTC or null | When the session ended (null if not yet ended) |
| `model` | string | AI model used (may be empty if not reported by the tool) |
| `cwd` | string | Working directory at session start |
| `git_head_start` | SHA string | Git HEAD at session start |
| `git_head_end` | SHA string or null | Git HEAD at session end |
| `prompts` | array of Prompt | All prompts in this session, ordered by timestamp |
| `edits` | array of Edit | All file edits in this session, ordered by timestamp |
| `executions` | array of Execution | All shell executions in this session, ordered by timestamp |

---

## Prompt Object

```json
{
  "id": 1,
  "timestamp": "2026-04-08T10:05:00Z",
  "content_hash": "sha256hex",
  "content": "write an HTTP handler for /health"
}
```

| Field | Type | Description |
|---|---|---|
| `id` | integer | Auto-incremented identifier, unique within the document |
| `timestamp` | ISO-8601 UTC | When the prompt was submitted |
| `content_hash` | SHA-256 hex | Hash of the prompt text (always present) |
| `content` | string or omitted | Raw prompt text; omitted when `--privacy` is used |

---

## Edit Object

```json
{
  "id": 1,
  "prompt_id": 1,
  "timestamp": "2026-04-08T10:05:03Z",
  "file_path": "cmd/server/handler.go",
  "tool": "Write",
  "before_hash": "sha256hex",
  "after_hash": "sha256hex",
  "line_start": 1,
  "line_end": 42
}
```

| Field | Type | Description |
|---|---|---|
| `id` | integer | Auto-incremented identifier |
| `prompt_id` | integer or null | Links to the prompt that triggered this edit |
| `timestamp` | ISO-8601 UTC | When the edit was applied |
| `file_path` | string | Repository-relative file path |
| `tool` | string | `Edit`, `MultiEdit`, or `Write` |
| `before_hash` | SHA-256 hex | Hash of the content before the edit |
| `after_hash` | SHA-256 hex | Hash of the content after the edit |
| `line_start` | integer or null | First line affected (1-based, best-effort) |
| `line_end` | integer or null | Last line affected (1-based, best-effort) |

---

## Execution Object

```json
{
  "id": 1,
  "timestamp": "2026-04-08T10:06:00Z",
  "command": "go test ./cmd/server/...",
  "classification": "test",
  "files_touched": ["cmd/server/handler.go"],
  "exit_code": 0,
  "duration_ms": 412
}
```

| Field | Type | Description |
|---|---|---|
| `id` | integer | Auto-incremented identifier |
| `timestamp` | ISO-8601 UTC | When the command was run |
| `command` | string or omitted | Raw shell command; omitted with `--privacy` |
| `classification` | string | One of: `test`, `run`, `git`, `install`, `other` |
| `files_touched` | array of strings | Best-effort list of file paths referenced in the command |
| `exit_code` | integer or null | Process exit code (null if not reported) |
| `duration_ms` | integer or null | Execution duration in milliseconds (null if not reported) |

### Classification rules

| Classification | Trigger |
|---|---|
| `test` | Contains `go test`, `pytest`, `npm test`, `yarn test`, `cargo test`, `jest`, `vitest`, `mocha`, `rspec` |
| `git` | Starts with `git ` |
| `install` | Contains `go get`, `pip install`, `npm install`, `yarn add`, `cargo add`, `apt `, `brew install` |
| `run` | Contains `go run`, `python `, `node `, `./`, `cargo run`, `npm start`, `yarn dev` |
| `other` | Anything not matched above |

---

## Privacy Mode

When a CGPF document is produced with `--privacy`:

- `prompts[].content` is **omitted** (the field does not appear in the JSON)
- `executions[].command` is **omitted**
- All hashes, classifications, timestamps, and file paths are **retained**

This allows a CGPF document to be shared with third parties (e.g. security
auditors, open-source collaborators) without exposing the raw prompt text or
shell commands that might contain secrets.

---

## Versioning Policy

- **Patch (0.1.x)**: bug fixes to the producer only; schema is unchanged.
- **Minor (0.x)**: additive changes only (new optional fields). Consumers
  must ignore unknown fields. `cgpf_version` string stays `"0.x"` where x
  is the latest minor.
- **Major (x.0)**: breaking changes. A new `cgpf_version` value is used.

Consumers should reject documents whose major version is higher than what
they support.

---

## Worked Example

Below is a complete minimal CGPF v0.1 document produced by barq-witness for
a session in which one prompt triggered one file write and one test run.

```json
{
  "cgpf_version": "0.1",
  "generated_by": "barq-witness v0.1.0",
  "generated_at": "2026-04-08T12:00:00Z",
  "repo": {
    "remote": "https://github.com/acme/myservice",
    "commit_range": {
      "from": "a1b2c3d4",
      "to":   "e5f6a7b8"
    }
  },
  "sessions": [
    {
      "id": "sess-20260408-xyz",
      "started_at": "2026-04-08T10:00:00Z",
      "ended_at":   "2026-04-08T10:47:00Z",
      "model": "claude-opus-4-5",
      "cwd": "/home/alice/myservice",
      "git_head_start": "a1b2c3d4",
      "git_head_end":   "e5f6a7b8",
      "prompts": [
        {
          "id": 1,
          "timestamp":    "2026-04-08T10:05:00Z",
          "content_hash": "7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069",
          "content":      "write an HTTP handler for /health that returns 200 OK"
        }
      ],
      "edits": [
        {
          "id":          1,
          "prompt_id":   1,
          "timestamp":   "2026-04-08T10:05:03Z",
          "file_path":   "cmd/server/health.go",
          "tool":        "Write",
          "before_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "after_hash":  "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
          "line_start":  1,
          "line_end":    18
        }
      ],
      "executions": [
        {
          "id":             1,
          "timestamp":      "2026-04-08T10:06:15Z",
          "command":        "go test ./cmd/server/...",
          "classification": "test",
          "files_touched":  ["cmd/server/health.go"],
          "exit_code":      0,
          "duration_ms":    312
        }
      ]
    }
  ]
}
```

---

## Implementations

| Tool | Language | Status |
|---|---|---|
| [barq-witness](https://github.com/yasserrmd/barq-witness) | Go | Reference implementation |

To add your tool: implement the hook scripts that write to a SQLite trace,
then call `barq-witness export` to produce a CGPF document. Or implement
the format directly and open a PR to the spec repo.
