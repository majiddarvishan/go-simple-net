# Q-ENV-01 — A/A Reference-Host Admission: Evidence Audit and Owner Decision (2026-11-20)

**STATUS: QENV-AA-01 OWNER POLICY APPROVED — OPTION B (2026-11-20); PHASE Q NOT AUTHORIZED.** No specific reference host or mode has been admitted; the symmetric-equivalence, drift, numerical and environment acceptance criteria remain pending.
This document refines the [host and memory admission specification](NETA_GO_PHASE_Q_HOST_MEMORY_ADMISSION_SPEC_2026-11-20.md) without changing its already approved underlying policies. Owner approval **OD-ENV-01=A** chooses a dedicated controlled machine; **OD-MEM-01=A** requires fail-closed attributable-memory validity. QENV-AA-01 **Option B is now owner-approved** as the separate admission framework (Decision Log §80), without changing the other policies.

**Source audit:** protected `main` `04253958bd2c50da0d86062833ad9da31c615d28`; inspected `scripts/qa005_compare.py`, `scripts/qa005_noise_probe.sh`, `scripts/qa005_qualify.sh`, `docs/PERFORMANCE_QUALIFICATION.md`. The immutable **source identity anchor**, **not** an approved numeric baseline, remains `9b7b2318f73d173a99d22489042f84f8ce58b745`.

## 1. What the current tools actually establish

1. `qa005_noise_probe.sh [core|full|qa004]` runs **10 paired, alternating, identical-exact-SHA A/A** runs per mode in clean temporary worktrees. A/A does **not** compare two library revisions and does **not** make a numerical baseline; it sets `reference_baseline_approved=NO`. The script's `noise-summary.json` counts relative PASS/FAIL/INCONCLUSIVE **excluding retained memory**. Its process-exit `PASS` does **not** require every available metric to be PASS.
2. `qa005_compare.py` computes paired log-ratio estimates and Bonferroni-adjusted **two-sided 95% intervals** for 10 pairs. It uses existing **relative regression** tolerances of 10% (throughput/P50/P95) and 15% (P99/CPU/op/allocs/op/deadline-detection P99; retained memory when valid). These were **not approved as A/A equivalence bounds, CV limits or absolute SLAs**.
3. Mode matrices are **8 Core cases**, **24 Full cases** (Full includes the 8 Core cases), and **5 QA-004 deadline-detection P99 cases**. Each QA-005 case currently has **6 non-retained comparable metrics** (throughput, P50, P95, P99, CPU/op, allocs/op): thus **48 Core / 144 Full** checks. QA-004 has **5** checks. Retained memory remains `INCONCLUSIVE` and may be omitted from *host-noise* admission only; it is **never waived from MEM-004/005 or full QA-005 qualification**.
4. QA-004 requires true terminal-settlement detection at **>=1,000 samples by comparator** and the **existing fixture explicitly runs 1,200** per case; do not substitute callback-start latency. QA-005 P99 observation guard is **>=1,000 iterations per individual case**. Missing, zero, nonfinite or unsupported values remain `INCONCLUSIVE` unless a separately owner-approved delta policy exists. Note: current `classify` actually checks nonpositive and nonfinite values, even though its error text says `near-zero`; it does not implement a positive near-zero cutoff.
5. `comparison.status` remains `INCONCLUSIVE` in A/A **by design**, because `baseline_approved=false`. `comparison.relative_status` also often remains `INCONCLUSIVE` because its mandatory metric matrix includes absent `retained_bytes`. Valid host-noise review must inspect **each per-case non-memory result** in `comparison.scenarios` and the raw evidence, not these overall statuses alone.
6. `environment_comparable=true` means the two recorded environment descriptors match; it **does not** prove physical CPU isolation, zero swap/throttle, stable temperature/frequency, no noisy neighbors, or a trace-free workload. Existing scripts capture snapshots and benchmark metadata, **not a continuous proof** of all these conditions.
7. The existing comparator's metric **PASS is a one-sided non-regression conclusion**, not proof of *A/A equivalence* or absence of time/order drift. A very favorable systematic A/B difference can still PASS. Therefore the owner's environment admission decision must separately review signed effect direction, paired CI, chronological/order drift, and host telemetry; **do not silently call PASS a calibrated equivalence test**.

## 2. Prior same-SHA evidence and interpretation

