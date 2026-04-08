# barq-witness Explainer Backends

The deterministic analyzer runs first and is authoritative. It evaluates all
signals, computes scores, and assigns tiers before any explainer is involved.
The explainer layer adds human-readable descriptions to segments after scoring.
Explainers never change tier or score -- they only annotate existing results
with a short natural-language sentence describing why a segment was flagged.

## Backends

| Backend | Key required | Default model | Air-gapped? | Config value |
|---|---|---|---|---|
| null | No | n/a | Yes | `backend = "null"` |
| claude | ANTHROPIC_API_KEY | claude-sonnet-4-6 | No | `backend = "claude"` |
| groq | GROQ_API_KEY | llama-3.3-70b-versatile | No | `backend = "groq"` |
| local | No (Ollama required) | liquid/lfm2.5-1.2b | Yes | `backend = "local"` |
| edge | No (Ollama required) | qwen2.5-coder:1.5b | Yes | `backend = "edge"` |

## Switching backends

Set the backend in `.witness/config.toml`:

```toml
[explainer]
backend = "local"
model = "liquid/lfm2.5-1.2b"
timeout_ms = 8000
```

The `model` field overrides the backend default. The `timeout_ms` field sets
the maximum time barq-witness will wait for an explainer response before
falling back to no description. If the explainer times out, the report is
still complete -- it just omits the optional description for that segment.

## Privacy and explainers

When `[privacy] mode = true`, prompt text is redacted before being sent to any
explainer backend. The explainer receives only the signal codes and file path
for a flagged segment, not the original prompt content or diff. This applies
to all backends including cloud backends.

The null backend always respects privacy because it never sends data anywhere.
It is the default backend when no explainer configuration is present.

## Cache

Explainer results are cached by `(edit_id, model)` in the `intent_matches`
table of the trace database. If you run `barq-witness report` multiple times
on the same commits, the explainer is only called once per (edit, model) pair.
Subsequent report runs reuse cached results without hitting the API, making
repeated runs fast and free.

To clear the explainer cache and force re-generation:

```bash
barq-witness cache clear --explainer
```
