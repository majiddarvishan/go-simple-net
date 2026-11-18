# neta-go — QA-004 / QA-005 / QA-007 approved design contracts

**Decision date:** 2026-11-18
**Repository:** `github.com/majiddarvishan/neta-go`
**Originally reviewed base:** `main` at `bf0f27e6116d6ad5bf99403df615bc57923b5883`
**Latest documentation-only review base:** `main` at `397dedf6435196195eafa2c50c9fdab71c2af2c1` (2026-11-18)
**Authority:** owner approvals through QA-004.3, QA-005.8 and QA-007.30, including explicit supersession of forced-close Completion and OnClose saturation policies. Subsequent appendices below are authoritative over earlier historical OPEN notes.
**Status:** OWNER-APPROVED DESIGN ONLY. Implementation, tests, measured SLA thresholds, and release remain open.
**Scope:** Documentation only. Do not interpret design approval as passing tests or a completed QA remediation.

## QA-004 — Pending Deadline Scan

- **QA-004.1 = A:** Retain the existing linear Pending Deadline scan; no unapproved switch to a heap, index, timing wheel, or adaptive algorithm.
- **QA-004.2 = B:** Use quantitative benchmarks and a regression gate against a stable baseline; a regression prompts investigation and owner-approved review, not automatic runtime strategy changes.
- Measure P50/P95/P99 scan latency, allocations, CPU under sustained load, and *actual* Deadline detection delay. Proposed representative Pending populations: 32, 128, 512, 2048, 8192. These are proposed benchmark cases, **not** numeric SLA or approval to change the runtime capacity.
- Preserve bounded shared Timer Manager, absolute deadlines, and exactly-once Pending completion.
- Existing 0/1/32/128/512 microbenchmark evidence is recorded in [QA-004 baseline](NETA_GO_QA004_PENDING_DEADLINE_SCAN_BASELINE_2026-11-18.md); no new measurement was made for this design document.

**Updated decision status:** QA-004.3=C approves relative P99 detection-delay regression qualification (15%, statistically validated) plus an evidence-based absolute P99 limit to be selected only after measurement and separate owner approval. Hardware qualification, the absolute numeric limit and executable CI gate are not complete. See the 2026-11-18 addendum.

## QA-005 — SLA, baseline and acceptance

- **QA-005.1 = B:** Balanced, multidimensional qualification: sustained throughput; P50/P95/P99 latency; CPU, memory, allocations/GC; deadline detection; concurrency fairness/stability and extended-load behavior.
- Separate controllable library overhead from network, hardware and application callback behavior. Performance measurements are environment- and workload-specific, not universal production guarantees.
- **QA-005.2 = B:** Record a versioned reference baseline with exact commit, Go version, hardware/OS/runtime details and fixed workload; repeat measurements and compare equivalent runs with noise-aware acceptance.
- Regression gating is required by policy *after* the owner approves quantitative targets and the reference environment. Do not define arbitrary acceptance percentages or break builds due to noisy unmatched measurements. No instrumentation cost is authorized in the normal production hot path.

**Updated decision status:** QA-005.3=C, .4=B, .5=C, .6=C, .7=B, .8=C are approved. Relative regression tolerances and workload dimensions are approved; qualifying reference hardware, empirical absolute SLA targets, statistical implementation and activated CI gate remain open. See addendum.

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

**Updated decision status:** QA-007.14–QA-007.30, including minimum 64 KiB budget, typed `Session.Diagnostics()` API, concurrency, late panics, Completion and OnClose policy, are owner-approved in the addendum. Raw panic value and independent full stacks are intentionally not subject to a hard whole-Session memory cap. Implementation and verification are open.

## Compatibility and release constraints

