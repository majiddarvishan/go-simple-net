"""Fast synthetic tests: fixed Option-A formula only, never physical qualification."""
import hashlib
import json
import math
from pathlib import Path
import sys
import tempfile
import unittest

HERE = Path(__file__).parent
sys.path.insert(0, str(HERE))
from qa005_ord_drift import analyze, fit_one, inverse3, MODE_COUNTS

ORDER = [1 if i % 2 == 0 else -1 for i in range(10)]
TIME = [-1 + 2 * i / 9 for i in range(10)]


class OrdDriftTests(unittest.TestCase):
    def test_zero_log_ratio_fits_original_mean_and_all_four_corners(self):
        for family in (5, 48, 144):
            model = fit_one([0.] * 10, ORDER, TIME, .075, family)
            self.assertEqual(model["fixed_bounds_result"], "WITHIN_FIXED_BOUNDS")
            self.assertEqual(model["joint_5m_family_size"], 5 * family)
            self.assertTrue(model["original_m_family_mean"]["inside_fixed_bound"])
            self.assertTrue(model["joint_5m_mean"]["inside_fixed_bound"])
            self.assertEqual(model["residual_df"], 7)
            self.assertEqual(len(model["joint_5m_corners"]), 4)
            self.assertEqual(
                {(v["first_side_order"], v["scaled_time"])
                 for v in model["joint_5m_corners"]},
                {(-1, -1.), (-1, 1.), (1, -1.), (1, 1.)},
            )

    def test_constant_ratio_outside_prospective_margin(self):
        model = fit_one([math.log(1.09)] * 10, ORDER, TIME, .05, 48)
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")
        self.assertFalse(model["joint_5m_mean"]["inside_fixed_bound"])

    def test_balanced_mean_cannot_hide_order_bias(self):
        y = [0.07 if o == 1 else -0.07 for o in ORDER]
        model = fit_one(y, ORDER, TIME, .05, 5)
        self.assertAlmostEqual(model["original_m_family_mean"]["estimate_log"], 0.)
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")
        self.assertTrue(any(not c["inside_fixed_bound"]
                            for c in model["joint_5m_corners"]))

    def test_balanced_mean_cannot_hide_time_drift(self):
        y = [t * .09 for t in TIME]
        model = fit_one(y, ORDER, TIME, .075, 5)
        self.assertAlmostEqual(model["original_m_family_mean"]["estimate_log"], 0.)
        self.assertTrue(model["original_m_family_mean"]["inside_fixed_bound"])
        self.assertEqual(model["fixed_bounds_result"], "OUTSIDE_FIXED_BOUNDS")

    def test_bonferroni_joint_mean_more_conservative_than_original(self):
        y = [.005 * t + (.004 if i % 2 else -.004)
             for i, t in enumerate(TIME)]
        for m in MODE_COUNTS.values():
            model = fit_one(y, ORDER, TIME, .05, m)
            original = model["original_m_family_mean"]
            joint = model["joint_5m_mean"]
            self.assertGreaterEqual(
                joint["ci_log"][1] - joint["ci_log"][0],
                original["ci_log"][1] - original["ci_log"][0],
            )
            self.assertAlmostEqual(
                model["alpha_two_sided_per_contrast"], .05 / (5 * m)
            )

    def test_zero_missing_nonfinite_and_rank_deficiency_fail_closed(self):
        for bad in ([0.] * 9, [float("nan")] * 10):
            with self.assertRaises(ValueError):
                fit_one(bad, ORDER, TIME, .05, 5)
        with self.assertRaises(ValueError):
            fit_one([0.] * 10, ORDER, [float(i) for i in ORDER], .05, 5)
        with self.assertRaises(ValueError):
            fit_one([0.] * 10, ORDER, TIME, .20, 6)
        with self.assertRaises(ValueError):
            inverse3([[1., 0, 0], [1., 0, 0], [0, 0, 1.]])

    def test_missing_or_substituted_evidence_never_claims_admission(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for mode in MODE_COUNTS:
                report = analyze(root, mode)
                self.assertEqual(report["qualification"], "DRIFT-INCONCLUSIVE")
                self.assertEqual(report["environment_validity"], "E-UNKNOWN")
                self.assertEqual(report["statistical_diagnostics"], "INCONCLUSIVE")
                self.assertEqual(report["scenarios"], {})
                self.assertTrue(report["reasons"])

    @staticmethod
    def make_complete_qa004(root):
        (root / "baseline").mkdir()
        (root / "candidate").mkdir()
        (root / "ord-evidence").mkdir()
        meta = {
            "source_sha": "a" * 40,
            "go_version": "go version go1.23.9 linux/amd64",
            "scenario_mode": "qa004",
            "harness_sha256": "b" * 64,
            "workload_config": "synthetic-1200x-qa004",
            "scenario_version": "qa004-or-qa005-selected-v2",
            "cpu_model": "synthetic",
            "machine_id": "synthetic-controlled-host",
            "goarch": "amd64", "goos": "linux", "gomaxprocs": "default",
        }
        for side in ("baseline", "candidate"):
            (root / side / "environment.json").write_text(json.dumps(meta))
        for pair in range(1, 11):
            ordered = (("baseline", "candidate") if pair % 2 else
                       ("candidate", "baseline"))
            for seq, side in enumerate(ordered):
                log_path = root / side / f"{pair:02}.log"
                lines = [
                    "BenchmarkPendingDeadlineDetection/synthetic-pending="
                    f"{n}-4 1200 1000 ns/op 1200 detect-samples 100000 detect-p99-ns"
                    for n in (32, 128, 512, 2048, 8192)
                ]
                log_path.write_text("\n".join(lines) + "\n")
                ordinal = (pair - 1) * 2 + seq
                start = (ordinal + 1) * 1_000_000_000
                stamp = lambda t: {"monotonic_ns": t, "utc_ns": t + 1000000000}
                samples = [
                    {"kind": kind, "sample_start": stamp(t),
                     "sample_end": stamp(t + 1),
                     "missing_sensors": [], "sensors": {"boot_id": "synthetic"}}
                    for kind, t in (
                        ("before_spawn", start - 100_000),
                        ("after_spawn", start + 10_000),
                        ("after_exit", start + 100_000),
                    )
                ]
                base = f"{pair:02}-{side}"
                trace = root / "ord-evidence" / (base + ".jsonl")
                trace.write_text("".join(json.dumps(x) + "\n" for x in samples))
                checksum = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
                record = {
                    "mode": "qa004", "pair": pair, "side": side,
                    "source_sha": meta["source_sha"], "toolchain": meta["go_version"],
                    "harness_sha256": meta["harness_sha256"],
                    "workload_config": meta["workload_config"],
                    "environment_sha256": checksum(root / side / "environment.json"),
                    "collector_sha256": hashlib.sha256(
                        (HERE / "qa005_ord_evidence.py").read_bytes()
                    ).hexdigest(),
                    "host_kernel": "synthetic linux 6.x",
                    "log_sha256": checksum(log_path),
                    "trace_sha256": checksum(trace),
                    "complete": True, "exit_code": 0,
                    "start_before_spawn": stamp(start),
                    "start_after_spawn": stamp(start + 10_000),
                    "exit_after_wait": stamp(start + 100_000),
                    "sample_count": 3, "missing_sensors": [],
                    "observed_max_sample_gap_ns": 110_000,
                }
                (root / "ord-evidence" / (base + ".json")).write_text(
                    json.dumps(record))
        (root / "comparison.json").write_text(json.dumps(
            {"matrix_complete": True, "relative_status": "PASS"}
        ))

    def test_complete_synthetic_qa004_model_never_admits_mode(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.make_complete_qa004(root)
            report = analyze(root, "qa004")
            self.assertEqual(report["case_metric_count"], 5, report["reasons"])
            self.assertEqual(report["statistical_diagnostics"],
                             "WITHIN_FIXED_BOUNDS_UNQUALIFIED")
            self.assertEqual(report["qualification"], "DRIFT-INCONCLUSIVE")
            self.assertEqual(report["host_mode_admission"], "NOT_PERFORMED")
            self.assertEqual(report["existing_regression_status"], "PASS")
            self.assertEqual(len(report["paired_runs"]), 20)
            self.assertEqual(len(report["pair_time_uncertainty"]), 10)
            self.assertEqual(len(report["scenarios"]), 5)

    def test_changed_host_identity_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.make_complete_qa004(root)
            path = root / "candidate" / "environment.json"
            meta = json.loads(path.read_text())
            meta["machine_id"] = "unexpected-host"
            path.write_text(json.dumps(meta))
            report = analyze(root, "qa004")
            self.assertEqual(report["statistical_diagnostics"], "INCONCLUSIVE")
            self.assertEqual(report["qualification"], "DRIFT-INCONCLUSIVE")

    def test_tampered_raw_benchmark_log_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.make_complete_qa004(root)
            with (root / "candidate" / "01.log").open("a") as log:
                log.write("tamper\n")
            report = analyze(root, "qa004")
            self.assertEqual(report["statistical_diagnostics"], "INCONCLUSIVE")
            self.assertEqual(report["scenarios"], {})
            self.assertEqual(report["qualification"], "DRIFT-INCONCLUSIVE")

    def test_process_midpoint_is_not_case_level_timestamp(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = analyze(Path(tmp), "qa004")
            self.assertIn("PER_CASE_UNPROVEN", report["time_precision"])


if __name__ == "__main__":
    unittest.main()
