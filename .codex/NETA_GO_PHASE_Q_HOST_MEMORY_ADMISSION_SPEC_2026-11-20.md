# Phase Q — Reference-host and attributable-memory admission specification (proposal, 2026-11-20)

**Status: DESIGN/REVIEW ONLY. No Phase-Q execution or numeric admission authorized.**

- Approved owner choices: **OD-ENV-01=A** (dedicated, controlled reference host) and **OD-MEM-01=A** (fail-closed `INCONCLUSIVE` until valid evidence), per [Decision Log §79](DECISION_LOG.md).
- Scope: operational admission criteria for **Q-ENV-01**, **MEM-004**, **MEM-005** and their prerequisites to **QA-004/005** and **MEM-006**. This does not reopen the completed 16/16 Phase D tasks or approve a new code/instrumentation implementation.
- Prepared against protected `main` SHA `0c01ac27bdffd9031f59b3ae466823f036839cf5`; do **not** use this moving commit as the benchmark reference anchor. The immutable source reference remains `QA004-QA005-SOURCE-V1` at `9b7b2318f73d173a99d22489042f84f8ce58b745` (source identity only).
- **Current dispositions**: Q-ENV-01 OPEN, MEM-004/005/006 OPEN, QA-004/005 OPEN, numerical baseline and absolute SLA NOT APPROVED, `retained-attributed-B` INCONCLUSIVE, mandatory Performance Gate DISABLED, `v1.0.0` NOT AUTHORIZED.
- Existing [performance protocol](../docs/PERFORMANCE_QUALIFICATION.md), [Hybrid-B category matrix](NETA_GO_DEV_02_HYBRID_B_CATEGORY_MATRIX_2026-11-19.md) and [Phase-Q normalized backlog](NETA_GO_PHASE_Q_EXECUTION_BACKLOG_2026-11-20.md) are authoritative on existing tested tooling and unfinished coverage. **All additional objective cutoffs below are explicitly PROPOSED; no owner-approved numerical limit is inferred.**

## 1. Q-ENV-01 — target reference-host profile (proposal)

**Policy fixed by owner:** dedicated Linux qualification machine, not a shared hosted CI runner or an automatically accepted existing workstation. Qualifying a *specific* machine is a separate, still-open task.

| Dimension | Proposed controlled configuration / evidence | Admission concern |
|---|---|---|
| Machine identity | Stable physical host ID, CPU model and topology, board/microcode, RAM, storage, OS/kernel and architecture; recorded without publishing private serials or credentials | Virtual/shared or changed hardware cannot silently share an approved baseline |
| CPU execution | Exclusive workload period, fixed reserved CPU set, enough CPUs for existing `go test -cpu=4` plus housekeeping, stable SMT/turbo policy, governor and effective frequency; store pre/during/post telemetry | Merely seeing `performance` governor is not proof of actual stable frequency or no throttling |
| Scheduler and interference | No unrelated benchmark/CI/production jobs on benchmark CPUs; record CPU steal/throttle and CPU PSI, pre/during/post process/host load | Prior owner-local Core P99 CV reached 33.8% even at 1s; a fast machine may still be invalid |
| RAM and swap | Sufficient resident memory for the 8 Core / 24 Full matrix and 30-minute soak; RAM, page faults, swap in/out, memory PSI and OOM/cgroup events captured | Available total RAM alone does not prove no benchmark-time swap interference |
| Storage and thermals | Local reliable storage for logs, no contending I/O; capture storage pressure, CPU temperature and thermal throttle evidence where available | Missing thermal instrumentation is an evidence gap, not evidence of no throttling |
| Toolchain | Pin exact `go version` including patch, `GOOS/GOARCH`, `go env`, selected compiler and build flags; same Go binary for both sides, record `GOMAXPROCS` and cgroup CPU/memory limits | `go.mod` says `go 1.23` (language minimum); previously observed Go 1.27.0 is historic data, **not** automatic approval of that toolchain |
| Workloads | Exact immutable baseline/candidate Git SHAs, clean detached trees, matching benchmark-fixture digests, same `-cpu=4`, scenario lists, run duration, affinity and environment | Core 8; Full 24 (including Core); QA-004 5 terminal-settlement P99 populations are separate matrices |
| Evidence custody | UTC timestamps; raw process output, comparator JSON, environment before/during/after, checksums and toolchain/host identity; log failures and deletions | No post-hoc rewriting, silently missing runs, or treating an archive exit code as a performance approval |

