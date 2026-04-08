.PHONY: test test-integration test-unit bench bench-compare

test-unit:
	go test ./internal/... ./cmd/... -count=1 -timeout=120s

test-integration:
	go test ./tests/integration/... -count=1 -timeout=120s -v

test: test-unit test-integration

bench:
	go test ./tests/bench/... -bench=. -benchmem -count=3 -timeout=300s

bench-compare:
	go test ./tests/bench/... -bench=. -benchmem -count=5 -timeout=300s > /tmp/new-bench.txt
	@which benchstat > /dev/null 2>&1 && benchstat $(BASE) /tmp/new-bench.txt || echo "install benchstat: go install golang.org/x/perf/cmd/benchstat@latest"