- Go protocol default V2; V1 is explicitly selectable and retains byte-for-byte Pinex C++ 1.5.0 Bind compatibility.
- V2 Bind drops SystemID's trailing NUL and permits up to 20 SystemID bytes; V1 retains 19 bytes plus NUL. Fixed Frame Header remains **10 bytes**.
- No protocol/wire format or timeout-default change is approved. **Later QA-007.26 and QA-007.30 choices explicitly supersede legacy callback cancellation, OnClose credit/admission and worker/lifecycle contracts; these source/runtime changes require separate implementation authorization.**
- **Do not create tag or GitHub Release `v1.0.0`** without a new, separate, explicit owner approval.
- The established protected-main check requirements remain in force.

## Tracker state and next authorization boundary

- QA-004: DESIGN APPROVED; workload-specific qualification and gate **not implemented/verified**.
- QA-005: DESIGN APPROVED; numbers, baseline and executable regression gate **not complete**.
- QA-007: DESIGN APPROVED; code, tests and live verification **not complete**.
- Other QA items reported as complete remain unchanged; historical audit descriptions are retained.
- Documentation may be committed separately; **no source edit, test execution, PR, merge, tag or release is authorized** in this step.


## Owner-approved final design addendum — 2026-11-18 (QA-004.3, QA-005.3–8, QA-007.14–30)

**Authority and state:** The owner individually approved every choice listed below in conversation. This is a documentation-only record; current production source, tests, CI, release policy and protected `main` have not been changed by this addendum. Earlier clauses contrary to this addendum are historical, superseded design/implementation evidence rather than the new target contract.

### QA-004.3 = C — actual Deadline detection acceptance

- Define detection delay as `max(0, internal terminal Timeout settlement timestamp - the Request's absolute deadline)`, measured at internal settlement rather than user Completion callback start/return.
- For representative Pending populations 32/128/512/2048/8192, record P50/P95/P99 detection delay, scan cost, allocation, CPU and any material scheduler contention separately. Use the approved QA-005 statistical comparison and **15% relative P99 regression tolerance**, with absolute comparison handling for zero/near-zero baseline values.
- Absolute P99 detection limit remains **unset** pending representative measurement and separate owner approval. A threshold without that evidence must not be invented.
- Keep Linear Scan and the bounded shared Timer Manager; no automatic heap/timing-wheel redesign.
- **Workload boundary:** V1 per-Session outstanding Request-credit maximum is **512** (default **128**). A single real Session with 2048/8192 accepted Requests violates that cap. 2048/8192 are permitted as isolated *synthetic scan microbenchmarks*, or as total work distributed over multiple valid Sessions when explicitly labeled; never pretend they are one-Session admissible live traffic.

### QA-005.3–8 — acceptance and baseline rules

| Decision | Approved |
| --- | --- |
| QA-005.3 = C | Staged environment: GitHub-hosted performance results are advisory/report-only. Mandatory numeric gate activates **only after** owner-approved stable reference environment and valid baseline exist. |
| QA-005.4 = B | Multi-dimensional fixed reference workload with **8 Core** and **24 Full** selected scenarios; dimensions include 64 B / 4 KiB / 64 KiB payloads; 1/32/256 Sessions; concurrency 1/4; Request/Response and NoWait; medium and near-capacity load. Full is a selected subset, not the Cartesian product. |
| QA-005.5 = C | Combine baseline-relative regression gate with separately approved absolute SLA limits; do not invent absolute throughput/latency values. |
| QA-005.6 = C | Repeated matched baseline/candidate comparison using statistical uncertainty; outcomes PASS / FAIL / INCONCLUSIVE. No hard failure merely from uncertain/noisy results. |
| QA-005.7 = B | **10 paired runs**, **95% confidence**, regression tolerance **10%** for throughput and P50/P95 latency; **15%** for P99 latency, CPU/op, allocations/op, attributable retained memory and Deadline Detection P99. Core 8; Full 24; **30-minute stability** exercise. |
| QA-005.8 = C | Versioned, immutable baselines; replacement only on valid comparable measurements and explicit owner approval. A merge to main must not auto-promote a baseline. Historical baselines remain accessible. |