**Hardware sizing remains an owner-reviewed selection** after identifying an actual candidate host. This specification deliberately does not invent a CPU-model, physical-core-count, RAM-size or Go-version purchase requirement without a representative measurement.

### 1.1 Proposed host preflight and stop conditions

1. Record the physical host/toolchain identity and dedicated/noisy-neighbor policy **before** any heavy run. Verify effective CPU affinity/cpuset and `-cpu=4` execution; refuse an unknown or changing cgroup quota.
2. Choose and lock a CPU governor/frequency and SMT/turbo policy appropriate to that host. Record actual thermal/frequency/throttling evidence, not only `scaling_governor`. No policy changes mid-matrix.
3. **Proposed hard-stop observations:** any OOM, nonzero benchmark-window swap-in/out (distinguish background historic swap allocation from activity), measurable cgroup CPU throttling on the workload, thermal throttling, unexpected external workload, or material CPU affinity/toolchain/config drift => host run **INCONCLUSIVE** until controlled and repeated. These are proposed strict review criteria, not pre-existing approved numeric thresholds.
4. If telemetry is unavailable for a proposed guardrail, explicitly label it **UNKNOWN / needs owner disposition**, not PASS. Do not mutate system settings or run privileged commands without owner/operator authorization.
5. Keep an environment/exception log for each mode; a machine accepted for QA-004 only is **not** automatically accepted for noisy QA-005 Core/Full.

### 1.2 Proposed A/A qualification decision rule

**QENV-AA-02.ORD owner selection PENDING:** the [order and chronological-drift policy options](NETA_GO_QENV_AA_ORDER_DRIFT_POLICY_2026-11-20.md) clarify that the existing deterministic 5+5 first-side alternation and before/after system snapshots do not prove DRIFT-VALID; Option A joint worst-case bound under the previously approved 5%/7.5% is recommended, not accepted. No host/Mode admission or additional run authorized.

**QENV-AA-02 OWNER OPTION A APPROVED (2026-11-20):** [prospective symmetric A/A equivalence contract](NETA_GO_QENV_AA_EQUIVALENCE_POLICY_2026-11-20.md) now fixes 5% for QA-005 throughput/P50/P95 and 7.5% for QA-005 P99/CPU/op/allocs/op and QA-004 terminal-settlement P99 using mode-family simultaneous two-sided 95% paired-log intervals ([Decision §81](DECISION_LOG.md)). These new *measurement noise* bounds remain distinct from the historical 10%/15% *product regression* limits. **QENV-AA-02.ORD order/drift bound remains OPEN**, no physical machine or mode admitted; no runner execution or code edit authorized.

**A/A policy APPROVED: QENV-AA-01 = Option B (2026-11-20):** the [QENV-AA-01 comparator audit and owner decision](NETA_GO_QENV_AA_ADMISSION_DECISION_2026-11-20.md) establishes fail-closed **mode-scoped** reference-host admission for QA-004, Core and Full separately, after an E-VALID host preflight. The same-SHA comparison's non-memory per-case non-regression PASS evidence and complete matrix are **necessary, not sufficient**: the existing comparator PASS is one-sided, not symmetric A/A equivalence; an independently owner-approved prospective equivalence/ordering-drift rule and actual dedicated-host evidence are still missing. **No mode or physical host is admitted.**

