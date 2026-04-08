# barq-witness Configuration Reference

Configuration file location: `.witness/config.toml` in your project root.

## [explainer]

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| backend | string | "null" | Explainer backend: null, claude, groq, local |
| model | string | "" | Model name (backend-specific default if empty) |
| endpoint | string | "" | API endpoint override (local backend) |
| timeout_ms | int | 5000 | Request timeout in milliseconds |

## [analyzer]

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| security_paths_extra | []string | [] | Additional glob patterns for security-sensitive paths |
| exclude_paths | []string | [] | Glob patterns for paths to exclude from analysis |
| enable_intent_matching | bool | false | Enable PROMPT_DIFF_MISMATCH signal (requires non-null explainer) |
| intent_match_threshold | float | 0.5 | Score below which PROMPT_DIFF_MISMATCH fires |

## [privacy]

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| mode | bool | false | Redact prompt text and diffs from all outputs and exports |

## [sync]

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| enabled | bool | false | Enable syncing to a team aggregator server |
| server_url | string | "" | URL of the barq-witness-server instance |
| author_uuid | string | "" | Your anonymized UUID for team aggregation |

## [[plugins]]

Repeat this section for each plugin:

| Key | Type | Description |
|-----|------|-------------|
| name | string | Plugin display name |
| path | string | Absolute or PATH-relative path to the plugin executable |

### Example

```toml
[explainer]
backend = "claude"
timeout_ms = 10000

[analyzer]
enable_intent_matching = true
intent_match_threshold = 0.6
security_paths_extra = ["internal/payments/**"]

[privacy]
mode = false

[sync]
enabled = true
server_url = "https://witness.example.com"
author_uuid = "550e8400-e29b-41d4-a716-446655440000"

[[plugins]]
name = "no-prod-secrets"
path = "/usr/local/bin/barq-no-prod-secrets"
```