- A baseline records exact source SHA, Go/compiler, OS, hardware, runtime/config, test dimensions, sample size and measured distributions. Compare only equivalent conditions.
- A material regression is FAIL only with sufficient statistical evidence that deterioration exceeds the approved practical tolerance; below-tolerance results can PASS if confidence supports that conclusion; otherwise mark INCONCLUSIVE. Do not silently count missing/invalid results as PASS. Statistical method, multiple-comparison handling and P99 within-run sample adequacy belong to validation tooling, not the production hot path.
- Zero or near-zero baseline metrics require meaningful absolute deltas rather than undefined/misleading percentages. No additional instrumentation is authorized on the normal production hot path.
- Pending count microbenchmarks at 2048/8192 are **not** authorization to raise public Session Request-credit limits.

### QA-007.14–30 — callback recovery, diagnostics, lifecycle and resource admission

| Decision | Owner-approved design |
| --- | --- |
| QA-007.14 = B | Configurable recent-ring byte budget **minimum 64 KiB**, default 256 KiB, maximum 2 MiB. |
| QA-007.15 = B | General exported `Session.Diagnostics()` (not a dedicated panic-only method). |
| QA-007.16 = B | Export typed diagnostic structs; `PanicRecord.Stack` is immutable snapshot-friendly `string`, not `[]byte`. |
| QA-007.17 = C | Retain raw `recover()` value internally for fidelity; never expose its possibly mutable `any` to callers. Public snapshot contains safe type/text. This may retain large referenced objects. |
| QA-007.18 = B | Safely render arbitrary panic value *without invoking user-defined String/Error methods or other user code*. |
| QA-007.19 = B | Bound `ValueText` to **1 KiB**, UTF-8-safe truncation with `ValueTruncated` marker; complete Stack is never truncated for retained records. |
| QA-007.20 = B | `SessionDiagnostics.CallbackPanics` typed nested group. |
| QA-007.21 = A | Exported fields shown below, with pointer presence for First and OnClose. |
| QA-007.22 = A | `Dropped` counts cumulative recent-ring evictions and oversized-record refusals. Do not evict saved records merely to reject a new individually oversized record. First/OnClose independent records are never discarded by ring capacity. |
| QA-007.23 = A | Dedicated per-Session diagnostic Mutex: linearize sequence, First/Recent/OnClose/Dropped updates and coherent snapshots. Not taken in normal healthy send/receive/callback path. |
| QA-007.24 = A | Record recovered panics from already-started callbacks even after Session `Done`; snapshots are coherent *as read*, not immutable across successive reads. |
| QA-007.25 = A | First cause admitted to Closing is immutable `Session.Err()`; a later panic is diagnostic only; if a panic initiates Close, `ErrCallbackPanic` is the cause. |
| QA-007.26 = A | On any Forced Close, cancel unstarted **Application** callbacks but **retain Completion notifications** of accepted Requests (all causes of Close). This explicitly supersedes legacy API-001b-FC-A's Completion cancellation. |
| QA-007.27 = A | Recover panic in a Completion without cancelling other Completion notifications. |
| QA-007.28 = A | Reserve capacity for eventual Completion at `SendRequest` admission using existing per-Session/owner Request credits; no second Completion count credit or goroutine per Request. Hold until actual callback completion/responsibility termination. |
| QA-007.29 = A | Linearizable Application Callback Start Claim versus Closing; no new claim after Closing, but a callback previously claimed may physically enter user code after Close. No lifecycle lock held while invoking user code. Completion/OnClose have their independent dispatch policy. |
| QA-007.30 = A | An `OnClose`-eligible Session must reserve one owning Client/Server OnClose event credit **before acceptance**, else reject new Session admission immediately. Retain credit through active Session, queued/running OnClose and release on callback return, including recovered panic. An instance with no configured OnClose does not reserve. Owner default **1,024**, existing configurable ceiling maximum **4,096**. Preserve the independent bounded OnClose worker pool. This explicitly supersedes WORKER-003's post-Done best-effort skip. |

