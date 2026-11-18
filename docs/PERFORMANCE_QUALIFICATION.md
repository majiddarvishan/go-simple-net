# QA-004 / QA-005 — performance qualification protocol

**Status:** instrumentation and comparison tooling implemented; no reference environment or mandatory performance gate approved. GitHub-hosted results are advisory only.

## Measurement population

QA-004 preserves the existing linear pending deadline scan, shared bounded timer manager and absolute Request deadlines. `BenchmarkPendingDeadlineScan` measures the scan at pending populations 32/128/512/2048/8192, along with historical 0/1 cases. `BenchmarkPendingDeadlineDetection` timestamps the internal terminal Timeout settlement separately from Completion callback start; 2048/8192 are explicitly synthetic Session fixtures, never valid single-Session admitted traffic. Per-Session real outstanding Request credit remains capped at 512.

QA-005 workload selection is fixed in `qa005_qualification_benchmark_test.go`: 8 Core and 24 Full total selected scenarios combining payloads 64 B/4 KiB/64 KiB, 1/32/256 Sessions, concurrency 1/4, Request/Response and SendNoWait, and medium versus elevated near-capacity offered load. Near capacity indicates offered workload only; qualification must also report actual admission, saturation and resource counters.

## Running the evidence

For a short advisory snapshot on an unqualified workstation:

```bash
make qa005-report
bash scripts/qa005_report.sh full  # all 24 selected workloads; advisory only
go test -run '^$' -bench '^BenchmarkPendingDeadlineScan$' -benchmem -count=3 -cpu=1,4
go test -run '^$' -bench '^BenchmarkPendingDeadlineDetection$' -benchtime=1200x -benchmem -count=3 -cpu=4
```

For the owner-approved stable reference machine, use two separate checked-out exact-SHA source trees (baseline and candidate), equivalent toolchain and host:

```bash
bash scripts/qa005_qualify.sh /path/to/baseline /path/to/candidate /path/to/output core
bash scripts/qa005_qualify.sh /path/to/baseline /path/to/candidate /path/to/output-full full
bash scripts/qa005_qualify.sh /path/to/baseline /path/to/candidate /path/to/output-deadline qa004
```

Each mode alternates baseline/candidate order across **10 matched pairs**, captures environment metadata and a JSON statistical report. The qa004 mode compares the five actual *terminal-settlement detection* P99 cases with the separately approved **15% relative threshold**; it uses at least 1,200 observations per population in each run. The Core and Full modes use their own distinct traffic metrics; they never demand QA-004 timing measurements inside unrelated QA-005 cases.

Preflight rejects a baseline or candidate without identical `qa005_qualification_benchmark_test.go` / `benchmark_test.go` (or the corresponding QA-004 fixture), or with modified tracked source files. **The original main commit before these fixtures were introduced cannot serve as an executable matched baseline by itself.** Source SHA, exact scenario mode and a benchmark-source SHA-256 fingerprint are recorded and checked for equal test definitions. A missing or mismatched fixture is not a passing comparison; the script exits before costly paired work. The comparator uses conservative Bonferroni-adjusted two-sided 95% confidence intervals over paired log ratios, and labels missing metrics, unequal environments, insufficient P99 samples or missing owner-approved baseline as INCONCLUSIVE. It never treats missing metrics as zero regression. `cpu-ns/op` uses Linux process user+system CPU time sampled *outside* the workload hot path; no metric is emitted where that source is unavailable. `detect-p99-ns` must come from actual internal timeout settlement. Wall-clock `ns/op` is not an acceptable CPU/detection substitute. `heap-live-B` is a process-wide instantaneous value **not** attributable retained library bytes; absent an accepted `retained-attributed-B` measurement, that metric stays **INCONCLUSIVE** rather than inventing a PASS.

**30-minute stability:** run `NETA_SOAK_SECONDS=1800 go test -count=1 -run '^TestMemoryStabilitySoak$' -timeout=34m -v` on the qualified machine. Shorter checks are development evidence only. The hosted final verification workflow does **not** automatically spend thirty minutes on this exercise.

## Statistical acceptance and baseline custody

Relative tolerances: 10% for throughput and P50/P95, and 15% for P99, CPU/op, allocations/op, retained memory and deadline-detection P99. Three outcomes: PASS, FAIL or INCONCLUSIVE. A regressed metric fails only when its lower bound exceeds tolerance; passes only when the upper bound is within tolerance. Zero/near-zero baselines require an owner-approved meaningful absolute-delta policy, otherwise INCONCLUSIVE.

The owner has **not** approved a stable qualification machine, an absolute P99 Deadline Detection limit, absolute throughput/latency SLAs, or an immutable reference baseline. Performance gates remain **DISABLED**. The comparator can only consider a baseline approved when a separately maintained, immutable owner-signed/reviewed manifest includes `owner_approved=true`, exact baseline source SHA, reference environment fields and the exact SHA-256 digest of the ten ordered baseline logs. Do not alter historical logs or manifests in place: add a newly versioned baseline after explicit owner authorization, preserving all previous versions. Neither a branch push nor a merge to main auto-promotes a baseline.

The feature-branch CI runs Go correctness, race and pinned C++ V1 golden compatibility plus advisory scan, 1,200-observation QA-004 deadline detection and both Core/Full QA-005 measurements. The P99 sample count is a minimum sampling safeguard, not statistical proof or an absolute SLA. It cannot replace an approved reference environment. Record real logs and results; do not claim full qualification from the hosted runner alone.

## Compatibility boundaries

No heap/wheel deadline redesign; no hot-path benchmark instrumentation except a nil internal terminal-settlement observation hook exclusively used by synthetic tests. V2 default Bind without NUL and 20-byte SystemID, V1 NUL Bind with 19-byte SystemID and 10-byte Header are unchanged. No PR, merge, tag or release without separate owner permission.
