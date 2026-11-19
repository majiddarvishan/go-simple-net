# QA-004 / QA-005 — performance qualification protocol

**Status:** instrumentation and comparison tooling implemented; no reference environment or mandatory performance gate approved. GitHub-hosted results are advisory only.

**Worktree cleanup finding:** Owner follow-up `git status --short --ignored` on the detached source checkout showed only `?? scripts/__pycache__/`. The offline Python `importlib` stats validator had generated untracked bytecode in that worktree. Commit `68c861233a280cac3929c0d3db85e9d10dbe94cf` disables writing bytecode in future local reference captures; this does not alter the pinned benchmark source SHA, collected metric values or qualification status. Old temporary cache cleanup remains a local action, not performed by this repository edit.

**Most recent owner-local evidence:** [source-v1 draft numeric capture, 370 benchmark rows](performance-baselines/QA004_QA005_SOURCE_V1_NUMERIC_CAPTURE_2026-11-18.md); 30/30 benchmark processes and source/digest checks complete, but overall collector exit **FAIL from temporary worktree cleanup**. Benchmark numeric data is **DRAFT/ADVISORY**, not owner-approved. Do not auto-promote or infer reference hardware stability from the completed capture.


## Measurement population

QA-004 preserves the existing linear pending deadline scan, shared bounded timer manager and absolute Request deadlines. `BenchmarkPendingDeadlineScan` measures the scan at pending populations 32/128/512/2048/8192, along with historical 0/1 cases. `BenchmarkPendingDeadlineDetection` timestamps the internal terminal Timeout settlement separately from Completion callback start; 2048/8192 are explicitly synthetic Session fixtures, never valid single-Session admitted traffic. Per-Session real outstanding Request credit remains capped at 512.

QA-005 workload selection is fixed in `qa005_qualification_benchmark_test.go`: 8 Core and 24 Full total selected scenarios combining payloads 64 B/4 KiB/64 KiB, 1/32/256 Sessions, concurrency 1/4, Request/Response and SendNoWait, and medium versus elevated near-capacity offered load. Near capacity indicates offered workload only. Each *successful* benchmark now emits actual accepted operation count, offered producer count, zero observed capacity rejections (any rejected operation fails the benchmark), and owner-ledger **post-workload** retained-Application/Completion bytes, queued bytes, outstanding Requests and OnClose credits. These snapshots are diagnostic and **not** a peak memory measure or proof of capacity saturation. A dedicated attributable retained-memory peak/distribution method and its acceptance rules remain unresolved; do not use post-workload zero values as the 15%-threshold retention metric.

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

The comparator requires the **exact selected 8 Core / 24 Full benchmark names**, not just the same scenario count; a Python unit test independently checks that the names match the Go benchmark fixture. Missing, malformed, duplicate or unreadable logs and invalid environment metadata produce an explicit machine-readable **INCONCLUSIVE** result, never a passing result.

Preflight rejects a baseline or candidate without identical `qa005_qualification_benchmark_test.go` / `benchmark_test.go` (or the corresponding QA-004 fixture), or with modified tracked source files. **The original main commit before these fixtures were introduced cannot serve as an executable matched baseline by itself.** Source SHA, exact scenario mode and a benchmark-source SHA-256 fingerprint are recorded and checked for equal test definitions. A missing or mismatched fixture is not a passing comparison; the script exits before costly paired work. The comparator uses conservative Bonferroni-adjusted two-sided 95% confidence intervals over paired log ratios, and labels missing metrics, unequal environments, insufficient P99 samples or missing owner-approved baseline as INCONCLUSIVE. It never treats missing metrics as zero regression. `cpu-ns/op` uses Linux process user+system CPU time sampled *outside* the workload hot path; no metric is emitted where that source is unavailable. `detect-p99-ns` must come from actual internal timeout settlement. Wall-clock `ns/op` is not an acceptable CPU/detection substitute. `heap-live-B` is a process-wide instantaneous value **not** attributable retained library bytes; absent an accepted `retained-attributed-B` measurement, that metric stays **INCONCLUSIVE** rather than inventing a PASS.

**30-minute stability:** run `NETA_SOAK_SECONDS=1800 go test -count=1 -run '^TestMemoryStabilitySoak$' -timeout=34m -v` on the qualified machine. Shorter checks are development evidence only. The hosted final verification workflow does **not** automatically spend thirty minutes on this exercise.

## Local A/A noise probe — before approving a reference machine

