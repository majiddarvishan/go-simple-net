#!/usr/bin/env bash
# Repeatable performance workloads shared by local runs and GitHub Actions.
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "$BASH_SOURCE")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="$(printenv GO 2>/dev/null || true)"
if [[ -z "$GO_BIN" ]]; then GO_BIN=go; fi
MODE=full
if [[ "$#" -gt 0 ]]; then MODE="$1"; fi
SOAK_SECONDS="$(printenv NETA_SOAK_SECONDS 2>/dev/null || true)"
if [[ -z "$SOAK_SECONDS" ]]; then SOAK_SECONDS=300; fi
ARTIFACT_BASE="$(printenv ARTIFACT_DIR 2>/dev/null || true)"
if [[ -z "$ARTIFACT_BASE" ]]; then ARTIFACT_BASE="$ROOT_DIR/artifacts/performance"; fi
if [[ "$ARTIFACT_BASE" != /* ]]; then ARTIFACT_BASE="$ROOT_DIR/$ARTIFACT_BASE"; fi

command -v "$GO_BIN" >/dev/null 2>&1 || {
    echo "ERROR: Go executable not found: $GO_BIN" >&2
    exit 127
}

if [[ "$MODE" == "soak" || "$MODE" == "full" ]]; then
    if ! [[ "$SOAK_SECONDS" =~ ^[0-9]+$ ]]; then
        echo "NETA_SOAK_SECONDS must be an integer between 300 and 600." >&2
        exit 2
    fi
    SOAK_NUM=$((10#$SOAK_SECONDS))
    if (( SOAK_NUM < 300 || SOAK_NUM > 600 )); then
        echo "NETA_SOAK_SECONDS must be an integer between 300 and 600." >&2
        exit 2
    fi
    SOAK_SECONDS="$SOAK_NUM"
fi

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SOURCE_SHA="$(git rev-parse --short=12 HEAD)"
RUN_DIR="$ARTIFACT_BASE/$RUN_STAMP-$SOURCE_SHA-$MODE-$$"
mkdir -p "$RUN_DIR"
LOG_FILE="$RUN_DIR/run.log"

exec > >(tee -a "$LOG_FILE") 2>&1

finish() {
    local rc=$?
    trap - EXIT
    {
        if (( rc == 0 )); then echo "status=PASS"; else echo "status=FAIL"; fi
        echo "mode=$MODE"
        echo "source_sha=$(git rev-parse HEAD)"
        echo "exit_code=$rc"
        echo "finished_utc=$(date -u +%FT%TZ)"
    } > "$RUN_DIR/status.txt"
    echo
    if (( rc == 0 )); then echo "Performance suite result: PASS"; else echo "Performance suite result: FAIL"; fi
    echo "Artifacts: $RUN_DIR"
    exit "$rc"
}
trap finish EXIT

run_logged() {
    local label="$1"
    local log_file="$2"
    shift 2
    echo
    echo "========== $label =========="
    "$@" 2>&1 | tee "$log_file"
}

run_timed_logged() {
    local label="$1"
    local time_file="$2"
    local log_file="$3"
    shift 3
    echo
    echo "========== $label =========="
    if [[ -x /usr/bin/time ]]; then
        /usr/bin/time -v -o "$time_file" "$@" 2>&1 | tee "$log_file"
    else
        "$@" 2>&1 | tee "$log_file"
        echo "GNU /usr/bin/time unavailable; external resource timing was not captured." > "$time_file"
    fi
}

capture_metadata() {
    {
        echo "started_utc=$(date -u +%FT%TZ)"
        echo "source_sha=$(git rev-parse HEAD)"
        echo "branch=$(git branch --show-current)"
        echo "go_version=$("$GO_BIN" version)"
        echo "GOOS=$("$GO_BIN" env GOOS)"
        echo "GOARCH=$("$GO_BIN" env GOARCH)"
        echo "GOMAXPROCS=$(printenv GOMAXPROCS 2>/dev/null || echo default)"
        echo "NETA_SOAK_SECONDS=$SOAK_SECONDS"
        echo "GO_ENV:"
        "$GO_BIN" env
        echo "SYSTEM:"
        uname -a
        echo "CPU:"
        if command -v lscpu >/dev/null 2>&1; then
            lscpu
        elif [[ -r /proc/cpuinfo ]]; then
            grep -m1 -E 'model name|Hardware' /proc/cpuinfo || true
        fi
        echo "MEMORY:"
        if command -v free >/dev/null 2>&1; then free -h; fi
        echo "CGROUP:"
        for f in /sys/fs/cgroup/cpu.max /sys/fs/cgroup/memory.max /sys/fs/cgroup/cpuset.cpus.effective; do
            if [[ -r "$f" ]]; then echo "$f=$(cat "$f")"; fi
        done
        echo "ULIMIT:"
        ulimit -a
        echo "WORKTREE:"
        git status --short --branch
        git diff --binary HEAD > "$RUN_DIR/worktree.patch"
    } > "$RUN_DIR/environment.txt" 2>&1
}

run_runtime_metadata() {
    run_logged "RUNTIME METADATA TEST" "$RUN_DIR/runtime-metadata.txt" \
        "$GO_BIN" test -count=1 -run '^TestRuntimeMetadata$' -v
}

run_smoke() {
    capture_metadata
    run_runtime_metadata
    run_logged "PERFORMANCE SMOKE" "$RUN_DIR/perf-smoke.txt" \
        "$GO_BIN" test -run '^$' \
        -bench '^BenchmarkProfileBaseline$' \
        -benchmem -benchtime=1s -count=1 -cpu=4 \
        -timeout=5m -outputdir="$RUN_DIR"
}

run_soak() {
    run_timed_logged "FIVE-MINUTE MIXED-PAYLOAD MEMORY SOAK" \
        "$RUN_DIR/memory-soak-time.txt" \
        "$RUN_DIR/memory-soak.txt" \
        env NETA_SOAK_SECONDS="$SOAK_SECONDS" \
        "$GO_BIN" test -count=1 -v \
        -run '^TestMemoryStabilitySoak$' -timeout=7m
}

capture_pprof_summaries() {
    local suffix="$1"
    local profile
    echo
    echo "========== PPROF SUMMARIES $suffix =========="
    for profile in cpu heap mutex block; do
        "$GO_BIN" tool pprof -top -nodecount=40 \
            "$RUN_DIR/$profile$suffix.pprof" \
            > "$RUN_DIR/$profile$suffix-top.txt"
        cat "$RUN_DIR/$profile$suffix-top.txt"
    done
}

run_full() {
    capture_metadata
    run_runtime_metadata

    run_logged "BENCHMARK MATRIX" "$RUN_DIR/benchmark-matrix.txt" \
        "$GO_BIN" test -run '^$' -bench '^Benchmark' \
        -benchmem -benchtime=250ms -count=3 -cpu=1,4 -timeout=35m

    run_logged "SESSION SCALING: 1 TO 512 SESSIONS" "$RUN_DIR/session-scale.txt" \
        "$GO_BIN" test -run '^$' -bench '^BenchmarkScaleSessions$' \
        -benchmem -benchtime=1s -count=1 -cpu=4 -timeout=15m

    run_soak

    run_timed_logged "SMALL-PAYLOAD CPU/HEAP/MUTEX/BLOCK PROFILES" \
        "$RUN_DIR/profile-time.txt" "$RUN_DIR/profile-run.txt" \
        "$GO_BIN" test -run '^$' -bench '^BenchmarkProfileBaseline$' \
        -benchmem -benchtime=10s -count=1 -cpu=4 -timeout=10m \
        -outputdir="$RUN_DIR" \
        -cpuprofile="$RUN_DIR/cpu.pprof" \
        -memprofile="$RUN_DIR/heap.pprof" \
        -mutexprofile="$RUN_DIR/mutex.pprof" \
        -blockprofile="$RUN_DIR/block.pprof"

    capture_pprof_summaries ""

    run_timed_logged "LARGE-PAYLOAD CPU/HEAP/MUTEX/BLOCK PROFILES" \
        "$RUN_DIR/profile-large-time.txt" "$RUN_DIR/profile-large-run.txt" \
        "$GO_BIN" test -run '^$' -bench '^BenchmarkProfileLargeBaseline$' \
        -benchmem -benchtime=5s -count=1 -cpu=4 -timeout=10m \
        -outputdir="$RUN_DIR" \
        -cpuprofile="$RUN_DIR/cpu-large.pprof" \
        -memprofile="$RUN_DIR/heap-large.pprof" \
        -mutexprofile="$RUN_DIR/mutex-large.pprof" \
        -blockprofile="$RUN_DIR/block-large.pprof"

    capture_pprof_summaries "-large"
}

case "$MODE" in
    smoke) run_smoke ;;
    soak)
        capture_metadata
        run_soak
        ;;
    full) run_full ;;
    *)
        echo "Usage: bash scripts/performance.sh {smoke|soak|full}"
        exit 2
        ;;
esac
