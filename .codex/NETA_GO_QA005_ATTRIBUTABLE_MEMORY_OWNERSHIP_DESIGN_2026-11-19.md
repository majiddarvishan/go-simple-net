# QA-005-MEM-001 — Attributable retained memory: approved B, ownership audit and design draft
Date: 2026-11-19
Repository: github.com/majiddarvishan/neta-go
Audit HEAD: f00e7920bd2f336ab9bbe5822fe11467a7851855
Branch: feature/qa004-qa005-qa007-implementation-2026-11-18
Status: **Option B approved by owner; detailed metric/implementation contract is DESIGN DRAFT (not implemented or qualified).**
Authority: .codex/NETA_GO_QA_004_005_007_APPROVED_CONTRACTS_2026-11-18.md; DECISION_LOG §§73–75.
Source anchor: QA004-QA005-SOURCE-V1 at **9b7b2318f73d173a99d22489042f84f8ce58b745**. Do not move or relabel it.

## 1. Decision and scope
The owner approved **B: Hybrid Ownership Accounting** for QA-005-MEM-001 on 2026-11-19. Use existing synchronized resource accounting where feasible, complemented by separately enabled, benchmark-scoped observation; avoid a new global per-frame atomic or network-hot-path profiling. This choice is not approval of a new numerical SLA, exact memory-metric coverage, reference computer, mandatory CI gate, production implementation, PR, merge, tag or release. The design below identifies proposed measurement details requiring evidence before use as a qualified metric.

Do not replace Request credit, Completion, OnClose admission, bounded FIFO, worker pools, callback ordering, existing resource ceilings, V1/V2 wire format, Session lifecycle or QA-004 linear scan. The previously verified QA-007 remains CLOSED, QA-004/005 remain OPEN.

## 2. Code-based ownership map (observed at audit HEAD)
| Retainer and transition | Original source | Current accounting | Measurement caveat |
| --- | --- | --- | --- |
| Outbound payload copied into serialized frame; queued -> active writer -> release on write/error/close | session.go:25-213,393-410; session_transport.go:519-607; session_internal.go:626-706,785-825 | per-Session retainedApplicationBytes + queuedAppBytes, owner retainedApplicationBytes and Server queue bytes | Full frame length is charged; queued and retained are overlapping views of one allocation, not additive. During in-flight admission a charge exists before allocation. |
| Inbound StreamRequest: header admission -> body allocation -> queued callback -> active callback -> return/cancel | session_transport.go:91-158,378-422; resource_charges.go:5-69 | applicationReadCharge; owner retained/queue, session incoming queue | Charge uses wire Header.Length, while only Body is heap-allocated. Queue charge ends on callback start; retained responsibility lasts to return. |
| Inbound matched Response -> Completion queue -> Completion return/cancel | session_transport.go:114-139,424-438; session.go:330-383; resource_charges.go:71-98 | completionReadCharge retainedCompletionBytes plus independently reserved Request credit | Request credit is a count, not bytes; never multiply it by frame length. Capacity-failed Response is drained via bounded scratch, without a retained Response payload. |
| Pending requests, timers, handler closures | session_internal.go:84-91,139,709-737; session_control.go:47-63 | Request-credit count, map entries not measured in bytes | map implementation/capacity, pendingRequest struct and user Completion closure are not represented by retained byte counters. |
| Worker queues and running tasks, across all three pools | worker_scheduler.go:8-173,199-273,364-442 | queue and active counts in pool; Application frame and Request credits live independently | CallbackTask/closure and per-Session queue backing storage are not measured; running task may retain same payload already charged. |
| OnClose-eligible Session -> queued/running OnClose -> callback return | session_internal.go:219-225,855-888; resource_charges.go:100-114 | one owner OnClose credit from pre-admission until callback completion, idempotent release | Credit counts responsibility and is transferred, never duplicated; Session + task memory not captured in bytes. |
| Control frame queues and active control write | session_internal.go:564-579,690-701; session_transport.go:440-455,519-607 | per-Session queuedControlBytes | Not in owner retainedApplicationBytes; distinguish full control allocations and active ownership from application bytes. |
| Panic history, saved stacks and raw recovered values | diagnostics.go:39-54,118-164 | bounded recentBytes only; First/OnClose/raw values exempt | First/OnClose can retain arbitrary external object graphs. Some same strings are referenced by first/recent; do not sum twice. |
| Bind, pre-bind, conn/net buffers, goroutines and runtime | session_internal.go; client_connect.go; server_lifecycle.go | session counts and pre-bind admission, not owner-retained bytes | Kernel/socket/Go runtime/host application memory excluded; internal per-Session metadata may be attributed in a separately qualified category. |

