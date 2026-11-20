# QENV-AA-02 — Prospective A/A Equivalence, Order Bias and Variance Admission (2026-11-20)

**STATUS: QENV-AA-02 OWNER OPTION A APPROVED (2026-11-20): 5% and 7.5% fixed prospective symmetric A/A equivalence margins.** [Decision Log §81](DECISION_LOG.md). **QENV-AA-02.ORD order/drift bound still OPEN**; no host/mode has been admitted and Phase Q is not authorized.

**Authoritative scope:** [QENV-AA-01 Option B, owner approved](NETA_GO_QENV_AA_ADMISSION_DECISION_2026-11-20.md) and [Decision Log §80](DECISION_LOG.md) require **independent, fail-closed mode admission** for `qa004`, `core`, `full` after dedicated-host E-VALID. The source audit references protected `main` `17cabe8dd08562fed4534291671b7cea813096f7`. Immutable Source-V1 anchor `9b7b2318f73d173a99d22489042f84f8ce58b745` is *source identity*, not a numeric baseline.

**Goal:** establish a prospective, reproducible, symmetric A/A *equivalence* decision on same-exact-SHA pairs. This is not the existing QA-004/QA-005 one-sided regression comparator, not an absolute SLA, and not memory-retention qualification.

## 1. Why the existing comparator is insufficient

- `scripts/qa005_compare.py` has `PAIRS=10`, `CONFIDENCE=0.95`, Bonferroni family-wise t intervals of paired **signed** log ratios and one-sided, regression-only decisions (PASS when the regression upper confidence bound is below its threshold).
- A candidate that is unexpectedly *much better* than identical baseline can still get PASS; that is not proof of stable A/A equivalence. Even every relative PASS does not establish a symmetric ratio bound or rule out order bias.
- `scripts/qa005_noise_probe.sh` deliberately excludes missing `retained_bytes` from its **host-noise-only** counts; the whole comparator is correctly INCONCLUSIVE without owner-approved numerical baseline. A process `run_status=PASS` only means evidence collection completed.
- History: QA-004 A/A 5/5 relative P99 PASS on the prior owner-local workstation, but QA-005 Core at 1s had 36/48 non-memory PASS, **12/48 INCONCLUSIVE** and P99 only 4/8 PASS. Those are not dedicated-host proof, nor approved equivalence bounds.
- Current A/A runner alternates baseline-first and candidate-first **deterministically** across the ten pairs (five of each). This partly counters position bias but does not randomize time, separate systematic warm-up/drift, or collect continuous host telemetry. Same code can still show a temporal trend.

## 2. Invariants regardless of a new owner choice

