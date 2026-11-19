#!/usr/bin/env python3
"""Fail-closed source/effective fixture equivalence for a test-only QA-005 overlay.

This checks equivalence, NOT numerical performance or retained-attributed-B.
Source baseline SHA must separately be verified by the invoking checkout.
"""
import argparse
import json
import re
from pathlib import Path

SOURCE_ANCHOR = "9b7b2318f73d173a99d22489042f84f8ce58b745"
ADAPTER_ID = "QA005-OWNED-BUFFER-OVERLAY-V8-UNQUALIFIED"
SOURCE_FILES = {
    "benchmark_test.go", "qa005_qualification_benchmark_test.go",
    "session.go", "session_internal.go", "session_transport.go",
    "session_control.go", "resource_charges.go",
    "types.go", "server_lifecycle.go",
}
COMPARABLE_FIELDS = (
    "adapter_id", "source_fixture_sha256", "effective_fixture_sha256",
    "source_file_sha256", "effective_file_sha256", "sampler_interval_ms",
    "coverage_status", "covered_categories", "observed_partial_categories",
    "excluded_categories", "coverage_exclusions_detail",
    "measurement_qualifies_retained_attributed_memory", "qualification",
)


def check_manifest(manifest, side):
    if not isinstance(manifest, dict):
        raise ValueError(f"{side}: manifest must be a JSON object")
    if manifest.get("adapter_id") != ADAPTER_ID:
        raise ValueError(f"{side}: unreviewed or missing adapter version")
    if manifest.get("qualification") != "DIAGNOSTIC_ONLY":
        raise ValueError(f"{side}: diagnostic qualification contract changed")
    if manifest.get("measurement_qualifies_retained_attributed_memory") is not False:
        raise ValueError(f"{side}: memory qualification must remain false")
    if manifest.get("source_tree_mutated") is not False:
        raise ValueError(f"{side}: source mutation cannot be asserted")
    if manifest.get("coverage_status") != "PARTIAL_INCONCLUSIVE":
        raise ValueError(f"{side}: incomplete coverage must remain INCONCLUSIVE")
    if manifest.get("covered_categories") != []:
        raise ValueError(f"{side}: falsely declared fully covered categories")
    if not manifest.get("observed_partial_categories"):
        raise ValueError(f"{side}: partial categories not described")
    if manifest.get("sampler_interval_ms") != 5:
        raise ValueError(f"{side}: sampler interval contract differs")
    for field in ("source_file_sha256", "effective_file_sha256"):
        digests = manifest.get(field)
        if not isinstance(digests, dict) or set(digests) != SOURCE_FILES:
            raise ValueError(f"{side}: {field} file set mismatch")
        if not all(isinstance(d, str) and re.fullmatch(r"[a-f0-9]{64}", d) for d in digests.values()):
            raise ValueError(f"{side}: invalid {field} digest")
    for field in ("source_fixture_sha256", "effective_fixture_sha256"):
        value = manifest.get(field)
        if not isinstance(value, str) or not re.fullmatch(r"[a-f0-9]{64}", value):
            raise ValueError(f"{side}: missing/invalid {field}")


def check_pair(baseline, candidate):
    check_manifest(baseline, "baseline")
    check_manifest(candidate, "candidate")
    mismatch = [field for field in COMPARABLE_FIELDS if baseline.get(field) != candidate.get(field)]
    if mismatch:
        raise ValueError("original/effective adapter mismatch: " + ", ".join(mismatch))
    return {
        "source_anchor_sha": SOURCE_ANCHOR,
        "adapter_id": ADAPTER_ID,
        "source_fixture_sha256": baseline["source_fixture_sha256"],
        "effective_fixture_sha256": baseline["effective_fixture_sha256"],
        "source_files_compared": sorted(SOURCE_FILES),
        "source_effective_compatibility": "PASS",
        "attributable_memory_qualification": "INCONCLUSIVE",
        "performance_gate": "DISABLED",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", type=Path, required=True)
    parser.add_argument("--candidate", type=Path, required=True)
    args = parser.parse_args()
    try:
        baseline = json.loads(args.baseline.read_text())
        candidate = json.loads(args.candidate.read_text())
        print(json.dumps(check_pair(baseline, candidate), indent=2, sort_keys=True))
    except (ValueError, OSError, json.JSONDecodeError) as exc:
        print(json.dumps({
            "source_effective_compatibility": "INCONCLUSIVE",
            "attributable_memory_qualification": "INCONCLUSIVE",
            "error": str(exc),
            "performance_gate": "DISABLED",
        }, sort_keys=True))
        raise SystemExit(1) from exc


if __name__ == "__main__":
    main()
