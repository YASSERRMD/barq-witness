# barq-witness Threat Model

## What barq-witness protects against

barq-witness records AI-generated code edits and flags segments that may not have been reviewed carefully. It is a developer tool, not a security enforcement mechanism.

## Trust boundaries

- The trace database (.witness/trace.db) is local and trusted. It should not be committed to version control.
- The barq-witness binary reads and writes only within the project directory.
- Explainer backends (Claude, Groq, Local) send prompt text to external services. Privacy mode prevents this.
- The team aggregator receives anonymized session summaries. No prompt text or diffs are sent.

## What barq-witness does NOT protect against

- A malicious AI model that deliberately avoids generating detectable patterns
- Developers who intentionally bypass the hook system
- Post-commit code modification outside the hook lifecycle

## Plugin trust

Plugins are executed as child processes with the same privileges as barq-witness. Only install plugins from trusted sources. Review plugin source code before use.

## Privacy mode

When privacy mode is enabled (.witness/config.toml: [privacy] mode = true):
- Prompt text is redacted in all outputs
- Diffs are redacted in CGPF exports
- Explainer backends receive only metadata, not content