1. **E-VALID first:** dedicated hardware identity, fixed Go patch/binary, `-cpu=4`, governor/frequency policy, cpuset, cgroup quota, swap/throttle/thermal and workload interference evidence. Host metadata equality alone is not E-VALID.
2. **Complete mode:** 10 alternating same-SHA pairs; no dropped/renamed scenarios, per-case positive finite data and minimum P99 observations. `qa004` 5 terminal-settlement cases with 1200x fixture; `core` 8 cases × 6 observable non-memory metrics = 48; `full` 24 cases (including Core) × 6 = 144. The memory category is absent only from this *host-noise* check, not from later QA-005 qualification.
3. **Existing regression guard remains mandatory:** for every applicable case/metric, existing one-sided comparator `PASS` under the separately approved 10%/15% **regression tolerances**. This is a necessary condition and is NOT reused as the A/A equivalence margin.
4. **New symmetric equivalence:** for each case and metric take the pair-indexed ratio `R_i = B_i/A_i` where `A` and `B` are two identical-source worktrees, then `y_i = ln(R_i)`; use both directions symmetrically regardless of throughput vs latency. Let `d` denote a **prospectively approved** allowed ratio deviation, with symmetric multiplicative region `[1/(1+d), 1+d]`; the log-equivalence region is `[-ln(1+d), +ln(1+d)]`. Do not treat percentages in raw arithmetic, unpaired observations or zeros as valid log evidence.
5. **Family-wise 95% confidence:** for each independently assessed mode, prespecify the family of all its 5/48/144 case×metric comparisons. A conservative proposed method uses a two-sided Bonferroni simultaneous interval per `y` with `alpha=0.05`, `m=5|48|144`: `mean(y) ± t_(1-0.05/(2*m),9) * sd(y)/sqrt(10)`. A case is equivalent only if the **entire interval** is *strictly inside* `[-ln(1+d),+ln(1+d)]`. All cases must satisfy the condition for that mode. This is deliberately more conservative than an adjusted TOST confidence interval; it shares the existing 95% family-wise discipline but introduces a **new** distinct decision.
6. **Order and temporal drift:** preserve timestamps and the A/B execution order. For every applicable case, report signed per-pair log ratios separately for baseline-first vs candidate-first (5 and 5), chronological pair-index trend, first-half/second-half direction, observed dispersion, near-zero/quantization and any notable outlier. *Failure to reject* an order-effect null hypothesis is **not** proof of no order bias with ten pairs; an owner-approved, testable ordering/temporal bound is required to say `DRIFT-VALID`. If raw timestamps/order or adequate accuracy are missing, mode is **A-INCONCLUSIVE**. Do not cherry-pick trials, increase repetitions after seeing failed values or treat a deterministic ABBA alternation as proven randomization.
7. **Mode gate:** the mode can be `A-ADMISSIBLE` only with E-VALID, all-case original regression PASS, all-case symmetric-equivalence PASS, DRIFT-VALID, and actual owner admission of that particular host/mode. Else `A-INCONCLUSIVE` or `A-NOT-ADMISSIBLE` with an explicit reason. No automatic spillover: `A(qa004)` does not imply `A(core)` or `A(full)`.
8. **Statistics/measurement constraints:** the t procedure assumes meaningful independent paired log observations and approximate sampling behavior; check long tails, systematic drift, quantization and sample sufficiency. A ten-pair CI may simply be too wide, especially with 144 metrics; in that case report INCONCLUSIVE, never relax the bound after viewing data. A low CV by itself, a CI centered near zero, or a very narrow CI with quantized measurements does not establish admission.

## 3. Owner-approved prospective symmetric A/A margins — Option A (2026-11-20)

Existing QA product-regression tolerances are **10% for throughput/P50/P95** and **15% for P99/CPU/allocations/deadline P99**. Those were not established for same-SHA equivalence. The following values are now **owner-approved new measurement-noise equivalence budgets under QENV-AA-02=A**, distinct from those existing product-regression tolerances. Their acceptance is **a rule approval**, not evidence that any host has met them.

| Family / benchmark mode | **Approved** symmetric A/A bound `d` under Option A | Basis / qualification boundary |
|---|---:|---|
| QA-005 throughput, P50, P95 | **5%** | Owner-approved 2026-11-20 prospective equivalence budget, not product SLA |
| QA-005 P99, CPU/op, allocations/op | **7.5%** | Owner-approved 2026-11-20; noisy single-Session P99 is not waived |
| QA-004 internal deadline-detection P99 | **7.5%** | Owner-approved 2026-11-20 for QA-004's five cases only |
| Retained-attributed memory | **NOT IN SCOPE** | MEM-004/005 Hybrid B validity and overhead separate; no memory host-noise `0` assumed |

These new margins were chosen **explicitly by the owner**; their being numerically half the older regression tolerances remains only a design rationale, not a claim of measured host precision. Large multiplicity adjustment with only 10 pairs may make some Mode admissions hard: that is an honest signal to stabilize the machine or separately revisit measurement design, not permission to ignore a failing case.

**Example of the approved rule**, not an observed benchmark: if a metric has `d=0.05`, its entire simultaneous log-CI must lie in `(-ln 1.05, +ln 1.05)`, or equivalently its geometric baseline/candidate ratio CI must lie strictly between `1/1.05` and `1.05`. An A/A non-regression PASS without this two-sided containment is not an equivalence PASS.

## 4. Owner decision: QENV-AA-02 = Option A (alternatives retained as history)