The owner can evaluate repeatability on a Linux workstation **without** selecting or approving a historical performance baseline:

```bash
bash scripts/qa005_noise_probe.sh core
# Optional, slower and independent:
# bash scripts/qa005_noise_probe.sh full
# bash scripts/qa005_noise_probe.sh qa004
```

The probe requires a clean tracked source tree and no untracked Go files. It uses the exact current SHA to create **two isolated detached Git worktrees** in temporary storage, runs ten alternated A/A pairs (identical code on both sides) through the existing comparator, records CPU governor, CPU/load, RAM, pressure and swap activity counters before and after, removes the temporary worktrees, and emits **one .tar.gz evidence bundle** under `artifacts/qa005-noise-probe/`. It does not fetch/push, switch the owner's branch, overwrite a benchmark baseline, modify production source or invoke GitHub Actions. Upload the file named by `SEND THIS ONE FILE:` for review.

A/A comparison is a **measurement-noise characterization only**. `comparison.json` is intentionally `INCONCLUSIVE` on qualification without an owner-approved manifest, and `noise-summary.json` separately counts observed PASS/FAIL/INCONCLUSIVE relative metrics **excluding absent attributable-retained-memory measurements**. An A/A FAIL suggests instability or measurement noise; an A/A PASS does **not** prove reference-machine suitability, nor does any A/A result compare historical baseline against new code. A changing CPU governor, active swap movement, nontrivial system pressure and background load can undermine reproducibility; do not claim a qualified machine from CPU/RAM specifications alone. The probe changes neither the QA-004/QA-005 Task states nor the disabled performance gate.

The first owner-run full local suite (Go 1.27, Intel Core i5-12400, 12 logical CPUs, approximately 31 GiB RAM) is valuable correctness and advisory performance evidence, but its 30-minute soak does not establish low benchmark variance. A separate, approved historical baseline revision with the **same benchmark fixtures**, unchanged toolchain and environment, statistically paired runs, an attributable retained-memory method and explicit absolute SLAs remain necessary for final qualification. Do not fabricate a baseline by comparing incompatible benchmark fixtures or approving A/A as a regression baseline.
## Owner-local A/A Core evidence (2026-11-18, 10 pairs)

Owner-uploaded archive: `aa-core-20261118T150600Z-1846220.tar.gz`. Source SHA on **both** sides: `a6c2b1373e01b716bf8bf8f89d71abe0708630fe`. Local environment: Go 1.27.0/linux-amd64, Intel i5-12400, benchmark `-cpu=4`, baseline/candidate metadata equal; 8 Core workloads × 20 logs = 160 scenario output rows, matrix complete, 0 observed capacity rejections, and zero post-workload tracked owner-resource counters in all 160 rows. All ten pairs were executed in alternating order. No data establishes retained-attributed peak memory, a historical regression, absolute SLA, or approved reference baseline.

| Metric | A/A PASS | A/A INCONCLUSIVE | A/A FAIL |
|---|---:|---:|---:|
| Throughput | 5 | 3 | 0 |
| P50 | 6 | 2 | 0 |
| P95 | 3 | 5 | 0 |
| P99 | 4 | 4 | 0 |
| CPU/op | 5 | 3 | 0 |
| Allocs/op | 8 | 0 | 0 |
| **Measured subtotal** | **31** | **17** | **0** |
| Attributable retained memory (not instrumented) | 0 | 8 | 0 |

**Noise finding:** Among 20 independent runs per scenario, P99 sample coefficient of variation was ~3.8% for RR 4KiB/32 Sessions, ~4.4% for NoWait 4KiB/256 Sessions, but ~38.5% for RR 64B/1 Session, ~43% for NoWait 64B/1 Session, ~38.3% for NoWait 64KiB/1 Session and ~49.8% for RR 64KiB/1 Session. These are sample variability indicators, not regression values or guaranteed limits. A/A comparator overall `status=INCONCLUSIVE`, `baseline_approved=false`, `environment_comparable=true`, `matrix_complete=true`, `performance_gate=DISABLED`.

**Host-condition observations:** All 12 logical CPUs reported `powersave` scaling governor; memory ~31 GiB, about 7.2 GiB swap allocated both before and after. Between the two system snapshots, `pswpin` increased by only 6 and `pswpout` by 0; this does **not** establish active swap thrashing. CPU PSI `some` ~3% at start and lower at end, plus nonzero I/O PSI; simultaneous CPU governor/load/thermal or scheduler interference may contribute, but cannot be attributed conclusively without repeated controlled runs.