**Approved public target type layout** (names and fields are design-approved; not yet implemented):

```go
type PanicRecord struct {
    Callback       string
    ValueType      string
    ValueText      string
    ValueTruncated bool
    Stack          string
    Time           time.Time
    Sequence       uint64
}

type CallbackPanicDiagnostics struct {
    First   *PanicRecord
    Recent  []PanicRecord
    OnClose *PanicRecord
    Dropped uint64
}

type SessionDiagnostics struct {
    CallbackPanics CallbackPanicDiagnostics
}
```

- `Recent` is sorted by increasing recording sequence (not real-time panic occurrence); timestamps come from recovery time. First is the first **recorded** panic and OnClose is separately recorded. Caller snapshots own their data/slices and cannot mutate Session state.
- Ring record count defaults to 8, min 1, max 32; it is allocated lazily upon the first panic. Recent ring's byte and count limits apply together. Individually oversized ring records increase Dropped without evicting current records. First/OnClose records and raw internal value are outside ring budget, so **no total hard per-Session memory cap is promised**.
- Exactly one internal terminal settlement per accepted Request; a Completion is admitted once and can be **started at most once**. Its start/return cannot be guaranteed within a finite period if user callback workers are permanently blocked. A Session's `Done()`, `Close()` and transport teardown never wait for user code. Pending settlement should preserve a prior winning valid Response; otherwise terminal error follows the winning event. No duplicate settlements or Completion starts.
- The original Session terminal cause and OnClose execution count never change because of a late panic. OnClose panic is separately diagnosed and does not re-close the Session.
- Three existing independent owner-scoped pools (Application, Completion, OnClose), approved worker ceilings, Request credits and owner generations stay bounded. No unbounded queue, overflow goroutines, per-Request worker or new wire format is approved.
- OnClose credit now counts **admitted OnClose-eligible live Sessions plus queued and running OnClose** within the same owner ledger. Reserving a credit transfers it to the OnClose notification after Done without double charge. When all credits are held, new eligible Session acceptance is denied; already-admitted Sessions keep working.
- **Implementation review checkpoints, not new owner choices:** ensure credit acquisition/release at Server Bind, Client Connect, rejected/failed handshake and reconnect generation transitions; preserve `Stop()` semantics without silently dropping owned Completion/OnClose tasks; verify the scheduler can retain queued work and worker ownership after Session Done. No further behavioral policy is inferred without owner approval.

### Explicit supersessions against older frozen designs

1. **API-001b-FC-A / WORKER-005 Forced Close A:** legacy queued-Completion cancellation and queued credit release on Close are superseded. Only unstarted Application callbacks are cancelled on Close; Completion results remain owned under existing Request-credit admission until dispatch/return or other valid completion of responsibility.
2. **WORKER-003 / N-ORD2 / API-001b-DONE-A:** legacy best-effort OnClose skip at event saturation is superseded for admitted eligible Sessions. Reserve on admission, reject new eligible Sessions on insufficient capacity, and preserve OnClose once admitted. At-most-once still applies; there is no guarantee of *timely* start or return when workers are blocked.
3. **Current docs and implementation:** `docs/LIFECYCLE.md` and historical Worker/Decision docs previously described the shipped behavior. Historical sections are retained as evidence with supersession notices. Source and tests are **not yet adapted**. Do not represent these design approvals as existing runtime guarantees.

### Qualification, release, and authorization boundary

- QA-004/QA-005/QA-007 stay **unchecked** in the canonical implementation Task Tracker; the contracts are frozen **at design level**, not verified in running code.
- Absolute numerical SLA limits, qualified reference hardware, executable statistical comparison, CI gate activation, reference workload implementation and QA-007 tests remain pending.
- No source, tests, CI config, default version, wire interoperability, tag, Release, PR or merge is authorized by this documentation-only update. Future implementation requires separate owner permission.
