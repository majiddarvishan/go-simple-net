#!/usr/bin/env bash
# Run ten alternated, matched pairs on two existing, immutable-source checkouts.
# CI hosted machines are not qualified reference environments: report only.
set -Eeuo pipefail
if (( $# < 3 || $# > 4 )); then
    echo "Usage: $0 BASELINE_CHECKOUT CANDIDATE_CHECKOUT OUTPUT_DIR [core|full]" >&2
    exit 2
fi
BASELINE="$(cd -- "$1" && pwd)"
CANDIDATE="$(cd -- "$2" && pwd)"
mkdir -p -- "$3"
OUTPUT="$(cd -- "$3" && pwd)"
MODE="${4:-core}"
if [[ "$MODE" != core && "$MODE" != full ]]; then
    echo "Mode must be core or full" >&2; exit 2
fi
SELECTOR='^BenchmarkQA005Qualification/Core/'
if [[ "$MODE" == full ]]; then SELECTOR='^BenchmarkQA005Qualification/'; fi
for side in baseline candidate; do
    mkdir -p "$OUTPUT/$side"
    if [[ "$side" == baseline ]]; then source_dir="$BASELINE"; else source_dir="$CANDIDATE"; fi
    (
        cd "$source_dir"
        python3 - "$OUTPUT/$side/environment.json" <<'PY'
import json, os, platform, subprocess, sys
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
    "scenario_version": "qa005-selected-v1",
    "workload_config": os.environ.get("NETA_QA005_WORKLOAD_CONFIG", "loopback-250ms-cpu4-" + os.environ["QA005_MODE"]),
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
        echo "QA005 $MODE pair=$name side=$side"
        (cd "$source_dir" && go test -run '^$' -bench "$SELECTOR" \
            -benchtime=250ms -count=1 -benchmem -cpu=4 -timeout=45m) \
            > "$OUTPUT/$side/$name.log" 2>&1
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
echo "Optional 30-minute stability: NETA_SOAK_SECONDS=1800 go test -run '^TestMemoryStabilitySoak$' -timeout=34m -v"
