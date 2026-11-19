#!/usr/bin/env python3
"""Compare three same-source QA-005 overhead feasibility logs, never a performance gate.

source-only vs overlay+sampler OFF isolates instrumented accounting+fixture cost.
overlay+sampler OFF vs ON isolates incremental sampler/probe cost. One short
GitHub-hosted measurement per side is NOT reference-machine A/A qualification.
"""
import argparse
import hashlib
import json
import math
import re
from pathlib import Path

ADAPTER_ID = "QA005-OWNED-BUFFER-OVERLAY-V16-UNQUALIFIED"
BENCHMARK = "BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium"
MODES = ("source", "overlay_off", "overlay_on")
METRICS = ("ns/op", "B/op", "allocs/op", "cpu-ns/op")


def _hash(raw):
    return hashlib.sha256(raw).hexdigest()


def read_benchmark(raw, mode):
    if not isinstance(raw, bytes):
        raise ValueError(f"{mode}: benchmark log must be raw bytes")
    text = raw.decode("utf-8", errors="strict")
    if re.search(r"(?m)^(FAIL|--- FAIL:|panic:|fatal error:)", text):
        raise ValueError(f"{mode}: Go failure in raw log")
    pattern = rf"^{re.escape(BENCHMARK)}-(\d+)\s+(\d+)\s+(.+)$"
    matches = re.findall(pattern, text, flags=re.MULTILINE)
    if len(matches) != 1:
        raise ValueError(f"{mode}: expected exactly one named benchmark result, found {len(matches)}")
    cpu, count, values = matches[0]
    if cpu != "4" or int(count) < 1:
        raise ValueError(f"{mode}: wrong -cpu or missing iterations")
    result = {"iterations": int(count), "cpu": int(cpu)}
    for unit in METRICS:
        found = re.findall(rf"(?<![\w/-])(\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\s+{re.escape(unit)}(?:\s|$)", values)
        if len(found) > 1 or (unit in METRICS[:3] and len(found) != 1):
            raise ValueError(f"{mode}: missing or duplicate {unit}")
        if found:
            amount = float(found[0])
            if not math.isfinite(amount) or amount < 0 or (unit == "ns/op" and amount == 0):
                raise ValueError(f"{mode}: invalid {unit}")
            result[unit] = amount
    return result


def summarize(raw_logs, manifest, source_sha, go_version, benchtime):
    if not isinstance(manifest, dict) or manifest.get("adapter_id") != ADAPTER_ID:
        raise ValueError("unreviewed/missing effective overlay adapter")
    if manifest.get("source_tree_mutated") is not False:
        raise ValueError("overlay must preserve original source")
    if manifest.get("measurement_qualifies_retained_attributed_memory") is not False:
        raise ValueError("memory measurement must not be marked qualified")
    if manifest.get("coverage_status") != "PARTIAL_INCONCLUSIVE" or manifest.get("covered_categories") != []:
        raise ValueError("incomplete category coverage not declared")
    for key in ("source_fixture_sha256", "effective_fixture_sha256"):
        if not re.fullmatch(r"[a-f0-9]{64}", str(manifest.get(key, ""))):
            raise ValueError(f"invalid {key}")
    if set(raw_logs) != set(MODES):
        raise ValueError("all three source/overlay variants required")
    if not re.fullmatch(r"[a-f0-9]{40}", source_sha):
        raise ValueError("invalid source SHA")
    if not go_version.startswith("go version go") or not re.fullmatch(r"[1-9]\d*x", benchtime):
        raise ValueError("missing fixed toolchain or iteration contract")
    samples = {mode: read_benchmark(raw_logs[mode], mode) for mode in MODES}
    if samples["source"]["iterations"] != samples["overlay_off"]["iterations"] or samples["source"]["iterations"] != samples["overlay_on"]["iterations"]:
        raise ValueError("iteration count differs across variants")
    if samples["source"]["iterations"] != int(benchtime[:-1]):
        raise ValueError("observed iteration count differs from requested benchtime")
    # Ratios are descriptive only and need a qualified reference environment,
    # paired observations and approval before any acceptance interpretation.
    def ratio(numerator, denominator, metric):
        n = samples[numerator].get(metric)
        d = samples[denominator].get(metric)
        return None if n is None or d is None or d == 0 else n/d

    return {
        "schema_version": "QA005-INSTRUMENTATION-OVERHEAD-PREVIEW-V1",
        "source_sha": source_sha,
        "go_version": go_version,
        "benchtime": benchtime,
        "cpu": 4,
        "benchmark": BENCHMARK,
        "adapter_id": ADAPTER_ID,
        "source_fixture_sha256": manifest["source_fixture_sha256"],
        "effective_fixture_sha256": manifest["effective_fixture_sha256"],
        "samples": samples,
        "raw_log_sha256": {mode: _hash(raw_logs[mode]) for mode in MODES},
        "illustrative_accounting_plus_fixture_nsop_ratio": ratio("overlay_off", "source", "ns/op"),
        "illustrative_incremental_sampler_nsop_ratio": ratio("overlay_on", "overlay_off", "ns/op"),
        "instrumentation_overhead_qualified": False,
        "attributable_retained_memory_qualified": False,
        "source_anchor_numerical_baseline_updated": False,
        "runs_per_mode": 1,
        "reference_machine_approved": False,
        "performance_gate": "DISABLED",
        "status": "FEASIBILITY_ONLY_INCONCLUSIVE",
        "note": "These are single unpaired-run, hosted-runner descriptive numbers, not MEM-005 A/A acceptance.",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for mode in MODES:
        parser.add_argument("--" + mode.replace("_", "-"), type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--go-version", required=True)
    parser.add_argument("--benchtime", default="25x")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        raw = {mode: getattr(args, mode).read_bytes() for mode in MODES}
        manifest = json.loads(args.manifest.read_text())
        summary = summarize(raw, manifest, args.source_sha, args.go_version, args.benchtime)
        args.output.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
        print(json.dumps({"status": summary["status"], "result_file": str(args.output),
                          "performance_gate": summary["performance_gate"]}, sort_keys=True))
    except (ValueError, OSError, UnicodeError, json.JSONDecodeError) as error:
        print(json.dumps({"status": "INCONCLUSIVE", "reason": str(error),
                          "performance_gate": "DISABLED"}, sort_keys=True))
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
