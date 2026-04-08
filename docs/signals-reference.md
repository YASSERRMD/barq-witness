# barq-witness Signals Reference

## Tier 1 -- Critical (weight 100)

| Code | Condition | Fires when |
|---|---|---|
| NO_EXEC | Edit in trace, no execution touched file before commit | A generated file was never run or tested before being committed |
| FAST_ACCEPT_SECURITY | Security-sensitive path accepted < 5s | A security-critical file was accepted too quickly to have been reviewed |
| TEST_FAIL_NO_RETEST | Test failed, code regenerated, tests never re-run | The author stopped after regeneration without verifying the fix |
| PROMPT_DIFF_MISMATCH | Intent match score < threshold | The committed diff does not match the original prompt intent |

## Tier 2 -- Elevated (weight 50)

| Code | Condition | Fires when |
|---|---|---|
| HIGH_REGEN | Same file edited 4+ times in 10 minutes | File was regenerated many times, author may have stopped reading carefully |
| NEVER_REOPENED | File not accessed after generation | File was generated and never opened again |
| LARGE_MULTIFILE | Session touched > 10 distinct files | High cognitive load session |
| FAST_ACCEPT_SECURITY_V2 | Security path accepted < 10s (relaxed threshold) | Phase 18 signal -- see changelog |
| COMMIT_WITHOUT_TEST | Edited test-adjacent file but no test execution in session | Code near tests was changed but no tests were run |

## Tier 3 -- Informational (weight 20)

| Code | Condition |
|---|---|
| NEW_DEPENDENCY | Dependency manifest modified |
| FAST_ACCEPT_GENERIC | Edit accepted < 3s |
| LONG_GENERATED_BLOCK | Single call added > 100 lines |

For plugin-contributed signals, codes use the `plugin:` prefix namespace.
