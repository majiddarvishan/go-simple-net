# QENV-AA-02.ORD — Pair Order and Chronological Drift Admission (Owner Decision Proposal, 2026-11-20)

**STATUS: PROPOSAL ONLY — OWNER CHOICE OPEN. Phase Q NOT AUTHORIZED. No machine/Mode admitted.**

**Owner-frozen prerequisites:**
- [Decision Log §80](DECISION_LOG.md): **QENV-AA-01=B**, E-VALID dedicated-host preflight and independent fail-closed A/A Mode acceptance for `qa004`, `core`, `full`.
- [Decision Log §81](DECISION_LOG.md) and [QENV-AA-02=A](NETA_GO_QENV_AA_EQUIVALENCE_POLICY_2026-11-20.md): same-SHA 10-pair **symmetric** equivalence using per-Mode 95% Bonferroni-adjusted paired-log confidence intervals, `d=0.05` for QA-005 throughput/P50/P95, `d=0.075` for QA-005 P99/CPU/op/allocs/op and QA-004 terminal-settlement P99. Bound in log units: `L=ln(1+d)`; allowable ratio `(exp(-L),exp(+L))`, **strict** inclusion.
- Existing product-regression comparator's distinct 10%/15% one-sided guards, Scenario matrix and P99 sample counts remain required; memory `retained-attributed-B` remains INCONCLUSIVE until MEM-004/005.
- Source review from `main` at `5267184348e02f7fd60cf816b02546c8adf9c2ee`: `scripts/qa005_qualify.sh`, `scripts/qa005_noise_probe.sh`, `scripts/qa005_compare.py`. Immutable Source-V1 anchor `9b7b2318f73d173a99d22489042f84f8ce58b745` is NOT a numerical baseline.

## 1. Source-grounded unresolved risk

`scripts/qa005_qualify.sh` invokes both worktrees in this deterministic order:
- Pairs **01,03,05,07,09**: baseline first, candidate second.
- Pairs **02,04,06,08,10**: candidate first, baseline second.

Each `go test -bench` process runs the whole selected Mode's cases. `pairing-run.log` records **pair index and side**, not reliable UTC/monotonic timestamps for each process and each individual benchmark case. The `system-before/after.txt` snapshots capture high-level host characteristics; they do **not** continuously prove CPU frequency stability, no transient throttling, or absence of other runnable processes.

**Identical SHA does not imply identical thermal state, Go warm-up, allocator/GC history, background system load or benchmark execution order effects.** Alternation balances exactly five first runs each; it does not establish a statistical bound on drift or one-sided warm-up. `qa005_compare.py` currently assesses **paired regression** only and has no `order_effect`, `time_trend` or `DRIFT-VALID` decision. A mode cannot be accepted using an overall comparator PASS or five-first/five-second counting alone.

**Measurement caveat:** benchmark case rows within a process execute at different times; recording only the process start/end is a bracket, NOT a precise in-case timing point. A new estimator may use carefully justified intervals or a non-intrusive per-case log timestamp capture, but it must document uncertainty and refuse false precision. If exact timing is necessary, any instrumentation affecting benchmark fixtures/harness digests requires separate owner authorization and source-parity review. Do not silently change the frozen source anchor.

## 2. Proposed shared validation envelope (regardless of option)

1. **Complete and immutable evidence** per Mode: two clean exact-SHA worktrees; 10 side-matched pair-indexed runs; 20 unmodified raw logs; exact 5/48/144 selected non-memory case-metrics, no dropped cases and sufficient per-case P99 samples. All runs and *all failed attempts* must be preserved, not repeatedly sampled until green.
2. **Fixed environment** on the already owner-selected dedicated-host policy: CPU model/affinity/NUMA/SMT/governor/frequency, exact Go patch/binary, `-cpu=4`, cgroups and thermal/swap/PSI interference tracked across each full Mode window. Historical before/after host snapshots alone are insufficient. Missing evidence => `E-UNKNOWN` and no Mode admission.
3. **Signed, unambiguous effect:** for case-metric `j` and pair `i`, define `y_ij = ln(B_ij/A_ij)` from identical-source data, using baseline- and candidate-worktree values **without performance-metric sign flipping**. Use `o_i=+1` when baseline ran first, `o_i=-1` otherwise. Record observed per-case ratio in both sides and absolute magnitude; zero, nonfinite and near-resolution readings require explicit disposition, never coerced to 0.
4. **Time and duration trace:** preserve order of each pair and side, monotonic as well as UTC timestamps, process run interval and benchmark-case time bracket (or formally proven bounded proxy), time since first run, and environment counters during each interval. Record differing side durations, warm-up/setup periods, benchmark case order and interruptions.
5. **Mode-by-mode, no cherry-picking:** evaluate the 5 QA-004, 48 Core, or 144 Full non-memory metrics as a complete fixed family; **Full includes its eight Core cases**, but a separate Core Mode run is not replaced by Full. The owner may accept QA-004 while Core/Full remain blocked, per QENV-AA-01=B.
6. **Fail closed on unmodeled instability:** nonmonotonic thermal episodes, frequency/CPU quota changes, unexpected CPU neighbors, swapped/throttled windows, major run-time discontinuities or poor time-proxy precision => `DRIFT-INCONCLUSIVE` / `E-INVALID`, never `DRIFT-VALID` merely because a linear fit looks quiet. A statistically non-significant order trend is NOT equivalence.
7. **No new standalone product SLA:** QENV-AA-02.ORD is a *host measurement-quality gate*, not a cross-revision QA-005 pass, approved numeric baseline or retained-memory proof.