**Next diagnostic, not a decision:** run the same A/A Core protocol with a longer per-benchmark duration while holding source SHA/host otherwise stable. For Core or Full only, `NETA_QA005_BENCHTIME=1s bash scripts/qa005_noise_probe.sh core` is supported by a **new optional qualification-runner override** (commit `24c057c85c113dc248263ae8e38957469e6cd9a6`); default remains 250ms. The runner records the new effective duration in metadata, making a 1s dataset explicitly non-interchangeable with prior 250ms A/A evidence. QA-004 retains fixed 1200x samples and rejects this override. Collect before/after machine telemetry again. Neither an A/A improvement nor a 1s probe automatically approves the reference environment.
## Second owner-local A/A Core probe: 1s vs 250ms (2026-11-18)

Uploaded evidence: `aa-core-20261118T152119Z-1872550.tar.gz` (new, **1s**) and `aa-core-20261118T150600Z-1846220.tar.gz` (prior, **250ms**). Both complete 10 paired baseline/candidate runs with 8 Core cases, a total of 160 benchmark rows per archive, matching environment metadata within each probe, exact scenario matrix, and **identical code SHA on both sides of each A/A probe**. The previous A/A source SHA was `a6c2b1373e01b716bf8bf8f89d71abe0708630fe`, the 1s probe SHA `193d9c2e58e021319a92b4abb37cce69d8074dc1`, but their benchmark-fixture SHA-256 is **identical** (`6a3adf62a50508f5c5aef8ac6e062176523d20e9fae95c11e225bf1e1e937172`). The changed code between these revisions consists of tooling/docs, not the production performance workload.

| Status for 48 observable non-retained-memory metrics | 250ms | 1s |
|---|---:|---:|
| PASS | 31 | 36 |
| INCONCLUSIVE | 17 | 12 |
| FAIL | 0 | 0 |
| Uninstrumented retained-attributed memory (additional) | 8 INCONCLUSIVE | 8 INCONCLUSIVE |

Exact 1s metric statuses across 8 scenarios: throughput 6 PASS / 2 INCONCLUSIVE; P50 7/1; P95 5/3; **P99 4/4 (unchanged from the 250ms probe)**; CPU/op 6/2; allocations/op 8/0. A/A comparison `status` and `relative_status` remain `INCONCLUSIVE`, while `matrix_complete=true`, `environment_comparable=true`, `baseline_approved=false`, `performance_gate=DISABLED`.

| P99 scenario, per-run sample CV across both A/A sides | 250ms | 1s |
|---|---:|---:|
| RR 64B, 1 Session | 38.5% | 16.3% |
| NoWait 64B, 1 Session | 43.0% | 16.9% |
| RR 64KiB, 1 Session | 49.8% | **33.8%** |
| NoWait 64KiB, 1 Session | 38.3% | 17.1% |
| RR 4KiB, 32 Sessions | 3.8% | 1.6% |
| NoWait 4KiB, 32 Sessions | 5.0% | 1.7% |
| RR 64B, 256 Sessions | 5.3% | 3.7% |
| NoWait 4KiB, 256 Sessions | 4.4% | 3.4% |

Across these eight scenarios, mean sample P99 CV decreased from about **23.5%** to **11.8%** in separate runs. This supports **better observed reproducibility**, not a controlled proof that longer `-benchtime` alone caused the change. Significant P99 variability remains, notably RR-64KiB-S1; the median P99 for that case also changed substantially between the two different capture periods (~7.01ms to ~3.34ms). **Neither run meets full reference qualification**.

1s probe host telemetry: i5-12400, 12 logical CPUs, Go 1.27.0, `powersave` scaling governor. 1-minute load averages before/after ~1.73/4.94, CPU PSI `some avg10` ~2.67%/1.01%, I/O PSI `some avg10` ~0.78%/0.77%, `pswpin` +124 and `pswpout` +0, swap used ~7.2 GiB. A nonzero swap-in delta does not by itself prove sustained thrashing; governor, changing load and host contention may influence measurements but causation is unverified.

**Next:** Evaluate QA-004 separately using `bash scripts/qa005_noise_probe.sh qa004` on the owner's workstation. This invokes the fixed `1200x` terminal-settlement P99 benchmark for five synthetic populations with 10 alternating A/A pairs and produces a single tarball. Do **not** set `NETA_QA005_BENCHTIME` for `qa004`; the runner intentionally rejects the override. Keep the Performance Gate DISABLED, reference machine NOT APPROVED and QA-004/QA-005 OPEN pending a valid cross-version baseline, retained-attributed memory method and absolute limits.
## Owner-local QA-004 A/A noise probe (2026-11-18; independent 10-pair detection)

