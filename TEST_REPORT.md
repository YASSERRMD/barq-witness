# TEST_REPORT.md -- Phase A Audit

Generated: 2026-04-08

---

## Initial run

All tests ran successfully with no failures or panics.

- **Total tests: 87 passed, 0 failed, 6 skipped**
- **Panics: 0**

### Failures

None.

### Skips

| Test | Package | Reason |
|------|---------|--------|
| TestEdgeExplainer_LiveExplain | internal/explainer | Ollama not running or edge model not present at localhost:11434 |
| TestClaudeExplainer_LiveExplain | internal/explainer | ANTHROPIC_API_KEY not set |
| TestClaudeExplainer_CacheHit | internal/explainer | ANTHROPIC_API_KEY not set |
| TestGroqExplainer_LiveExplain | internal/explainer | GROQ_API_KEY not set |
| TestLocalExplainer_SkippedWhenOllamaNotRunning | internal/explainer | Ollama is running; unavailability test inverted (Ollama was actually up at test time) |
| TestLocalExplainer_LiveExplain | internal/explainer | Ollama model 'qwen2.5-coder:1.5b' not found (HTTP 404 from local API) |

---

## Coverage summary (by package)

| Package | Coverage % |
|---------|------------|
| github.com/yasserrmd/barq-witness/internal/adapters/aider | 83.2% |
| github.com/yasserrmd/barq-witness/internal/adapters/claudecode | 82.1% |
| github.com/yasserrmd/barq-witness/internal/adapters/codex | 63.3% |
| github.com/yasserrmd/barq-witness/internal/adapters/cursor | 75.5% |
| github.com/yasserrmd/barq-witness/internal/analyzer | 88.1% |
| github.com/yasserrmd/barq-witness/internal/cgpf | 62.8% |
| github.com/yasserrmd/barq-witness/internal/config | 81.2% |
| github.com/yasserrmd/barq-witness/internal/daemon | 61.6% |
| github.com/yasserrmd/barq-witness/internal/explainer | 37.1% |
| github.com/yasserrmd/barq-witness/internal/hooks | 87.5% |
| github.com/yasserrmd/barq-witness/internal/installer | 80.3% |
| github.com/yasserrmd/barq-witness/internal/mcp | 42.6% |
| github.com/yasserrmd/barq-witness/internal/plugin | 91.7% |
| github.com/yasserrmd/barq-witness/internal/render | 81.9% |
| github.com/yasserrmd/barq-witness/internal/server | 57.1% |
| github.com/yasserrmd/barq-witness/internal/store | 54.8% |
| github.com/yasserrmd/barq-witness/internal/watcher | 59.1% |
| github.com/yasserrmd/barq-witness/plugins/license-header-check | 23.5% |
| github.com/yasserrmd/barq-witness/plugins/no-prod-secrets | 3.7% |
| **Total** | **63.2%** |

### No test files (excluded from coverage)

| Package | Note |
|---------|------|
| github.com/yasserrmd/barq-witness/cmd/barq-witness | no test files |
| github.com/yasserrmd/barq-witness/cmd/barq-witness-server | no test files |
| github.com/yasserrmd/barq-witness/internal/adapters | no test files (interface-only package) |
| github.com/yasserrmd/barq-witness/internal/diff | no test files |
| github.com/yasserrmd/barq-witness/internal/model | no test files |
| github.com/yasserrmd/barq-witness/internal/tui | no test files |
| github.com/yasserrmd/barq-witness/internal/util | no test files |

---

## Coverage gaps (below 70%)

The following packages fall below the 70% threshold:

| Package | Coverage % | Key uncovered functions |
|---------|------------|------------------------|
| plugins/no-prod-secrets | 3.7% | main(), writeResponse() -- only ScanForSecrets() is covered |
| plugins/license-header-check | 23.5% | main(), writeResponse() -- only CheckMissingLicenseHeader() is covered |
| internal/explainer | 37.1% | claude.Explain, claude.IntentMatch, claude.callMessages, groq.Explain, groq.IntentMatch, groq.callChat, edge.Explain, edge.IntentMatch, local.IntentMatch, lru.Set, null.nullTemplate (nearly all gated on API keys / Ollama) |
| internal/mcp | 42.6% | New(), toolGetSegment(), toolListSessions(), and partial toolGetReport/toolGetStats coverage |
| internal/store | 54.8% | AllSessions(), PromptsForSession(), PromptByID(), EditsForSession(), mcp_queries (EditByID, GetStats, RecentSessions all at 0%) |
| internal/server | 57.1% | Start(), Stop(), partial handleIngest, handleStats, handleDashboard |
| internal/watcher | 59.1% | poll() at 41.7% |
| internal/daemon | 61.6% | handlePrompt() 0%, handleExecution() 0%, IsDaemonRunning() 0%, jsonInt64Ptr() 0% |
| internal/cgpf | 62.8% | detectRemote() 15.4%, nullString() 0%, trimSHA() 0%, parseFilesTouched partial |
| internal/adapters/codex | 63.3% | New, Source, RecordSession, RecordEdit, RecordExecution, RecordPrompt all 0% (only ImportFromLog tested) |

