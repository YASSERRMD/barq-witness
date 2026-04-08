.PHONY: test test-integration test-unit bench

test-unit:
	go test ./internal/... ./cmd/... -count=1 -timeout=120s

test-integration:
	go test ./tests/integration/... -count=1 -timeout=120s -v

test: test-unit test-integration

bench:
	go test ./tests/bench/... -bench=. -benchmem -count=3 -timeout=300s
