#!/usr/bin/env bash
# Advisory CI-safe one-pass snapshot (not a ten-pair qualification).
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "$BASH_SOURCE")/.." && pwd)"
OUT="${ARTIFACT_DIR:-$ROOT/artifacts/qa005-advisory}"
mkdir -p "$OUT"
{
    echo "status=ADVISORY_ONLY"
    echo "source_sha=$(git -C "$ROOT" rev-parse HEAD)"
    echo "go_version=$(go version)"
    echo "os=$(uname -a)"
    echo "started=$(date -u +%FT%TZ)"
} > "$OUT/metadata.txt"
cd "$ROOT"
go test -run '^$' -bench '^BenchmarkQA005Qualification/Core/' \
    -benchtime=250ms -count=1 -benchmem -cpu=4 -timeout=25m \
    2>&1 | tee "$OUT/core-benchmark.log"
echo "No baseline comparison performed; gate disabled." | tee "$OUT/status.txt"
