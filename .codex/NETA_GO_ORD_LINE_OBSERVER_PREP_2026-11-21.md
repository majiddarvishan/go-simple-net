# ORD-EVID case-line arrival observer — implementation and limits (2026-11-21)

**Status:** opt-in, report-only Phase-Q evidence-readiness increment; **NOT** a completed Mode admission, A/A campaign, E-VALID proof, approved baseline or Performance Gate. Normative parent: [QENV-AA-02.ORD](NETA_GO_QENV_AA_ORDER_DRIFT_POLICY_2026-11-20.md) and [report-only 5m model](NETA_GO_ORD_EVID_REPORT_ONLY_MODEL_2026-11-20.md). Work item **QENV-AA-02.ORD-EVID remains OPEN**.

## What is captured

When the **existing** \`scripts/qa005_qualify.sh\` runner is explicitly invoked with \`NETA_QA005_ORD_EVID=1\`, it continues to use unchanged exact-SHA sources and benchmark fixture, mode, Go test flags, sample count, and \`scripts/qa005_compare.py\`. An additive Python observer \`qa005_ord_stream_capture.py\` relays the **exact bytes** of child \`go test\`'s merged stdout+stderr to the existing raw log and records one event for every LF-delimited line, including non-benchmark rows, invalid UTF-8, CRLF and unterminated final lines.

\`<pair>-<side>.lines.jsonl\` contains byte offset, line number, SHA-256 of the exact raw line, row kind, benchmark name and reported iterations, monotonic \`os.read\` start/end timestamps and UTC post-read timestamps. The observer script SHA-256 and full sidecar SHA-256 are bound into the existing ORD process record in \`qa005_ord_evidence.py\`. The independently executable \`qa005_ord_stream_verify.py\` re-parses the unchanged raw log and rejects absent, duplicated, reordered or modified case rows, invalid JSON types, mismatched hashes, or out-of-process/nonmonotonic receipt timestamps. The analyzer \`qa005_ord_drift.py\` reads those verification results **before** fitting the existing 5m order/time model. The 20-run synthetic QA004 test produces sidecars with matching integrity evidence.

All reports unconditionally retain **\`qualification=DRIFT-INCONCLUSIVE\`, \`environment_validity=E-UNKNOWN\`, \`host_mode_admission=NOT_PERFORMED\` and \`performance_gate=DISABLED\`**. A numeric \`WITHIN_FIXED_BOUNDS_UNQUALIFIED\` value, if present, is a diagnostic only.

## Interpret timestamps conservatively

A \`benchmark_result\` event proves only that its complete output line was received by the Python observer at the measured receipt bracket. It does **not** measure the benchmark's execution start/end or its internal \`testing.B\` iteration interval, nor prove that the observer received bytes immediately after the result was computed. Go stdout buffering, Python scheduling, pipe blocking, GC/CPU interference, mixed stdout/stderr, process startup, benchmark warm-up and source logger buffering may delay observation.

The only safe per-case time envelope without further proof is **[observer-parent process launch, receipt]**, which can be extremely wide for late cases. \`precision_proved\` is **always false**. The 5m statistical fit still uses conservative process-pair midpoint proxy; observed line arrivals are additional evidence, not magically precise per-case covariates. No earlier measured A/A archive can be promoted to VALID by retroactive timestamp inference.

The observer is itself a workload perturbation (one extra child process, pipe, JSON event write and SHA computations). This instrumentation must not be assumed to have zero observer effect; an independently approved overhead/interference study, fixed controlled host, high-integrity CPU/frequency/thermal/PSI/cgroup/swap telemetry, per-case time sensitivity, nonlinear/model adequacy and dedicated-host owner review remain prerequisites. No source, tag, numeric margin or wire API is changed.

## Lightweight verification (never heavy A/A)

The dedicated \`.github/workflows/phase-q-ord-evid.yml\` runs:

\`\`\`sh
bash -n scripts/qa005_qualify.sh
PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s scripts -p 'test_qa005_ord_*.py' -v
\`\`\`

It also checks default comparator/command retention, collector/observer Python syntax and previous Go/C++ workflow checks through branch protection. The tests cover raw-byte parity, invalid UTF-8, NUL, CRLF, trailing partial line, duplicate/missing or altered case/evidence, Boolean-as-integer spoofing, sample time ordering, exact 20-run synthetic QA004 provenance and no automatic admission. Tests do **not** run production Go qualification benchmarks.

## Still OPEN

- Bound unobserved time between actual end of benchmark computation and observer receipt, including Go internal buffering.
- Demonstrate case-level time-sensitivity robustness under those intervals, non-linear drift/model adequacy, and complete controlled reference-host sensor coverage.
- Calibrate observer overhead and backpressure independently without assuming it is negligible.
- Separately authorize real 10-pair QA004/Core/Full host qualification; full memory attribution and retained-memory/soak gates are also still open.

**Task accounting is unchanged:** 18 normalized open top-level packages = 9 Qualification, 7 Owner Decisions, 2 Release. Q-COR-01 closed; ORD-EVID open. No new tag, release, benchmark run, numeric SLA, product regression or Performance Gate activation.
