"""No implied numerical gate: only exact source/effective adapter equivalence."""
import copy
import importlib.util
import pathlib
import unittest

SCRIPT = pathlib.Path(__file__).with_name("qa005_overlay_compatibility.py")
spec = importlib.util.spec_from_file_location("qa005_overlay_compatibility", SCRIPT)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


def fixture():
    return {
        "adapter_id": mod.ADAPTER_ID,
        "qualification": "DIAGNOSTIC_ONLY",
        "measurement_qualifies_retained_attributed_memory": False,
        "source_tree_mutated": False,
        "coverage_status": "PARTIAL_INCONCLUSIVE",
        "covered_categories": [],
        "observed_partial_categories": ["F1", "F2", "F3", "F4"],
        "excluded_categories": ["metadata", "foreign graphs"],
        "coverage_exclusions_detail": {"M1": "unknown"},
        "sampler_interval_ms": 5,
        "source_file_sha256": {k: "a"*64 for k in mod.SOURCE_FILES},
        "effective_file_sha256": {k: "b"*64 for k in mod.SOURCE_FILES},
        "source_fixture_sha256": "c"*64,
        "effective_fixture_sha256": "d"*64,
    }


class OverlayCompatibilityTest(unittest.TestCase):
    def test_identical_source_and_effective_contract_is_compatibility_only(self):
        a = fixture()
        status = mod.check_pair(a, copy.deepcopy(a))
        self.assertEqual(status["source_effective_compatibility"], "PASS")
        self.assertEqual(status["attributable_memory_qualification"], "INCONCLUSIVE")
        self.assertEqual(status["performance_gate"], "DISABLED")

    def test_rejects_original_source_and_effective_adapter_drift(self):
        for field in ("source_fixture_sha256", "effective_fixture_sha256"):
            with self.subTest(field=field):
                a, b = fixture(), fixture()
                b[field] = "e"*64
                with self.assertRaisesRegex(ValueError, "mismatch"):
                    mod.check_pair(a, b)
        a, b = fixture(), fixture()
        b["effective_file_sha256"]["session_control.go"] = "e"*64
        with self.assertRaisesRegex(ValueError, "effective_file_sha256"):
            mod.check_pair(a, b)

    def test_rejects_empty_or_incomplete_coverage_claim(self):
        for key, value in (("coverage_status", "COMPLETE"), ("covered_categories", ["F1"]),
                           ("measurement_qualifies_retained_attributed_memory", True)):
            with self.subTest(key=key):
                a = fixture()
                a[key] = value
                with self.assertRaises(ValueError):
                    mod.check_manifest(a, "baseline")

    def test_rejects_missing_digests_unreviewed_version(self):
        a = fixture()
        del a["source_file_sha256"]["session.go"]
        with self.assertRaisesRegex(ValueError, "file set"):
            mod.check_manifest(a, "baseline")
        b = fixture()
        b["adapter_id"] = "V0"
        with self.assertRaisesRegex(ValueError, "adapter version"):
            mod.check_manifest(b, "candidate")


if __name__ == "__main__":
    unittest.main()
