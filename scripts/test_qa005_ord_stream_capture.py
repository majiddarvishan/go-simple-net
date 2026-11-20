"""Fast no-benchmark streaming tests; never qualifies a host or Mode."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

from qa005_ord_stream_capture import observe_pipe

SCRIPT = Path(__file__).with_name("qa005_ord_stream_capture.py")


class StreamingObserverTests(unittest.TestCase):
    def run_data(self, data, *, chunk_size=7):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            raw = directory / "raw.log"
            events = directory / "lines.jsonl"
            r, w = os.pipe()
            with os.fdopen(r, "rb", buffering=0) as source, raw.open("xb") as log, events.open("xb") as sidecar:
                state = {}
                reader = threading.Thread(
                    target=observe_pipe, args=(source, log, sidecar, state),
                    kwargs={"chunk_bytes": chunk_size}, daemon=True
                )
                reader.start()
                with os.fdopen(w, "wb", buffering=0) as output:
                    for at in range(0, len(data), 11):
                        output.write(data[at:at + 11])
                reader.join(timeout=5)
                self.assertFalse(reader.is_alive(), "observer deadlocked")
            raw_data = raw.read_bytes()
            event_data = events.read_bytes()
            records = [json.loads(x) for x in event_data.splitlines()]
            return raw_data, records, state, event_data

    def test_preserves_every_byte_and_benchmark_order_across_chunks(self):
        payload = (
            b"unknown:\xff\x00\r\n"
            b"BenchmarkPendingDeadlineDetection/synthetic-pending=32-4 1200 1000 ns/op\n"
            b"BenchmarkQA005Qualification/Core/Peer-4 200 10 ns/op\r\n"
            b"not-ended"
        )
        raw, rows, state, events = self.run_data(payload)
        self.assertEqual(raw, payload)
        self.assertTrue(state["complete"], state)
        self.assertEqual(state["bytes"], len(payload))
        self.assertEqual(state["log_sha256"], hashlib.sha256(payload).hexdigest())
        self.assertEqual(state["events_sha256"], hashlib.sha256(events).hexdigest())
        self.assertEqual(state["benchmark_rows"], 2)
        self.assertEqual(
            [r["kind"] for r in rows],
            ["other", "benchmark_result", "benchmark_result", "other"],
        )
        self.assertEqual([r["line_index"] for r in rows], [1, 2, 3, 4])
        self.assertEqual(rows[1]["benchmark_iterations"], 1200)
        self.assertEqual(
            rows[1]["benchmark_name"],
            "BenchmarkPendingDeadlineDetection/synthetic-pending=32-4",
        )
        self.assertEqual(sum(r["length_bytes"] for r in rows), len(payload))
        self.assertEqual(
            [r["byte_offset"] for r in rows],
            [
                0,
                rows[0]["length_bytes"],
                rows[0]["length_bytes"] + rows[1]["length_bytes"],
                len(payload) - rows[3]["length_bytes"],
            ],
        )
        for row in rows:
            self.assertLessEqual(
                row["read_start_monotonic_ns"], row["read_end_monotonic_ns"]
            )
            self.assertEqual(
                row["benchmark_name"] is not None,
                row["kind"] == "benchmark_result",
            )

    def test_subprocess_cli_preserves_binary_output_and_exit_status(self):
        with tempfile.TemporaryDirectory() as directory:
            events = Path(directory) / "lines.jsonl"
            payload = b"\xff\x00BenchmarkName-4 20 100 ns/op\r\n"
            cmd = [
                sys.executable, "-B", str(SCRIPT), "--events", str(events),
                "--", sys.executable, "-c",
                "import sys;sys.stdout.buffer.write(%r);sys.exit(7)" % payload,
            ]
            result = subprocess.run(
                cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10
            )
            self.assertEqual(result.returncode, 7, result.stderr)
            self.assertEqual(result.stdout, payload)
            first = events.read_bytes()
            again = subprocess.run(
                cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10
            )
            self.assertNotEqual(again.returncode, 0)
            self.assertEqual(events.read_bytes(), first)

    def test_subprocess_cli_benchmark_line_observed(self):
        with tempfile.TemporaryDirectory() as directory:
            events = Path(directory) / "lines.jsonl"
            payload = b"BenchmarkPendingDeadlineDetection/synthetic-pending=32-4 1200 123 ns/op\n"
            cmd = [
                sys.executable, "-B", str(SCRIPT), "--events", str(events),
                "--", sys.executable, "-c",
                "import sys;sys.stdout.buffer.write(%r)" % payload,
            ]
            result = subprocess.run(
                cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, payload)
            rows = [json.loads(s) for s in events.read_text().splitlines()]
            self.assertEqual(rows[0]["kind"], "benchmark_result")
            self.assertEqual(rows[0]["benchmark_iterations"], 1200)

    def test_empty_data_is_not_a_qualification_result(self):
        raw, rows, state, _ = self.run_data(b"")
        self.assertEqual(raw, b"")
        self.assertEqual(rows, [])
        self.assertTrue(state["complete"])
        self.assertEqual(state["benchmark_rows"], 0)

    def test_single_unterminated_line(self):
        raw, rows, state, _ = self.run_data(b"BenchmarkOne-4 1 9 ns/op")
        self.assertTrue(state["complete"])
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["length_bytes"], len(raw))
        self.assertEqual(rows[0]["kind"], "benchmark_result")

    def test_long_line_rejected_without_qualification(self):
        import qa005_ord_stream_capture as observer

        old = observer.MAX_LINE_BYTES
        observer.MAX_LINE_BYTES = 100
        try:
            _, _, state, _ = self.run_data(b"x" * 150)
            self.assertFalse(state["complete"])
            self.assertIn("Unbounded", state["error"])
        finally:
            observer.MAX_LINE_BYTES = old


if __name__ == "__main__":
    unittest.main()
