#!/usr/bin/env python3
"""QA-005 paired, noise-aware comparison; advisory until owner gate activation.

Input: exactly ten pair-indexed benchmark logs per side, immutable baseline
approval manifest, matching environment descriptors. No files are modified in
the baseline tree. Missing evidence is INCONCLUSIVE, never PASS.
"""
import argparse
import hashlib
import json
import math
import re
import statistics
from pathlib import Path

REQUIRED_METRICS = {
    "throughput_ops_s": (0.10, "higher"),
    "p50_ns": (0.10, "lower"),
    "p95_ns": (0.10, "lower"),
    "p99_ns": (0.15, "lower"),
    "cpu_ns_op": (0.15, "lower"),
    "allocs_op": (0.15, "lower"),
    "retained_bytes": (0.15, "lower"),
    "deadline_detection_p99_ns": (0.15, "lower"),
}
REQUIRED_ENV = ("go_version", "goos", "goarch", "gomaxprocs", "cpu_model",
                "machine_id", "scenario_version", "workload_config",
                "scenario_mode", "harness_sha256")
PAIRS = 10
CONFIDENCE = 0.95
P99_MIN_OBSERVATIONS = 1000


def student_t_critical(alpha, degrees):
    """Two-sided t critical value, numerically integrated for Python stdlib."""
    coefficient = math.gamma((degrees + 1) / 2) / (
        math.sqrt(degrees * math.pi) * math.gamma(degrees / 2)
    )

    def cdf_positive(x):
        segments = 800
        step = x / segments
        total = coefficient * (1 + x * x / degrees) ** (-(degrees + 1) / 2)
        total += coefficient
        for index in range(1, segments):
            z = index * step
            pdf = coefficient * (1 + z * z / degrees) ** (-(degrees + 1) / 2)
            total += (4 if index % 2 else 2) * pdf
        return 0.5 + step * total / 3

    target = 1 - alpha / 2
    low, high = 0.0, 32.0
    for _ in range(52):
        midpoint = (low + high) / 2
        if cdf_positive(midpoint) < target:
            low = midpoint
        else:
            high = midpoint
    return (low + high) / 2


def parse_benchmark(path):
    found = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.startswith("BenchmarkQA005Qualification/"):
            continue
        parts = line.split()
        if len(parts) < 4:
            continue
        name = re.sub(r"-[0-9]+$", "", parts[0])
        if name in found:
            raise ValueError(f"duplicate benchmark {name} in {path}")
        try:
            iterations = int(parts[1])
        except ValueError:
            continue
        metrics = {}
        for index in range(2, len(parts) - 1, 2):
            try:
                metrics[parts[index + 1]] = float(parts[index])
            except ValueError:
                continue
        no_wait = "/NoWait-" in name
        prefix = "delivery-" if no_wait else ""
        found[name] = {
            "observations": iterations,
            "throughput_ops_s": metrics.get("ops/s"),
            "p50_ns": metrics.get(prefix + "p50-ns"),
            "p95_ns": metrics.get(prefix + "p95-ns"),
            "p99_ns": metrics.get(prefix + "p99-ns"),
            "allocs_op": metrics.get("allocs/op"),
            "retained_bytes": metrics.get("heap-live-B"),
            # CPU/op and deadline P99 must come from matched dedicated evidence,
            # never be silently approximated from wall clock or callback start.
            "cpu_ns_op": metrics.get("cpu-ns/op"),
            "deadline_detection_p99_ns": metrics.get("detect-p99-ns"),
        }
    return found


def classify(base, candidate, tolerance, direction, critical):
    if len(base) != PAIRS or len(candidate) != PAIRS:
        return {"status": "INCONCLUSIVE", "reason": "missing paired samples"}
    if any(v is None or not math.isfinite(v) or v <= 0 for v in base + candidate):
        return {"status": "INCONCLUSIVE",
                "reason": "missing, zero or near-zero baseline/candidate requires approved absolute-delta policy"}
    signed_logs = [
        math.log(b / c) if direction == "higher" else math.log(c / b)
        for b, c in zip(base, candidate)
    ]
    mean = statistics.mean(signed_logs)
    sem = statistics.stdev(signed_logs) / math.sqrt(PAIRS)
    lower, upper = mean - critical * sem, mean + critical * sem
    threshold = -math.log1p(-tolerance) if direction == "higher" else math.log1p(tolerance)
    status = "FAIL" if lower > threshold else ("PASS" if upper <= threshold else "INCONCLUSIVE")
    return {"status": status, "geometric_regression": math.expm1(mean),
            "confidence_interval_log": [lower, upper], "threshold_log": threshold}