---

## Untested packages

Packages inside `internal/` that have no test file at all:

| Package | Files |
|---------|-------|
| internal/adapters | adapter.go (interface definitions only) |
| internal/diff | gitdiff.go |
| internal/model | events.go |
| internal/tui | tui.go |
| internal/util | git.go, hash.go, log.go |

---

## CLI commands without tests

All files under `cmd/barq-witness/` and `cmd/barq-witness-server/` have zero test coverage. The entire CLI surface is untested:

| File | Description |
|------|-------------|
| cmd/barq-witness/main.go | Root cobra command setup |
| cmd/barq-witness/airgap.go | airgap subcommand |
| cmd/barq-witness/daemon.go | daemon start/stop subcommand |
| cmd/barq-witness/export.go | export subcommand |
| cmd/barq-witness/import.go | import subcommand |
| cmd/barq-witness/init.go | init subcommand |
| cmd/barq-witness/mcp_cmd.go | mcp subcommand |
| cmd/barq-witness/record.go | record subcommand |
| cmd/barq-witness/report.go | report subcommand |
| cmd/barq-witness/sync.go | sync subcommand |
| cmd/barq-witness/tui_cmd.go | tui subcommand |
| cmd/barq-witness/watch.go | watch subcommand |
| cmd/barq-witness-server/main.go | standalone server binary |

---

## Decayed phase tests

barq-witness documents 22 development phases. Cross-referencing test files against phase functionality:

| Phase | Feature area | Test status |
|-------|-------------|-------------|
| 1 | Core model / events | No test file (internal/model) |
| 2 | SQLite store | Tests present (store_test.go, migrate_test.go) |
| 3 | Daemon IPC | Tests present (daemon_test.go) -- handlePrompt and handleExecution uncovered |
| 4 | Hook parser | Tests present (hooks/parser_test.go) |
| 5 | Installer / settings.json | Tests present (installer/settings_test.go) |
| 6 | Analyzer core | Tests present (analyzer_test.go) |
| 7 | Signals | Tests present (signals_test.go) |
| 8 | Render (text + markdown) | Tests present (render_test.go) |
| 9 | CGPF export | Tests present (cgpf_test.go) -- detectRemote, nullString, trimSHA uncovered |
| 10 | Config | Tests present (config_test.go) |
| 11 | Explainer (null) | Tests present (explainer_test.go) |
| 12 | Explainer (Claude / Groq / local / edge) | Tests present but all live paths skipped due to missing API keys / model |
| 13 | Plugin system | Tests present (plugin_test.go) |
| 14 | MCP server | Tests present (mcp/server_test.go) -- toolGetSegment, toolListSessions at 0% |
| 15 | HTTP server | Tests present (server_test.go) -- Start/Stop never called |
| 16 | Watcher | Tests present (watcher_test.go) -- poll() poorly covered |
| 17 | diff package | No test file (internal/diff) |
| 18 | util package | No test file (internal/util) |
| 19 | TUI | No test file (internal/tui) |
| 20 | adapters/aider | Tests present (aider/adapter_test.go) -- adapter lifecycle (New/RecordSession/etc.) at 0% |
| 21 | adapters/codex, cursor | Tests present for ImportFromLog only; adapter lifecycle functions all at 0% |
| 22 | adapters/claudecode | Tests present (claudecode/adapter_test.go) -- RecordPrompt at 0% |

---

## go vet

`go vet ./...` produced no output (clean).

## go build

`go build ./...` produced no output (clean).

---

## Coverage output note

`coverage.out` is stored at the repository root. It is excluded from version control via .gitignore.
