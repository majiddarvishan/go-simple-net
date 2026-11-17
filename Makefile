GO ?= go

.PHONY: help check verify race phase8 phase9 perf-smoke soak

help:
	@echo "Available targets:"
	@echo "  make check       - gofmt, go vet and uncached unit tests"
	@echo "  make race        - uncached race detector"
	@echo "  make verify      - all correctness checks"
	@echo "  make perf-smoke  - short Phase 8 benchmark smoke test"
	@echo "  make phase8      - complete Phase 8 benchmark, soak and profiles"
	@echo "  make phase9      - coverage snapshot and paired Client/Server example smoke"
	@echo "  make soak        - five-minute mixed-payload memory soak"

check:
	@GO="$(GO)" bash scripts/ci-check.sh check

race:
	@GO="$(GO)" bash scripts/ci-check.sh race

verify:
	@GO="$(GO)" bash scripts/ci-check.sh verify

perf-smoke:
	@GO="$(GO)" bash scripts/phase8-benchmark.sh smoke

phase8:
	@GO="$(GO)" bash scripts/phase8-benchmark.sh full

phase9:
	@GO="$(GO)" ARTIFACT_DIR="$${ARTIFACT_DIR:-artifacts/phase9}" bash scripts/phase9-release-check.sh

soak:
	@GO="$(GO)" bash scripts/phase8-benchmark.sh soak
