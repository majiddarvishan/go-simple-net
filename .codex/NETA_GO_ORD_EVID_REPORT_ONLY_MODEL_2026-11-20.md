# QENV-AA-02.ORD-EVID — Report-only 5m Statistical Diagnostics (2026-11-20)

**Scope:** synthetic and read-only validation tooling, **not** qualified host evidence, approval of a numeric baseline, or permission to run 10-pair benchmarks. This is a subordinate implementation-readiness increment of still-OPEN `QENV-AA-02.ORD-EVID` under `Q-ENV-01`. The normative policy is [ORD Option A](NETA_GO_QENV_AA_ORDER_DRIFT_POLICY_2026-11-20.md) and [QENV-AA-02=A](NETA_GO_QENV_AA_EQUIVALENCE_POLICY_2026-11-20.md). No Source-V1 fixture, runtime Go API, tag or Performance Gate modification.

## Implementation

- `scripts/qa005_ord_drift.py` is an **independent, report-only consumer** of `qa005_qualify.sh`'s existing outputs. It does **not** modify `scripts/qa005_compare.py`, its 10%/15% regression guard, its historical fixture digests, either benchmark command or logs. It is invoked **only** for `NETA_QA005_ORD_EVID=1`, **after** the unchanged comparator and without additional benchmark work.
- Requires precisely 20 complete side/pair records (10 A/A pairs), unchanged raw baseline/candidate logs, per-side `environment.json`, source/fixture/toolchain parity, log/evidence SHA-256 integrity, actual alternating execution order, non-overlapping monotonic process chronology and complete 5/48/144 case×non-memory-metric matrices. A missing or malformed field results in `INCONCLUSIVE` rather than inferred zero or sample deletion.
- Checks sensor sample chronology, full pre-spawn/post-exit envelopes, consistency of measured maximum sampling gap and explicit missing-sensor flags. Actual host validity **is not established** merely by collecting the sensors.
- For each case-metric `j`, computes ten unflipped `y_i=ln(candidate_i/baseline_i)`; original Bonferroni `m`-metric two-sided 95% t interval (df=9); additive model `y_i = μ + β_order*o_i + β_time*t_i + ε_i` (df=7); its full `X'X`, inverse condition, covariance, residuals, first/last half and order strata; and the **joint 5m** two-sided Bonferroni 95% original mean plus four modeled `(order,time)=(-1,-1),(-1,+1),(+1,-1),(+1,+1)` interval bounds. The only numerical margin values are previously owner-approved 5% and 7.5% symmetric log-ratio bounds; no new threshold.
- The model's `t_i` is the **monotonic midpoint of the two entire go-test processes in each pair**, scaled from earliest to latest to `[-1,+1]`. It additionally reports each pair's broad earliest/latest process-time envelope. **This is not an observed individual benchmark-case time**; it cannot prove the sensitivity of each case's time slope/CI to its unobserved location inside the process. Neither model-fit adequacy nor nonlinear/transient drift exclusion is thereby proved.
- Emits `ord-drift-diagnostics.json` with `qualification=DRIFT-INCONCLUSIVE`, `environment_validity=E-UNKNOWN`, `host_mode_admission=NOT_PERFORMED` and `performance_gate=DISABLED` **unconditionally**. `WITHIN_FIXED_BOUNDS_UNQUALIFIED` is **only a numerical diagnostic**, never a mode-admission PASS. Raw status labels from the preexisting comparator are not substituted for symmetric equivalence.

## Non-benchmark regression tests

`python3 -B -m unittest discover -s scripts -p 'test_qa005_ord_*.py' -v`

The synthetic suite covers all three 5/48/144 family-wise multiplicities, four order/time corners, separated original versus joint intervals, rank-deficient designs, nonfinite/missing values, order/temporal drift, a complete synthetic 20-run QA004 evidence set, immutable provenance and detection of tampered raw logs. The dedicated GitHub workflow runs these tests only; it does **not** run a real 10-pair A/A campaign. The existing default `NETA_QA005_ORD_EVID=0` behavior is unchanged.

## Still needed before QENV-AA-02.ORD-EVID can close

1. Auditable and non-perturbative per-case timing brackets (or a formally demonstrated and robust sensitivity bound for each full-process proxy), without silently changing the frozen workload fixture.
2. Telemetry completeness and meaningful continuous/interval thermal/frequency/CPU-scheduler/PSI/cgroup/swap interference review, plus a **specific** controlled host/toolchain preflight and a justified sampling-gap/bias assessment.
3. Model adequacy review for the ten-pair observational fit: residual structure, nonlinear drift/outliers, quantization, uncertainty, order/time collinearity, case-time sensitivity; fail closed on any missing proof.
4. Separately authorized real A/A measurements for each Mode, independent per-mode regression, original symmetric A/A equivalence and joint 5m drift verification. No host Mode can be accepted without explicit owner review; no baseline or absolute SLA is selected here.

**Accounting:** Q-COR-01 closed earlier with exact released-tag correctness evidence; the normalized remaining work packages are **18 = 9 Qualification + 7 Owner Decisions + 2 Release**. This report-only increment does not close an additional top-level task or toggle Performance Gate.