- **First, a repeatability probe only**: two distinct clean worktrees at the **same exact SHA** on the proposed host. Existing `scripts/qa005_noise_probe.sh` runs **10 alternating pairs** per mode (`core`, `full`, `qa004`) and packages evidence. It expressly sets baseline approval to false. It is not an historical regression test.
- Existing `qa005_qualify.sh`: Core/Full legal durations `250ms`, `500ms`, `1s`, `2s`, default `250ms`; QA-004 uses fixed `1200x` and rejects a `NETA_QA005_BENCHTIME` override. **Proposed starting calibration:** 1s Core/Full (supported, previously lower noise) and untouched 1200x QA-004. A changed duration creates a **different dataset**; never mix or compare as equivalent.
- **Existing statistical contract preserved:** 10 matched alternating pairs, Bonferroni-adjusted paired-log-ratio 95% confidence, 10% throughput/P50/P95 and 15% P99/CPU/allocs/deadline-detection relative limits. These are regression tolerances, not host CV or absolute SLA limits.
- **Proposed A/A acceptance filter:** identical-source `matrix_complete=true`, `environment_comparable=true`, all mandatory benchmark rows/samples present, no resource/capacity errors, and every **observable, applicable non-retained-memory** relative metric resolves PASS against the existing tolerance. A non-memory FAIL or INCONCLUSIVE requires investigation and repeat with evidence; do not selectively drop the unstable P99 workload. Count each specific case/metric, not just a green process exit.
- The A/A comparator's **overall** `INCONCLUSIVE` is expected while `baseline_approved=false` and retained-attributed memory is absent; do not demand `overall PASS` for an A/A diagnostic, and do not turn the relative submetric PASS into machine or performance acceptance.
- **Owner acceptance remains mandatory** after examining per-scenario variability (especially RR 64KiB / 1 Session), actual host telemetry and all three mode archives. If strict all-metric A/A pass proves technically infeasible, do not relax the rule silently: document the noisy cases and request a narrower owner decision. Do **not** adopt an invented universal CV% bound.
- **Later**, after separate explicit Phase-Q permission and host acceptance, run actual cross-version pairs and consider a signed/reviewed numerical manifest. An A/A bundle **never** sets `owner_approved=true`.

### 1.3 Existing command map — DOCUMENTATION, DO NOT RUN WITHOUT AUTHORIZATION

```bash
# Run from a clean neta-go Git worktree on the later approved dedicated host.
NETA_QA005_BENCHTIME=1s bash scripts/qa005_noise_probe.sh core
NETA_QA005_BENCHTIME=1s bash scripts/qa005_noise_probe.sh full
env -u NETA_QA005_BENCHTIME bash scripts/qa005_noise_probe.sh qa004
```

The existing script records `lscpu`, governor, load, PSI, cgroup and swap snapshots before/after, but does **not** by itself prove continuous thermal/frequency stability, absence of external processes or exact CPU pinning. Those checks must be separately evidenced or tooling explicitly authorized. A successful tarball is **data**, not an approved host.

## 2. MEM-004 — attributable-memory validity contract

**Definition retained from Hybrid B:** maximum observed *simultaneously library-owned* retained storage during an active fixed QA-005 workload window. Partial F1–F5/M1/D1 diagnostic counters, sums of independent session HWM, `HeapAlloc`, RSS, credits or the zeroed post-run resource ledger are **not** that value.

### 2.1 Category and ownership closure matrix (required proof, not new implementation approval)

