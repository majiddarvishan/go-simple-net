#!/usr/bin/env python3
"""Opt-in streaming observation of go-test output without altering raw bytes.

Arrival timestamps are Python observer timestamps, NOT benchmark-case start/end.
All stdout+stderr data (including invalid UTF-8 and unterminated last lines) is
copied verbatim into the canonical log. Ancillary line events are independently
hashed and must be validated before any later statistical interpretation.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import time

BENCHMARK_ROW = re.compile(rb"^(Benchmark\S+)\s+([0-9]+)(?:\s|$)")
CHUNK_BYTES = 65536
MAX_LINE_BYTES = 1024 * 1024


def observe_pipe(pipe, raw_log, event_file, state, *, chunk_bytes=CHUNK_BYTES):
    """Concurrent pipe consumer; fail closed on lost bytes or event capture errors.

    The child owns the pipe write end. This function runs concurrently with
    the parent's existing sampled-host-telemetry loop (in the opt-in path).
    raw_log and event_file are binary streams; events must be exclusive-create.
    """
    pending = bytearray()
    offset = 0
    line_no = 0
    sha = hashlib.sha256()
    event_sha = hashlib.sha256()
    last_observed_ns = None
    observed_count = 0

    def emit(raw_line, first_byte_offset, sample_before_ns, sample_after_ns, utc_ns):
        nonlocal line_no, last_observed_ns, observed_count
        line_no += 1
        match = BENCHMARK_ROW.match(raw_line.rstrip(b"\r\n"))
        if sample_after_ns < sample_before_ns:
            raise ValueError("Non-monotonic observer clock")
        if last_observed_ns is not None and sample_after_ns < last_observed_ns:
            raise ValueError("Observer time regressed")
        last_observed_ns = sample_after_ns
        record = {
            "line_index": line_no,
            "byte_offset": first_byte_offset,
            "length_bytes": len(raw_line),
            "line_sha256": hashlib.sha256(raw_line).hexdigest(),
            "read_start_monotonic_ns": sample_before_ns,
            "read_end_monotonic_ns": sample_after_ns,
            "read_end_utc_ns": utc_ns,
            "kind": "benchmark_result" if match else "other",
            "benchmark_name": match.group(1).decode("utf-8") if match else None,
            "benchmark_iterations": int(match.group(2)) if match else None,
        }
        encoded = (json.dumps(record, sort_keys=True, separators=(",", ":"))
                   + "\n").encode("utf-8")
        event_file.write(encoded)
        event_sha.update(encoded)
        observed_count += bool(match)

    try:
        # Never decode, translate newlines, reorder, redact or echo log bytes.
        while True:
            before = time.monotonic_ns()
            chunk = os.read(pipe.fileno(), chunk_bytes)
            after = time.monotonic_ns()
            utc = time.time_ns()
            if not chunk:
                break
            raw_log.write(chunk)
            sha.update(chunk)
            pending.extend(chunk)
            while True:
                pos = pending.find(b"\n")
                if pos == -1:
                    if len(pending) > MAX_LINE_BYTES:
                        raise ValueError("Unbounded output line; refuse timing attribution")
                    break
                if pos + 1 > MAX_LINE_BYTES:
                    raise ValueError("Unbounded output line; refuse timing attribution")
                raw_line = bytes(pending[:pos + 1])
                del pending[:pos + 1]
                emit(raw_line, offset, before, after, utc)
                offset += len(raw_line)
        if pending:
            emit(bytes(pending), offset, before, after, utc)
            offset += len(pending)
        raw_log.flush()
        event_file.flush()
        state.update({
            "complete": True,
            "bytes": offset,
            "lines": line_no,
            "benchmark_rows": observed_count,
            "log_sha256": sha.hexdigest(),
            "events_sha256": event_sha.hexdigest(),
            "timing_semantics": "stdout_line_arrival_bracket_not_case_execution_time",
        })
    except (OSError, ValueError, OverflowError, UnicodeError) as exc:
        state.update({"complete": False, "error": str(exc)})
    finally:
        pipe.close()


def cli():
    """Opt-in observer of unchanged child argv; never a qualification gate."""
    import argparse
    from pathlib import Path
    import subprocess
    import sys

    parser = argparse.ArgumentParser()
    parser.add_argument("--events", type=Path, required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("Missing source command")
    if args.events.exists():
        parser.error("Refusing to replace existing per-line timing evidence")
    args.events.parent.mkdir(parents=True, exist_ok=True)
    child = None
    try:
        with args.events.open("xb") as event_file:
            child = subprocess.Popen(command, stdout=subprocess.PIPE,
                                     stderr=subprocess.STDOUT)
            state = {}
            observe_pipe(child.stdout, sys.stdout.buffer, event_file, state)
            if not state.get("complete") and child.poll() is None:
                child.terminate()
            return_code = child.wait(timeout=10)
            if not state.get("complete"):
                print("ORD stream capture INCONCLUSIVE: " +
                      state.get("error", "unknown"), file=sys.stderr)
                return 2
            if return_code < 0:
                return 128 + -return_code
            return return_code
    except (OSError, subprocess.TimeoutExpired) as exc:
        if child is not None and child.poll() is None:
            child.kill()
            child.wait()
        print("ORD stream capture INCONCLUSIVE: " + str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    import sys
    sys.exit(cli())
