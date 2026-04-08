# Getting Started with barq-witness

## Install

**Option 1 -- curl installer (Linux / macOS)**

```bash
curl -fsSL https://raw.githubusercontent.com/YASSERRMD/barq-witness/main/scripts/install.sh | sh
```

**Option 2 -- build from source**

```bash
git clone https://github.com/YASSERRMD/barq-witness.git
cd barq-witness
go build -o barq-witness ./cmd/barq-witness
sudo mv barq-witness /usr/local/bin/
```

Verify the installation:

```bash
barq-witness version
# barq-witness v0.1.0
```

---

## Initialise your repository

Inside any git repository where you use Claude Code:

```bash
barq-witness init
```

This command:

1. Creates `.witness/` (added to `.gitignore` automatically).
2. Creates `.witness/trace.db` (the local SQLite trace database).
3. Merges four hook entries into `.claude/settings.json` so Claude Code
   calls `barq-witness record` on every relevant event.

Commit `.claude/settings.json` so your teammates also get the hooks:

```bash
git add .claude/settings.json
git commit -m "chore: add barq-witness hooks"
```

---

## Use Claude Code normally

From this point, just use Claude Code as you always do. barq-witness hooks
run as silent background observers and record:

- Every user prompt
- Every file edit (Edit, MultiEdit, Write tools)
- Every shell command (Bash tool)
- Session start and end

Nothing is sent anywhere. Everything stays in `.witness/trace.db`.

---

## View the attention map

After making commits, run:

```bash
barq-witness report
```

By default this compares HEAD~1..HEAD. Options:

```bash
# Compare a specific commit against its parent
barq-witness report --commit abc123

# Compare a range
barq-witness report --from abc123 --to def456

# Show top 5 segments in markdown
barq-witness report --format markdown --top 5
```

---

## Enable GitHub PR comments

Copy the example workflow into your repository:

```bash
cp .github/workflows/example-pr.yml /path/to/your/repo/.github/workflows/barq-witness.yml
```

Or add the action step manually to your existing PR workflow:

```yaml
- name: barq-witness attention map
  uses: YASSERRMD/barq-witness@main
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

See [docs/how-it-works.md](how-it-works.md) for the full architecture.
