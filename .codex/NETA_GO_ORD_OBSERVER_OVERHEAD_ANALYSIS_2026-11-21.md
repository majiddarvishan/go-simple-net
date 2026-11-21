# ORD-EVID observer overhead and pipe backpressure analysis (2026-11-21)

**Status:** source-level analysis plus synthetic transport tests. Observer overhead is **NOT MEASURED**; `overhead_proved=false`; backpressure is possible. No Reference Host or uninstrumented control comparison was run. All qualification reports remain `DRIFT-INCONCLUSIVE`.

## Actual capture path

The process launched by the evidence collector is the capture wrapper. The wrapper launches the benchmark child with `stdout=PIPE` and merges stderr into that pipe. The wrapper's reader loop then:

1. timestamps immediately before `os.read`;
2. reads up to `CHUNK_BYTES = 65536`;
3. timestamps immediately after the read and captures UTC;
4. forwards the raw chunk to the wrapper's stdout (the raw log sink);
5. scans buffered bytes for complete lines and writes JSONL sidecar events.

The raw-log forwarding and sidecar event writes happen in the same reader process that drains the child pipe. A slow/blocking raw-log sink or sidecar write can stop the reader from draining the pipe. Once the pipe fills, the benchmark child can block on stdout writes. Scheduling delay of the wrapper can have the same effect. Therefore this is an **inline observer**, not a zero-overhead tap; its effect on benchmark throughput is unknown until measured on the approved host.

The wrapper enforces a 1 MiB maximum line length. The current synthetic slow-reader test uses a 512 KiB preamble line to exercise output larger than ordinary pipe capacity, while preserving bytes and avoiding a long stress campaign.

## Timestamp semantics

- `read_start_monotonic_ns`: immediately before the `os.read` call.
- `read_end_monotonic_ns`: immediately after that `os.read` call.
- `read_end_utc_ns`: wall-clock sample immediately after the read.
- Multiple lines completed from the same read chunk share that chunk's read bracket.
- If a line spans multiple chunks, its event is emitted when the newline arrives and carries the bracket of the chunk that completes the line; `read_start` is not the first-byte arrival time.
- These values are observer receipt/read brackets, not child `write()` timestamps and not benchmark case start/end timestamps.

The analyzer continues to use the outer pre-spawn clock as the conservative earliest possible case-time bound and the line receipt as the latest bound. The observer's read-start field is not used as a case start estimate. The continuous case-time enclosure remains a candidate and is not independently proved.

## Synthetic checks

The added tests:
- verify that two benchmark lines delivered in one pipe read share the same read bracket;
- send a 512 KiB line plus a benchmark row through the capture wrapper while the downstream reader deliberately drains slowly;
- verify byte-for-byte raw output, both line events, and successful process completion.

These tests validate transport semantics and byte preservation only. They do not measure throughput overhead, scheduling perturbation, CPU cost, sidecar write latency, or sampling accuracy on a Reference Host.

## Report contract

The report now exposes `observer_overhead`:
- `measurement_status = NOT_MEASURED`;
- `overhead_proved = false`;
- `backpressure_possible = true`;
- inline raw-log and sidecar writes;
- read chunk and maximum line sizes; and
- explicit timestamp semantics.

No numeric overhead estimate or negligible-overhead claim is emitted.

## Remaining qualification work

On the approved Reference Host, a controlled observer-on/observer-off paired comparison is still required to estimate throughput and latency perturbation, CPU consumption, and scheduling effects. It must preserve the exact benchmark binary/configuration, workload, host state and sampling window, and must not be substituted with synthetic timing. Sampling cadence and telemetry coverage also require host evidence and owner-approved acceptance limits.

No heavy benchmark, A/A, stress, soak, extended fuzz, new baseline/SLA, Performance Gate, memory PASS, public API/wire, v1.0.0 source or tag change is included here.
