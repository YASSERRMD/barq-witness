# barq-witness

barq-witness is a local-first provenance recorder for Claude Code sessions. It hooks into Claude Code as a passive observer, capturing every prompt, file edit, and command execution into a local SQLite trace stored at `.witness/trace.db`. From that trace and the git diff of a commit range, it computes a deterministic, risk-weighted attention map that tells code reviewers exactly which AI-generated segments deserve the most scrutiny and why -- no LLM, no network calls, no cloud dependency, every signal auditable from first principles.

**Status: pre-alpha**

Part of the [Barq ecosystem](https://github.com/yasserrmd).
