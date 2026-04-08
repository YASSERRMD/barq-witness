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

---

## Phase B -- Resolution log

**Performed: 2026-04-08**

### Scope

Phase B goal was to get the test suite to a clean green state by fixing any `go vet` issues and data races.

### Steps taken

| Step | Command | Result |
|------|---------|--------|
| 1 | `go test ./... -count=1` | 141 passed, 0 failed, 6 skipped -- already clean |
| 2 | `go vet ./...` | No issues found -- already clean |
| 3 | `go test -race ./... -count=1 -timeout=120s` | No data races detected -- already clean |
| 4 | `go test ./... -count=1 -timeout=120s` (final confirm) | 141 passed, 0 failed, 6 skipped |

### Fixes applied

None required. The suite entered Phase B in a fully clean state:
- `go vet` reported zero issues across all packages.
- The race detector found zero data races in all packages including `internal/daemon` (goroutine-based) and `internal/store` (SQLite).
- All 141 tests passed; the 6 skips are environment-gated (Ollama / API key) and were already documented in Phase A.

The test count increased from 87 (Phase A) to 141 (Phase B) because Phase A counted top-level test functions only, while Phase B counts every sub-test expanded by `go test -v`, reflecting table-driven subtests across packages such as `plugins/no-prod-secrets`.

### Baseline commit

`f7c97d9` -- test: Phase A audit -- test inventory and coverage report

### Final test count (Phase B)

**141 passed, 0 failed, 6 skipped**

---

## Phase C -- Coverage backfill

**Performed: 2026-04-08**

### Scope

Phase C goal was to bring every `internal/` package above 70% line coverage by adding happy-path and error-path tests for previously untested or under-covered packages.

### New test files added

| File | Packages covered |
|------|-----------------|
| `internal/testutil/testutil.go` + `testutil_test.go` | Shared test helpers (NewFixtureStore, NewFixtureRepo, NewFixtureSession) |
| `internal/model/model_test.go` | Session, Prompt, Edit, Execution zero-value and pointer-field roundtrips |
| `internal/util/util_test.go` | HeadSHA, SHA256Hex, SHA256HexString, OpenLogger |
| `internal/diff/diff_test.go` | ChangedFiles, ChangedFilePaths, FileChange fields, initial-commit IsNew |
| `internal/store/store_extra_test.go` | AllSessions, PromptsForSession, PromptByID, EditsForSession, EditByID, GetStats, RecentSessions |
| `internal/explainer/explainer_extra_test.go` | LocalExplainer HTTP paths, reason codes, privacy mode, GroqExplainer Available |
| `internal/explainer/explainer_http_test.go` | EdgeExplainer Explain/IntentMatch/CacheHit, ClaudeExplainer Name/Available/Close, EnrichSegments |
| `internal/explainer/internal_test.go` | ClaudeExplainer.Explain, GroqExplainer.Explain, lruCache, buildExplainPrompt, buildIntentPrompt, parseIntentJSON, extractText helpers |
| `internal/mcp/mcp_extra_test.go` | barq_get_segment, barq_list_sessions, barq_get_stats, unknown method, malformed JSON |
| `internal/daemon/daemon_extra_test.go` | handlePrompt, handleExecution, unknown op, invalid JSON, session start/end, edit with prompt linkage, IsDaemonRunning |
| `internal/server/server_extra_test.go` | handleIngest (all error paths), handleStats, handleDashboard, Stop nil-server, queryStats, tier aggregation |
| `internal/watcher/watcher_extra_test.go` | markdown format, real git repo poll, text format, zero topN |
| `plugins/no-prod-secrets/secrets_extra_test.go` | ScanForSecrets multiline, boundary, variants, both patterns, diff context |
| `plugins/license-header-check/license_extra_test.go` | CheckMissingLicenseHeader all edge cases (SPDX, copyright, empty, subdirectory, test files) |
| `internal/adapters/codex/codex_extra_test.go` | New, Source, RecordSession/Edit/Execution/Prompt NoOp, ImportFromLog all paths |
| `internal/cgpf/cgpf_extra_test.go` | Privacy mode, non-privacy, repo path detection, specific session ID, round-trip marshal, FilesTouched |

### Coverage by package (before -> after)

| Package | Phase A | Phase C |
|---------|---------|---------|
| internal/adapters/aider | 83.2% | 83.2% |
| internal/adapters/claudecode | 82.1% | 82.1% |
| internal/adapters/codex | 63.3% | 93.9% |
| internal/adapters/cursor | 75.5% | 75.5% |
| internal/analyzer | 88.1% | 88.1% |
| internal/cgpf | 62.8% | 74.4% |
| internal/config | 81.2% | 81.2% |
| internal/daemon | 61.6% | 90.1% |
| internal/diff | 0% (no tests) | 79.6% |
| internal/explainer | 37.1% | 89.7% |
| internal/hooks | 87.5% | 87.5% |
| internal/installer | 80.3% | 80.3% |
| internal/mcp | 42.6% | 75.0% |
| internal/model | 0% (no tests) | 100.0% |
| internal/plugin | 91.7% | 91.7% |
| internal/render | 81.9% | 81.9% |
| internal/server | 57.1% | 74.8% |
| internal/store | 54.8% | 80.5% |
| internal/testutil | 0% (no tests) | 81.0% |
| internal/util | 0% (no tests) | 100.0% |
| internal/watcher | 59.1% | 81.8% |
| plugins/license-header-check | 23.5% | 71.2% |
| plugins/no-prod-secrets | 3.7% | 72.4% |
| **Total** | **63.2%** | **81.9%** |

### Packages below 70% (remaining)

| Package | Coverage | Note |
|---------|----------|------|
| plugins/license-header-check | 71.2% | `main()` and `writeResponse()` require process-level integration testing; not feasible in unit tests |
| plugins/no-prod-secrets | 72.4% | Same -- `main()` reads from os.Stdin and is untestable without a subprocess harness |

All `internal/` packages are now at or above 70% line coverage.

### Test counts (Phase C)

**All tests pass. 0 failures. 6 environment-gated skips (unchanged from Phase B).**
