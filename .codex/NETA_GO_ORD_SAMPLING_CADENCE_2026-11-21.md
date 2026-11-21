# ORD-EVID sampling cadence integrity and gap diagnostics (2026-11-21)

**Status:** software-only evidence integrity and report diagnostics. The recorded cadence and observed gaps are surfaced, but `sampling_accuracy_proved=false` and `acceptance_limit=NOT_ESTABLISHED`. This does not qualify host sampling accuracy and does not change `DRIFT-INCONCLUSIVE`.

## Provenance binding

The collector already accepts `--interval-ms` in the bounded range 100..5000 ms and records `sampling_interval_ms` in its process record. This increment also writes the same exact integer into every raw telemetry snapshot. The verifier requires:
- exact integer type (Boolean and float lookalikes rejected);
- supported 100..5000 ms range;
- every snapshot value equal to the process record; and
- one consistent configured interval across all 20 A/A process records.

The snapshot copies are covered by the raw trace SHA-256, so the recorded cadence is now bound into the trace content rather than existing only as an unbound process-record field. This is integrity consistency, not cryptographic signing of the entire evidence bundle.

## Observed gap statistics

The verifier still checks the maximum across **all** adjacent snapshot starts against the collector's raw-trace summary. For cadence statistics, it separately uses only gaps ending at a `during` snapshot (the scheduled polling samples). It excludes the immediate `before_spawn → after_spawn` and final `during → after_exit` boundary gaps, which are not scheduled polling intervals. It reports, per process:
- scheduled-gap count;
- median scheduled gap (integer or half-nanosecond result);
- nearest-rank p95 scheduled gap;
- maximum scheduled gap; and
- maximum-scheduled-gap / recorded-interval ratio.

The report-level `sampling_accuracy` block also reports the maximum all-snapshot boundary gap separately, identifies the collector-recorded interval, lists the 20 run summaries, and explicitly carries:
- `measurement_status = OBSERVED_UNQUALIFIED`;
- `acceptance_limit = NOT_ESTABLISHED`; and
- `sampling_accuracy_proved = false`.

No pass/fail threshold is invented. A large ratio is visible evidence for review, not an automatic host-admission decision.

## Tests

Synthetic tests cover Boolean/float/out-of-range interval values, a snapshot interval differing from its record, cross-run interval mismatch, gap distribution calculations, and an actual short collector invocation proving that every emitted snapshot carries the recorded interval. The short invocation uses a trivial child command and does not run a benchmark.

## Limits and remaining work

The measured gaps include collector scheduling, snapshot execution, OS scheduling and possible observer/host contention. They do not isolate the observer's own overhead. Actual sampling accuracy, acceptable gap limits, and the effect of observer-on/off execution require a controlled Reference-Host study and owner-approved thresholds. No heavy benchmark, A/A, stress, soak, extended fuzz, baseline/SLA, Performance Gate, memory PASS, public API/wire, v1.0.0 source or tag change is included.