## 3. Three owner options — QENV-AA-02.ORD (none selected)

| Option | Method and pass rule | Pros | Limitations |
|---|---|---|---|
| **A — Joint worst-case bound under already-approved 5%/7.5% (RECOMMENDED)** | Retain 10 balanced alternating pairs. For every case-metric estimate both order and chronological trend in a prespecified model; require the **full simultaneous uncertainty envelope over both first-side orders and whole run-window** to remain inside the **same existing approved d**, with no separate new percentage or tolerated hidden drift. Keep original symmetric mean-equivalence and regression guards independently. Evidence must support the model; no timestamp => INCONCLUSIVE. | Does not invent a new noise budget; strongest direct tie to approved symmetric equivalence, preserves existing scheduling policy | Conservative with n=10, 144 Full metrics, temporal model assumptions; demands adequate timestamps/telemetry and potentially narrowly authorized tooling |
| **B — Fresh counterbalanced confirmation cohort** | Retain original 10-pair dataset, then on the accepted candidate host conduct **a new, disjoint ten-pair** A/A cohort with preregistered seed, balanced randomized first-side order and per-run clock/telemetry; require both cohorts to pass the fixed d gate and an owner-specified comparison of order-stratum/time effects. | Stronger replication and less susceptibility to one deterministic schedule or a single short-lived system state | At least twice the qualification measurement effort; new schedule tool requires separately authorized edits; the **effect-bound method still needs definition** and may remain inconclusive |
| **C — Conservative hold** | Report order-vs-time diagnostics, but **no numerical DRIFT-VALID** until additional independent study sets a defensible design and owner signs its rule. | No unsupported claim from ten pairs or missing traces | Blocks all three Mode admissions and delays Q-ENV-01 qualification |

**Recommendation: A** as the next *policy design choice*, with rigorous preregistration and fail-closed handling. A **does not approve** an implementation, host, benchmark execution, new timestamp logging, randomization, or numerical performance baseline. The owner still decides whether to select it.

## 4. Option A — proposed precise evaluation (ONLY if owner later approves A)

**This section defines a proposed contract, NOT an already-approved method.**

For each case-metric `j`, the ten paired log ratios are `y_ij`. Set actual pair time `t_i` from the paired process timestamps, monotonically scaled so earliest valid midpoint is `-1` and latest is `+1`; do not substitute index if real timestamps are required but missing. Let `o_i ∈ {-1,+1}` represent first-side order.

Fit the preregistered additive association model:

`y_ij = μ_j + β_order,j · o_i + β_time,j · t_i + ε_ij`

