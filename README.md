# barq-witness

**Local-first provenance recorder for Claude Code sessions.**

barq-witness hooks into Claude Code as a passive observer, capturing every
prompt, file edit, and command execution into a local SQLite trace. From that
trace and the git diff of a commit range, it produces a deterministic,
risk-weighted attention map that tells code reviewers exactly which
AI-generated segments deserve the most scrutiny -- and why.

**Status: pre-alpha.** Looking for early users and feedback.

---

## Why does it exist?

When developers use Claude Code heavily, the volume of AI-generated changes
outpaces their ability to read and understand each one. This is comprehension
debt: code that looks correct, passes CI, but was never truly understood by
a human before it shipped. Traditional code review provides no signal about
which hunks were generated, how quickly they were accepted, or whether they
were ever run.

barq-witness fills that gap by recording the full session provenance locally
and surfacing the highest-risk segments at review time.

---

## How is it different from CodeRabbit, Bugbot, or Macroscope?

barq-witness uses no LLM. Every flag is computed deterministically from the
local trace and the git diff -- fully auditable, zero false positives from
model hallucination, and zero cost per review.

---

## Quickstart

**1. Install**

```bash
curl -fsSL https://raw.githubusercontent.com/YASSERRMD/barq-witness/main/scripts/install.sh | sh
```

Or build from source:

```bash
git clone https://github.com/YASSERRMD/barq-witness.git
go build -o barq-witness ./cmd/barq-witness
```

**2. Initialise your repository**

```bash
cd your-repo
barq-witness init
git add .claude/settings.json
git commit -m "chore: add barq-witness hooks"
```

**3. Use Claude Code normally**

Hooks capture prompts, edits, and executions silently in the background.
No workflow change required.

**4. View the attention map**

```bash
# After making a commit:
barq-witness report

# Compare a range:
barq-witness report --from abc123 --to def456

# Output markdown for a PR description:
barq-witness report --format markdown --top 10

# Export the full trace as CGPF JSON:
barq-witness export --out trace.json
```

**5. Enable GitHub PR comments (optional)**

Add the action to your workflow:

```yaml
- uses: YASSERRMD/barq-witness@main
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

See [docs/getting-started.md](docs/getting-started.md) for the full setup
guide.

---

## What signals does it detect?

barq-witness evaluates nine deterministic signals across three tiers:

| Tier | Signal | Trigger |
|---|---|---|
| 1 | NO_EXEC | Edit never executed locally before commit |
| 1 | FAST_ACCEPT_SECURITY | Security path accepted in under 5 seconds |
| 1 | TEST_FAIL_NO_RETEST | Test failed, code regenerated, never re-tested |
| 2 | HIGH_REGEN | Same file edited 4+ times in 10 minutes |
| 2 | NEVER_REOPENED | File never accessed again after generation |
| 2 | LARGE_MULTIFILE | Session touched more than 10 distinct files |
| 3 | NEW_DEPENDENCY | Edit modified a dependency manifest |
| 3 | FAST_ACCEPT_GENERIC | Any file accepted in under 3 seconds |
| 3 | LONG_GENERATED_BLOCK | Single edit added more than 100 lines |

Full details: [docs/signals-reference.md](docs/signals-reference.md)

---

## Privacy

Everything is local. Nothing is sent anywhere. The trace lives in
`.witness/trace.db` (gitignored by default). The tool has no network code.
The GitHub Action runs in your own CI, not ours.

Use `barq-witness export --privacy` to redact prompt text and command strings
before sharing a trace.

Full details: [docs/privacy.md](docs/privacy.md)

---

## Architecture

```
Claude Code hooks --> .witness/trace.db --> barq-witness report
                                                  |
                                          git diff (go-git)
                                                  |
                                       deterministic risk scorer
                                                  |
                                          text / markdown output
```

Full details: [docs/how-it-works.md](docs/how-it-works.md)

---

## Trace export (CGPF)

barq-witness can export the trace as a structured JSON document conforming
to the Code Generation Provenance Format (CGPF) v0.1:

```bash
barq-witness export --out trace.json
barq-witness export --privacy --out trace-redacted.json
```

CGPF is an open format designed for interoperability between AI coding tools.
Spec: [docs/cgpf-spec.md](docs/cgpf-spec.md)

---

## License

MIT. See [LICENSE](LICENSE).

**Author**: Mohamed Yasser ([@yasserrmd](https://github.com/YASSERRMD))

Part of the Barq ecosystem: barq-wasm, barq-db, barq-mesh-web, BarqTrain,
barflow.