| Prior probe (2026-11-18; local, not approved reference) | Observed non-memory relative metrics | Decision meaning |
|---|---|---|
| QA-005 Core at default 250ms, i5-12400 / Go 1.27 | **31 PASS / 17 INCONCLUSIVE / 0 FAIL** of 48 | No host admission |
| QA-005 Core at 1s, i5-12400 / Go 1.27 | **36 PASS / 12 INCONCLUSIVE / 0 FAIL** of 48; **P99 4/8 PASS** | Increased duration improved observed variability but Core not yet reproducible enough for all requirements |
| QA-004 1200x, same general workstation class, same-SHA 10 pairs | **5/5 P99 relative PASS**; sample P99 CV 0.93–3.01% | Good provisional QA-004 repeatability *only*; not general QA-005 host admission, numerical baseline or absolute SLA |

All were under an uncontrolled/powersave environment with observed background load and swap-in activity in prior captures. They cannot substitute for fresh **dedicated-host** data. Shorten/lengthen benchmark duration only with explicit configuration identity; **1s Core/Full is a proposed calibration**, not a retroactive comparison to earlier 250ms evidence.

## 3. Three levels of status — never collapse them

| Level | Reviewable evidence | Allowed disposition |
|---|---|---|
| **E: Environment / tooling validity** | Dedicated machine identified, pinned exact Go binary/version, `-cpu=4`, governor, CPU/cgroup limits, interference/thermal/swap telemetry; clean same-SHA worktrees and matching fixture digest | `E-VALID`, `E-INVALID`, or `E-UNKNOWN`; **no source-performance PASS** |
| **A(m): A/A repeatability for mode `m`** | The complete 10 alternating pairs for `qa004`, `core`, `full`; case-by-case signed CI and missing metric analysis, sample guards, temporal drift/ordering and per-mode telemetry | `A-ADMISSIBLE`, `A-NOT-ADMISSIBLE`, `A-INCONCLUSIVE` **scoped to each mode**; not a baseline decision |
| **Q: Final performance / memory qualification** | Owner-accepted host for relevant modes, baseline/candidate fixture parity, valid 10 cross-version pairs, approved immutable **numeric** manifest, absolute SLA, validated `retained-attributed-B` and observer bias | `PASS/FAIL/INCONCLUSIVE` per metric per approved contract; **Q remains OPEN now** |

If **E** is invalid/unknown, no **A(m)** may be admitted even if `relative_status` contains PASS. If **A(full)** is unresolved, do not call QA-005 Full ready even if **A(qa004)** is admitted. **A(m) is a host/workload repeatability assessment, not permission to run the heavy qualification.**

## 4. Invariant evidence and fail-closed conditions (same for all policy options)

- Identical exact SHA on both A/A sides, clean tracked source and no untracked Go files; full raw 10+10 logs and ordered alternating runs; equal Go/OS/CPU/fixture metadata and a recorded benchmark duration. The same-SHA probe is **not** a cross-version baseline.
- All selected cases present: Core **8**, Full **24**, QA-004 **5**; each metric has ten valid positive finite paired values where required; each P99 case meets sample count. **Do not drop noisy single-Session cases**, substitute scenarios, or reinterpret near-zero as acceptable without approved absolute-delta rules.
- Environment validity: no OOM, observed benchmark-time swap/throttle/thermal interference, significant benchmark-CPU neighbor, material configuration drift, telemetry gaps that prevent ruling out an applicable failure mode, unapproved cgroup or affinity changes. Proposed hard stops in the admission spec remain **proposals subject to owner review**, not retroactively approved numerical bounds.
- Record **per-case, per-metric** relative PASS/FAIL/INCONCLUSIVE; the signed log-CI direction and pair-order drift; no selective repeated testing until a favorable result without preserving ALL attempts. Explicitly explain the comparator's absence of a distinct A/A equivalence test.
- The probe's `run_status=PASS` or GitHub Actions PASS **only certifies the run/packaging tools completed**; it does not mean `Q-ENV-01 DONE`.
- `retained_bytes` is *excluded from the host-noise-only non-memory count*, **never silently regarded as zero, waived, or qualified**. MEM-004 and MEM-005 remain blockers to the canonical `retained-attributed-B` value.
- If evidence is noisy or incomplete, keep it **INCONCLUSIVE**, document root-cause and propose a controlled rerun or explicit new decision; no Performance Gate, numerical baseline or source-anchor change.

## 5. Owner decision: admission policy for A/A evidence

