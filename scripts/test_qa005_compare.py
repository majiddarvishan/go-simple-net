import importlib.util
import pathlib
import re
import subprocess
import sys
import json
import tempfile
import unittest

FILE = pathlib.Path(__file__).with_name("qa005_compare.py")
SPEC = importlib.util.spec_from_file_location("qa005_compare", FILE)
MOD = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MOD)


class ComparatorTests(unittest.TestCase):
    def test_equal_samples_pass(self):
        values = [100.0] * 10
        result = MOD.classify(values, values, 0.10, "higher", MOD.student_t_critical(0.05, 9))
        self.assertEqual(result["status"], "PASS")

    def test_clear_loss_fails(self):
        baseline = [1000.0] * 10
        candidate = [700.0] * 10
        result = MOD.classify(baseline, candidate, 0.10, "higher", MOD.student_t_critical(0.05, 9))
        self.assertEqual(result["status"], "FAIL")

    def test_uncertain_results_inconclusive(self):
        baseline = [100.0] * 10
        candidate = [130, 90, 120, 100, 120, 90, 130, 95, 125, 90]
        result = MOD.classify(baseline, candidate, 0.10, "lower", MOD.student_t_critical(0.05, 9))
        self.assertEqual(result["status"], "INCONCLUSIVE")

    def test_missing_and_zero_inconclusive(self):
        self.assertEqual(MOD.classify([1.0]*9, [1.0]*9, .1, "lower", 2.2)["status"], "INCONCLUSIVE")
        self.assertEqual(MOD.classify([0.0]*10, [1.0]*10, .1, "lower", 2.2)["status"], "INCONCLUSIVE")
        self.assertEqual(MOD.classify([None]*10, [1.0]*10, .1, "lower", 2.2)["status"], "INCONCLUSIVE")

    def test_parse_named_metrics(self):
        with tempfile.TemporaryDirectory() as dirname:
            log = pathlib.Path(dirname) / "01.log"
            log.write_text(
                "BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium-4 1200 900 ns/op "
                "250 allocs/op 300000 ops/s 400 p50-ns 900 p95-ns 1300 p99-ns 0.5 gc-cycles\n"
            )
            result = MOD.parse_benchmark(log)
            self.assertEqual(len(result), 1)
            self.assertEqual(next(iter(result.values()))["p99_ns"], 1300.0)

    def test_incomplete_matrix_cannot_qualify(self):
        names = sorted(name for name in MOD.EXPECTED_QA005_SCENARIOS if "/Core/" in name)
        one_run = {name: {} for name in names}
        logs = [dict(one_run) for _ in range(20)]
        self.assertTrue(MOD.complete_matrix(logs, "core"))
        del logs[0][names[0]]
        self.assertFalse(MOD.complete_matrix(logs, "core"))

    def test_full_requires_exact_8_core_16_full(self):
        names = sorted(MOD.EXPECTED_QA005_SCENARIOS)
        logs = [{name: {} for name in names} for _ in range(20)]
        self.assertTrue(MOD.complete_matrix(logs, "full"))
        self.assertFalse(MOD.complete_matrix(logs, "core"))
        logs[8][names[0] + "-extra"] = {}
        self.assertFalse(MOD.complete_matrix(logs, "full"))

    def test_empty_or_unknown_matrix_is_not_pass(self):
        self.assertFalse(MOD.complete_matrix([{} for _ in range(20)], "core"))
        self.assertFalse(MOD.complete_matrix([{ "unexpected": {} } for _ in range(20)], "full"))
        self.assertFalse(MOD.complete_matrix([{} for _ in range(19)], "core"))

    def test_qa004_detection_is_not_a_qa005_traffic_metric(self):
        self.assertIn("deadline_detection_p99_ns", MOD.DEADLINE_METRICS)
        self.assertNotIn("deadline_detection_p99_ns", MOD.REQUIRED_METRICS)

    def test_qa004_matrix_and_internal_settlement_parser(self):
        names = [f"BenchmarkPendingDeadlineDetection/synthetic-pending={n}"
                 for n in (32, 128, 512, 2048, 8192)]
        with tempfile.TemporaryDirectory() as folder:
            path = pathlib.Path(folder) / "qa004.log"
            path.write_text("\n".join(
                f"{name}-4 1200 2200000 ns/op 1200 detect-samples "
                f"100000 detect-p50-ns 200000 detect-p95-ns "
                f"350000 detect-p99-ns" for name in names) + "\n"
            )
            parsed = MOD.parse_benchmark(path)
            self.assertEqual(len(parsed), 5)
            self.assertTrue(MOD.complete_matrix([parsed] * 20, "qa004"))
            self.assertEqual(next(iter(parsed.values()))["observations"], 1200)
            self.assertEqual(next(iter(parsed.values()))["deadline_detection_p99_ns"], 350000)
            self.assertFalse(MOD.complete_matrix([parsed] * 20, "full"))

    def test_missing_evidence_produces_parseable_inconclusive_json(self):
        with tempfile.TemporaryDirectory() as folder:
            root = pathlib.Path(folder)
            report = root / "result.json"
            result = subprocess.run([
                sys.executable, str(FILE), "--baseline", str(root / "missing-base"),
                "--candidate", str(root / "missing-candidate"),
                "--output", str(report),
            ], text=True, capture_output=True, check=True)
            self.assertEqual(result.returncode, 0)
            parsed = json.loads(report.read_text())
            self.assertEqual(parsed["status"], "INCONCLUSIVE")
            self.assertTrue(parsed["missing_files"])

    def test_process_heap_is_not_attributable_retained_memory(self):
        with tempfile.TemporaryDirectory() as folder:
            log = pathlib.Path(folder) / "01.log"
            log.write_text(
                "BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium-4 "
                "1200 900 ns/op 10000 heap-live-B 900 cpu-ns/op "
                "1000 ops/s 1 allocs/op 100 p50-ns 200 p95-ns 300 p99-ns\\n"
            )
            metrics = next(iter(MOD.parse_benchmark(log).values()))
            self.assertEqual(metrics["cpu_ns_op"], 900)
            self.assertIsNone(metrics["retained_bytes"])

    def test_go_fixture_and_exact_selected_scenarios_match(self):
        fixture = FILE.parents[1] / "qa005_qualification_benchmark_test.go"
        content = fixture.read_text(encoding="utf-8")
        pattern = re.compile(
            r'\{"(Core|Full)", "(RR|NoWait)", (\d+), (\d+), (\d+), "(medium|near)"\}'
        )
        found = {
            f"BenchmarkQA005Qualification/{m[1]}/{m[2]}-{m[3]}B-S{m[4]}-C{m[5]}-{m[6]}"
            for m in pattern.finditer(content)
        }
        self.assertEqual(found, MOD.EXPECTED_QA005_SCENARIOS)
        self.assertEqual(len(found), 24)

    def test_substituted_scenario_is_rejected_even_with_correct_counts(self):
        expected = sorted(name for name in MOD.EXPECTED_QA005_SCENARIOS if "/Core/" in name)
        log = {name: {} for name in expected}
        log.pop(expected[0])
        log["BenchmarkQA005Qualification/Core/RR-64B-S1-C1-unapproved"] = {}
        self.assertFalse(MOD.complete_matrix([log] * 20, "core"))

    def test_invalid_metadata_is_inconclusive_without_traceback(self):
        with tempfile.TemporaryDirectory() as folder:
            root = pathlib.Path(folder)
            baseline, candidate = root / "baseline", root / "candidate"
            for side in (baseline, candidate):
                side.mkdir()
                (side / "environment.json").write_text("{not-json\\n")
                for index in range(1, MOD.PAIRS + 1):
                    (side / f"{index:02d}.log").write_text("not benchmark evidence\\n")
            report = root / "comparison.json"
            completed = subprocess.run([
                sys.executable, str(FILE),
                "--baseline", str(baseline),
                "--candidate", str(candidate),
                "--output", str(report),
            ], text=True, capture_output=True, check=True)
            parsed = json.loads(report.read_text())
            self.assertEqual(parsed["status"], "INCONCLUSIVE")
            self.assertIn("malformed", parsed["reason"])
            self.assertNotIn("Traceback", completed.stderr)

    def test_duplicate_benchmark_in_one_log_refused(self):
        with tempfile.TemporaryDirectory() as folder:
            path = pathlib.Path(folder) / "01.log"
            line = "BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium-4 1200 900 ns/op"
            path.write_text(line + "\n" + line + "\n")
            with self.assertRaisesRegex(ValueError, "duplicate benchmark"):
                MOD.parse_benchmark(path)


if __name__ == "__main__":
    unittest.main()