### Correctness invariants
- Every allocation/object is accounted at most once under one *owning responsibility*, independent of multiple references, queues, or lifetime states.
- Admission **reserve** is not allocation; a temporary, not-yet-materialized reservation must not be reported as actually retained heap bytes. Report reservations separately.
- Queue membership is a subset/lifecycle state of retained payload ownership; never add queued counters to retained totals.
- No double charge when writer claims a frame, when queued callback begins, on Completion terminal settlement, or when OnClose ownership transfers to its Task.
- Callback-visible payload remains owned by the library until callback return; the caller retaining a Payload slice beyond callback return is outside the library-owned lifetime (its HeapAlloc consequences belong to the application).
- Recovered panic processing and late callbacks after Session.Done must preserve existing QA-007 semantics; first/OnClose raw external graph size is *unknown*, not zero.
- A memory release follows the existing idempotent charge release and actual ownership termination, not merely Session.Done, and not the time an error is observed.
- Do not change user API, raw ownership transfer behavior, retry policy, error taxonomy, or binding/callback order to improve observability.

## 3. Four different quantities (do not conflate)
1. **Retained application charge (B)**: policy-accounting frame length including wire header, possibly reserved before bytes exist. An enforced quota, not heap usage.
2. **Allocated bytes (B/op)**: total allocation traffic over the interval, including freed objects; not retained bytes.
3. **Live Heap (B)**: process-wide runtime.MemStats.HeapAlloc, including Go runtime, test harness, host code, garbage-collector state and external references; not neta attribution.
4. **Attributable retained memory (B)**: a versioned, explicitly scoped measure of live storage owned by *neta-go* at an instant, with retention identity, release lifecycle and coverage/exclusions. This is not the allocator's physical RSS/HeapAlloc and must not imply exact physical allocator size.

## 4. Proposed B accounting contract (detailed, pending adapter proof)
Maintain two independent planes:
- **Admission/accounting plane**: existing Session + owner ledgers, unchanged for correctness and limits. Optional high-water bookkeeping must update *under the existing owner lock* only if experimentally justified. Do not introduce a contended global atomic, separate mutex or scan on each network operation.
- **Observation plane**: benchmark-only snapshots over real owner Sessions/dispatcher structures, with an explicit coverage table and monotonic sampling clock. Publication only after consistent evidence and instrumentation-effect qualification.

Versioned category ledger:
- **F1 outgoing app-owned frame storage:** bytes of the unique live `outboundFrame.bytes` backing allocation (queued, active writer or in-flight admission once allocated), counted once. Report requested/capacity bytes; allocator bucket overhead is not assumed.
- **F2 inbound Application body storage:** unique `Frame.Body` backing bytes owned by waiting/running OnRequest or read-in-progress, not full wire frame length. No duplicate if the same payload is referenced by the callback task.
- **F3 inbound Completion body storage:** unique matched Response body backing bytes until its Completion truly returns/cancels. ErrCapacity scratch drain is not F3; it is transient scratch handled separately.
- **F4 Control frame storage:** unique live control-frame backing bytes in control, barrier or active writer.
- **M1 library metadata (separate):** Session/pendingRequest/outboundFrame/charge/callbackTask objects, callback queue backing arrays and worker metadata where ownership/capacity can be measured reliably. For maps, allocator bucket bytes are not derivable exactly from len(map); any model must declare estimator version and error/coverage, and may not silently present len(map)*sizeof as real heap.
- **D1 diagnostics (separate):** library-created unique panic stack/value text, ring entries and retained internal diagnostic metadata. First and Recent can share Stack strings; deduplicate references. The size of arbitrary raw panic objects/foreign closure capture graphs is unknown and reported as excluded/externally reachable.
- **E1 excluded:** user-owned input payload, user handler object graphs and captured values beyond known internal allocations, TCP/kernel buffers, runtime allocator/GC overhead, goroutine stacks, host and benchmark-harness data, preexisting externally owned objects.

