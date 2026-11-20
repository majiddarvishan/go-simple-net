#!/usr/bin/env bash
# Run ten alternated, matched pairs on two existing, immutable-source checkouts.
# CI hosted machines are not qualified reference environments: report only.
set -Eeuo pipefail
if (( $# < 3 || $# > 4 )); then
    echo "Usage: $0 BASELINE_CHECKOUT CANDIDATE_CHECKOUT OUTPUT_DIR [core|full|qa004]" >&2
    exit 2
fi
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ORD_EVID="${NETA_QA005_ORD_EVID:-0}"
if [[ "$ORD_EVID" != 0 && "$ORD_EVID" != 1 ]]; then
    echo "NETA_QA005_ORD_EVID must be 0 or 1" >&2; exit 2
fi
if [[ "$ORD_EVID" == 1 && ! -f "$SCRIPT_DIR/qa005_ord_evidence.py" ]]; then
    echo "ORD-EVID collector missing; no run allowed" >&2; exit 2
fi
BASELINE="$(cd -- "$1" && pwd)"
CANDIDATE="$(cd -- "$2" && pwd)"
mkdir -p -- "$3"
OUTPUT="$(cd -- "$3" && pwd)"
MODE="${4:-core}"
export QA005_MODE="$MODE"
if [[ "$MODE" != core && "$MODE" != full && "$MODE" != qa004 ]]; then
    echo "Mode must be core, full or qa004" >&2; exit 2
fi
SELECTOR='^BenchmarkQA005Qualification/Core/'
# Keep the established 250ms qualification workload by default. A/A
# repeatability diagnostics may extend each benchmark sample locally; the
# effective duration is recorded in each environment's workload_config.
BENCHTIME="${NETA_QA005_BENCHTIME:-250ms}"
case "$BENCHTIME" in
    250ms|500ms|1s|2s) ;;
    *) echo "INCONCLUSIVE: NETA_QA005_BENCHTIME must be 250ms, 500ms, 1s or 2s" >&2; exit 2 ;;
esac
FIXTURES=(qa005_qualification_benchmark_test.go benchmark_test.go)
if [[ "$MODE" == full ]]; then SELECTOR='^BenchmarkQA005Qualification/'; fi
if [[ "$MODE" == qa004 ]]; then
    if [[ -n "${NETA_QA005_BENCHTIME:-}" ]]; then
        echo "INCONCLUSIVE: qa004 requires fixed 1200x samples; unset NETA_QA005_BENCHTIME" >&2
        exit 2
    fi
    SELECTOR='^BenchmarkPendingDeadlineDetection$'
    BENCHTIME="1200x"
    FIXTURES=(qa004_deadline_benchmark_test.go benchmark_test.go)
fi
export QA005_BENCHTIME="$BENCHTIME"

# Never run expensive pairs against a source that lacks the benchmark or has
# a differently defined workload. A missing baseline harness (including the
# original main before QA-005) is not a valid comparison.
for source_dir in "$BASELINE" "$CANDIDATE"; do
    for fixture in "${FIXTURES[@]}"; do
        if [[ ! -f "$source_dir/$fixture" ]]; then
            echo "INCONCLUSIVE: missing $fixture in $source_dir; no benchmark pairs executed" >&2
            exit 2
        fi
    done
    if [[ -n "$(git -C "$source_dir" status --porcelain --untracked-files=no)" ]]; then
        echo "INCONCLUSIVE: tracked source changes in $source_dir; require an immutable exact-SHA checkout" >&2
        exit 2
    fi
    # Untracked Go files are compiled by 'go test' but absent from the
    # recorded commit and fixture digest: reject this silent source drift.
    if [[ -n "$(git -C "$source_dir" ls-files --others --exclude-standard -- '*.go' '**/*.go')" ]]; then
        echo "INCONCLUSIVE: untracked Go source in $source_dir; use clean exact-SHA worktrees" >&2
        exit 2
    fi
done
for fixture in "${FIXTURES[@]}"; do
    if ! cmp -s "$BASELINE/$fixture" "$CANDIDATE/$fixture"; then
        echo "INCONCLUSIVE: workload fixture $fixture differs between exact-SHA checkouts" >&2
        exit 2
    fi
done

