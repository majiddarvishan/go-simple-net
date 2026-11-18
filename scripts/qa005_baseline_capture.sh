#!/usr/bin/env bash
# Local, draft-only numeric reference capture. NEVER approves a baseline/gate.
# bash scripts/qa005_baseline_capture.sh [all|core|full|qa004]
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"
umask 077
MODE="${1:-all}"
if (( $# > 1 )); then echo "Usage: $0 [all|core|full|qa004]" >&2; exit 2; fi
case "$MODE" in all|core|full|qa004) ;; *) echo "Invalid mode $MODE" >&2; exit 2;; esac
for tool in git go python3 tar mktemp tee; do
    command -v "$tool" >/dev/null || { echo "Missing $tool" >&2; exit 2; }
done
if [[ "$(git rev-parse --show-toplevel)" != "$ROOT" ]] ||
   [[ -n "$(git status --porcelain --untracked-files=no)" ]] ||
   [[ -n "$(git ls-files --others --exclude-standard -- '*.go' '**/*.go')" ]]; then
    echo "INCONCLUSIVE: require a clean neta-go working tree." >&2; exit 2
fi
if [[ "$(go env GOOS)" != linux ]]; then
    echo "INCONCLUSIVE: Linux CPU/op measurement required." >&2; exit 2
fi
ANCHOR="$(python3 - "$ROOT/docs/performance-baselines/qa004-qa005-source-v1.json" <<'PY'
import json, re, sys
r = json.load(open(sys.argv[1]))
assert r["baseline_id"] == "QA004-QA005-SOURCE-V1"
assert r["approval"]["future_source_anchor_approved"] is True
assert r["approval"]["numeric_measurement_baseline_approved"] is False
assert r["approval"]["mandatory_performance_gate_enabled"] is False
assert re.fullmatch(r"[a-f0-9]{40}", r["source_commit_sha"])
print(r["source_commit_sha"])
PY
)"
git cat-file -e "$ANCHOR^{commit}" || {
    echo "Source anchor missing locally; git fetch origin first." >&2; exit 2;
}
DURATION="${NETA_QA005_BENCHTIME:-1s}"
case "$DURATION" in 250ms|500ms|1s|2s) ;; *) echo "Invalid traffic duration" >&2; exit 2;; esac
BASE="$ROOT/artifacts/qa005-reference-capture"
NAME="source-v1-$MODE-$(date -u +%Y%m%dT%H%M%SZ)-$$"
OUT="$BASE/$NAME"
ARCHIVE="$BASE/$NAME.tar.gz"
mkdir -p "$OUT"
TEMP=""
SOURCE=""
ADDED=0
FAILURES=0

