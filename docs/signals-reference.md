# Signals Reference

barq-witness evaluates nine deterministic signals against each AI-generated
code segment. Every signal is a pure function of the local trace and the git
diff -- no LLM, no heuristic weights, no probabilities.

## Tier 1 -- Critical (weight 100 each)

These signals indicate conditions that most commonly correlate with unreviewed
or untested generated code. A segment with any Tier 1 signal should receive
priority review attention.

### NO_EXEC

**Rule**: There is an edit row in the trace for this file, but no execution
row in the same session touched the same file path after the edit timestamp.

**Why it matters**: If Claude Code wrote or modified a file but nothing was
run before the commit, the author likely accepted the output without running
it. This is the most common pattern in comprehension-debt incidents.

**Example**: Claude writes `internal/auth/validator.go` but `go test ./...`
is never run in the session.

**Quiet when**: Any execution after the edit touches the file (by file path
in the `files_touched` column).

---

### FAST_ACCEPT_SECURITY

**Rule**: The edit is in a security-sensitive path AND the time between the
triggering prompt and the edit is less than 5 seconds.

**Why it matters**: Security-critical code (auth, crypto, payments, session
management) accepted in under 5 seconds was almost certainly not read before
committing.

**Security-sensitive paths** (matched case-insensitively):
`**/auth/**`, `**/oauth/**`, `**/login/**`, `**/session/**`, `**/token*`,
`**/jwt*`, `**/password*`, `**/secret*`, `**/credential*`, `**/crypto/**`,
`**/encrypt*`, `**/payment/**`, `**/billing/**`, `**/checkout/**`,
`**/wallet/**`, `**/admin/**`, `**/sudo/**`, `**/permission*`, `**/rbac/**`,
`**/.env*`, `**/config/secrets*`

**Example**: Claude rewrites `src/auth/jwt.go` and the developer accepts it
in 3 seconds.

**Quiet when**: The file is not in a security-sensitive path, or the
acceptance time is 5 seconds or more.

---

### TEST_FAIL_NO_RETEST

**Rule**: The session trace shows the sequence:
1. A test execution with a non-zero exit code (test failure)
2. A file edit after the failure (presumably fixing the failing code)
3. No subsequent test execution after the edit

**Why it matters**: The classic pattern of "tests fail, Claude regenerates
the code, developer commits without re-running tests."

**Example**: `go test ./...` exits 1, Claude rewrites the file, and no
further `go test` is run before the commit.

**Quiet when**: A test execution runs after the edit, or the test before
the edit passed (exit code 0), or there was no test execution in the session.

---

## Tier 2 -- Elevated (weight 50 each)

These signals indicate elevated risk that warrants attention but are not
on their own cause for concern.

### HIGH_REGEN

**Rule**: The same file was edited 4 or more times within a 10-minute sliding
window in the same session.

**Why it matters**: Frequent regeneration of the same file suggests Claude
Code was struggling to get the output right. After multiple iterations,
authors often stop reading each version carefully.

**Example**: `pkg/api/handler.go` is written and rewritten 5 times in 8
minutes as the author tries to get the output right.

**Quiet when**: The file was edited fewer than 4 times in any 10-minute
window.

---

### NEVER_REOPENED

**Rule**: The file was not accessed by any subsequent tool call in the
session after the edit -- neither another edit nor any execution that
touched the file.

**Why it matters**: If a file was generated and then never visited again
(by any tool), the author likely did not review it.

**Example**: Claude writes `internal/util/parser.go` and then the session
moves on to other files without ever touching `parser.go` again.

**Quiet when**: Any subsequent tool call (edit or execution) touches the
file after the generating edit.

---

### LARGE_MULTIFILE

**Rule**: The session that produced this edit touched more than 10 distinct
files.

**Why it matters**: When a single session produces edits across many files,
the author's attention is spread thin and each individual file receives less
scrutiny.

**Example**: A session generates a new feature spanning 12 files -- models,
controllers, migrations, tests, and config.

**Quiet when**: The session touched 10 or fewer distinct files.

---

## Tier 3 -- Informational (weight 20 each)

These signals provide useful context but are low severity on their own.

### NEW_DEPENDENCY

**Rule**: The edit modified a dependency manifest file.

**Dependency manifest files**: `package.json`, `go.mod`, `requirements.txt`,
`Cargo.toml`, `pyproject.toml`, `Gemfile`

**Why it matters**: AI assistants routinely suggest adding new packages.
Reviewers should check that any new dependency is intentional, well-maintained,
and not a supply-chain risk.

**Example**: Claude adds `github.com/some-package/v2` to `go.mod`.

---

### FAST_ACCEPT_GENERIC

**Rule**: The acceptance time (prompt to edit) is under 3 seconds, and the
file is not in a security-sensitive path (which would already be flagged by
FAST_ACCEPT_SECURITY at Tier 1).

**Why it matters**: Very fast acceptance on any file may indicate the author
is rubber-stamping output.

**Example**: Claude generates a 40-line struct and the author accepts in 2
seconds.

**Quiet when**: Acceptance time is 3 seconds or more, or the prompt
timestamp is not available.

---

### LONG_GENERATED_BLOCK

**Rule**: A single tool call (Edit, MultiEdit, or Write) added more than 100
lines to the file.

**Why it matters**: Very large single-call generations are more likely to
contain subtle errors because reading and validating 100+ lines of generated
code is cognitively demanding.

**Example**: Claude writes a 200-line database migration in a single Write
call.

**Quiet when**: The edit added 100 lines or fewer.

---

## Score and tier assignment

Each matched signal contributes its weight to the segment score:

```
score = sum of weights of all matched signals
```

The tier of a segment is the lowest tier number among its matched signals:
- If any Tier 1 signal fires, the segment is Tier 1.
- If any Tier 2 signal fires (and no Tier 1), the segment is Tier 2.
- Otherwise the segment is Tier 3.

Segments with score 0 are excluded from the report.