| Coverage class | Required proof or approved exclusion | Unresolved evidence as of V18 / DEV-02 |
|---|---|---|
| F1 outgoing, F2 Request, F3 Response backing | Distinct allocation identity, true capacity/ownership through queue→writer or callback→release, no double-count on transfer and no loss after `Session.Done` | Existing test-only category diagnostics support selected lifetimes, not a globally simultaneous validated peak |
| F4 Control backing + **pre-Session Bind** | Account from first library-created byte until owner release, including before Session admission, or formally review any excluded interval and its error impact | Pre-Session encoder/Bind interval missing; **PARTIAL_INCONCLUSIVE** |
| F5 unreserved inbound body | Include Bind/unreserved read buffer until handler return or failed-read rollback; avoid treating decoded objects/runtime bytes as covered | Existing lifecycle diagnostic, incomplete physical storage/peak inference |
| M1 Session / Client epochs / Pending and charges | Retired generations still held by callbacks, independent generation checks, charged versus actual storage, distinct pending identities and release after callback | Old retired Client generations and observer-prolonged liveness unresolved; partial `unsafe.Sizeof` is a named *logical* measurement |
| M1 Session queues, worker/task, maps and metadata | Unique backing-array/object ownership, alias dedup, actual duration under contention and transitions; prohibit `len(map)` or quota bytes as allocator storage | Worker/queue *subset* diagnostic exists; Session queues, maps/ledger buckets and other metadata excluded/unknown |
| D1 library-rendered panic diagnostics | Count distinct neta-created stack/value text and record structures only while neta retains them; First/OnClose/Recent aliases once; respect eviction and transfer | Stack-text length for some paths exists; remaining D1 metadata/text/record ownership unknown |
| Out-of-scope foreign/runtime objects | Explicit exclusions with bounded and documented impact: foreign panic raw-value graphs, user callback closure graphs, Go allocator buckets/GC overhead, goroutine stacks, kernel/socket memory, post-transfer user diagnostics | **Do not** retroactively claim these bytes zero or infer them from process heap. M1/D1 incremental **Option B (§77)** remains approved, separate from OD-MEM-01 fail-closed **Option A (§79)** |

For **every in-scope category**, a coverage record requires allocation site, owner, identity, byte derivation, transfer/alias handling, release, test cases and the precise reason for any exclusion. Missing size or interval = **UNKNOWN/EXCLUDED**, not measured zero. A test that intentionally generates zero objects in a category cannot validate the category's accounting in nonzero cases.

### 2.2 Temporal and global peak proof (no arbitrary sample interval)

1. The workload-window clock must bracket every participating Session/Client generation and its callbacks; event, owner and observer timestamps need defensible clock/synchronization semantics. A frozen owner registry must detect sessions created/removed between ticks and retired generations that still own bytes; observer references must not change lifetime unnoticed.
2. Per-owner sequential reads are **not an atomic simultaneous global snapshot**. Define and test a consistent snapshot or a mathematically justified skew/error envelope. A summed Session event HWM is merely an upper bound; never emit it as a measured global peak.
3. Log sampler max start-to-start gap, maximum snapshot duration, actual sample times and coverage failures. The previous **5ms sampler missed sub-ms peaks**, including observed 0 B alongside a 212 B independent HWM upper bound; therefore **choosing 1ms, 5ms or any interval without a lifetime/error proof does not establish a valid peak**.
4. Proposed proof strategy for later owner-reviewed work: deterministic short-lived allocation/release fixtures, lifetime under asynchronous callbacks/forced close/reconnect, and compare against a ground-truth event ledger or provably bounded observation loss. If a peak can occur entirely in an unobserved gap, the canonical result remains **INCONCLUSIVE**, regardless of a low sampled peak.
5. Capture and fail-closed on overflow, missing owner, alias duplication, differing effective-source/overlay digest, cross-owner snapshot skew without accepted bound, missing/invalid category, and observer GC/liveness effects. Do not enable new hot-path instrumentation as an implicit workaround.
6. **No new numerical max-gap/missed-peak/coverage bound is approved.** A proposed bound must be justified by measured workload lifetime distribution and worst-case adversarial short peaks, then separately reviewed by the owner. An unprovable bound means **INCONCLUSIVE** rather than a fabricated pass.

### 2.3 Proposed memory result-state matrix

| Evidence condition | Allowed output |
|---|---|
| Coverage/lifetime and simultaneous bounded peak verified; accepted temporal error, source/fixture parity and observer overhead; host/baseline separately accepted | Candidate `retained-attributed-B` can be submitted **for owner review**, not automatically approved |
| One or more known in-scope categories missing, unknown size, early generation not visible, skew or short peaks not bounded | `INCONCLUSIVE` + named partial diagnostic values only |
| All category samples zero but fixture never exercised category or missed short allocation | `INCONCLUSIVE`, **not** 0 retained bytes |
| Foreign/app/runtime graph excluded by previously approved Option B | Document excluded scope; never count as library-owned, but also never claim whole-process memory |
| Owner does not approve numerical baseline or absolute SLA | No performance PASS / no Performance Gate regardless of some diagnostics passing |

