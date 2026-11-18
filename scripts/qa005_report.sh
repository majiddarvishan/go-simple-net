#!/usr/bin/env bash
# Advisory CI-safe one-pass snapshot (not a ten-pair qualification).
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "$BASH_SOURCE")/.." && pwd)"
MODE="${1:-core}"
SELECTOR='^BenchmarkQA005Qualification/Core/'
if [[ "$MODE" == full ]]; then
    SELECTOR='^BenchmarkQA005Qualification/'
elif [[ "$MODE" != core ]]; then
    echo "Usage: $0 [core|full]" >&2
    exit 2
fi
OUT="${ARTIFACT_DIR:-$ROOT/artifacts/qa005-advisory}"
mkdir -p "$OUT"
{
    echo "status=ADVISORY_ONLY"
    echo "mode=$MODE"
    echo "source_sha=$(git -C "$ROOT" rev-parse HEAD)"
    echo "go_version=$(go version)"
    echo "os=$(uname -a)"
    echo "started=$(date -u +%FT%TZ)"
} > "$OUT/metadata.txt"
cd "$ROOT"
go test -run '^$' -bench "$SELECTOR" \
    -benchtime=250ms -count=1 -benchmem -cpu=4 -timeout=25m \
    2>&1 | tee "$OUT/${MODE}-benchmark.log"
echo "No baseline comparison performed; gate disabled." | tee "$OUT/status.txt"
