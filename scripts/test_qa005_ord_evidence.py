"""Fast, no-benchmark tests for the opt-in ORD-EVID collection wrapper."""
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).parent
SCRIPT = HERE / "qa005_ord_evidence.py"
ROOT = HERE.parent

class OrdEvidenceTests(unittest.TestCase):
    def invoke(self, root, pair=1, side="baseline", body="print('ord-fixture')"):
        out = pathlib.Path(root)
        command = [
            sys.executable, "-B", str(SCRIPT),
            "--output", str(out / "evidence"),
            "--log", str(out / "benchmark.log"),
            "--pair", str(pair),
            "--side", side,
            "--mode", "core",
            "--interval-ms", "100",
            "--", sys.executable, "-c", body,
        ]
        return subprocess.run(
            command, cwd=ROOT, text=True, capture_output=True, timeout=15
        )

    def test_success_keeps_log_and_fails_closed_on_host_admission(self):
        with tempfile.TemporaryDirectory() as directory:
            result = self.invoke(directory)
            self.assertEqual(result.returncode, 0, result.stderr)
            root = pathlib.Path(directory)
            self.assertEqual((root / "benchmark.log").read_text(), "ord-fixture\n")
            record = json.loads((root / "evidence/01-baseline.json").read_text())
            self.assertEqual(record["schema"], "neta-qa005-ord-process-evidence-v1")
            self.assertEqual(record["pair"], 1)
            self.assertEqual(record["side"], "baseline")
            self.assertEqual(record["mode"], "core")
            self.assertEqual(record["exit_code"], 0)
            self.assertTrue(record["complete"])
            self.assertEqual(record["qualification"], "DRIFT-INCONCLUSIVE")
            self.assertLessEqual(
                record["start_before_spawn"]["monotonic_ns"],
                record["start_after_spawn"]["monotonic_ns"],
            )
            self.assertLessEqual(
                record["start_after_spawn"]["monotonic_ns"],
                record["exit_after_wait"]["monotonic_ns"],
            )
            self.assertGreaterEqual(record["sample_count"], 3)
            self.assertEqual(len(record["log_sha256"]), 64)
            self.assertEqual(len(record["trace_sha256"]), 64)
            trace = (root / "evidence/01-baseline.jsonl").read_text().splitlines()
            self.assertEqual(len(trace), record["sample_count"])
            self.assertEqual(json.loads(trace[0])["kind"], "before_spawn")

    def test_nonzero_exit_is_recorded_and_not_qualified(self):
        with tempfile.TemporaryDirectory() as directory:
            result = self.invoke(directory, body="import sys; sys.exit(7)")
            self.assertEqual(result.returncode, 7, result.stderr)
            record = json.loads(
                (pathlib.Path(directory) / "evidence/01-baseline.json").read_text()
            )
            self.assertFalse(record["complete"])
            self.assertEqual(record["exit_code"], 7)
            self.assertEqual(record["qualification"], "DRIFT-INCONCLUSIVE")

    def test_existing_evidence_refused_without_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            original = self.invoke(directory)
            self.assertEqual(original.returncode, 0, original.stderr)
            before = (pathlib.Path(directory) / "benchmark.log").read_bytes()
            duplicate = self.invoke(directory)
            self.assertNotEqual(duplicate.returncode, 0)
            self.assertEqual(
                (pathlib.Path(directory) / "benchmark.log").read_bytes(), before
            )

    def test_out_of_range_pair_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            result = self.invoke(directory, pair=11)
            self.assertEqual(result.returncode, 2)
            self.assertFalse((pathlib.Path(directory) / "benchmark.log").exists())

if __name__ == "__main__":
    unittest.main()