Uploaded evidence: `aa-qa004-20261118T202222Z-2358305.tar.gz`. Both worktrees were **identical exact SHA** `ff38ceb29107fa156e0f87555f59bc6a5fa393d9`, Go 1.27.0, linux/amd64, Intel i5-12400. Mode `qa004`, workload configuration `loopback-1200x-cpu4-qa004`, matching harness digest `3ca4f929f3bd73004159d607ff02e60011dd2ac83ef190a3f932de3086f5d7bf`, machine marker `local-aa-probe`. All **10 alternating pairs**, **20 individual process logs**, and **5 benchmark cases/log** completed `PASS`: 100 selected benchmark-case rows, exactly 1,200 true terminal-settlement samples per row = **120,000** samples overall. The complete benchmark matrix and machine metadata validated.

| Synthetic Pending | Median P99 baseline (µs) | Median P99 candidate (µs) | A/A sample P99 CV, 20 rows | Conservative upper CI regression bound | Relative metric status |
|---:|---:|---:|---:|---:|---|
| 32 | 227.99 | 228.76 | 1.52% | +2.97% | PASS |
| 128 | 229.17 | 230.52 | 3.01% | +4.18% | PASS |
| 512 | 349.88 | 352.03 | 2.28% | +2.64% | PASS |
| 2048 | 797.93 | 797.79 | 2.70% | +3.60% | PASS |
| 8192 | 1481.83 | 1483.62 | 0.93% | +1.46% | PASS |

CV is the sample coefficient of variation for the 20 run-level P99 figures (not for the 24,000 individual settlement durations). The confidence limits are the exponentiated Bonferroni-adjusted paired log-ratio upper bounds against the previously approved **15% relative tolerance**; values are advisory **A/A measurement repeatability**, not an absolute timeout SLA. Populations 2048 and 8192 are **synthetic scan fixtures**, not single-Session outstanding Request credit capacity. The file's `comparison.json` correctly reports `relative_status=PASS` across all five P99 metrics, but **overall `status=INCONCLUSIVE`, `baseline_approved=false`, `performance_gate=DISABLED`** because this is same-SHA A/A, not an approved cross-version historical baseline comparison.

Host snapshot before/after: CPU frequency governor `powersave` on all 12 logical CPUs, 1-min load average ~1.09 -> 1.70, CPU PSI some avg10 ~2.99 -> 2.92, I/O PSI some avg10 ~0.40 -> 0.59. `pswpin` +87, `pswpout` +0, major faults +321; pre-allocated swap ~7.3 GiB, no evidence of sustained swap-out thrashing. A/A success suggests QA-004 terminal-settlement P99 is **more repeatable on this host** than several prior QA-005 single-Session traffic P99 cases. It does not yet authorize the host as globally qualified reference equipment.

**Remaining owner decisions / engineering work:** select and approve a reference setup and historical code baseline that can run identical fixtures, make and verify **ten cross-version matched pairs** (on the same host/toolchain), define and verify attributable retained-memory evidence for QA-005, and approve absolute SLA/zero-baseline acceptance rules. Existing QA-005 A/A Core 1s still had **4/8 P99 comparisons INCONCLUSIVE**, so do **not** generalize QA-004's stable results to QA-005. QA-007 remains closed. No production code, public API, wire contract, baseline approval, release or performance gate change.
## Owner-approved source anchor: QA004-QA005-SOURCE-V1 (2026-11-19, Option A)

The owner selected **Option A**: use the **current** implementation as the starting **source-code** reference for measuring future changes, instead of reconstructing a benchmark comparison with historical `main`.

**Exact immutable source commit:** `9b7b2318f73d173a99d22489042f84f8ce58b745` on `feature/qa004-qa005-qa007-implementation-2026-11-18`. The branch name may advance, so pin the SHA, not `HEAD`. A machine-readable **source-anchor** record is available at [`qa004-qa005-source-v1.json`](performance-baselines/qa004-qa005-source-v1.json). This is *not* the numerical `--approved-manifest` expected by `scripts/qa005_compare.py` and has no baseline log digest.