**Owner selection recorded 2026-11-20: QENV-AA-01 = OPTION B**, scoped, fail-closed per-mode admission (Decision Log §80). The alternatives below are retained as decision history; A and C were **not** selected.

| Option | Criterion for host admission | Advantage | Limitation |
|---|---|---|---|
| **A — Global strict** | An eligible dedicated host needs a complete E-VALID record **and all applicable non-memory case/metric results PASS in QA-004 + Core + Full**, with an owner-accepted drift/equivalence review. All three modes must qualify together; no partial host admission. | Simple single go/no-go; no qualification gap hidden | A single noisy Full P99 leaves QA-004 progress blocked; Bonferroni with only 10 pairs can be highly conservative and may require more controlled hardware/research |
| **B — Scoped, fail-closed per mode (OWNER APPROVED)** | E-VALID first; then **each mode is admitted separately** only if **all its applicable non-memory case/metric results PASS**, pair-order/drift is credible, and owner accepts the mode evidence. `A(qa004)`, `A(core)`, `A(full)` may differ; Full remains blocked if any of its 144 case/metrics is not accepted. | Allows reproducible QA-004 to progress without pretending noisy QA-005 scenarios passed; still no intra-mode cherry-picking or automatic final QA acceptance | More bookkeeping and explicit scope on every result; full production performance qualification can still remain blocked |
| **C — Environment-only with qualification deferred** | E-VALID permits only a provisional **host candidate** record. All `A(m)` stay INCONCLUSIVE until a separate, newly approved mode-specific statistical criterion is designed, regardless of relative PASS counts. | Avoids overinterpreting a one-sided regression comparator as equivalence | No workload repeatability acceptance yet; requires new design/owner decision before any quantitative qualification |

**Owner-approved framework: B**, with explicit additional review of signed paired effects, potential A/B order drift and physical CPU stability before any `A(m)` admission. This keeps the fail-closed user-selected memory policy intact. It also separates QA-004 (historically more repeatable) from QA-005 Full without weakening complete Full admission.

**Important unresolved statistical detail:** even Option B's requirement that every current *non-regression* result PASS is only a necessary filter, **not sufficient proof of symmetric A/A equivalence**. A precise, prospective equivalence interval or empirically justified drift/variance limit must be reviewed by the owner **before claiming final mode admission**. Current historical 10%/15% thresholds are **not auto-converted** into A/A equivalence bounds. Until then `A(m)` remains `A-INCONCLUSIVE` even where 5/5 relative QA-004 checks PASS.

## 6. Work plan and remaining approvals

1. **Decision CLOSED (docs only):** owner chose **QENV-AA-01=B**. [Decision Log §80](DECISION_LOG.md). This chooses the mode-scoped admission framework, **not** a host, numeric equivalence tolerance or permission to execute tests.
2. **Documentation completed, owner decision still pending:** [QENV-AA-02 symmetric equivalence / order-drift proposals](NETA_GO_QENV_AA_EQUIVALENCE_POLICY_2026-11-20.md) specify 95% simultaneous paired-log CIs by mode and present A/B/C strategies, including illustrative new 5% / 7.5% equivalence margins. **Not approved:** QENV-AA-02 selection, numerical margins and the distinct order/drift bound. Preserve the existing comparator unchanged until separately authorized code edits.
3. Obtain actual dedicated-host inventory, exact Go patch, CPU governor/affinity, swap/thermal telemetry availability; document host operator controls before any performance measurement.
4. **Only after explicit Phase-Q permission:** collect E and per-mode A/A evidence on that machine. Score against the prospectively approved policy, explain all INCONCLUSIVE entries, obtain specific owner host/mode acceptance.
5. **Later and separately:** approve immutable numerical baseline (OD-BASE-01), absolute P99/throughput/latency SLA (OD-SLA-01), MEM-004/005 canonical attribution and observer error, QA-004/005 true cross-version pairs; leave mandatory Performance Gate DISABLED until OD-GATE-01 independently approved.

**Status:** Q-ENV-01, QA-004/005 and MEM-004/005/006 all OPEN; **20 unique open packages** unchanged (10 Qualification + 7 other Owner Decisions + 3 Release). QENV-AA-01 **is CLOSED as an approved policy subdecision** of Q-ENV-01, **not** an additional counted top-level package. Q-ENV-01 and all individual modes remain OPEN/NOT ADMITTED until their independent evidence and prospective equivalence limits are accepted. No tests, benchmark, stress, soak, CI settings, production source, numerical baseline, tag or release have been authorized or modified by this proposal.
