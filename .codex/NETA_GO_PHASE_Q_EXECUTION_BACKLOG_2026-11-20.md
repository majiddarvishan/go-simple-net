# neta-go — Phase Q execution backlog and decision gates (2026-11-20)

**Status: PLAN ONLY / PHASE Q NOT AUTHORIZED.** No benchmark, soak, stress, extended fuzz or full qualification was executed for this reconciliation. This is the unique **normalized actionable work-package inventory** linked by [TASKS.md](TASKS.md); older checkboxes in historical sections can refer to the same package and must not be double counted.

- Evidence baseline: protected `main` at `bd8421cf3cabc5cb72263ed964434a48bf54ef22`, with [PRs #24–#27](https://github.com/majiddarvishan/neta-go/pulls?q=is%3Apr+is%3Amerged) merged. On that exact SHA, [Go 44630477642](https://github.com/majiddarvishan/neta-go/actions/runs/44630477642) and [C++ wire 44630477596](https://github.com/majiddarvishan/neta-go/actions/runs/44630477596) are SUCCESS. Required contexts remain `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0`.
- Approved immutable source comparator anchor: `9b7b2318f73d173a99d22489042f84f8ce58b745` (`QA004-QA005-SOURCE-V1`). This is **source identity only**, not an approved numeric benchmark baseline.
- Phase D development **16/16 complete**, plus two independently merged general-purpose peer-router examples (14 runnable commands in total). `WithClientOnRequestHandler` is merged by PR #27.
- QA-004/005, MEM-004/005/006 and heavy Phase Q acceptance remain OPEN. Existing hosted CI, partial owned-buffer adapters and earlier owner-local A/A probes are **historic or advisory**; no new `retained-attributed-B` approval is implied.
- Performance Gate **DISABLED**; tag, release, production changes, new numeric baseline and anchor changes **NOT AUTHORIZED**.

## Normalized open backlog: 10 Qualification + 9 owner decisions + 3 Release = 22 distinct packages

Each ID is one work package, even if several historical `[ ]` entries in TASKS.md refer to it. The phase-Q table lists required execution (all currently `[ ]`); owner decisions are tracked independently and need not all block the initial V1 release. `Reference host` describes whether a stable owner-approved comparison machine is required for acceptance, **not** permission to run.

### Phase Q — 10 qualification work packages (all deferred)

| ID / priority | Work package | Dependencies | Reference host | Acceptance and durable evidence |
|---|---|---|---|---|
| Q-ENV-01 / P0 | Reference-host stability & A/A admissibility | OD-ENV-01 hardware/toolchain choice | YES | Locked CPU/governor/toolchain/workload/environment manifest; load, thermal, swap and scheduler interference captured; ten paired identical-source A/A runs repeatable within proposed decision bounds; owner accepts machine; noisy result stays INCONCLUSIVE. |
| Q-COR-01 / P0 | Exact-SHA correctness + Go race | Phase-Q authorization; exact post-D `main` SHA | NO | `make verify`/race, core lifecycle, concurrent Client OnRequest and two-way TCP, callback-panic/Stop/resource-credit tests; logs and exit codes archived. Existing CI on #27 is evidence, not this deferred sign-off. |
| Q-COR-02 / P1 | Stress/load/reconnect and teardown | Q-COR-01, Phase-Q authorization | NO (dedicated load host useful) | Bounded overload, disconnected peers, several concurrent Sessions, close/reconnect churn, no unexpected retained credits/workers; reproducible seed/config/logs, no silent retries. |
| Q-COR-03 / P1 | Extended fuzz and parser mutation | Q-COR-01 | NO | Reproducible seeded and extended fuzz over V1/V2 Bind, Header, Stream and invalid frames; minimize/report crashes or hangs. Historical short PR fuzz CI is not equivalent. |
| Q-COR-04 / P1 | Live interoperability (Go↔C++ V1) | Q-COR-01 and compatible peer availability | NO (C++ runtime/test environment required) | Both real TCP directions, explicit V1 selection, golden fixture and fault cases, version/peer SHA, success/failure archived. C++ pinned wire compatibility alone is not full live interop. |
| QA-004 / P0 | Pending Deadline Scan P99 qualification | Q-ENV-01, OD-SLA-01 | YES | Use five approved Pending populations with terminal-settlement timestamp evidence; credible matched baseline/candidate P99 distributions, valid sample counts, 95%/relative comparison and owner-approved **absolute P99** threshold. An advisory scan is not a PASS. |
| QA-005 / P0 | 8 Core / 24 Full performance acceptance | Q-ENV-01, OD-BASE-01, OD-SLA-01, MEM-004/005 | YES | Exact identical workload fixture/source-adapter semantics, all 8 Core + 24 Full cases; **ten alternating matched baseline/candidate pairs**, 95% confidence and established 10%/15% tolerance rules, no missing evidence; PASS/FAIL/INCONCLUSIVE documented per metric, no baseline silently promoted. |
| MEM-004 / P0 | Hybrid-B attributable retained-memory validity | OD-MEM-01, Q-ENV-01 for final comparison | MIXED | Reconcile F1–F5, M1 and D1 coverage/exclusions; prove allocation/ownership/release, dedup aliases, retired Session generations, no false zeros, valid **simultaneous** workload observation and bounded gaps/bias. F4 pre-Session, maps/queues, diagnostic strings and incomplete categories stay INCONCLUSIVE until actually justified. `retained-attributed-B` cannot be emitted from partial V18 diagnostics, summed HWMs, quotas, HeapAlloc or post-run zero. |
| MEM-005 / P0 | Observer overhead and sampling accuracy | MEM-004 adapter/fixture design, Q-ENV-01 | YES | Three-mode source/overlay OFF/ON matched measurements, ten credible pairs, variability/CI and measured sampling max-gap/snapshot duration; test observer GC/liveness bias and short peaks; approve overhead and temporal error bounds or return INCONCLUSIVE. |
| MEM-006 / P1 | Qualified 30-minute soak | Q-ENV-01, Q-COR-01, MEM-004/005 validity | YES | At least 30 min with accepted traffic/teardown, worker/Session lifecycle and retained-memory series, leak/regression checks, immutable logs, identical run configuration, explicit exit and errors; no inference from short CI or earlier different-SHA soak. |

### Owner decisions — 9 independent packages

| ID / priority | Question requiring owner action | Options or boundary |
|---|---|---|
| OD-ENV-01 / P0 | Approve reference machine / CPU isolation, governor, Go version and stability criteria | A: dedicated pinned Linux machine (recommended for reproducibility); B: existing workstation subject to accepted A/A + strict isolation; C: hosted shared CI only (not recommended for numerical baseline). Approval is distinct from a passing probe. |
| OD-MEM-01 / P0 | Approve measured/excluded Hybrid-B category coverage and acceptable max-gap, observer bias and missing-short-peak bounds | A: maintain fail-closed INCONCLUSIVE for incomplete categories (recommended until proof); B: accept narrower formally versioned partial metric **after** explicit contract review, not under existing full metric name; C: extend observation instrumentation within newly approved scope. Current Option-B M1/D1 ownership does not preapprove any incomplete value. |
| OD-BASE-01 / P0 | Approve actual immutable numerical reference logs + manifest (`owner_approved=true`) | Required after qualified Q-ENV-01/MEM validity; source anchor approval alone is insufficient. |
| OD-SLA-01 / P0 | Approve absolute QA-004 P99 and other applicable throughput/latency limits | Requires stable measured evidence; keep current relative policies (10% and 15%) unchanged meanwhile. |
| OD-GATE-01 / P2 | Decide whether and when to turn mandatory Performance Gate ON | Optional, **DISABLED** until independently approved host, numbers and CI policy; never an automatic consequence of green CI. |
| OD-NOTES-01 / P0 | Approve current Unreleased release notes after PR #27 | Include default V2, NUL-free Bind, V2 20-byte/V1 19-byte ID, additive Client OnRequest, peer routers and subsequent changes; supersedes three duplicated PV-01-R / PV-02A-R / PV-02B-R owner requests, which remain unchecked until approval. |
| OD-CHECK-01 / P2 | Change protected-main required C++ check context before renaming CI job | Optional branding/admin decision; **preserve old required context** until coordinated setting and verified PR/postmerge run. Do not block functional qualification solely for naming. |
| PV-02 / P3 | Approve safe peer/version negotiation mechanism | Future feature; explicit configuration vs a proven backwards-compatible negotiation scheme, not required for V1 release. |
| PV-04/PV-05 / P3 | Approve peer capabilities / future version-specific API scope | Future feature; design/implementation authorization still absent, not required for V1 release. |

### Release — 3 packages

| ID / priority | Gate | Acceptance |
|---|---|---|
| R-01 / P0 | Final exact-SHA release candidate and CI/qualification audit | After Phase Q and OD-NOTES-01/OD-BASE-01/OD-SLA-01 as applicable, record actual protected-main SHA, two mandatory checks, regression outcomes and publishable notes; no result inferred from older commits. |
| R-02 / P0 | Admin/publisher preflight | Authorized admin verifies strict-up-to-date/bypass controls and publisher rights; check v1.0.0 tag/release collision immediately before publication. GitHub branch `protected=true` does not prove all settings. |
| P9-06 / P0 | Tag and GitHub Release `v1.0.0` | Requires separate explicit owner permission **after** R-01/R-02 and resolved release blockers. No tag or release during reconciliation. |

## Dependency / execution order

1. **Before Phase Q permission (allowed now):** review this backlog, inspect source/CI evidence, resolve owner choices OD-ENV-01 and OD-MEM-01, propose objective admission criteria. No heavy runner starts.
2. **On explicit Phase Q authorization:** Q-COR-01 and Q-ENV-01 can proceed independently; then Q-COR-02/03/04 and source/fixture-equivalent MEM-004 diagnostics. If a qualified metric is infeasible, report INCONCLUSIVE and return the narrow choice to the owner.
3. **On an accepted reference host and reviewed metric contract:** MEM-005 overhead → MEM-006 soak; QA-004 P99 and QA-005 matched matrix with immutable evidence. Baseline and absolute SLA need OD-BASE-01 and OD-SLA-01; do not set either unilaterally.
4. **Release candidate:** R-01, OD-NOTES-01, R-02 and then separate P9-06 approval. OD-GATE-01 and future PV-02/04/05, OD-CHECK-01 are distinct decisions, not permission to silently modify the V1 release.

## Scope and status accounting

| Bucket | Done | Open | Notes |
|---|---:|---:|---|
| Authorized Phase D | 16 | 0 | 6 DEV + 5 EX-C + 5 EX-S |
| Extra peer-router examples | 2 | 0 | PR #27 completes end-to-end pairing, distinct from the historical 12-command CI |
| Qualification work packages | 0 accepted in Phase Q | 10 | Historic light CI and prototypes do not count as Phase Q closure |
| Owner decisions | 0 new approvals for these gates | 9 | Includes 2 future PV categories and optional gate/job rename |
| Release work packages | 0 | 3 | Nothing published |

**22 open packages** is a normalized planning count, not the number of raw `[ ]` markers scattered throughout historic implementation notes. Work packages count once even when the same requirement appears in QA-005, MEM-006, Phase 9 or an older checklist. Source changes/implementation fixes found during Phase Q must become separately authorized concrete tasks, not be silently assumed complete.