**Future qualified comparison (not yet run):** use one clean, detached checkout at that exact source SHA and a second at a future candidate SHA, with bit-identical test workload fixtures and compatible instrumentation; match the host, Go version, settings and measurement duration. Run ten alternating matched pairs for QA-004/Core/Full on the owner-reviewed reference environment, capture and preserve logs with SHA-256, include a valid attributable retained-memory metric and apply only separately owner-approved absolute SLA/zero-baseline policies. Previously collected A/A metrics characterize noise; they must not be treated as historical code-regression evidence.

**What remains unapproved:** reference machine, immutable numeric reference measurements, per-mode validated comparator manifest, 15% retained-attributable-memory evidence, absolute SLA limits, and mandatory gate activation. QA-004/005 remain **OPEN**; QA-007 is **CLOSED**. There is no historical `main` performance comparison and no automatic promotion of this anchor into a release qualification.
## Local first numeric reference capture for future changes (Owner Option A)

After owner approval of the **source anchor only** (`QA004-QA005-SOURCE-V1`, exact SHA `9b7b2318f73d173a99d22489042f84f8ce58b745`), a single local-only collector is available:

```bash
git pull --ff-only
bash -n scripts/qa005_baseline_capture.sh
bash scripts/qa005_baseline_capture.sh
```

Runs on Linux from a clean checkout with `git`, `go`, `python3` and `tar`. It reads the approved source-anchor JSON, then creates a **temporary detached worktree at that exact historical SHA** (not the moving branch HEAD). It executes **10 independent runs per mode**: QA-005 8 Core workloads and 24 Full workloads with default **1s** benchmark samples (the better-observed A/A diagnostic duration), plus QA-004 five P99 terminal-settlement scenarios with fixed **1200x** samples; full means 8 Core + 16 additional Full cases. An optional `NETA_QA005_BENCHTIME=250ms|500ms|1s|2s` changes only the traffic modes; QA-004 retains 1200x.

The collector records the exact Go/host settings, source SHA, benchmark-fixture digest, ordered SHA-256 of ten raw logs for each mode, original run logs, run durations, case medians/ranges/sample CV, host-load/PSI/governor/swap snapshots, and one tarball in `artifacts/qa005-reference-capture/`. Per-mode outputs are in `<mode>/baseline/{01..10}.log`, `<mode>/baseline/environment.json`, `<mode>/statistics.json`, and `<mode>/DRAFT-NOT-APPROVED.json`. The draft includes **`owner_approved=false`** and must never be hand-edited to true or promoted without owner review. A source-only selection does **not** approve the machine, numeric values, absolute SLAs, attributable retained-memory metric or mandatory performance gate.

**Important statistical boundary:** the ten stored single-source observations are a candidate *reference profile*, not ten interleaved **cross-version** matched pairs. A future real regression assessment must run `scripts/qa005_qualify.sh` on both the pinned source SHA and a future candidate SHA with **matching benchmark fixture bytes**, toolchain, workload configuration and host, producing ten new alternating baseline/candidate pairs and a **separate, freshly reviewed numerical approval manifest**. A draft digest from the single-source capture cannot be copied into a new paired comparison because its ordered baseline logs will differ. These 1s data must not be compared numerically with 250ms evidence as though they were the same workload.

**Fail-closed:** if any mandatory benchmark scenario is missing or substituted, fewer than 1200 QA-004 detections are recorded, CPU/op evidence is absent, a capacity rejection is observed or a tracked owner-resource end counter is nonzero, the respective mode is `FAIL` (with logs retained). Temporary worktrees are cleaned and the collector attempts to package one tarball even on error. This collection script has not yet been run on the owner's workstation; its status is **PREPARED / AWAITING LOCAL EVIDENCE**. The owner's earlier successful 30-minute soak is distinct historical correctness/stability evidence and is not represented as a new run.
## Statistical acceptance and baseline custody

Relative tolerances: 10% for throughput and P50/P95, and 15% for P99, CPU/op, allocations/op, retained memory and deadline-detection P99. Three outcomes: PASS, FAIL or INCONCLUSIVE. A regressed metric fails only when its lower bound exceeds tolerance; passes only when the upper bound is within tolerance. Zero/near-zero baselines require an owner-approved meaningful absolute-delta policy, otherwise INCONCLUSIVE.

The owner **has approved the immutable source-code anchor** `QA004-QA005-SOURCE-V1` for future revisions, but has **not** approved a stable qualification machine, numerical reference benchmark evidence, absolute P99 Deadline Detection limit, absolute throughput/latency SLAs, or a quantitative comparator approval manifest. Performance gates remain **DISABLED**. The comparator can only consider a baseline approved when a separately maintained, immutable owner-signed/reviewed manifest includes `owner_approved=true`, exact baseline source SHA, reference environment fields and the exact SHA-256 digest of the ten ordered baseline logs. Do not alter historical logs or manifests in place: add a newly versioned baseline after explicit owner authorization, preserving all previous versions. Neither a branch push nor a merge to main auto-promotes a baseline.

