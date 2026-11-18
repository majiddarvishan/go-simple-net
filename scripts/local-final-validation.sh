#!/usr/bin/env bash
# Single-command LOCAL final validation for neta-go. No GitHub calls, PR,
# merge, tag, release or baseline promotion. All results in ONE .tar.gz.
set -uo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)" || exit 2
cd "$ROOT" || exit 2
umask 077
if [[ $# == 1 && "$1" == "--help" ]]; then
    echo "Usage: bash scripts/local-final-validation.sh"
    echo "Runs local Go/race, C++ wire, QA-004/005 and 30m soak."
    echo "Writes ONE evidence bundle under artifacts/local-final-validation/."
    exit 0
fi
if (( $# != 0 )); then echo "Usage: bash scripts/local-final-validation.sh" >&2; exit 2; fi

BASE="$ROOT/artifacts/local-final-validation"
ID="run-$(date -u +%Y%m%dT%H%M%SZ)-$$"
DIR="$BASE/$ID"
ARCHIVE="$BASE/$ID.tar.gz"
mkdir -p "$DIR" || exit 2
TABLE="$DIR/stages.tsv"
printf 'stage\tstatus\texit_code\tseconds\tlog\n' > "$TABLE"
FAILURES=0
COMPLETED=0
ACTIVE=""
SHA="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
trap 'exit 130' INT
trap 'exit 143' TERM

finish() {
    local original=$?
    trap - EXIT
    if [[ -n "$ACTIVE" ]]; then
        printf '%s\tINTERRUPTED\t%s\t0\t%s.log\n' "$ACTIVE" "$original" "$ACTIVE" >> "$TABLE"
        FAILURES=$((FAILURES + 1))
    fi
    local status=PASS
    if (( original != 0 || FAILURES > 0 || COMPLETED != 11 )); then status=FAIL; fi
    {
        printf 'local_execution=%s\n' "$status"
        printf 'source_sha=%s\n' "$SHA"
        printf 'branch=%s\n' "$(git branch --show-current 2>/dev/null || echo unknown)"
        printf 'finished_utc=%s\n' "$(date -u +%FT%TZ)"
        printf 'stages_completed=%s/11\n' "$COMPLETED"
        printf 'stages_failed=%s\n' "$FAILURES"
        echo 'performance_gate=DISABLED'
        echo 'reference_baseline_qualification=INCONCLUSIVE_NOT_RUN'
        echo 'reason=No approved baseline, 10 matched reference pairs or absolute SLA'
        echo
        cat "$TABLE"
    } > "$DIR/summary.txt"
    if command -v python3 >/dev/null 2>&1; then
        python3 - "$DIR" "$status" "$SHA" <<'PY' || true
import csv, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
with (root / "stages.tsv").open() as stream:
    stages = list(csv.DictReader(stream, delimiter="\t"))
report = {
    "local_execution": sys.argv[2],
    "source_sha": sys.argv[3],
    "performance_gate": "DISABLED",
    "reference_baseline_qualification": "INCONCLUSIVE_NOT_RUN",
    "stages": stages,
    "files": sorted(str(p.relative_to(root)) for p in root.rglob("*") if p.is_file()),
}
(root / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
PY
    fi
    rm -f -- "$DIR/cpp-golden-bin"
    local archived=0
    if tar -czf "$ARCHIVE" -C "$BASE" "$ID"; then archived=1; fi
    echo
    echo "=========================================="
    echo "LOCAL FINAL VALIDATION: $status"
    echo "Source SHA: $SHA"
    echo "Completed: $COMPLETED/11; failures: $FAILURES"
    echo "Reference baseline: INCONCLUSIVE (not run)"
    if (( archived )); then echo "SEND THIS ONE FILE: $ARCHIVE"; else echo "ARCHIVE ERROR: logs at $DIR" >&2; fi
    echo "=========================================="
    if (( original >= 128 )); then exit "$original"; fi
    if [[ "$status" == FAIL ]] || (( archived == 0 )); then exit 1; fi
    exit 0
}
trap finish EXIT

stage() {
    local name="$1"; shift
    local start="$(date +%s)" rc state=PASS
    ACTIVE="$name"
    echo
    echo "========== $name =========="
    (set -euo pipefail; "$@") 2>&1 | tee "$DIR/$name.log"
    rc=$?
    if (( rc != 0 )); then state=FAIL; FAILURES=$((FAILURES + 1)); fi
    printf '%s\t%s\t%s\t%s\t%s.log\n' \
        "$name" "$state" "$rc" "$(($(date +%s) - start))" "$name" >> "$TABLE"
    COMPLETED=$((COMPLETED + 1))
    ACTIVE=""
    echo "STAGE $name: $state"
    return 0
}

preflight() {
    local program
    for program in git go make python3 g++ tar tee grep awk; do
        command -v "$program" >/dev/null || { echo "Missing: $program" >&2; return 1; }
    done
    [[ "$(git rev-parse --show-toplevel)" == "$ROOT" ]] || return 1
    [[ "$(git branch --show-current)" == \
        "feature/qa004-qa005-qa007-implementation-2026-11-18" ]] || {
        echo "Switch to the neta-go QA implementation feature branch." >&2; return 1;
    }
    if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
        echo "Tracked working-tree changes detected; use a clean checkout." >&2
        git status --short --branch; return 1
    fi
    if [[ -n "$(git ls-files --others --exclude-standard -- '*.go' '**/*.go')" ]]; then
        echo "Untracked .go file detected; use an exact-source checkout." >&2; return 1
    fi
    echo "SOURCE: $(git rev-parse HEAD)"
    go version
    python3 --version
    g++ --version | head -n 1
    echo "Running all checks locally only."
}
metadata() {
    {
        echo "started_utc=$(date -u +%FT%TZ)"
        echo "source_sha=$SHA"
        echo "go_version=$(go version)"
        echo "goos=$(go env GOOS)"
        echo "goarch=$(go env GOARCH)"
        echo "uname=$(uname -srm)"
        if command -v lscpu >/dev/null 2>&1; then lscpu; fi
        if command -v free >/dev/null 2>&1; then free -h; fi
        git status --short --branch
    } > "$DIR/environment.txt" 2>&1
}
go_verify() { ARTIFACT_DIR="$DIR/go-verify" make verify; }
go_smoke() { ARTIFACT_DIR="$DIR/go-smoke" make smoke; }
cpp_wire() {
    g++ -std=c++20 -O2 -Wall -Wextra \
        -I testdata/neta-cpp-1.5.0/source/include \
        testdata/neta-cpp-1.5.0/golden.cpp -o "$DIR/cpp-golden-bin"
    "$DIR/cpp-golden-bin" > "$DIR/cpp-golden-actual.txt"
    diff -u testdata/neta-cpp-1.5.0/golden.txt "$DIR/cpp-golden-actual.txt"
    go test -count=1 ./internal/wire
}
comparator() {
    bash -n scripts/local-final-validation.sh \
        scripts/qa005_report.sh scripts/qa005_qualify.sh
    python3 -m unittest discover -s scripts -p test_qa005_compare.py -v
}
scan() {
    go test -run '^$' -bench '^BenchmarkPendingDeadlineScan$' \
        -benchtime=150ms -benchmem -count=1 -cpu=1,4 \
        -cpuprofile="$DIR/qa004-scan-cpu.pprof" -timeout=20m
}
detection() {
    go test -run '^$' -bench '^BenchmarkPendingDeadlineDetection$' \
        -benchtime=1200x -benchmem -count=1 -cpu=4 \
        -cpuprofile="$DIR/qa004-detection-cpu.pprof" -timeout=20m
}
qa005_core() { ARTIFACT_DIR="$DIR/qa005-core" bash scripts/qa005_report.sh core; }
qa005_full() { ARTIFACT_DIR="$DIR/qa005-full" bash scripts/qa005_report.sh full; }
soak_30m() {
    NETA_SOAK_SECONDS=1800 go test -count=1 -run '^TestMemoryStabilitySoak$' \
        -timeout=34m -v
}
evidence_check() {
    local core full detection_count ledger_count
    core="$(grep -c '^BenchmarkQA005Qualification/Core/' \
        "$DIR/qa005-core/core-benchmark.log" || true)"
    full="$(grep -c '^BenchmarkQA005Qualification/' \
        "$DIR/qa005-full/full-benchmark.log" || true)"
    detection_count="$(grep -c '^BenchmarkPendingDeadlineDetection/' \
        "$DIR/qa004-detection.log" || true)"
    ledger_count="$(grep -c 'post-retained-app-B' \
        "$DIR/qa005-full/full-benchmark.log" || true)"
    echo "Core=$core (want 8), Full=$full (want 24)"
    echo "Deadline detection=$detection_count (want 5), resource rows=$ledger_count (want 24)"
    [[ "$core" == 8 && "$full" == 24 && "$detection_count" == 5 && "$ledger_count" == 24 ]] || return 1
    awk '/^BenchmarkPendingDeadlineDetection\// {
        if ($0 !~ /[[:space:]]1200[[:space:]]+detect-samples/) bad=1
    } END {exit bad}' "$DIR/qa004-detection.log"
    echo "Sample count and selected workload matrix shape: PASS."
    echo "This does not constitute stable-baseline qualification."
}

stage preflight preflight
if (( FAILURES > 0 )); then echo "Preflight failed; no tests started." >&2; exit 1; fi
metadata
stage go-verify go_verify
stage go-smoke go_smoke
stage cpp-v1-wire cpp_wire
stage comparator-tests comparator
stage qa004-scan scan
stage qa004-detection detection
stage qa005-core qa005_core
stage qa005-full qa005_full
stage soak-30m soak_30m
stage evidence-check evidence_check
