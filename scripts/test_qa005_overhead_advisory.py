"""Fail-closed QA-005 short overhead feasibility parser tests."""
import copy
import importlib.util
import pathlib
import unittest

SOURCE = pathlib.Path(__file__).with_name("qa005_overhead_advisory.py")
spec = importlib.util.spec_from_file_location("qa005_overhead_advisory", SOURCE)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


def fixture():
    return {"adapter_id": mod.ADAPTER_ID, "source_tree_mutated": False,
            "measurement_qualifies_retained_attributed_memory": False,
            "coverage_status": "PARTIAL_INCONCLUSIVE", "covered_categories": [],
            "source_fixture_sha256": "a"*64, "effective_fixture_sha256": "b"*64}


def logs():
    return {mode: (f"goos: linux\n"
                   f"BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium-4\t 25\t"
                   f"{n} ns/op\t {n*2} cpu-ns/op\t 22136 B/op\t 70 allocs/op\n"
                   f"PASS\n").encode()
            for mode, n in (("source", 1300), ("overlay_off", 1400), ("overlay_on", 1500))}


class OverheadAdvisoryTest(unittest.TestCase):
    def test_valid_three_way_preview_cannot_enable_gate(self):
        result = mod.summarize(logs(), fixture(), "a"*40, "go version go1.23.8 linux/amd64", "25x")
        self.assertEqual(result["status"], "FEASIBILITY_ONLY_INCONCLUSIVE")
        self.assertFalse(result["instrumentation_overhead_qualified"])
        self.assertFalse(result["attributable_retained_memory_qualified"])
        self.assertEqual(result["performance_gate"], "DISABLED")
        self.assertEqual(result["samples"]["source"]["B/op"], 22136)
        self.assertGreater(result["illustrative_incremental_sampler_nsop_ratio"], 1)
        self.assertEqual(len(result["raw_log_sha256"]["overlay_on"]), 64)

    def test_rejects_missing_or_duplicate_or_failed_benchmark(self):
        for bad in (b"PASS\n", logs()["source"]*2,
                    b"--- FAIL: TestSomething\n" + logs()["source"],
                    logs()["source"].replace(b"25", b"0", 1),
                    logs()["source"].replace(b"1300 ns/op", b"0 ns/op")):
            with self.subTest(bad=bad[:30]):
                with self.assertRaises(ValueError):
                    mod.read_benchmark(bad, "source")

    def test_rejects_unapproved_adapter_or_coverage(self):
        for key, value in (("adapter_id", "unknown"),
                           ("coverage_status", "COMPLETE"),
                           ("covered_categories", ["F1"]),
                           ("measurement_qualifies_retained_attributed_memory", True)):
            with self.subTest(key=key):
                manifest = fixture()
                manifest[key] = value
                with self.assertRaises(ValueError):
                    mod.summarize(logs(), manifest, "a"*40, "go version go1.23.8 linux/amd64", "25x")

    def test_rejects_mismatched_benchtime(self):
        with self.assertRaisesRegex(ValueError, "iteration count"):
            mod.summarize(logs(), fixture(), "a"*40, "go version go1.23.8 linux/amd64", "50x")
        with self.assertRaisesRegex(ValueError, "all three"):
            mod.summarize({"source": logs()["source"]}, fixture(), "a"*40,
                          "go version go1.23.8 linux/amd64", "25x")

    def test_no_retained_attributed_field_or_acceptance_path(self):
        raw = SOURCE.read_text()
        self.assertNotIn("retained-attributed-B", raw)
        self.assertNotIn('"performance_gate": "ENABLED"', raw)


if __name__ == "__main__":
    unittest.main()