The feature-branch CI runs Go correctness, race and pinned C++ V1 golden compatibility plus advisory scan, 1,200-observation QA-004 deadline detection and both Core/Full QA-005 measurements. The P99 sample count is a minimum sampling safeguard, not statistical proof or an absolute SLA. It cannot replace an approved reference environment. Record real logs and results; do not claim full qualification from the hosted runner alone.

## Compatibility boundaries

No heap/wheel deadline redesign; no hot-path benchmark instrumentation except a nil internal terminal-settlement observation hook exclusively used by synthetic tests. V2 default Bind without NUL and 20-byte SystemID, V1 NUL Bind with 19-byte SystemID and 10-byte Header are unchanged. No PR, merge, tag or release without separate owner permission.

## QA-005-MEM-001 — Proposed retained-memory qualification method (design only, 2026-11-19)

The owner selected **B / Hybrid Ownership Accounting**, recorded in [the code-based ownership audit](../.codex/NETA_GO_QA005_ATTRIBUTABLE_MEMORY_OWNERSHIP_DESIGN_2026-11-19.md) and the [measurement/baseline compatibility plan](../.codex/NETA_GO_QA005_MEMORY_MEASUREMENT_QUALIFICATION_PLAN_2026-11-19.md). This **does not activate** any runner, adapter, code instrumentation, baseline metric or performance gate.

Two existing comparability checks are mandatory: (1) the pinned original source SHA and unchanged workload fixtures, and (2) if a measurement-only test overlay is prototyped, exact matching **effective compiled** benchmark fixture and versioned adapter digests. The suggested `go test -overlay` route has not been verified and must be proven on clean, detached trees before relying on it; do not alter the source-V1 commit. Proposed in-workload sampled peak of covered library-owned retained storage is **not yet an approved `retained-attributed-B` comparator value**. Sampling can miss short peaks and independently observed owner charge peaks cannot be summed as a simultaneous exact peak. Any missing coverage, unknown retained external panic references, nonzero accounting-failure flags or absent/invalid values is INCONCLUSIVE; never convert post-workload zero counters or process HeapAlloc into attributable memory. Existing ten-pair/95%/15% rules remain unchanged and do not imply reference machine approval.

## QA-005 MEM-004B — Advisory owned-buffer overlay proof, NOT qualification (2026-11-19)

