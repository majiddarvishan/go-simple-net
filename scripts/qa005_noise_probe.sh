#!/usr/bin/env bash
# QA-004/005 A/A noise probe. Runs entirely on the LOCAL host.
# Both temporary exact-SHA worktrees use the SAME code. This is NOT a
# regression baseline, owner approval, release gate or SLA qualification.
set -Eeuo pipefail

ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"
umask 077
if (( $# > 1 )); then
    echo "Usage: bash scripts/qa005_noise_probe.sh [core|full|qa004]" >&2
    exit 2
fi
MODE="core"
if (( $# == 1 )); then MODE="$1"; fi
case "$MODE" in
    core|full|qa004) ;;
    *) echo "Expected core, full or qa004" >&2; exit 2 ;;
esac

for tool in git go python3 tar mktemp; do
    command -v "$tool" >/dev/null 2>&1 || {
        echo "Required local tool not found: $tool" >&2; exit 2;
    }
done
if [[ "$(git rev-parse --show-toplevel)" != "$ROOT" ]]; then
    echo "Run only from a neta-go Git worktree." >&2
    exit 2
fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
    echo "INCONCLUSIVE: tracked changes in source checkout." >&2
    git status --short --branch
    exit 2
fi
if [[ -n "$(git ls-files --others --exclude-standard -- '*.go' '**/*.go')" ]]; then
    echo "INCONCLUSIVE: untracked Go source; use exact-SHA checkout." >&2
    exit 2
fi

SOURCE_SHA="$(git rev-parse HEAD)"
OUTPUT_BASE="$ROOT/artifacts/qa005-noise-probe"
RUN_NAME="aa-$MODE-$(date -u +%Y%m%dT%H%M%SZ)-$$"
OUT="$OUTPUT_BASE/$RUN_NAME"
ARCHIVE="$OUTPUT_BASE/$RUN_NAME.tar.gz"
mkdir -p "$OUT"
WORKDIR=""
BASELINE=""
CANDIDATE=""
ADDED_BASE=0
ADDED_CANDIDATE=0

finish() {
    local original=$?
    trap - EXIT
    local cleanup_failed=0
    if (( ADDED_CANDIDATE )); then
        git -C "$ROOT" worktree remove "$CANDIDATE" || cleanup_failed=1
    fi
    if (( ADDED_BASE )); then
        git -C "$ROOT" worktree remove "$BASELINE" || cleanup_failed=1
    fi
    if [[ -n "$WORKDIR" ]]; then
        rmdir "$WORKDIR" 2>/dev/null || true
    fi
    local status=PASS
    if (( original != 0 || cleanup_failed != 0 )); then status=FAIL; fi
    {
        echo "run_status=$status"
        echo "mode=$MODE"
        echo "source_sha=$SOURCE_SHA"
        echo "baseline_source_sha=$SOURCE_SHA"
        echo "candidate_source_sha=$SOURCE_SHA"
        echo "comparison_kind=A/A identical source"
        echo "regression_qualification=NOT_PERFORMED"
        echo "reference_baseline_approved=NO"
        echo "performance_gate=DISABLED"
        echo "finished_utc=$(date -u +%FT%TZ)"
        if (( cleanup_failed )); then
            echo "worktree_cleanup=FAILED; inspect paths manually"
            echo "temp_worktrees=$WORKDIR"
        fi
    } > "$OUT/probe-status.txt"
    local pack_ok=0
    if tar -czf "$ARCHIVE" -C "$OUTPUT_BASE" "$RUN_NAME"; then pack_ok=1; fi
    echo
    echo "================================================"
    echo "A/A NOISE PROBE: $status"
    echo "Identical source on both sides: $SOURCE_SHA"
    echo "Baseline qualification: NOT PERFORMED"
    if (( pack_ok )); then
        echo "SEND THIS ONE FILE: $ARCHIVE"
    else
        echo "Archive failed; raw evidence: $OUT" >&2
    fi
    echo "================================================"
    if (( original >= 128 )); then exit "$original"; fi
    if [[ "$status" != PASS ]] || (( pack_ok == 0 )); then exit 1; fi
    exit 0
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

capture_environment() {
    local path="$1"
    {
        echo "date_utc=$(date -u +%FT%TZ)"
        echo "source_sha=$SOURCE_SHA"
        echo "go_version=$(go version)"
        echo "goos=$(go env GOOS)"
        echo "goarch=$(go env GOARCH)"
        echo "os=$(uname -srm)"
        echo "GOMAXPROCS=$(printenv GOMAXPROCS || echo default)"
        echo "CPU:"
        if command -v lscpu >/dev/null 2>&1; then
            lscpu
        elif [[ -r /proc/cpuinfo ]]; then
            grep -m1 'model name' /proc/cpuinfo || true
        fi
        echo "LOAD:"
        if [[ -r /proc/loadavg ]]; then cat /proc/loadavg; fi
        echo "MEMORY:"
        if command -v free >/dev/null 2>&1; then free -h; fi
        echo "PRESSURE:"
        for file in /proc/pressure/cpu /proc/pressure/memory /proc/pressure/io; do
            if [[ -r "$file" ]]; then
                echo "$file"
                cat "$file"
            fi
        done
        echo "VMSTAT_SWAP_AND_MAJOR_FAULTS:"
        if [[ -r /proc/vmstat ]]; then
            grep -E '^(pswpin|pswpout|pgmajfault) ' /proc/vmstat || true
        fi
        echo "CPU_FREQUENCY_GOVERNOR:"
        for file in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
            if [[ -r "$file" ]]; then echo "$file=$(cat "$file")"; fi
        done
        echo "CGROUP_LIMITS:"
        for file in /sys/fs/cgroup/cpu.max /sys/fs/cgroup/memory.max \
                    /sys/fs/cgroup/cpuset.cpus.effective; do
            if [[ -r "$file" ]]; then echo "$file=$(cat "$file")"; fi
        done
    } > "$path" 2>&1
}

capture_environment "$OUT/system-before.txt"
WORKDIR="$(mktemp -d /tmp/neta-qa005-aa.XXXXXXXX)"
BASELINE="$WORKDIR/baseline"
CANDIDATE="$WORKDIR/candidate"
echo "Creating temporary A/A exact-SHA worktrees (no branch switches)."
git -C "$ROOT" worktree add --quiet --detach "$BASELINE" "$SOURCE_SHA"
ADDED_BASE=1
git -C "$ROOT" worktree add --quiet --detach "$CANDIDATE" "$SOURCE_SHA"
ADDED_CANDIDATE=1

# This is an A/A noise probe, never a signed/reference baseline comparison.
unset NETA_QA005_APPROVED_MANIFEST
export NETA_QA005_REFERENCE_MACHINE="local-aa-probe"
echo "Running 10 paired alternating $MODE benchmark sets locally."
bash "$ROOT/scripts/qa005_qualify.sh" \
    "$BASELINE" "$CANDIDATE" "$OUT/paired" "$MODE" \
    2>&1 | tee "$OUT/pairing-run.log"
capture_environment "$OUT/system-after.txt"

python3 - "$OUT/paired/comparison.json" "$OUT/noise-summary.json" <<'PY'
import json
import pathlib
import sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if not data.get("matrix_complete") or not data.get("environment_comparable"):
    raise SystemExit("INCONCLUSIVE: missing/incomparable A/A benchmark evidence")
if data.get("baseline_approved") is not False:
    raise SystemExit("Refusing accidental baseline approval during A/A probe")
states = {"PASS": 0, "FAIL": 0, "INCONCLUSIVE": 0}
unknown = set()
for scenario in data.get("scenarios", {}).values():
    for metric, evaluation in scenario.items():
        if metric == "retained_bytes":
            continue  # Not instrumented; cannot assess it from process heap.
        state = evaluation.get("status", "INCONCLUSIVE")
        if state not in states:
            unknown.add(state)
        else:
            states[state] += 1
summary = {
    "probe_kind": "A/A: identical exact SHA, ten alternating pairs",
    "reference_baseline_qualification": "NOT_PERFORMED",
    "performance_gate": "DISABLED",
    "source_sha_comparison": "identical",
    "metric_counts_excluding_uninstrumented_retained_memory": states,
    "unexpected_metric_states": sorted(unknown),
    "relative_status_including_memory": data.get("relative_status"),
    "warning": ("A/A FAIL on any comparable metric suggests noise or "
                "measurement instability; A/A PASS does NOT approve a "
                "machine, a historical baseline, an absolute SLA, or a gate."),
}
pathlib.Path(sys.argv[2]).write_text(
    json.dumps(summary, sort_keys=True, indent=2) + "\n", encoding="utf-8")
print(json.dumps(summary, sort_keys=True, indent=2))
if unknown:
    raise SystemExit("Unexpected comparator metric state")
PY
echo "A/A noise evidence captured. See noise-summary.json and system-before/after.txt."