def complete_matrix(logs, mode):
    """Refuse incomplete, duplicated, or mixed benchmark matrices.

    Ten matched pairs are not meaningful if either side silently omitted or
    substituted workloads, even when every observed metric looks favorable.
    """
    if len(logs) != 2 * PAIRS or mode not in ("core", "full"):
        return False
    expected_core = 8
    expected_full = 0 if mode == "core" else 16
    names = set(logs[0])
    if len(names) != expected_core + expected_full:
        return False
    core_count = sum("/Core/" in name for name in names)
    full_count = sum("/Full/" in name for name in names)
    if core_count != expected_core or full_count != expected_full:
        return False
    return all(set(one_run) == names for one_run in logs)


def main():
    cli = argparse.ArgumentParser()
    cli.add_argument("--baseline", required=True, type=Path)
    cli.add_argument("--candidate", required=True, type=Path)
    cli.add_argument("--approved-manifest", type=Path)
    cli.add_argument("--output", required=True, type=Path)
    args = cli.parse_args()
    required_files = [directory / "environment.json"
                      for directory in (args.baseline, args.candidate)]
    required_files += [directory / f"{i:02d}.log"
                       for directory in (args.baseline, args.candidate)
                       for i in range(1, PAIRS + 1)]
    missing = [str(path) for path in required_files if not path.is_file()]
    if missing:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        report = {
            "status": "INCONCLUSIVE", "relative_status": "INCONCLUSIVE",
            "performance_gate": "DISABLED — report-only",
            "reason": "missing qualification evidence",
            "missing_files": missing,
        }
        args.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\\n")
        print(json.dumps(report, indent=2))
        return
    baseline_meta = json.loads((args.baseline / "environment.json").read_text())
    candidate_meta = json.loads((args.candidate / "environment.json").read_text())
    comparable = all(
        baseline_meta.get(key) is not None and
        baseline_meta.get(key) == candidate_meta.get(key)
        for key in REQUIRED_ENV
    )
    baseline_digest = hashlib.sha256(
        b"".join((args.baseline / f"{i:02d}.log").read_bytes()
                 for i in range(1, PAIRS + 1))
    ).hexdigest()
    owner_approved = False
    if args.approved_manifest is not None:
        manifest = json.loads(args.approved_manifest.read_text())
        owner_approved = (
            manifest.get("owner_approved") is True and
            manifest.get("baseline_source_sha") == baseline_meta.get("source_sha") and
            manifest.get("benchmark_sha256") == baseline_digest and
            manifest.get("reference_environment") == {
                key: baseline_meta.get(key) for key in REQUIRED_ENV
            }
        )
    base_logs = [parse_benchmark(args.baseline / f"{i:02d}.log")
                 for i in range(1, PAIRS + 1)]
    cand_logs = [parse_benchmark(args.candidate / f"{i:02d}.log")
                 for i in range(1, PAIRS + 1)]
    cases = sorted(set().union(*(set(log) for log in base_logs + cand_logs)))
    matrix_complete = complete_matrix(base_logs + cand_logs, baseline_meta.get("scenario_mode"))
    number_tests = max(1, len(cases) * len(REQUIRED_METRICS))
    critical = student_t_critical((1 - CONFIDENCE) / number_tests, PAIRS - 1)
    comparison = {}
    for case in cases:
        comparison[case] = {}
        for metric, (tolerance, direction) in REQUIRED_METRICS.items():
            pairs_base = [log.get(case, {}).get(metric) for log in base_logs]
            pairs_candidate = [log.get(case, {}).get(metric) for log in cand_logs]
            result = classify(pairs_base, pairs_candidate, tolerance, direction, critical)
            if metric in ("p99_ns", "deadline_detection_p99_ns"):
                counts = [log.get(case, {}).get("observations", 0)
                          for log in base_logs + cand_logs]
                if min(counts) < P99_MIN_OBSERVATIONS:
                    result = {"status": "INCONCLUSIVE", "reason": "insufficient P99 observations"}
            comparison[case][metric] = result
    relative = [result["status"] for case in comparison.values() for result in case.values()]
    relative_status = "FAIL" if "FAIL" in relative else (
        "PASS" if relative and all(x == "PASS" for x in relative) else "INCONCLUSIVE"
    )
    # Missing owner-approved stable baseline or environment cannot qualify PASS.
    status = relative_status if comparable and owner_approved and matrix_complete else "INCONCLUSIVE"
    # Absolute SLA values and the mandatory gate have not been owner-approved.
    report = {
        "status": status, "relative_status": relative_status,
        "baseline_approved": owner_approved, "environment_comparable": comparable,
        "matrix_complete": matrix_complete, "observed_scenarios": len(cases),
        "confidence": CONFIDENCE, "paired_runs": PAIRS,
        "multiple_comparison_method": "Bonferroni family-wise t intervals",
        "reference_baseline_sha256": baseline_digest,
        "absolute_sla_status": "UNSET — owner approval required",
        "performance_gate": "DISABLED — report-only",
        "scenarios": comparison,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    print(json.dumps({key: report[key] for key in (
        "status", "relative_status", "baseline_approved", "environment_comparable",
        "absolute_sla_status", "performance_gate", "matrix_complete", "observed_scenarios")}, indent=2))


if __name__ == "__main__":
    main()