Versioned test-only overlay now observes live library-owned **backing-buffer categories F1–F4** under existing Session locks and reports (1) a sampled, potentially skewed global peak and (2) the separately summed per-Session event HWM **upper bound**, neither exposed as `retained-attributed-B`. [Exact-SHA verification](https://github.com/majiddarvishan/neta-go/actions/runs/44420200998) passed optional synthetic/race/one-shot checks: in one 64 B RR operation, sampled peak 74 B and HWM upper sum 212 B over 2 samples. The prior short smoke at [44419652396](https://github.com/majiddarvishan/neta-go/actions/runs/44419652396) observed **0 B** despite successful traffic, demonstrating that short peaks escape a 5 ms sampler. Missing unclassified data, metadata and application-reachable objects cannot be silently assumed zero; in-flight pre-enqueue Stream buffers are also excluded. Original five source+fixture files remain byte-identical to pinned source-V1; no numeric baseline, per-case A/A overhead, reference environment, absolute SLA or mandatory gate approval has occurred. QA-004/005 remain OPEN, QA-007 CLOSED and Performance Gate DISABLED.


## QA-005 MEM-004C: F1 pre-Queue ownership diagnostic (2026-11-19)

On exact feature SHA 26aa2873812abfe1349679be306bd2e8fd363702, [Actions 44423378544](https://github.com/majiddarvishan/neta-go/actions/runs/44423378544) passed 5 Python adapter tests, 3 Go tests with -race and one actual RR 64B benchmark operation through a read-only Go overlay. Source-V1 is unchanged and six original source/fixture GitHub blobs match. Historical 370 baseline rows were not rerun or supplemented. The V2 adapter tracks F1 backing storage before Queue admission, without a production source change.

One-op sampled buffer peak **0 B**, 2 samples, sum of independent per-Session event-HWMs **212 B**: proof of missed short allocations, **not** a qualified peak or zero memory. Historic optional-step failures at 50abb663 (duplicate registration classification) and ae9f78b6 (Python syntax) remain visible despite green outer workflows. Memory qualification remains INCONCLUSIVE, numerical reference and reference host unapproved, Performance Gate DISABLED.


## QA-005 MEM-004D — Frozen Session identity and lifetime validation (2026-11-19)

[Actions 44424670741](https://github.com/majiddarvishan/neta-go/actions/runs/44424670741) on SHA `58e655a79fe61084179b82ed1b42882bc02cc8a2` verified test-only V3 overlay with **5 Python tests**, **7 Go lifecycle tests under -race** and a one-operation request/response benchmark. A frozen Session registry prevents disappearing Server Sessions from dropping still-owned Callback buffers from the observer, while detecting unregistered new Server Sessions. Short-lived peaks remain unobservable with two 5ms samples: **sampled 0 B vs independent HWM upper 212 B**. These are diagnostics, not global retained bytes or an approved baseline. The immutable Source-V1 SHA, historical 370 rows, numeric approval manifest and mandatory Performance Gate remain untouched. MEM-004/005/006 OPEN, full memory qualification INCONCLUSIVE.


## QA-005 MEM-004E: partial coverage, observed sampling gaps and two-source adapter verification (2026-11-19)

Only seven Go source/benchmark blobs needed by V6 were checked and found identical between the immutable Source-V1 anchor `9b7b2318f73d173a99d22489042f84f8ce58b745` and feature source; there is no new numerical baseline. Advisory V4 [44425666635](https://github.com/majiddarvishan/neta-go/actions/runs/44425666635) verified pre-Queue F4 Unbind/Enquire accounting/rejection; V5 [44425989651](https://github.com/majiddarvishan/neta-go/actions/runs/44425989651) verified Session deduplication and marked every F1–F4 category PARTIAL_INCONCLUSIVE, none fully covered; V6 [44426342867](https://github.com/majiddarvishan/neta-go/actions/runs/44426342867) reported sampler max-gap and snapshot-duration diagnostics. All were optional-step PASS, but no memory PASS.

At `629772e238a89110009c9596b61f02c67596c945`, [Actions 44426946019](https://github.com/majiddarvishan/neta-go/actions/runs/44426946019) passed exact historical-source read-only overlay generation, four Python adapter-equivalence tests, seven-file original/effective fingerprints and Go -race lifecycle checks from the source-anchor worktree. Compatibility PASS; attributable retained-memory INCONCLUSIVE. Prior [44426784337](https://github.com/majiddarvishan/neta-go/actions/runs/44426784337) FAILED before jobs due malformed YAML and is preserved (repaired at 629772e2). Same-SHA ON/OFF actual instrumentation overhead has NOT been measured. The overlay bookkeeping runs even with the sampler disabled, so future MEM-005 must distinguish binary-source instrumentation effect from the sampling effect. Latest smoke sampled 0 B, 2 samples, HWM upper 212 B, max gap 202518 ns, snapshot max 661 ns; these are not reference measurements and cannot be retroactively used for retained-attributed-B. MEM-004/005/006 remain OPEN, QA-004/005 OPEN, QA-007 CLOSED, gate DISABLED.


## QA-005 MEM-004F/MEM-005 early tooling evidence (2026-11-19)

V7 adds an overlay-only Server admission generation under Server.mu, atomically snapshotted with the frozen owner set: a Server Session admitted and removed entirely between ticks is now detected and rejects validity. Original `types.go`, `server_lifecycle.go`, source-V1 SHA `9b7b2318f73d173a99d22489042f84f8ce58b745`, historic 370 archived rows and production Go code remain unchanged. [Actions 44486113768](https://github.com/majiddarvishan/neta-go/actions/runs/44486113768) PASS three jobs and separately audited optional lifecycle + source parity (9 Python + 10 Go -race + 4 Python compatibility tests). Historical green workflows [44485521207](https://github.com/majiddarvishan/neta-go/actions/runs/44485521207) and [44485822929](https://github.com/majiddarvishan/neta-go/actions/runs/44485822929) had **failing optional V7 stages** and were corrected with two independent commits. Full attributed retained memory still INCONCLUSIVE.

MEM-005 adds `scripts/qa005_overhead_advisory.py` and an advisory same-source three-mode Core RR 64B 25x smoke: no overlay, overlay Sampler OFF, overlay Sampler ON. [Actions 44486737303](https://github.com/majiddarvishan/neta-go/actions/runs/44486737303) verifies all jobs and optional stage, 5 Python tests, raw log hashes, exact Go scenario identity, 25 iterations each, compiler version and original/effective overlay identity. Observed 37486 / 41786 / 35932 ns/op respectively. **Single hosted-runner observations cannot establish instrumentation overhead direction, confidence or acceptability.** No numerical conclusion or new SLA may be derived; the required stable Go 1.27 owner reference, 10 paired Core measurements, 30m stability, sample density thresholds and full 24 matrix remain OPEN. First [44486575046](https://github.com/majiddarvishan/neta-go/actions/runs/44486575046) failed YAML before any job; corrected independently at fbadb551. MEM-004/005/006 and QA-004/005 remain OPEN, QA-007 CLOSED; Gate DISABLED.


## QA-005 MEM-004G V8–V9 partial owned-buffer integrity (2026-11-19)

V8's test-only F5 observes unreserved nondrained Bind inbound Body backing storage, from `readFrame` allocation until read error or `handleInboundFrame` return; Go net.Pipe tests cover both terminal paths. V9 additionally fails closed if Session Total disagrees with sum F1–F5 counters, including a deliberate silent mismatch exercised by a Go race-checked lifecycle test. [Actions 44489743062](https://github.com/majiddarvishan/neta-go/actions/runs/44489743062) V8 PASS all three jobs and four optional stages. [Actions 44490084898](https://github.com/majiddarvishan/neta-go/actions/runs/44490084898) V9 PASS all three jobs/four optional stages, 11 Python and 12 Go -race lifecycle tests, source anchor adapter parity, one real Core RR 1x diagnostic and the unqualified three-mode 25x overhead preview. Earlier [44488997407](https://github.com/majiddarvishan/neta-go/actions/runs/44488997407) and [44489465368](https://github.com/majiddarvishan/neta-go/actions/runs/44489465368) were overall-green with FAILING optional tests; their corrections 0c0dcca0 and 538eb70d preserve the original failure evidence. Latest 1x sampled peak=0 B (2 samples), independent Session HWM upper=212 B and max observed sample gap=171249 ns. All are DIAGNOSTIC_ONLY. F4 pre-Session allocation, M1 map/queue/Worker metadata, D1 diagnostic string/panic graphs, decoded objects and allocator overhead remain unmeasured; a correct counter sum is NOT complete library heap ownership. Historical 370 rows and fixed Source-V1 unchanged; MEM-004/005/006 remain OPEN, no mandatory performance gate.

## QA-005 M1/D1 Option B — V10/V11/V12 source-anchored evidence (2026-11-19)

Owner approved incremental neta-created metadata/diagnostic-only scope B (DECISION_LOG §77), not all M1/D1 byte coverage. V10 deduplicates retained internally generated Stack string lengths by per-Session Panic record Sequence; First/OnClose/Recent and recent eviction tests under -race confirm no duplicate aliased event charging, while raw user graph objects are excluded. [Actions 44510555448](https://github.com/majiddarvishan/neta-go/actions/runs/44510555448): all three jobs and optional stages PASS. V11 instruments *pendingRequest* object identity/lifetime in a temporary compile view from successful SendRequest to Completion release even after Done/map reset; the diagnostic first reports **object count only**. [Actions 44511176165](https://github.com/majiddarvishan/neta-go/actions/runs/44511176165): PASS all stages. V12 adds an expressly **nonphysical** static struct-footprint diagnostic `probe-m1-pending-struct-logical-peak-B` based on compile-time `unsafe.Sizeof(pendingRequest{})` × unique owned count (overflow checked), without accounting for Map, Closure, callback captures or allocator classes. [Actions 44511644311](https://github.com/majiddarvishan/neta-go/actions/runs/44511644311): all three jobs + advisory overlay/source parity/three-mode overhead PASS; 14 Python anchored tests, Go -race lifecycle tests, 4 Python adapter parity tests and pinned Source-V1 Go -race. 1x one-pending smoke 40 Go struct bytes, **not retained-attributed-B**. Different category observation times make a sum of their independent peaks invalid as a global instantaneous maximum. Complete library attribution remains INCONCLUSIVE; 25x hosted overhead preview remains FEASIBILITY_ONLY_INCONCLUSIVE, no approved host/10-pair bound or mandatory gate. Original source/QA source anchor `9b7b2318f73d173a99d22489042f84f8ce58b745` and prior 370 rows unchanged.
