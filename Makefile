GO ?= go

.PHONY: help check verify race benchmark smoke perf-smoke soak

help:
	@echo "Available targets:"
	@echo "  make check       - gofmt, go vet and uncached unit tests"
	@echo "  make race        - uncached race detector"
	@echo "  make verify      - all correctness checks"
	@echo "  make perf-smoke  - short benchmark smoke test"
	@echo "  make benchmark      - complete benchmark, soak and profiles"
	@echo "  make smoke      - coverage snapshot and paired Client/Server example smoke"
	@echo "  make soak        - five-minute mixed-payload memory soak"

check:
	@GO="$(GO)" bash scripts/ci-check.sh check

race:
	@GO="$(GO)" bash scripts/ci-check.sh race

verify:
	@GO="$(GO)" bash scripts/ci-check.sh verify

perf-smoke:
	@GO="$(GO)" bash scripts/performance.sh smoke

benchmark:
	@GO="$(GO)" bash scripts/performance.sh full

smoke:
	@GO="$(GO)" ARTIFACT_DIR="$${ARTIFACT_DIR:-artifacts/smoke}" bash scripts/example-smoke.sh

soak:
	@GO="$(GO)" bash scripts/performance.sh soak