## 3. MEM-005 — measuring observer perturbation

- Preserve original pinned-source / effective compiled fixture and measurement-overlay digest parity; same fixture and toolchain, **not** different production code.
- Use the existing three independently labeled modes: **original source (no overlay)**, **test overlay present with sampler OFF**, **same overlay with sampler ON**. Separate baseline-to-overlay bookkeeping cost from OFF-to-ON sampling cost. The observer can affect GC reachability and lifetime even if run-time bookkeeping appears small.
- Proposed experiment: ten balanced/alternating paired measurements for the **same applicable Core/Full workload**, 95% paired statistical intervals, actual source/harness identity, matching host/governor/affinity, CPU/op, throughput, allocation and latency plus observer work/samples. Historical hosted 25x feasibility numbers are **not** an accepted overhead result.
- Track peak-to-peak observer allocation and reference retention, freeze-induced lifetime extension, snapshot runtime, sampling gaps, forced-close/reconnect paths, and category coverage on the same experimental run.
- **Owner approval needed** for explicit acceptable overhead and worst-case temporal error. Existing 10%/15% *product regression* tolerances are **not** silently redefined as allowed observer overhead. Without an accepted bound, report `FEASIBILITY_ONLY_INCONCLUSIVE` / `INCONCLUSIVE` and keep MEM-005 OPEN.

## 4. Required artifacts and review checklist

| Evidence ID | Produced during future authorized work | Blocking review |
|---|---|---|
| ENV-INV | Dedicated host inventory, exact Go binary/version/flags, OS/kernel, CPU topology/affinity/governor, cgroup, thermal/frequency and owner identity record (redacted) | Host selected and reviewable |
| ENV-AA | Raw A/A `core`, `full`, `qa004` 10-pair archives, all individual metrics, duration/fixture SHA and machine telemetry | Missing/incomparable/non-PASS applicable metric => Q-ENV-01 not accepted |
| MEM-CAT | Versioned per-category allocation/transfer/release/exclusion ledger, alias/generation tests and trace identity | Missing or partial => MEM-004 INCONCLUSIVE |
| MEM-TIME | Peak ground truth or accepted bounding model, max-gap/snapshot duration, short-life fixtures and cross-owner simultaneity evidence | Unbounded missed peak/skew => MEM-004 INCONCLUSIVE |
| MEM-OBS | Three-mode paired results, parity hashes, observer-induced GC/lifetime effects and paired 95% intervals | Owner-accepted overhead threshold needed; otherwise MEM-005 OPEN |
| REVIEW | Exact immutable source+fixture SHA, archived logs and SHA-256, machine acceptance by owner, numerical manifest approval as a **separate** later act | Nothing auto-promoted into baseline or mandatory Performance Gate |

**Suggested review order** (not authorization): inspect/approve a particular host profile and evidence guardrails → explicitly authorize bounded Q-ENV-01 A/A → review all A/A raw results → owner formally accepts/rejects host → propose/correct MEM-004 temporal and category coverage semantics → separately authorize MEM-005 three-mode measurement → consider MEM-006 soak, QA-004/005 reference comparisons, OD-BASE-01/OD-SLA-01 and only then possible OD-GATE-01.

## 5. Remaining owner-specific decisions (not answered by choosing A/A)

1. **Actual dedicated machine:** physical hardware/cpu pinning and Go patch/toolchain chosen, whether thermal/CPU telemetry is accessible; a host ID and concrete inventory must be attached before Q-ENV-01 admission.
2. **Proposed A/A rule:** accept the all-applicable-nonmemory-metrics-PASS filter across 8 Core + 24 Full + 5 QA-004 cases, and proposed hard stops (swap/throttle/OOM/toolchain drift), or request evidence-backed amendments. **Proposal only.**
3. **MEM acceptance envelope:** exact approved maximum observer overhead, observer-induced liveness/GC effects and proven maximum missed-peak error or alternative event-driven accounting; **no numerical values are chosen here**.

**No owner answer above has been presumed. No benchmark, local command, privilege change, production edit, new metric promotion, numeric baseline, source-anchor change or release action occurs in this planning task.**
