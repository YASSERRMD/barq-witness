# Air-Gapped Deployment Checklist

barq-witness is designed to run without any network access. Follow this checklist to verify your deployment is fully air-gapped.

## Prerequisites

- [ ] Ollama installed and running at `http://localhost:11434` (or your configured endpoint)
- [ ] Edge model pulled: `ollama pull qwen2.5-coder:1.5b`
- [ ] barq-witness binary copied to the air-gapped machine

## Configuration

Set `.witness/config.toml`:

```toml
[explainer]
backend = "edge"
model = "qwen2.5-coder:1.5b"
timeout_ms = 5000

[privacy]
mode = true

[sync]
enabled = false
```

## Verification steps

Run each command and verify the expected output:

1. **Verify no DNS lookups occur:**
   ```
   barq-witness version
   ```
   Expected: prints version string, no network activity.

2. **Verify explainer is local-only:**
   ```
   barq-witness report --explainer edge --format text
   ```
   Expected: report generates using local Ollama, no external connections.

3. **Verify sync is disabled:**
   Check config: `sync.enabled = false`

4. **Verify privacy mode:**
   Check config: `[privacy] mode = true`
   Run `barq-witness report --format json` and verify `prompt_text` fields are hashed (64-char hex strings).

## Automated check

Run:
```
barq-witness check-airgap
```
This command verifies all four items above and prints PASS or FAIL for each.

## What requires network access (opt-in only)

- `backend = "claude"` -- requires ANTHROPIC_API_KEY and outbound HTTPS
- `backend = "groq"` -- requires GROQ_API_KEY and outbound HTTPS  
- `sync.enabled = true` -- sends aggregate counts to your configured server_url
