import importlib.util
import pathlib
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
                "250 allocs/op 300000 ops/s 400 p50-ns 900 p95-ns 1300 p99-ns 0.5 gc-cycles\\n"
            )
            result = MOD.parse_benchmark(log)
            self.assertEqual(len(result), 1)
            self.assertEqual(next(iter(result.values()))["p99_ns"], 1300.0)


if __name__ == "__main__":
    unittest.main()
