# barq-witness

> Local-first AI code provenance recorder. Every edit your AI coding tool makes is recorded, scored, and surfaced -- no cloud required.

barq-witness hooks into Claude Code (and other AI coding tools) and writes a tamper-evident local trace of every prompt, edit, and test execution. At report time a deterministic engine scores each segment by risk tier, so you know exactly which AI-generated changes deserve a second look -- without sending a single byte to an LLM unless you explicitly opt in.

## Quick start

### Install (no Go required)

**macOS / Linux -- one line:**
```sh
curl -fsSL https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.sh | sh
```

**Windows -- PowerShell one line:**
```powershell
iwr -useb https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.ps1 | iex
```

**Install a specific version:**
```sh
# macOS / Linux
BARQ_VERSION=v1.1.1 curl -fsSL https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.sh | sh

# Windows PowerShell
$env:BARQ_VERSION="v1.1.1"; iwr -useb https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.ps1 | iex
```

**Install to a custom directory:**
```sh
# macOS / Linux (no sudo needed if the dir is yours)
BARQ_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.sh | sh
```

**Install via Go (if Go is available):**
```sh
go install github.com/yasserrmd/barq-witness/cmd/barq-witness@latest
```

### Wire it up

```sh
cd your-project
barq-witness init        # creates .witness/ and wires Claude Code hooks
barq-witness report      # run after making commits
```

The `.witness/` directory is gitignored by default.

Add the hooks to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse":  [{ "matcher": ".*", "hooks": [{ "type": "command", "command": "barq-witness record pre  --event-file $CLAUDE_HOOK_EVENT_FILE" }] }],
    "PostToolUse": [{ "matcher": ".*", "hooks": [{ "type": "command", "command": "barq-witness record post --event-file $CLAUDE_HOOK_EVENT_FILE" }] }],
    "Stop":        [{ "matcher": ".*", "hooks": [{ "type": "command", "command": "barq-witness record stop --event-file $CLAUDE_HOOK_EVENT_FILE" }] }]
  }
}
```

Then generate a report:

```bash
barq-witness report
```

## What it captures

- **Sessions** -- each Claude Code session is recorded as a discrete unit with start time, working directory, and exit status.
- **Prompts** -- every user prompt and its accompanying context are captured verbatim (or hashed in privacy mode).
- **Edits** -- every file write, patch, and delete that Claude Code performs is recorded alongside the diff and the prompt that triggered it.
- **Executions** -- every shell command or test run Claude Code invokes is recorded with its exit code and stdout/stderr summary.

## What it flags

| Signal code | Description |
|---|---|
| `NO_EXEC` (tier 1) | Generated code was never executed locally before the session ended |
| `FAST_ACCEPT_SECURITY` (tier 1) | A security-sensitive file path was accepted in under 5 seconds |
| `TEST_FAIL_NO_RETEST` (tier 1) | A test failed, code was regenerated, but tests were never re-run |
| `PROMPT_DIFF_MISMATCH` (tier 1/2) | The committed diff does not match the original prompt intent |
| `FAST_ACCEPT_SECURITY_V2` (tier 2) | A security-sensitive file path was accepted in 5-9 seconds |
| `COMMIT_WITHOUT_TEST` (tier 2) | A test-adjacent file was edited but no tests were run in the session |
| `HIGH_REGEN` (tier 2) | The same file was regenerated 4 or more times within 10 minutes |
| `NEVER_REOPENED` (tier 2) | A generated file was never accessed again after initial generation |
| `LARGE_MULTIFILE` (tier 2) | The session touched more than 10 distinct files |

For the full signal reference including tier 3 signals and plugin signals, see [docs/signals-reference.md](docs/signals-reference.md).

## Supported AI coding agents

- **Claude Code** -- full support (hooks-based capture)
- **Cursor** -- read-only via `barq-witness import cursor --log <path>`
- **Codex CLI** -- read-only via `barq-witness import codex --log <path>`
- **Aider** -- read-only via `barq-witness import aider --chat <path>`
- **Custom agents** -- via the plugin API and CGPF format

## Privacy posture

- Trace is stored locally in `.witness/trace.db`, gitignored by default.
- No network calls in the core engine. Optional explainer and sync features are clearly labeled and opt-in.
- Privacy mode hashes prompts and commands so even the local trace contains no source content.
- The team aggregator (if used) only ever receives aggregate counts, never code or prompts.
- Air-gapped deployments are supported; see [docs/air-gapped.md](docs/air-gapped.md) for the verified checklist.

See also: [docs/privacy.md](docs/privacy.md) and [docs/threat-model.md](docs/threat-model.md).

## LLM integration (optional)

The deterministic engine is the load-bearing wall. LLMs in barq-witness only describe and explain -- they never decide what is risky. The engine runs first, LLMs annotate after. This means a network outage, a rate-limit, or a deliberate choice to run fully offline never degrades the core signal quality.

Pluggable explainer backends: `null` (default, no network), `claude`, `groq`, `local` (Ollama, air-gap safe), `edge` (qwen2.5-coder:1.5b, optimized for constrained environments). One config line in `.witness/config.toml` swaps between them.

See [docs/explainer.md](docs/explainer.md) for configuration details.

## Architecture at a glance

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
          +---------+---------+
          |                   |
    Terminal output      GitHub Action
     (text/TUI)        (PR comment update)
```

## Components

| Component | What it is | Required? |
|---|---|---|
| `barq-witness` | Single Go binary, the core CLI | Yes |
| `.claude/settings.json` hooks | Capture layer for Claude Code | Yes (for Claude Code) |
| `.witness/trace.db` | Local SQLite trace store | Yes |
| `barq-witness daemon` | Optional long-running event receiver | No |
| `barq-witness tui` | Live interactive dashboard | No |
| `barq-witness mcp` | MCP server for AI assistant queries | No |
| VS Code extension | Inline gutter markers and side panel | No |
| `barq-witness-server` | Self-hosted team aggregator | No |
| Plugins | Custom signal extensions via stdin/stdout | No |

## Status

v1.1.x is stable and ready for production use. CGPF is frozen at v1.0, with no breaking changes planned. Phase numbers in commit history map directly to features; see [CHANGELOG.md](CHANGELOG.md) for the full history from phase 1 through phase 22. We are actively looking for early users in regulated industries and edge AI environments -- if that is you, open an issue or reach out directly.

## Roadmap

- Support for additional AI coding agents as they mature
- Optional encrypted trace storage for shared developer machines
- Browser-based session replay viewer
- Federated team aggregator with end-to-end encrypted sync

## Author and ecosystem

Mohamed Yasser, part of the Barq ecosystem. This project is the comprehension layer for any team using AI coding tools, intentionally designed to run on the same edge infrastructure the rest of the Barq ecosystem targets.

Related projects:

- [barq-wasm](https://github.com/yasserrmd/barq-wasm)
- [barq-db](https://github.com/yasserrmd/barq-db)
- [barq-mesh-web](https://github.com/yasserrmd/barq-mesh-web)
- [BarqTrain](https://github.com/yasserrmd/BarqTrain)
- [barflow](https://github.com/yasserrmd/barflow)

## License

MIT. See [LICENSE](LICENSE) file.