| Option | Statistical strategy | Advantages | Costs/risks |
|---|---|---|---|
| **A — Fixed, prospective family margins (OWNER APPROVED)** | Owner approved **exact 5% / 7.5%** above before any run. Use 10 pairs and the mode-family 95% simultaneous log-CIs, plus a separately approved order/drift bound. Every metric must fit its own margin. | Transparent, reproducible and quick to pre-register; keeps measurement-noise budget tighter than old 10%/15% product-regression thresholds | For 10 pairs and 144 Full comparisons may yield INCONCLUSIVE even on a reasonably good dedicated host; prospective order bound still needs formal approval |
| **B — Independent pilot calibration then holdout confirmation** | On an explicitly authorized dedicated host, run a preregistered exploratory same-SHA pilot; estimate variability and order effects, propose per-family `d` and drift controls from that independent data, obtain **new owner approval**, and then collect a **fresh non-overlapping 10-pair confirmation set**. No reused pilot evidence in final pass. | Bounds can be engineered to real host timing/noise and a documented precision target rather than guessed | More executions and extra review; must prevent data-derived margins from being loosened to excuse unstable cases |
| **C — Defer symmetric numerical admission** | Keep the host only E-candidate; no mode admitted and no numerical comparison PASS until method and limits are accepted in a later contract | Maximum conservatism with no new thresholds now | Cannot finish Q-ENV-01 Mode admission or dependent numerical qualification |

**Owner selection: Option A (2026-11-20),** including the fixed 5% and 7.5% equivalence budgets. A separate **QENV-AA-02.ORD** rule for order/drift remains to be designed and approved; without it no Mode can be admitted. If Option A proves impracticable on qualified hardware, request a new explicit decision instead of retroactively relaxing limits. If Option A becomes impracticable on stable hardware, later request a new explicit owner decision rather than rewriting historic evidence or relaxing thresholds silently.

## 5. Explicit unresolved order/drift subcriterion

**QENV-AA-02.ORD remains a separate subdecision regardless of A/B/C choice.** Select a prospective maximum tolerated order-associated ratio difference and time-trend impact, with defensible uncertainty intervals and measurement duration; or design stronger counterbalanced randomized rounds, preserving auditable fixed seed and recorded schedule. Five baseline-first and five candidate-first pairs **do not** establish an upper bound on order effect merely because average signed effects cancel. The existing script has no formal `order_bias_status` output and no continuous thermal/CPU monitoring. Before any Mode acceptance, the reviewer must either approve auditable evidence and a bound or retain **DRIFT-INCONCLUSIVE**. This requires no choice or tool change in the present documentation-only step.

Proposed evidence for future dedicated-host runs: immutable per-process start/end UTC + monotonic timestamps, pair-index/side execution sequence, same-run system pressure, clock/thermal/cgroup state and an independent, versioned report of signed order and trend effects. Any new runner/comparator instrumentation requires separate source/test authorization and must not silently mutate historical fixtures or approved Source-V1.

## 6. Status, custody and acceptance boundary

**Policy decision CLOSED:** QENV-AA-02 **A**, including fixed prospective `5%/7.5%` limits, is owner-approved and logged in [Decision §81](DECISION_LOG.md). **Pending separate owner decision:** QENV-AA-02.ORD must establish an explicit order/drift bound or validated experimental design **before any Mode admission**.

**Not approved / not executed:** no dedicated host identity, Go binary/toolchain, order/drift bound, new qualified A/A evidence, Phase Q heavy run, baseline owner manifest, `retained-attributed-B`, absolute SLA or Performance Gate. Source-V1 SHA, approved QENV-AA-01=B mode isolation and all MEM-004/005 fail-closed obligations remain unchanged.

**Work-package accounting:** QENV-AA-02 **A is a CLOSED policy/margin subdecision**; .ORD is **OPEN**, both within still-OPEN Q-ENV-01, not new top-level open items. The project remains **20 distinct open work packages** (10 Qualification + 7 other Owner Decisions + 3 Release). No release/tag, no source/benchmark/CI code edit and no performance claim.