- The fitting design must be full-rank with ten complete pairs, reported `X'X` and condition number; residual degrees of freedom `n-3=7`. Explain any collinearity, elapsed-time imbalance, heterogeneous residuals or outlier sensitivity. This is an **observational** separation, not an assertion of causal identification.
- Prespecify two order values `{-1,+1}` and the **full observed window time endpoints** `{-1,+1}`. For each of the four `(order,time)` corners, compute the estimated **mean log ratio** and standard error using the full fitted covariance matrix, not an unadjusted point estimate.
- Keep the already approved original per-case **ten-pair** symmetric-mean test: a two-sided Bonferroni `m`-metric 95% t interval around the unadjusted paired mean (with df 9) strictly inside `[-L_j,+L_j]`. Existing 10%/15% regression checks stay independent.
- For a **joint order/time confidence gate**, form **two-sided simultaneous 95% t intervals over all `4 × m` corner contrasts for the Mode**: `estimate ± t_{1-0.05/(2·4m),7} × SE`. All four corner intervals for **every applicable case-metric** must be strictly contained in `(-L_j,+L_j)`. This uses the **existing approved d** as the entire ratio budget; it does not halve or raise it. If a 95% family-wise claim is needed for the **combined** original-mean and four-corner evidence simultaneously, jointly adjust across **5 × m** contrasts instead (with each contrast's relevant df), and record the selected joint reporting semantics *prospectively*, not after seeing data. A conservative combined 5m family correction is **recommended** for the eventual final admission certificate.
- Also disclose raw `y_i` by pair and first-side order, first/last half summaries, per-run observed environmental shifts and residual/linearity diagnostics. **Model-fit PASS never overrides direct evidence of transient/nonlinear drift**. If adequacy of this small-n parametric model cannot be justified, classify the Mode `DRIFT-INCONCLUSIVE` even when the model CI fits.
- Side start/end timestamps may be coarse relative to each benchmark-case's timing; report the possible timing interval and sensitivity of the fitted slope/CI to it. A time proxy without adequate sensitivity proof => `DRIFT-INCONCLUSIVE`.
- A missing/negative/nonfinite sample, a rank-deficient design, inconsistent workload chronology, insufficient P99 observations, missing thermal/CPU telemetry, or unsuitable statistical assumptions is **INCONCLUSIVE**; never impute defaults or let empty case families PASS.

### Example (hypothetical, NOT an observed result)

For QA-005 throughput with approved `d=5%`, a modeled worst-case corner's simultaneous ratio confidence interval must lie **strictly inside** `(1/1.05, 1.05)`. A CI of `(0.97, 1.03)` qualifies for that **single corner**; `(0.95, 1.03)` does **not**. All other corners, all other metrics, the original mean-equivalence gate, regression checks and E-VALID must still pass before per-Mode owner admission. Nothing can be inferred from PASS on one corner alone.

## 5. Suggested evidence/implementation contract — not permission to edit code

Proposed new *report-only*, independently versioned diagnostic artifact, without changing the existing `qa005_compare.py` regression result semantics:

```text
qa005-aa-order-drift/<mode>/
  schedule.json             # prespecified first-side order / pair ids / source SHA
  process-timing.jsonl      # start/end UTC+monotonic; side; pair; exit; duration
  environment-window.jsonl  # CPU governor/freq/thermal/pressure, cgroup, swap and process interference
  drift-comparison.json     # case-metric fit, df, ratios, 4-corner CIs, original mean CI, residual checks
  acceptance-summary.json   # E-VALID, REGRESSION, EQUIVALENCE, DRIFT, owner-reviewed mode disposition
```

Requirements: preserve every original `01.log..10.log`, `comparison.json`, `noise-summary.json` and pre/post machine snapshots unchanged; add a manifest/hash binding timestamp/telemetry records to exact run, Go toolchain and fixture SHA; make the classifier fail closed; keep log timestamps and overhead analysis independent of measured benchmark timing. The existing `NETA_QA005_REFERENCE_MACHINE=local-aa-probe` marker is **not an actual host identity**. Operational telemetry collection or modified runners may require privileges/operator consent; **no tool/source change or execution here**.

## 6. Explicit owner boundary and next action

**Owner decision required: QENV-AA-02.ORD Option A, B or C.** Neither the family-wise corner formula nor collection/timestamp changes, drift status, any host/Mode acceptance or benchmark authorization is approved by writing this document.

If A is approved, lock the exact joint-family CI reporting method (recommended 5m conservative), timestamp precision and fail-closed adequacy criteria in the Decision Log *before any new data*. Implementation/tooling becomes a distinct separately authorized task, followed by concrete dedicated-host inventory, E-VALID and a separately authorized Phase Q collection. If B, pre-register the independent cohort and effect comparison; if C, keep `DRIFT-INCONCLUSIVE`.

**Unchanged state:** `Q-ENV-01`, `QA-004/005`, `MEM-004/005/006`, Q-COR qualification and release tasks remain OPEN. Exactly **20 normalized top-level packages** are outstanding (10 Qualification + 7 other owner decisions + 3 Release). `retained-attributed-B` INCONCLUSIVE, mandatory Performance Gate DISABLED, immutable source anchor unchanged and `v1.0.0` unpublished.