finish() {
    local prior=$?
    trap - EXIT
    local cleanup=0
    if (( ADDED )); then git -C "$ROOT" worktree remove "$SOURCE" || cleanup=1; fi
    if [[ -n "$TEMP" ]]; then rmdir "$TEMP" 2>/dev/null || true; fi
    local result=PASS
    if (( prior != 0 || cleanup != 0 || FAILURES != 0 )); then result=FAIL; fi
    {
        echo "capture_status=$result"
        echo "source_baseline=QA004-QA005-SOURCE-V1"
        echo "source_sha=$ANCHOR"
        echo "modes=$MODE"
        echo "traffic_benchtime=$DURATION"
        echo "numeric_reference_approved=false"
        echo "hardware_approved=false"
        echo "retained_attributed_memory=NOT_MEASURED"
        echo "absolute_slas=NOT_APPROVED"
        echo "performance_gate=DISABLED"
        echo "candidate_regression=NOT_RUN"
        echo "finished_utc=$(date -u +%FT%TZ)"
        if (( cleanup )); then echo "cleanup_failure=$TEMP"; fi
    } > "$OUT/status.txt"
    local packed=0
    if tar -czf "$ARCHIVE" -C "$BASE" "$NAME"; then packed=1; fi
    echo "=============================================="
    echo "LOCAL NUMERIC EVIDENCE: $result (source $ANCHOR)"
    echo "Numerical baseline and performance gate: NOT APPROVED"
    if (( packed )); then echo "SEND THIS ONE FILE: $ARCHIVE"; fi
    echo "=============================================="
    if (( prior >= 128 )); then exit "$prior"; fi
    if [[ "$result" != PASS ]] || (( packed == 0 )); then exit 1; fi
    exit 0
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

snapshot() {
    local dest="$1"
    {
        date -u +%FT%TZ
        go version
        uname -srm
        if command -v lscpu >/dev/null; then lscpu; fi
        if command -v free >/dev/null; then free -h; fi
        if [[ -r /proc/loadavg ]]; then cat /proc/loadavg; fi
        for f in /proc/pressure/cpu /proc/pressure/memory /proc/pressure/io; do
            if [[ -r "$f" ]]; then echo "$f"; cat "$f"; fi
        done
        if [[ -r /proc/vmstat ]]; then
            grep -E '^(pswpin|pswpout|pgmajfault) ' /proc/vmstat || true
        fi
        for f in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
            if [[ -r "$f" ]]; then echo "$f=$(cat "$f")"; fi
        done
    } > "$dest" 2>&1
}
snapshot "$OUT/system-before.txt"
TEMP="$(mktemp -d /tmp/neta-numeric-ref.XXXXXXXX)"
SOURCE="$TEMP/source"
git -C "$ROOT" worktree add --quiet --detach "$SOURCE" "$ANCHOR"
ADDED=1
[[ "$(git -C "$SOURCE" rev-parse HEAD)" == "$ANCHOR" ]] || exit 1
[[ -z "$(git -C "$SOURCE" status --porcelain --untracked-files=no)" ]] || exit 1

capture() {
    local mode="$1" selector bench a log index path
    path="$OUT/$mode/baseline"
    mkdir -p "$path"
    if [[ "$mode" == qa004 ]]; then
        selector='^BenchmarkPendingDeadlineDetection$'
        bench="1200x"
        a="qa004_deadline_benchmark_test.go"
    else
        selector='^BenchmarkQA005Qualification/Core/'
        if [[ "$mode" == full ]]; then selector='^BenchmarkQA005Qualification/'; fi
        bench="$DURATION"
        a="qa005_qualification_benchmark_test.go"
    fi
    [[ -f "$SOURCE/$a" && -f "$SOURCE/benchmark_test.go" ]] || return 1
    export QA005_MODE="$mode" QA005_BENCHTIME="$bench"
    python3 - "$SOURCE" "$path/environment.json" <<'PY'
import hashlib, json, os, pathlib, platform, subprocess, sys
root=pathlib.Path(sys.argv[1])
def run(*cmd):
    return subprocess.check_output(cmd, cwd=root, text=True).strip()
mode=os.environ["QA005_MODE"]
fixtures=(("qa004_deadline_benchmark_test.go", "benchmark_test.go")
          if mode=="qa004" else
          ("qa005_qualification_benchmark_test.go", "benchmark_test.go"))
cpu=""
try:
    cpu=next((line.split(":",1)[1].strip() for line in
              pathlib.Path("/proc/cpuinfo").read_text().splitlines()
              if line.startswith("model name")), "")
except OSError:
    pass
metadata={
    "source_sha":run("git","rev-parse","HEAD"),
    "go_version":run("go","version"),
    "goos":run("go","env","GOOS"),
    "goarch":run("go","env","GOARCH"),
    "gomaxprocs":os.environ.get("GOMAXPROCS","default"),
    "cpu_model":cpu,
    "machine_id":os.environ.get("NETA_QA005_REFERENCE_MACHINE",
        "local-hash-"+hashlib.sha256(platform.node().encode()).hexdigest()[:16]),
    "scenario_version":"qa004-or-qa005-selected-v2",
    "scenario_mode":mode,
    "harness_sha256":hashlib.sha256(b"".join((root/f).read_bytes()
                                    for f in fixtures)).hexdigest(),
    "workload_config":os.environ.get("NETA_QA005_WORKLOAD_CONFIG",
        "loopback-"+os.environ["QA005_BENCHTIME"]+"-cpu4-"+mode)
}
pathlib.Path(sys.argv[2]).write_text(json.dumps(metadata,indent=2,sort_keys=True)+"\n")
PY
    local timer start
    printf 'run\tseconds\tresult\n' > "$OUT/$mode/runs.tsv"
    for index in $(seq 1 10); do
        printf -v log '%02d' "$index"
        echo "MODE=$mode replicate=$log/10 source=$ANCHOR"
        start="$(date +%s)"
        local state=PASS
        if ! (cd "$SOURCE" && go test -run '^$' -bench "$selector" \
             -benchtime="$bench" -count=1 -benchmem -cpu=4 -timeout=45m) \
             > "$path/$log.log" 2>&1; then state=FAIL; fi
        printf '%s\t%s\t%s\n' "$log" "$(($(date +%s) - start))" "$state" \
            >> "$OUT/$mode/runs.tsv"
        if [[ "$state" != PASS ]]; then
            echo "Benchmark failed: $mode/$log.log" >&2; return 1
        fi
    done
    python3 - "$SOURCE/scripts/qa005_compare.py" "$OUT/$mode" "$mode" "$ANCHOR" <<'PY'
import hashlib,importlib.util,json,pathlib,statistics,sys
spec=importlib.util.spec_from_file_location("neta_comparator",sys.argv[1])
mod=importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
folder=pathlib.Path(sys.argv[2])
mode,anchor=sys.argv[3],sys.argv[4]
base=folder/"baseline"
files=[base/f"{i:02d}.log" for i in range(1,11)]
runs=[mod.parse_benchmark(p) for p in files]
metadata=json.loads((base/"environment.json").read_text())
if metadata["source_sha"]!=anchor or metadata["scenario_mode"]!=mode:
    raise SystemExit("INCONCLUSIVE: wrong pinned source or mode")
# Matrix check only. No A/A or cross-version comparison is claimed.
if not mod.complete_matrix(runs+runs,mode):
    raise SystemExit("INCONCLUSIVE: selected benchmark matrix incomplete")
issues=[]
for idx,run in enumerate(runs):
    for name,metric in run.items():
        if mode=="qa004":
            if metric["observations"]<1200 or not metric.get("deadline_detection_p99_ns"):
                issues.append(f"{idx+1}:{name}:deadline/samples")
        else:
            for key in ("throughput_ops_s","p50_ns","p95_ns","p99_ns",
                        "cpu_ns_op","allocs_op"):
                if metric.get(key) is None:
                    issues.append(f"{idx+1}:{name}:{key}")
            for line in files[idx].read_text().splitlines():
                if line.startswith(name+"-4 "):
                    tokens=line.split()
                    for key in ("capacity-rejections","post-retained-app-B",
                                "post-retained-completion-B","post-queued-app-B",
                                "post-outstanding-requests","post-onclose-credits"):
                        try:
                            pos=tokens.index(key)
                            if float(tokens[pos-1])!=0:
                                issues.append(f"{idx+1}:{name}:{key}:nonzero")
                        except (ValueError,IndexError):
                            issues.append(f"{idx+1}:{name}:{key}:missing")
                    break
            else:
                issues.append(f"{idx+1}:{name}:no-benchmark-line")
if issues:
    (folder/"invalid-evidence.json").write_text(json.dumps(issues,indent=2)+"\n")
    raise SystemExit("INCONCLUSIVE: see invalid-evidence.json")
digest=hashlib.sha256(b"".join(p.read_bytes() for p in files)).hexdigest()
keys=(("deadline_detection_p99_ns",) if mode=="qa004" else
      ("throughput_ops_s","p50_ns","p95_ns","p99_ns","cpu_ns_op","allocs_op"))
statistics_by_case={}
for name in sorted(runs[0]):
    statistics_by_case[name]={}
    for key in keys:
        values=[r[name][key] for r in runs]
        mean=statistics.mean(values)
        statistics_by_case[name][key]={
          "median":statistics.median(values),"min":min(values),"max":max(values),
          "sample_cv_percent":statistics.stdev(values)/mean*100 if mean else None
        }
report={
 "status":"CAPTURED_ADVISORY_ONLY","source_sha":anchor,"mode":mode,
 "runs":10,"scenario_count":len(runs[0]),"benchmark_rows":sum(map(len,runs)),
 "ordered_logs_sha256":digest,"environment":metadata,
 "metrics":statistics_by_case,
 "retained_attributed_memory":"NOT_MEASURED",
 "numeric_baseline_owner_approved":False,"performance_gate":"DISABLED"
}
(folder/"statistics.json").write_text(json.dumps(report,indent=2,sort_keys=True)+"\n")
draft={
 "owner_approved":False,"baseline_source_sha":anchor,
 "benchmark_sha256":digest,
 "reference_environment":{k:metadata[k] for k in mod.REQUIRED_ENV},
 "approval_kind":"DRAFT_NUMERIC_EVIDENCE_NOT_APPROVED",
 "performance_gate":"DISABLED"
}
(folder/"DRAFT-NOT-APPROVED.json").write_text(
    json.dumps(draft,indent=2,sort_keys=True)+"\n")
print(f"CAPTURED {mode}: 10 full logs x {len(runs[0])} selected cases")
print(f"ordered SHA256: {digest}; numeric approval=false")
PY
}
if [[ "$MODE" == all ]]; then modes=(core full qa004); else modes=("$MODE"); fi
for mode in "${modes[@]}"; do
    if capture "$mode" 2>&1 | tee "$OUT/$mode-console.log"; then
        echo "$mode PASS" >> "$OUT/modes.txt"
    else
        echo "$mode FAIL" >> "$OUT/modes.txt"
        FAILURES=$((FAILURES+1))
    fi
done
snapshot "$OUT/system-after.txt"
if (( FAILURES )); then exit 1; fi
echo "All requested modes captured, pending review; approval and gate disabled."
