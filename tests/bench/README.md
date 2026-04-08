# Benchmark Suite

This directory contains performance benchmarks for barq-witness.

## Running Benchmarks

```bash
# Run all benchmarks once
go test ./tests/bench/... -bench=. -benchmem -count=1 -timeout=300s

# Run with 5 iterations for statistical stability (used for baseline comparisons)
go test ./tests/bench/... -bench=. -benchmem -count=5 -timeout=300s

# Compare against baseline using benchstat
go test ./tests/bench/... -bench=. -benchmem -count=5 -timeout=300s > /tmp/new.txt
benchstat tests/bench/baseline.txt /tmp/new.txt
```

## Benchmarks

### analyzer_bench_test.go

| Benchmark | Description |
|-----------|-------------|
| BenchmarkAnalyzeSmall | Analyze 10 sessions x 100 edits (1000 total) against HEAD |
| BenchmarkAnalyzeMedium | Analyze 100 sessions x 100 edits (10000 total) against HEAD |

Both benchmarks use a temp dir with no real git repo so analyzer.Analyze falls
through the git diff path quickly. The bottleneck is the SQLite query layer.

### store_bench_test.go

| Benchmark | Description |
|-----------|-------------|
| BenchmarkStoreInsertEdit | 1000 edits inserted per b.N iteration |
| BenchmarkStoreQuerySessions | AllSessions on a 1000-session DB |
| BenchmarkStoreEditsForSession | EditsForSession on a session with 1000 edits |

### report_bench_test.go

| Benchmark | Description |
|-----------|-------------|
| BenchmarkReportSmall | Text + Markdown rendering of a 10-segment report |
| BenchmarkReportMedium | Text + Markdown rendering of a 100-segment report |

Report benchmarks use a synthetic analyzer.Report (no git, no store) so they
measure only the renderer hot path.

## Methodology

- All benchmarks use `b.TempDir()` so no cleanup is needed.
- No network calls are made in any benchmark.
- `b.ResetTimer()` is called after setup to exclude seeding time.
- The committed `baseline.txt` captures the initial performance profile of v1.1.1.
- Use `benchstat` to detect regressions: a p < 0.05 delta of >= 10% is actionable.

## Installing benchstat

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```
