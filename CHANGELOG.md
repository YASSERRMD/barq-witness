# Changelog

All notable changes to barq-witness are documented here.

## [1.1.0] - 2026-04-08

### Added
- Phase 17: Core documentation (docs/how-it-works.md, docs/signals-reference.md, docs/privacy.md, docs/explainer.md)
- Phase 18: Two new risk signals -- FAST_ACCEPT_SECURITY_V2 (tier 2) and COMMIT_WITHOUT_TEST (tier 2)
- Phase 19: Edge LLM backend for air-gapped environments (qwen2.5-coder:1.5b via Ollama), `check-airgap` subcommand, docs/air-gapped.md
- Phase 20: Read-only import adapters for Cursor (`import cursor`), Codex CLI (`import codex`), and Aider (`import aider`)
- Phase 21: Native MCP server (`barq-witness mcp`) exposing trace query tools to AI assistants
- Phase 22: Comprehensive README rewrite, v1.1.0 release

### Changed
- CGPF remains frozen at v1.0 (no breaking changes)

## [1.0.0] - 2026-04-08

### Added
- Phase 1: Core hook integration with Claude Code (session, prompt, edit, execution recording)
- Phase 2: Deterministic risk analyzer with 9 signals across 3 tiers
- Phase 3: Text and Markdown renderers with ANSI color support
- Phase 4: CGPF (Code Generation Provenance Format) v1.0 export
- Phase 5: GitHub Action composite wrapper for PR comment posting
- Phase 6: Initial test suite and CI workflow
- Phase 7: Config file support (.witness/config.toml)
- Phase 8: Pluggable explainer backends (Null, Claude, Groq, Local/Ollama)
- Phase 9: Intent matching tier-2 signal (PROMPT_DIFF_MISMATCH), migration system
- Phase 10: Daemon mode with Unix socket, fallback-transparent record subcommands
- Phase 11: Watch mode and TUI dashboard with bubbletea/lipgloss
- Phase 12: Multi-agent adapter layer (Claude Code, Cursor, Codex, Aider stubs), source field
- Phase 13: Self-hosted team aggregator server with HTML dashboard and sync subcommand
- Phase 14: Plugin system with stdin/stdout NDJSON protocol, no-prod-secrets and license-header-check reference plugins
- Phase 15: VS Code extension with gutter decorations, hover tooltips, side panel, JSON report format
- Phase 16: v1.0 release polish -- benchmarks, release build matrix, Homebrew tap

### Core Principles
- No LLM in the core engine; LLMs are optional and only describe/explain, never decide
- Everything runs locally by default; network calls are opt-in
- Single static Go binary, no cgo
- CGPF trace format is a public contract
- Privacy mode works for every feature