Proposed metrics for design review, **not yet emitted**:
- `retained-attributed-B`: selected QA-005 comparison statistic, proposed **sampled peak of F1+F2+F3+F4 plus validated eligible M1/D1** across benchmark-owned neta Client+Server. Do not publish if the coverage/ownership and sampling validity checks fail.
- `retained-attributed-sampled-p50-B`, `-p95-B`, `-mean-B`, `-peak-observed-B`, `-sample-count`, `-max-gap-ns`: optional time-distribution and evidence diagnostics. A sampling peak is a *lower bound* on true event peak.
- `retained-charged-application-B`, `retained-charged-completion-B`, `retained-queued-app-B`, `retained-onclose-credit-count`, `retained-pending-count`, `memory-accounting-fault-count`: coverage and resource-accounting diagnostics, never additional additive memory terms.
- `memory-schema-version`, `covered-categories`, `excluded-categories`, `sample-interval`, `ownership-ledger-reconciled` and comparable measurement fixture SHA: mandatory provenance.
- Peak for each independently locked owner can be tracked exactly for **its own charge** within that owner ledger; sum of independently attained owner peaks is an **upper bound**, not a simultaneous global peak. The global sampled sum is not a linearizable snapshot without synchronization. Never label either as exact global peak.

Do not use `unsafe.Sizeof` or `cap` to claim a full Go heap footprint: they describe structural size/requested slot capacity, not allocation buckets, shared underlying arrays, closures or reachable foreign object graphs. Where a class is not accurately measured, show a separately named partial/lower-bound category or mark memory qualification **INCONCLUSIVE**. Further approval is needed for the definitive gate metric and coverage policy.

## 5. Proposed edge-case acceptance matrix
| Event | Required accounting result |
| --- | --- |
| Reject outgoing at queue/request admission | No bytes allocated to admitted frame, charges rolled back, no Pending/Completion orphan; sample provisional reservation separately. |
| Allocation/build error or Close racing snapshot | Release reservations once; any allocated buffer's lifetime ends only when no neta owner retains it; no double-release on concurrent Close. |
| Partial inbound read, invalid Header, scratch drain | Release provisional incoming/Completion charges; scratch excluded from persistent retained metric but optionally recorded as transient memory. |
| OnRequest queued -> running / success / panic | Release queue at start, retained body at actual callback return (even after Done); panic takes existing QA-007 path. |
| Request Timeout or Cancel / late Response | Exactly one terminal Completion; Request credit retained through handler responsibility; dropped/late Response does not become F3. |
| Forced Close with queued Completion | Preserve all accepted Completion tasks and credits; cancel only unstarted Application callbacks, release their bodies once. |
| OnClose admission failure / reconnect / callback panic | Pre-admission credit rollback on failure; generation-shared owner ledger; release exactly once on handler return or valid cancelled task path. |
| Diagnostic ring eviction and first/OnClose records | Distinct owned strings not counted more than once, lazy ring allocation, saved raw object reachability labelled unbounded/unknown. |
| Completion/OnClose callback permanently blocked | Charge remains while responsibility remains; the benchmark must time out/report inconclusive instead of reporting zero. |
| End-of-workload cleanup | All existing charge counters zero where expected; compare reconciled live references separately; resourceAccountingFailed anywhere invalidates the memory result. |

## 6. Overhead and evidence boundary
- No continuously running sampling goroutine in the library. No per-frame stack/heap profiler, reflect traversal, allocation stack capture or new owner-global atomic.
- Benchmark sampling must use bounded frequency and short lock durations; never hold resource, Session or worker locks over I/O or user callbacks. Snapshot lock order follows existing invariants (no nested owner/Session/worker locks without proof).
- Quantify probe effect with same-SHA uninstrumented vs instrumented A/A throughput, P99, CPU/op, allocations/op and GC. Regression/measurement instability remains INCONCLUSIVE, not an automatic green light.
- Keep QA-005's fixed 8 Core + 24 Full, ten alternating matched pairs / 95% confidence / 15% retained threshold, plus 30m stability requirement; no rerun of 370 prior results just to prepare a design.
- The current source-V1 archive has no attributable memory. It cannot be retroactively filled with a fabricated zero or converted into an approved numeric baseline.

## 7. Further steps / approval gates
1. [x] Owner chooses B as the architecture family.
2. [x] Audit ownership, charge lifetimes, gaps, double-counting and exclusions; record this design draft.
3. [ ] Establish a *versioned and reproducible equal-on-both-sides* benchmark measurement adapter, resolve effective fixture fingerprinting and provenance without mutating pinned source SHA.
4. [ ] Prove exact coverage for proposed memory categories, concurrency-safe snapshots and peak semantics; obtain owner review of final `retained-attributed-B` definition and missing-coverage policy.
5. [ ] Implement any approved minimal instrumentation, each stage in its own commit, with race/correctness/overhead checks and independent docs updates.
6. [ ] Capture matched qualified evidence after reference machine and metric method are separately approved. Source baseline SHA fixed.
7. [ ] Separately approve numerical baseline, absolute SLAs and mandatory performance gate; do NOT enable now.

No production files changed, tests run or numerical PASS claimed for this design step.