for side in baseline candidate; do
    mkdir -p "$OUTPUT/$side"
    if [[ "$side" == baseline ]]; then source_dir="$BASELINE"; else source_dir="$CANDIDATE"; fi
    (
        cd "$source_dir"
        python3 - "$OUTPUT/$side/environment.json" <<'PY'
import hashlib, json, os, pathlib, platform, subprocess, sys
def command(*args):
    return subprocess.check_output(args, text=True).strip()
cpu_model = ""
try:
    cpu_model = next((line.split(":", 1)[1].strip() for line in
                      open("/proc/cpuinfo") if line.startswith("model name")), "")
except OSError:
    pass
metadata = {
    "source_sha": command("git", "rev-parse", "HEAD"),
    "go_version": command("go", "version"),
    "goos": command("go", "env", "GOOS"),
    "goarch": command("go", "env", "GOARCH"),
    "gomaxprocs": os.environ.get("GOMAXPROCS", "default"),
    "cpu_model": cpu_model,
    "machine_id": os.environ.get("NETA_QA005_REFERENCE_MACHINE", platform.node()),
    "scenario_version": "qa004-or-qa005-selected-v2",
    "scenario_mode": os.environ["QA005_MODE"],
    "harness_sha256": hashlib.sha256(
        b"".join(pathlib.Path(name).read_bytes() for name in (
            ("qa004_deadline_benchmark_test.go", "benchmark_test.go")
            if os.environ["QA005_MODE"] == "qa004" else
            ("qa005_qualification_benchmark_test.go", "benchmark_test.go")
        ))
    ).hexdigest(),
    "workload_config": os.environ.get(
        "NETA_QA005_WORKLOAD_CONFIG",
        "loopback-" + os.environ["QA005_BENCHTIME"] + "-cpu4-" + os.environ["QA005_MODE"]
    ),
}
with open(sys.argv[1], "w") as output:
    json.dump(metadata, output, indent=2, sort_keys=True)
PY
    )
done
for i in $(seq 1 10); do
    printf -v name '%02d' "$i"
    if (( i % 2 == 0 )); then order=(candidate baseline); else order=(baseline candidate); fi
    for side in "${order[@]}"; do
        if [[ "$side" == baseline ]]; then source_dir="$BASELINE"; else source_dir="$CANDIDATE"; fi
        echo "QA-004/005 $MODE pair=$name side=$side"
        if [[ "$ORD_EVID" == 1 ]]; then
            # Opt-in report-only wrapper, unchanged go test arguments and logs.
            (cd "$source_dir" && python3 -B "$SCRIPT_DIR/qa005_ord_evidence.py" \
                --environment "$OUTPUT/$side/environment.json" \
                --output "$OUTPUT/ord-evidence" \
                --pair "$i" --side "$side" --mode "$MODE" \
                --log "$OUTPUT/$side/$name.log" -- \
                go test -run '^$' -bench "$SELECTOR" \
                -benchtime="$BENCHTIME" -count=1 -benchmem -cpu=4 -timeout=45m)
        else
            (cd "$source_dir" && go test -run '^$' -bench "$SELECTOR" \
                -benchtime="$BENCHTIME" -count=1 -benchmem -cpu=4 -timeout=45m) \
                > "$OUTPUT/$side/$name.log" 2>&1
        fi
    done
done
args=()
if [[ -n "${NETA_QA005_APPROVED_MANIFEST:-}" ]]; then
    args=(--approved-manifest "$NETA_QA005_APPROVED_MANIFEST")
fi
python3 "$CANDIDATE/scripts/qa005_compare.py" \
    --baseline "$OUTPUT/baseline" --candidate "$OUTPUT/candidate" \
    --output "$OUTPUT/comparison.json" "${args[@]}"
echo "Report-only comparison: $OUTPUT/comparison.json"
if [[ "$ORD_EVID" == 1 ]]; then
    # Read-only statistical diagnostics over existing, unchanged logs;
    # never an A/A mode/host admission or a replacement for the comparator.
    python3 -B "$SCRIPT_DIR/qa005_ord_drift.py" \
        --paired-dir "$OUTPUT" --mode "$MODE" \
        --output "$OUTPUT/ord-drift-diagnostics.json"
    echo "ORD-EVID process/time/host traces: $OUTPUT/ord-evidence"
    echo "ORD-EVID report: $OUTPUT/ord-drift-diagnostics.json (DRIFT-INCONCLUSIVE)"
fi
echo "Optional 30-minute stability: NETA_SOAK_SECONDS=1800 go test -run '^TestMemoryStabilitySoak$' -timeout=34m -v"
