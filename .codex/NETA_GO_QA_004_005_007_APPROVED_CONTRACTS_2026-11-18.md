# neta-go — QA-004 / QA-005 / QA-007 approved design contracts

**Decision date:** 2026-11-18
**Repository:** `github.com/majiddarvishan/neta-go`
**Reviewed base:** `main` at `bf0f27e6116d6ad5bf99403df615bc57923b5883`
**Status:** OWNER-APPROVED DESIGN ONLY. Implementation, tests, measured SLA thresholds, and release remain open.
**Scope:** Documentation only. Do not interpret design approval as passing tests or a completed QA remediation.

## QA-004 — Pending Deadline Scan

- **QA-004.1 = A:** Retain the existing linear Pending Deadline scan; no unapproved switch to a heap, index, timing wheel, or adaptive algorithm.
- **QA-004.2 = B:** Use quantitative benchmarks and a regression gate against a stable baseline; a regression prompts investigation and owner-approved review, not automatic runtime strategy changes.
- Measure P50/P95/P99 scan latency, allocations, CPU under sustained load, and *actual* Deadline detection delay. Proposed representative Pending populations: 32, 128, 512, 2048, 8192. These are proposed benchmark cases, **not** numeric SLA or approval to change the runtime capacity.
- Preserve bounded shared Timer Manager, absolute deadlines, and exactly-once Pending completion.
- Existing 0/1/32/128/512 microbenchmark evidence is recorded in [QA-004 baseline](NETA_GO_QA004_PENDING_DEADLINE_SCAN_BASELINE_2026-11-18.md); no new measurement was made for this design document.

**OPEN:** Reference workloads/hardware, numerical tolerances, allowed detection latency, CI gating policy, and final representative workload matrix. No claim that the performance gate currently exists or is green.

## QA-005 — SLA, baseline and acceptance

- **QA-005.1 = B:** Balanced, multidimensional qualification: sustained throughput; P50/P95/P99 latency; CPU, memory, allocations/GC; deadline detection; concurrency fairness/stability and extended-load behavior.
- Separate controllable library overhead from network, hardware and application callback behavior. Performance measurements are environment- and workload-specific, not universal production guarantees.
- **QA-005.2 = B:** Record a versioned reference baseline with exact commit, Go version, hardware/OS/runtime details and fixed workload; repeat measurements and compare equivalent runs with noise-aware acceptance.
- Regression gating is required by policy *after* the owner approves quantitative targets and the reference environment. Do not define arbitrary acceptance percentages or break builds due to noisy unmatched measurements. No instrumentation cost is authorized in the normal production hot path.

**OPEN:** Numerical SLA thresholds/tolerances and statistical rule, fixed reference hardware, exact workload, baseline update and approval process, automated CI gate activation.

## QA-007 — Callback Panic behavior

| Decision | Owner-approved contract |
| --- | --- |
| QA-007 = B | Recover user Callback panic; close the related Session, not the whole process. |
| QA-007.1 = A | Cover all user callbacks, including OnClose. |
| QA-007.2 = B | Report through ErrCallbackPanic and Session error diagnostics; no independent OnPanic callback. |
| QA-007.3 = B | Preserve callback name, panic value and complete goroutine stack in diagnostics; omit stack from ordinary Error(). |
| QA-007.4 = A | Close immediately; reject new requests; finalize Pending exactly once; cancel queued unstarted callbacks; do not forcibly stop active callbacks; run OnClose at most once. |
| QA-007.5 = A | OnClose panic cannot replace the original Session close cause; preserve separate OnClose panic diagnostics; never repeat Close/OnClose. |
| QA-007.6 = B | Preserve first recorded panic and bounded ring of recent panics; count dropped diagnostics; independent OnClose diagnostics; thread-safe. |
| QA-007.7 = B | Ring capacity is configurable with internal default/bounds. |
| QA-007.8 = B | Lazy-allocate the ring on first panic; healthy Sessions pay no ring-allocation cost. |
| QA-007.9 = B | Enforce both count and byte budget for recent records; never truncate retained stack traces; first panic and OnClose are budget-exempt. |
| QA-007.10 = B | Ring default 8 records, minimum 1, maximum 32; recent-record byte budget default 256 KiB, maximum 2 MiB. |
| QA-007.11 = B | Capture timestamp at panic recovery, assign per-Session increasing sequence number under synchronized insertion; ring order is recording order, not guaranteed physical occurrence order. |
| QA-007.12 = B | Return independent, thread-safe diagnostic snapshots; never expose internal mutable state to callers. |
| QA-007.13 = B | Retain diagnostics while the Session is reachable; no TTL; independent snapshots outlive the Session if retained by the caller. |

### Special handling

- If a recent diagnostic would exceed the recent-ring byte budget by itself, do not retain that diagnostic in the ring; increment the dropped count. No saved stack trace may be truncated.
- First-recorded Panic and separate `OnClose` Panic diagnostics remain complete outside the ring budget. Thus **there is no hard total per-Session byte bound**.
- Session original terminal cause stays authoritative; panic handling must never trigger repeated Close or OnClose.
- Active user callback goroutines cannot be forcibly terminated by the library.
- All concurrent registration and snapshot operations must be race-free and preserve a consistent snapshot.
- The above is a design contract; `ErrCallbackPanic` or any new diagnostics API is **not claimed to exist in current code**.

### Confirmed numerical configuration

| Property | Approved |
| --- | --- |
| Ring default | 8 records |
| Ring allowed minimum / maximum | 1 / 32 records |
| Recent records default byte budget | 256 KiB |
| Recent records maximum byte budget | 2 MiB |
| Record storage | Lazy allocation on panic |
| First panic and OnClose | Separate, outside budget |

**OPEN, owner decision required before final API/implementation:** Minimum configurable byte budget; exact exported diagnostics method/type and immutable snapshot representation; remaining concurrency edge cases and pathological panic-value/stack-size memory behavior. Do not invent these contracts.

## Compatibility and release constraints

- Go protocol default V2; V1 is explicitly selectable and retains byte-for-byte Pinex C++ 1.5.0 Bind compatibility.
- V2 Bind drops SystemID's trailing NUL and permits up to 20 SystemID bytes; V1 retains 19 bytes plus NUL. Fixed Frame Header remains **10 bytes**.
- No protocol, timeout, resource, worker or callback-order changes are authorized by these design decisions.
- **Do not create tag or GitHub Release `v1.0.0`** without a new, separate, explicit owner approval.
- The established protected-main check requirements remain in force.

## Tracker state and next authorization boundary

- QA-004: DESIGN APPROVED; workload-specific qualification and gate **not implemented/verified**.
- QA-005: DESIGN APPROVED; numbers, baseline and executable regression gate **not complete**.
- QA-007: DESIGN APPROVED; code, tests and live verification **not complete**.
- Other QA items reported as complete remain unchanged; historical audit descriptions are retained.
- Documentation may be committed separately; **no source edit, test execution, PR, merge, tag or release is authorized** in this step.
