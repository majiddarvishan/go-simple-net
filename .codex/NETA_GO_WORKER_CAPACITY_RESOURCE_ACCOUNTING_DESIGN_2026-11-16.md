# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** WORKER DESIGN COMPLETE — WORKER-001 through WORKER-009C CLOSED / OWNER APPROVED  
**Repository:** `github.com/majiddarvishan/neta-go`, branch `main`  
**Implementation:** NOT AUTHORIZED. Documentation/design only.

## 1. Authority and scope

This document consolidates the Worker Capacity and Resource Accounting design after the owner-approved Q-003-R1, PERF-002 A, PERF-003 A model, API-001b-FC-A, N-ORD2, DONE-A and C-ORD-B decisions.

Binding source: [DECISION_LOG.md](DECISION_LOG.md). This document does not reopen those decisions.

The following are separate resource classes and must not be conflated:

1. **Application execution:** user `OnRequest` handlers.
2. **Completion execution:** per-Request user completion callbacks.
3. **OnClose execution:** Session lifecycle callback.
4. **Internal engine:** Reader/Writer, protocol/control processing, pending correlation, timers, terminal settlement, transport teardown and resource accounting. This engine is not an arbitrary-user-callback worker pool and must not wait for user callbacks.

Numeric worker defaults, public option names, queue/event ceilings and payload-byte limits are not approved by this document.

## 2. WORKER-001 — Owner-scoped worker capacity (OWNER APPROVED, OPTION A)

The owner explicitly approved Option A.

### Binding rules

- Every Client instance owns an independent callback-resource domain.
- Every Server instance owns an independent callback-resource domain.
- All Sessions owned by one Server share that Server's corresponding aggregate worker/resource budgets.
- All reconnect generations of one Client share that Client's corresponding aggregate worker/resource budgets.
- A callback that started in an old Session generation continues consuming the owning Client/Server active-worker capacity until it returns, even after Session teardown and reconnect.
- Independent Client and Server instances do not consume each other's worker permits and do not require a hidden process-global callback pool or process-global active-worker cap.
- WORKER-001 defines the **ownership boundary only**. It does not approve physical pool implementation, numeric capacities, defaults, queue sizes, public option names or OnClose saturation behavior.

### Rationale

Per-instance ownership keeps unrelated library users isolated, avoids hidden cross-instance contention and allows each Client/Server to be configured for its own workload. Reconnect cannot reset a single instance's accounting, so permanently blocked callbacks cannot multiply capacity by creating new Session generations.

A process may of course contain multiple instances and therefore multiple independent budgets. A mandatory process-wide aggregate cap is not part of the V1 contract selected here.

## 3. Approved callback and lifecycle constraints

| Domain | Binding contract | Capacity details still open |
|---|---|---|
| Application | Q-003-R1 per-Session concurrency plus WORKER-002 owner-wide Application active ceiling; 1 is serial, >1 permits overlap | Numeric defaults, event-count and active-payload bounds |
| Completion | C-ORD-B per-Session concurrency plus WORKER-002 owner-wide Completion active ceiling; FIFO start selection by terminal-settlement enqueue order | Numeric defaults, queue metadata and payload-byte bounds |
| OnClose | N-ORD2 independent sublane; eligible only after Done; may overlap Completion; best-effort/at-most-once; WORKER-002 owner-wide active ceiling | Active+queued event budget and saturation disposition |
| Internal engine | Independent of all user callbacks; Done closes after internal teardown and immutable terminal-cause publication | Internal loop implementation and API-002 Stop aggregation |

Forced Close A remains binding: unstarted Application and Completion callbacks are cancelled; started callbacks cannot be killed and retain active capacity until return. A running Completion also retains its PERF-003 Request credit until return. Done never joins user callbacks.

## 4. Resource ledgers — keep each obligation distinct

### 4.1 Application

- Waiting incoming frames remain charged to the existing Q-009/Q-010 per-Session and Server aggregate byte budgets.
- A frame moving from the waiting queue to active callback ownership must transfer its accounting without an uncharged gap or accidental double charge.
- Each started handler consumes one per-Session active slot and one owning Client/Server active Application slot until callback return.
- Active callback payload and goroutine stack are not covered by the waiting-frame byte budget.
- WORKER-007 A is approved: per-Session incoming/outgoing Application frame counts plus a Server-aggregate queued-frame count across both directions.

### 4.2 Completion

- PERF-003 Request credits are reserved before SendRequest acceptance and cover the complete admitted Request lifecycle: response wait, terminal settlement, queued Completion and running Completion.
- Response/timeout/close settlement reuses the already-reserved credit; it must not allocate a second terminal-event credit.
- A physically executing Completion also consumes an active Completion-worker permit. The permit is distinct from the Request credit.
- A queued Completion cancelled by Forced Close A releases its Request credit and library-owned queued payload/metadata. A started Completion retains both Request credit and active-worker permit until it returns.
- Completion payload bytes retained by the library while queued or executing need a separate owner-scoped byte ceiling under WORKER-008. User retention of the owned payload after callback return remains outside library accounting.

### 4.3 OnClose

- At most one OnClose event may be admitted for each closed Session.
- OnClose never consumes PERF-003 Request credits.
- It uses the WORKER-002 OnClose active-worker ceiling plus a separate bounded active+queued event population per owning instance.
- A started OnClose retains its active-worker and event charges until return.
- If the event budget is exhausted, the WORKER-003 approved policy permanently skips that OnClose invocation. Done and internal teardown still complete.

## 5. Recommended scheduler and worker lifecycle (PROPOSAL)

- Maintain per-Session FIFO work metadata and a bounded ready-Session set per callback class.
- Use fair ready-Session rotation rather than draining one busy Session indefinitely.
- Worker selection must check the Session-local concurrency limit and the owning Client/Server class capacity before committing a callback from queued to started.
- The queued-to-started transition and Forced Close cancellation must have one linearization boundary. If Forced Close wins, the callback must never start and all its queued charges are released once. If start wins, the callback is running and retains its charges until return.
- Never hold registry/global hot-path locks while invoking user code.
- Never create an unbounded goroutine per Request, frame or callback event.
- Lazy allocation is preferred; do not eagerly allocate each Session's maximum queue or payload capacity.
- WORKER-004 has selected three independent physical class pools. WORKER-006 has approved per-class round-robin ready-Session scheduling. WORKER-007 has approved Application queue count limits. N-ORD2's Completion/OnClose isolation remains binding.

## 6. Saturation and liveness invariants

| Saturation | Required behavior |
|---|---|
| Application active capacity full | Keep work in the existing bounded incoming queue; if Q-009/Q-010 admission fails, close only the offending Session |
| Completion active capacity full | Keep admitted completions bounded by their existing Request credits; do not spawn overflow goroutines; reject new SendRequest when PERF-003 credits are exhausted |
| OnClose event budget full | Permanently skip the new OnClose invocation; Done/internal cleanup/reconnect proceed; no retry or overflow goroutine (WORKER-003 approved) |
| All workers in one class block forever | Only that class loses execution capacity; internal engine and other independently bounded classes continue |
| Reconnect while old callback runs | Old callback remains charged to the same owner; new Session cannot reset or bypass the owner budget |
| Forced Close races with callback start | Exactly one winner: cancel-before-start or started-and-charged-through-return |

No library can terminate arbitrary user code. Finite active capacity therefore intentionally bounds the damage of permanent callback stalls; it cannot guarantee callback progress after every worker in a class is permanently blocked.

## 7. Memory accounting principles

- Q-008/Q-009/Q-010 queue-byte budgets are not whole-process RSS limits.
- Account library-owned payload bytes while they are waiting or actively retained by a library callback work item; transfer charges atomically when ownership changes.
- Count limits protect metadata-heavy zero-length/small events; byte limits protect large payloads. Neither dimension substitutes for the other.
- PERF-002 A remains binding: outbound accepted payload is snapshotted; inbound payload is owned and may be retained by user code; the library must not recycle user-visible inbound storage.
- Payload memory deliberately retained by the application after callback return is outside the library's enforceable memory ceiling.
- Do not preallocate the maximum configured capacity for every Session. Limits are lazy admission ceilings.

## 8. Performance implications

- Owner-scoped budgets avoid a mandatory process-global synchronization point on every callback admission.
- Per-Session queues and ready-session rotation localize contention; global/owner counters still need race-free reservation but must not be held while calling user code.
- Bounded class-isolated worker loops avoid per-event goroutine creation and reduce allocation/scheduler churn. WORKER-004 fixes the pool topology; WORKER-006 approves fair ready-Session selection.
- Independent class budgets prevent blocked Application callbacks from consuming Completion/OnClose capacity and prevent Completion saturation from consuming OnClose's reserved capacity.
- WORKER-006's approved per-class round-robin over eligible Sessions improves tail latency and prevents a high-volume Session from monopolizing future callback starts, but cannot preempt running callbacks or overcome a class whose workers are all permanently blocked.
- No throughput/latency SLA or numeric default is inferred. Measurement and benchmarks remain unauthorized.

## 9. WORKER-002 — Independent owner-wide active-worker ceilings (OWNER APPROVED, OPTION A)

The owner explicitly approved Option A.

### Binding V1 contract

Each Client instance and each Server instance has three independently configurable aggregate active-worker ceilings:

1. **Application:** active `OnRequest` callbacks.
2. **Completion:** active per-Request Completion callbacks.
3. **OnClose:** active Session `OnClose` callbacks.

Rules:

- A Session's configured per-class concurrency is intersected with the remaining active capacity of its corresponding owner/class ceiling.
- Active capacity is isolated by class. Application, Completion and OnClose cannot borrow, consume or steal one another's active-worker permits.
- These ceilings count callbacks physically executing, not queued events. Each class's waiting/event population remains bounded separately.
- WORKER-001 scope applies: all Sessions of one Server share its three ceilings; all reconnect generations of one Client share its three ceilings. Started callbacks from retired Session generations remain charged until return.
- Independent Client/Server instances remain independent; no process-global active-worker ceiling is introduced.
- No numeric defaults, public option names, idle-worker lifecycle or event capacities are selected.

### Rationale

Explicit class ceilings make upper bounds independent of current Session count, prevent one class from consuming another class's execution reserve and keep reconnect churn from multiplying active work. The cost is three additional capacity dimensions per Client/Server; this does not require eager creation of all worker goroutines.

## 10. WORKER-003 — OnClose event-budget saturation (OWNER APPROVED, OPTION A)

The owner approved **Option A: skip the new OnClose event when the event budget is full**.

### Binding V1 contract

- Each owning Client/Server has a separate bounded OnClose event budget. It counts **queued + currently executing** OnClose events.
- At most one OnClose event may be admitted per closed Session.
- After Session `Done` closes, if an OnClose callback is configured, the engine makes one nonblocking reservation attempt.
- If reservation succeeds, the event is queued. Its event credit remains held while queued and while the callback runs, and is released only after callback return.
- If reservation fails because the event budget is full, the OnClose invocation is skipped permanently. No retry, overflow goroutine, hidden queue or wait is allowed.
- A started OnClose callback holds both its WORKER-002 active-worker permit and its event credit until return.
- Skipping OnClose never delays or changes Done, terminal cause, socket closure, internal cleanup, reconnect eligibility, Application/Completion capacity or PERF-003 Request credits.
- Client reconnect generations share the same Client event budget. Queued/running events from retired generations remain charged until return.
- No numeric event-budget default, public option name or diagnostic API is selected.

### Rationale

This preserves transport and internal-engine liveness even if OnClose callbacks block forever. It follows N-ORD2/DONE-A's independent, best-effort/at-most-once contract and avoids coupling connection admission/reconnect to arbitrary user callback progress. Counting queued + running events also bounds retained Session references and event metadata.


## 11. WORKER-004 — Three Class-Isolated Worker Pools (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Every Client instance and every Server instance owns three logically and physically isolated callback worker pools: Application, Completion and OnClose.
2. Each class pool has its own scheduler, ready-session/work queue, dispatch path, lock domain and wake-up mechanism. A common internal worker-loop implementation may be reused, but the pools must not share execution capacity, queues, scheduler locks or wake-up state.
3. Workers are created lazily when dispatchable work exists and no reusable idle worker in that same class is available. No worker is created per Request, frame or callback.
4. A worker executes one callback synchronously at a time and is reusable only within its own class pool. It never executes work belonging to another callback class.
5. The number of worker goroutines in a class is bounded by that class's configured WORKER-002 active-worker ceiling. An idle worker does not hold an active-callback permit, but its goroutine and stack remain part of the bounded worker footprint.
6. A blocked callback occupies only a worker in its own class. Saturation or permanent blocking in Application must not occupy Completion or OnClose workers; Completion must not occupy Application or OnClose workers; OnClose must not occupy either other class.
7. The internal protocol/transport engine remains outside all three user-callback pools and must not wait for arbitrary user callback return.
8. WORKER-002 owner-wide active ceilings, per-Session concurrency intersections, WORKER-001 owner scope and retired-generation accounting remain binding. Client reconnect does not create a new pool or reset old callback charges.
9. Independent Client/Server instances remain isolated. No mandatory process-global worker pool or process-global callback ceiling is introduced.
10. Pool lifecycle, scheduling and resource accounting are independent per class even if implementation code for worker loops or small internal primitives is shared.

### Rationale and performance consequences

Three independent pools provide the strongest protection against cross-class head-of-line blocking. A permanently blocked Application handler cannot consume the goroutines or scheduler path required for Completion or OnClose. Separate queues, locks and wakeups reduce cross-class lock contention and keep the hot dispatch path easier to reason about.

The cost is that idle workers cannot be borrowed by another class, and each owner has three pool lifecycle domains. Lazy creation is mandatory to avoid eagerly paying the sum of all configured ceilings. Idle-worker retirement is a separate WORKER-005 decision.

### Rejected alternative

Option B — one shared owner worker pool with strict per-class active reservations — is not selected for V1. Although it can reuse idle goroutines across classes, it introduces a shared dispatch/wakeup/lock path and additional cross-class scheduling/fairness complexity. That trade-off is not justified when arbitrary user callbacks may block indefinitely.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.  
**Next:** WORKER-005 — Worker Lifecycle and Idle Worker Retirement. Numeric worker ceilings, idle timeout defaults, queue/event limits and payload-byte limits remain open.


## 12. WORKER-005 — Worker Lifecycle and Idle Worker Retirement (OWNER APPROVED, OPTION B, 2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION B.**  
**Implementation:** NOT AUTHORIZED.

The owner approved finite idle retirement rather than retaining every worker until owner shutdown.

### Binding V1 contract

1. Each Client/Server retains the three class-isolated WORKER-004 pools. Workers are created lazily and reused only within their own class.
2. After a callback returns, if the worker has no dispatchable work, it may enter Idle and start a finite grace timeout belonging to that class pool.
3. Work arriving before timeout expiry reuses/wakes the idle worker. If the timeout expires and no dispatchable work exists, the worker atomically removes itself from the pool's worker count and exits.
4. The default warm-idle floor is **zero**. After sufficient inactivity, a class pool may return to zero idle worker goroutines.
5. Idle timeout is independently configurable for Application, Completion and OnClose. Numeric defaults and valid ranges are deferred to WORKER-009.
6. A worker executing user code is not idle and must never be retired, interrupted or have its active permit released because of idle timeout. A permanently blocked callback remains charged until actual return.
7. Retirement does not discard queued work or release Application queue/event charges, Completion Request credits, OnClose event credits or other work-item ownership.
8. Enqueue versus retirement has one linearization point. If enqueue wins, the idle worker is reused/woken; if retirement wins, the pool count is decremented before exit and dispatch creates a replacement when needed. No lost wakeup, stranded ready Session or worker-count overshoot is permitted.
9. The timeout begins only after callback return and after the worker establishes that no dispatchable work is available. It never bounds callback duration.
10. No goroutine per timeout and no process-global reaper. Reusable timer/wait state is preferred; repeated idle cycles should avoid unnecessary timer allocation.
11. Owner Stop/Close and active-worker drain/cancellation semantics remain API-002.

### Rationale

Finite idle retirement prevents long-lived owners from retaining their historical peak number of worker goroutines and stacks indefinitely. A zero warm-idle floor gives predictable quiescent resource use across many independent owners. A finite grace period still permits reuse during short gaps, trading possible cold-start cost after long inactivity for lower idle memory and goroutine retention.

**Detailed design:** this document §12.  
**Next:** WORKER-006 — fair scheduling between Sessions. Numeric idle timeout defaults/ranges remain open for WORKER-009.

## 13. WORKER-006 — Fair Scheduling Between Sessions (OWNER APPROVED, OPTION A, 2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “برای WORKER-006 گزینه A را تأیید می‌کنم.”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Each of the three independent callback-class pools (Application, Completion and OnClose) schedules eligible Sessions using its own FIFO round-robin ready-Session queue.
2. Each Session may have at most one ready node in a given class pool. The node is reusable and represents the Session's eligibility, not an individual callback.
3. A worker selects one callback from the head Session's per-Session FIFO queue per turn. If the Session still has queued work and a free per-Session concurrency slot, its ready node moves to the tail. It does not drain its entire backlog in one turn.
4. If the Session has no queued work, remove/clear its ready state. If its per-Session concurrency limit is full, it is temporarily ineligible; when a callback returns, the Session is re-evaluated and, if work remains, re-enters at the tail.
5. Preserve FIFO callback-start selection within each Session. For Completion, C-ORD-B remains binding: order is the internal engine's serialized terminal-result settlement/enqueue order, not SendRequest admission order. When Completion concurrency is greater than one, callback return order remains unspecified.
6. Fairness is scoped to continuously eligible Sessions within one owner and one callback class. It does not guarantee equal CPU time or preempt already-running callbacks.
7. Application, Completion and OnClose ready queues, locks and wake-up state remain physically independent under WORKER-004. No cross-class scheduling or capacity borrowing.
8. No scheduler lock is held while invoking user code. Dispatch and ready-node rotation use bounded critical sections. No per-callback goroutine or per-turn heap allocation is required; ready nodes are reusable per Session/class.
9. A Session's eligibility, callback start and Forced Close cancellation must retain the existing single linearization boundary. A callback cancelled before start is never dispatched; a callback whose start wins retains its active permits until return.
10. The round-robin policy does not change WORKER-001 owner scope, WORKER-002 active ceilings, WORKER-003 OnClose event saturation, WORKER-004 pool isolation or WORKER-005 idle retirement.

### Rationale and performance consequences

One callback per eligible Session turn prevents a continuously busy Session from draining all available dispatch opportunities ahead of other ready Sessions. A single reusable ready node per Session/class bounds scheduler metadata by the Session population rather than callback volume. O(1) queue rotation is possible with a per-class FIFO ready list and a short isolated lock domain.

The policy gives dispatch fairness, not CPU-time fairness: a slow callback remains non-preemptive and occupies its own class worker until return. Weighted/quantum scheduling would add configuration and could increase tail latency for low-volume Sessions without guaranteeing CPU fairness.

### Rejected alternatives

- **Option B — weighted round-robin / configurable per-Session quantum:** not selected. It adds public policy and scheduling state; callback-count weights cannot guarantee CPU-time fairness and large quanta worsen tail latency.
- **Option C — global FIFO by enqueue time:** not selected. A high-volume Session can flood the queue and delay sparse Sessions; enqueue-volume order is not Session fairness.

**Detailed design:** this document §13.  
**Next:** WORKER-007 — Queue Capacity and Event Capacity. Numeric limits and safe defaults remain open for WORKER-009.

---

## 14. WORKER-007 — Queue Capacity and Event Capacity (OWNER APPROVED, OPTION A, 2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “گزینه A”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Add a configurable incoming Application callback-frame count ceiling per Session, alongside the existing incoming byte ceiling. It bounds admitted inbound Stream Request events waiting to start an Application callback. A matching Stream Response destined for a Completion callback is already bounded by its PERF-003 Request credit and must not acquire a duplicate Completion event-count credit.
2. Add a configurable outgoing Application frame count ceiling per Session, alongside the existing outgoing byte ceiling. It covers every admitted Application frame, including the frame currently owned by the Writer, until the full frame is written or definitively discarded.
3. Add one configurable Server-aggregate queued Application frame count ceiling covering the combined incoming and outgoing queued Application frame populations. It is distinct from the already-approved aggregate queued-frame byte ceiling. It is a Server scope; Client reconnect generations continue to share Client-owned capacity according to WORKER-001 and the later payload accounting contract.
4. Incoming count admission is reserved before the body is allocated/read once the fixed header has established a valid Application Request frame. The count remains held while the frame is waiting to start its Application callback and is released at the callback-start linearization point. The in-progress Reader reservation must not create an unbounded or unaccounted frame.
5. If the incoming per-Session count or Server aggregate count cannot admit a valid Request frame, close only the originating Session under the already-approved Q-009/Q-010 overload policy. Do not drop, pause/resume, or defer without bound.
6. If the outgoing per-Session count or Server aggregate count is full, reject the new Send immediately with Queue Full, keep the Session connected, and create no pending/sequence state or transmitted frame for a rejected SendRequest.
7. Keep the existing byte limits independent: outgoing 4 MiB per Session (Q-008), incoming 4 MiB per Session (Q-010), and 256 MiB aggregate queued Application frame bytes per Server (Q-010). The aggregate byte budget covers incoming + outgoing queued Application frames. A frame is charged to exactly one queued owner bucket at a time.
8. Completion count remains the PERF-003 per-Session and Server-aggregate Request credit covering the complete Request lifecycle through callback return/cancellation. Do not add a second Completion event-count ceiling or reserve another credit on terminal settlement.
9. OnClose count remains the WORKER-003 owner-scoped event budget covering queued + running OnClose events. Do not add a second OnClose queue/event count.
10. Control-PDU headroom remains separate at the existing configurable 4 KiB per Session. Do not merge Control, Application or Completion populations into one cross-class event queue/budget.
11. All count limits are lazy admission ceilings, not eager preallocation. Numeric defaults, valid ranges, public option names and maxima remain deferred to WORKER-009/API-004.

### Rationale and performance consequences

Byte-only limits do not adequately bound queue-node and event metadata for tiny or empty frames. Per-Session counts isolate noisy peers; the Server aggregate count prevents the sum of many individually compliant Sessions from producing an unbounded queued-frame population. Reusing PERF-003 and WORKER-003 credits avoids duplicate Completion/OnClose accounting.

The additional hot-path cost is a bounded count reservation/release alongside byte admission. The Server aggregate count is a shared cache-line/counter and may contend under high fan-in; implementation should keep the reservation path short and avoid a long-held lock. Any sharding/token-cache optimization requires later measurements and must preserve a strict aggregate ceiling.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §14.  
**Next:** WORKER-008 — Resource Accounting and Payload Memory Ceiling.

---

## 15. WORKER-008 — Resource Accounting and Payload Memory Ceiling (OWNER APPROVED, OPTION A, 2026-11-17)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “A”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Maintain two independent retained-payload byte ledgers: **Application** and **Completion**. Each has a per-Session ceiling and an aggregate per-owner ceiling. Owner scope follows WORKER-001: each Client and each Server has its own budgets; all reconnect generations of a Client share the same owner budget.
2. **Application retained bytes** cover inbound Stream Request frame storage from validated-header reservation and before body allocation, through body assembly, waiting callback queue and active OnRequest callback until callback return or cancellation. They also cover outbound private serialized Application frame storage from accepted Send admission through full Writer completion or definitive discard.
3. **Completion retained bytes** cover a matched Stream Response frame from before body allocation through queued Completion delivery and active Completion callback until callback return or Forced Close A cancellation. PERF-003 Request credits remain a separate count/lifecycle ledger; the Completion byte ledger does not create another event credit.
4. A started callback retains its payload-byte charge until actual return, even after Session Done, socket teardown or Client reconnect. An unstarted callback cancelled by Forced Close A releases its library-owned payload and its existing queue/Request credits exactly once.
5. A payload moving from an incoming queue to an active OnRequest callback releases only the waiting-queue byte/count charges at the callback-start linearization point. Its retained Application byte charge follows the same owned buffer until callback return. Moving an outgoing frame from FIFO to the single Writer releases no byte/count charge; release occurs only after full Write or definitive discard.
6. The outgoing private frame is charged by its full serialized frame length, including the 10-byte Pinex header, consistently with Q-008. Incoming Stream Request and Response reservations use the declared full frame length conservatively even if the final user-visible allocation stores only the body.
7. Reserve the declared destination byte budget and applicable queue/count admission before allocating or reading the body. A partial body read holds its full declared reservation; read failure, invalidation or close releases it exactly once. An oversized declared frame is rejected before body allocation.
8. The existing Q-008/Q-009/Q-010 queue byte/count ceilings remain independent admission sub-limits. They are not added to retained-payload ledgers to estimate physical RSS: the same frame may be subject to multiple independent ceilings, but it is one physical buffer and must not be double-counted in any single memory total.
9. After the outgoing frame is fully written, the original request payload storage is released. Pending correlation retains only required Request metadata, not a duplicate outgoing payload.
10. Unknown/late Stream Responses with no live Pending Request do not resurrect a Request or create Completion work; they are handled by the bounded protocol discard/close policy, not retained as an unowned Completion payload.
11. Control PDU storage remains in its separate Q-010 per-Session 4 KiB reserve. OnClose has no payload ledger under the current API. No maximum queue or payload capacity is eagerly preallocated.
12. The library never recycles or pools an inbound backing array after it is exposed to user code. If the application retains that slice after callback return, that retention is outside the library-enforceable byte ceiling.
13. **Completion retained-byte admission failure — OWNER APPROVED OPTION A (2026-11-17):** if a syntactically valid, in-limit Response matches a live Pending Request but its declared full frame length cannot reserve either the Session or owner Completion retained-byte budget, do not allocate/read the Response Body into a payload buffer and do not close the Session solely for this capacity failure. Drain exactly the declared Body length through one small fixed-size scratch buffer whose memory is independent of the declared Body length. After a complete drain, if this Request still wins terminal settlement, enqueue its normal Completion callback with no Response payload and a typed terminal Resource Capacity error. Keep the Session open. The existing PERF-003 Request credit remains held until that Completion callback returns or is cancelled under Forced Close A; no second Completion/event credit is created. If timeout/close/another terminal event wins while draining, it remains the sole settlement; after a successful drain, do not deliver a duplicate Resource Capacity callback. If the drain fails because of EOF, timeout/read error or concurrent transport close, close the Session under the existing transport-error/Forced Close policy; do not also emit the Resource Capacity completion. The drain is limited by the validated frame maximum and existing Reader/deadline policy; exact timeout behavior remains API-002 work. Oversized or malformed frames continue to follow the protocol-error policy, not this capacity path.

### Rationale and performance consequences

Two class-separated ledgers provide a direct finite bound for payload memory retained by user callback work without introducing a counter for every lifecycle stage. Per-Session limits isolate a single noisy Session; aggregate Client/Server limits prevent reconnect generations or many Sessions from multiplying the owner's budget. Queue-to-active transfer is a release of queue-only admission, not a release/re-reservation of the underlying payload bytes.

The hot path adds a bounded reservation/release alongside queue/count admission. Session-local and owner counters must be updated with short race-safe operations; no owner lock may be held during user code or network I/O. Any future sharding must preserve a strict aggregate ceiling. These ledgers bound library-owned frame/payload storage, not process RSS, allocator fragmentation, Go metadata/stacks, kernel buffers or memory retained by the application.

### Explicitly unresolved; must close before Architecture Freeze

- The exact C++ Pinex 1.5.0 maximum is source-verified as 1 MiB total Frame including the header; cross-language confirmation remains pending (see PROTOCOL-001).
- Numeric retained-byte ceilings, count defaults, worker ceilings, valid ranges and public option identifiers remain WORKER-009/API-004 design work.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §15.  
**Next:** WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits.

---

## 16. Explicitly not decided

- WORKER-009A and WORKER-009B1/B2/B3 defaults are CLOSED / OWNER APPROVED; WORKER-009C proposed explicit hard maxima, valid ranges, cross-field constraints and semantic Functional-Option mapping remain under owner review.
- WORKER-008 Completion retained-byte exhaustion policy is resolved. Pinex 1.5.0 maximum Frame is source-verified as 1 MiB total; only golden-frame and Go/C++ cross-language confirmation remain open.
- Exact maximum Pinex frame size and Go/C++ cross-language confirmation.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.
- Final exported option identifiers and constructor signatures under API-001/API-004.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.

## 17. WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits

**Status:** WORKER-009A, WORKER-009B1, WORKER-009B2, WORKER-009B3 and WORKER-009C CLOSED / OWNER APPROVED; WORKER-009C is OPTION A.  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 establishes zero-configuration defaults and finite validation boundaries without weakening the bounded-resource contracts.

- **WORKER-009A — CLOSED:** derive the three owner active-worker defaults from GOMAXPROCS; keep other resource quotas fixed and finite.
- **WORKER-009B1 — CLOSED / OWNER APPROVED:** per-Session Application/Completion callback concurrency defaults and independent idle-retirement grace defaults for all three worker classes.
- **WORKER-009B2 — CLOSED / OWNER APPROVED:** OnClose event-credit, Application frame-count and outstanding Request-credit defaults; see §3.2 for the six approved V1 defaults.
- **WORKER-009B3 — CLOSED / OWNER APPROVED, OPTION A:** retained Application and Completion payload-byte defaults.
- **WORKER-009C — CLOSED / OWNER APPROVED / OPTION A:** explicit configuration hard maxima, valid ranges, cross-field constraints and semantic Functional-Option mapping.

The existing Q-008/Q-010 defaults remain binding and are not reopened. B1, B2 and B3 defaults are owner approved. WORKER-009C hard maxima and validation boundaries are now approved in §3.4.

### 2. WORKER-009A — CPU-derived worker defaults (OWNER APPROVED OPTION A, 2026-11-17)

The owner approved Option A.

At Client/Server effective-configuration construction, capture:

`P = max(1, runtime.GOMAXPROCS(0))`

Resolve the default owner-wide active-worker ceilings once:

| Worker class | Approved default formula | Default cap |
|---|---:|---:|
| Application | `min(max(4, 4 × P), 128)` | 128 |
| Completion | `min(max(2, 2 × P), 64)` | 64 |
| OnClose | `min(max(2, P), 32)` | 32 |

Binding interpretation:

1. These are per-Client or per-Server owner-wide active-worker ceilings, independently maintained for the three physically isolated callback classes.
2. The formula is resolved once when effective configuration is built and is immutable for that Client/Server lifetime. A later CPU quota or GOMAXPROCS change does not resize a live owner.
3. Worker goroutines remain lazy, reused only within their class, and subject to WORKER-005 finite idle retirement with zero warm-idle floor. The formula is a ceiling, not eager goroutine preallocation.
4. Per-Session callback concurrency is a separate limit and intersects the remaining owner/class ceiling. WORKER-009A does not decide the per-Session concurrency default.
5. The values 128/64/32 are caps on the CPU-derived **defaults**. They are not approved hard maxima for explicit user overrides; those belong to WORKER-009C.
6. All queue, event, Request-credit and retained-payload byte quotas remain fixed finite defaults independent of P. They remain separately configurable through API-004 Functional Options.
7. No live adaptation, hot reload, process-global worker pool or process-global worker ceiling is introduced.

Illustrative resolved values:

| P | Application | Completion | OnClose |
|---:|---:|---:|---:|
| 1 | 4 | 2 | 2 |
| 2 | 8 | 4 | 2 |
| 4 | 16 | 8 | 4 |
| 8 | 32 | 16 | 8 |
| 16 | 64 | 32 | 16 |
| 32 or more | 128 | 64 | 32 |

### 3. WORKER-009B — numeric defaults (B1/B2/B3 OWNER APPROVED)

The following matrix records owner-approved WORKER-009B1/B2/B3 defaults. The formulas in §2 and all values in §§3.1–3.3 are owner approved; hard maxima and validation ranges remain WORKER-009C.

#### 3.1 WORKER-009B1 — per-Session concurrency and idle retirement (OWNER APPROVED, 2026-11-17)

| Setting | Approved default | Scope | Reason |
|---|---:|---|---|
| Application callback concurrency | 1 | Per Session | Preserves serial behavior by default; users opt into overlap explicitly |
| Completion callback concurrency | 1 | Per Session | Preserves FIFO callback-start order without overlap by default |
| Application worker idle grace | 30s | Application pool per owner | Reuses workers across short traffic gaps |
| Completion worker idle grace | 30s | Completion pool per owner | Avoids churn during response bursts |
| OnClose worker idle grace | 30s | OnClose pool per owner | Gives close bursts a reuse window while retaining finite retirement |

**Owner approval (2026-11-17):** the owner approved all five values in this table. Concurrency defaults of 1 preserve serial callback execution within each Session; higher positive values remain configurable and are still intersected with the owner-wide class ceiling. Completion callback start selection remains FIFO; when Completion concurrency is raised above 1, callback return order is not guaranteed under C-ORD-B. Each class's 30s grace is independently configurable and starts only after a callback returns and the worker has no dispatchable work. It never interrupts a running callback. WORKER-005's zero warm-idle floor, class isolation and enqueue-versus-retirement linearization remain binding. Explicit zero is invalid for these positive grace settings.


The already-approved zero warm-idle floor remains in force. The grace period starts only after a worker returns from user code and has no dispatchable work; it never interrupts a running callback.

#### 3.2 WORKER-009B2 — event, frame-count and Request-credit defaults

| Setting | Approved default | Scope | Accounting and saturation |
|---|---:|---|---|
| OnClose event credits | 1,024 | Per Client/Server owner | Queued + running; on saturation permanently skip the new OnClose per WORKER-003 |
| Incoming Application frame count | 256 | Per Session | Reader reservation + waiting Request callback frames; if admission fails, close only that Session |
| Outgoing Application frame count | 256 | Per Session | Includes current Writer-owned frame until full Write/discard; fail fast with Queue Full |
| Aggregate queued Application frame count | 65,536 | Per Server | Combined incoming + outgoing queued frames; incoming overflow closes offender, outgoing overflow returns Queue Full |
| Outstanding Request credits | 128 | Per Session | Full async Request lifecycle through Completion callback return/cancellation |
| Aggregate outstanding Request credits | 32,768 | Per Client/Server owner | Same Request credits; no duplicate Completion event credit |

Rationale: count limits protect tiny/empty-frame metadata that byte budgets alone do not bound. The 1,024 OnClose event budget matches the approved 1,024 bound-Session default for a single close wave, while remaining finite across churn and permanently blocked callbacks. The 65,536 Server frame ceiling limits the aggregate population independently of the per-Session 256 ceilings. PERF-003 Request credits remain the sole Completion lifecycle/count mechanism; no second Completion event credit is created.

**Owner approval (2026-11-17):** the owner approved all six defaults in §3.2 (Option A). OnClose credits count queued + running events per Client/Server owner and saturation permanently skips the new OnClose event under WORKER-003. Incoming count reserves after a valid fixed header and before body allocation/read; it covers the in-progress Reader reservation plus waiting Request callback frames and is released at callback start. If either the Session or Server incoming count cannot admit the frame, close only that Session. Outgoing count includes the Writer-owned frame until full Write or definitive discard; local or Server aggregate exhaustion returns Queue Full without closing the Session or creating Pending/Sequence state. The Server aggregate frame count covers incoming + outgoing queued Application frames, not a second physical population. Outstanding Request credits are reserved before accepting callback-based SendRequest and remain through response wait, terminal settlement, queued Completion and active Completion callback until callback return or permitted cancellation. The per-Session limit is 128; the aggregate limit is 32,768 per Client/Server owner, shared by all Server Sessions and all Client reconnect generations. A saturated Request-credit admission rejects the new request before enqueue/transmission with a typed capacity error; it does not close the Session. SendNoWait and SendResponse do not consume Request credits. All six limits are independently configurable, lazy admission ceilings (not preallocation), and explicit zero is invalid. The aggregate frame-count and Request-credit defaults are intentionally lower than the sum of their theoretical per-Session ceilings; no cross-field requirement forces aggregate capacity to equal or exceed the product of Session count and local capacity. These defaults do not set WORKER-009C hard maxima.

#### 3.3 WORKER-009B3 — retained-payload byte defaults (OWNER APPROVED OPTION A, 2026-11-17)

| Setting | Approved default | Scope | Accounting |
|---|---:|---|---|
| Retained Application bytes | 16 MiB | Per Session | Inbound Request payload from header reservation through callback return/cancellation, plus outbound private frame through full Write/discard |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner | All library-owned Application payloads across that owner's Sessions and reconnect generations |
| Retained Completion bytes | 16 MiB | Per Session | Matched Response payload from header reservation through Completion callback return/cancellation |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner | All library-owned Completion payloads for that owner |

Rationale:

- The 16 MiB Application Session ceiling provides headroom beyond the independent 4 MiB incoming and 4 MiB outgoing queue-byte limits for active callbacks and in-progress frames. Those queue limits remain independent admission sub-limits; they are not added to the retained-byte ledger as separate physical buffers.
- The 512 MiB Application owner ceiling is twice the existing 256 MiB Server queued-Application-byte default, leaving bounded headroom for active/in-progress Application payloads. It is not an RSS limit.
- The earlier 4 MiB Completion-per-Session draft is superseded by the approved 16 MiB default. Against the source-verified 1 MiB total-frame maximum, 4 MiB would permit only four maximum-size Response payloads for one Session. 16 MiB is a less restrictive bounded burst allowance; the 256 MiB owner ceiling still caps aggregate Completion payload retention.
- These values are admission ceilings, not preallocations. User-retained inbound payload after callback return remains outside the library-owned retained-byte ledger.

The pinned Pinex C++ 1.5.0 source confirms a 1 MiB total-frame maximum including the 10-byte header (maximum Body 1,048,566 bytes). Cross-language confirmation remains pending; WORKER-009B does not set this protocol limit.

**Separate WORKER-008 owner decision (2026-11-17):** Completion retained-byte admission exhaustion is CLOSED / OWNER APPROVED, OPTION A. For a valid matching Response whose full declared frame cannot reserve the Session or owner Completion byte budget, the Reader drains exactly the Body through a small fixed-size scratch buffer (no body-sized allocation). If drain succeeds and the Request has not already been terminally settled, deliver one Completion callback with no payload and a typed terminal Resource Capacity error; keep the Session open and retain its existing Request credit until callback return/cancellation. If another terminal event wins during drain, do not deliver a duplicate. If drain fails, close the Session under transport-error/Forced Close policy. Malformed or over-maximum frames do not use this path.

**Owner approval (2026-11-17):** the owner approved Option A and all four values above as official V1 defaults. Each is an independently configurable positive finite admission ceiling for library-owned retained payload. The Client's aggregate Application and Completion ledgers are shared across reconnect generations; Server ledgers cover all Sessions owned by that Server. Queue byte/count limits remain independent admission sub-limits and are not added to retained-payload bytes as if they were separate physical buffers. Queue-to-active transitions release queue-only charges while the retained-byte charge follows the same buffer. No capacity is eagerly allocated. These defaults are not WORKER-009C hard maxima and do not establish an RSS limit.

### 3.4 WORKER-009C — hard maxima and validation boundary (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

The owner approved Option A. The values below are binding V1 configuration-validation ceilings, not recommended operating points, preallocations, or process-wide limits. They apply to the corresponding Client/Server owner or Session ledger.

| Setting | V1 default | Approved valid range / hard maximum |
|---|---:|---:|
| Application active workers per owner | CPU-derived, cap 128 | 1..512 |
| Completion active workers per owner | CPU-derived, cap 64 | 1..256 |
| OnClose active workers per owner | CPU-derived, cap 32 | 1..128 |
| Application callback concurrency per Session | 1 | 1..32 |
| Completion callback concurrency per Session | 1 | 1..32 |
| Each worker-class idle grace | 30s | 1ms..10m |
| OnClose event credits per owner | 1,024 | 1..4,096 |
| Incoming Application frames per Session | 256 | 1..1,024 |
| Outgoing Application frames per Session | 256 | 1..1,024 |
| Aggregate queued Application frames per Server | 65,536 | 1..262,144 |
| Outstanding Request credits per Session | 128 | 1..512 |
| Aggregate outstanding Request credits per owner | 32,768 | 1..131,072 |
| Retained Application bytes per Session | 16 MiB | 10 bytes..64 MiB |
| Aggregate retained Application bytes per owner | 512 MiB | 10 bytes..2 GiB |
| Retained Completion bytes per Session | 16 MiB | 10 bytes..64 MiB |
| Aggregate retained Completion bytes per owner | 256 MiB | 10 bytes..1 GiB |

**Approved validation and cross-field rules:**

1. Apply all Functional Options first, then validate the completed effective configuration as a unit.
2. Validation must finish before creating/opening any Socket, starting any Goroutine, or creating other active network/runtime resources.
3. All count and byte limits require a positive value (minimum 1), except retained-payload byte limits whose minimum is 10 bytes. Worker idle grace is valid from 1ms through 10m. The only intentional zero-valued exception across the configuration remains `EnquireLinkIdleInterval=0`, which disables automatic outgoing Enquire Link probes only.
4. Owner and per-Session limits are independent. An aggregate limit may be lower than a per-Session limit or lower than the theoretical sum of all Session capacities; admission must satisfy each applicable ledger independently. Do not reject a configuration merely because aggregate capacity is below that theoretical sum.
5. Per-Session Application/Completion concurrency is intersected with the corresponding owner/class active-worker ceiling at dispatch. Do not reject a larger Session concurrency value solely because the owner ceiling is lower.
6. Queue bytes/counts, retained-payload bytes, Request credits, OnClose event credits, and worker permits are independent accounting ledgers. Do not double-count one payload during queue-to-active transitions or sum overlapping ledgers as separate physical memory.
7. Protect all byte conversions, capacity calculations, and counter increments/decrements against integer overflow/underflow. Failed reservations must not partially mutate accounting.
8. All owner ceilings are scoped to one Client or Server instance, not process-wide. Client reconnect generations share that Client's owner ledgers; separate Client/Server instances retain independent budgets.
9. The 10-byte retained-byte minimum covers only a header-sized frame. It does not guarantee admission of larger valid frames; existing directional overload and bounded Completion-drain policies remain binding.
10. WORKER-009C does not set Maximum Frame Size. PROTOCOL-001 source-verifies 1 MiB total Frame including Header against pinned Pinex C++ 1.5.0; golden-frame and cross-language confirmation remain pending.

**Approved Functional-Option mapping:** provide one typed, independent Functional Option per semantic setting. Owner worker ceilings, OnClose credits, owner Request credits, and owner retained-byte ceilings are scoped to their owning Client/Server; aggregate queued Application frame count is Server-only; per-Session callback concurrency and per-Session frame/Request/retained-byte limits attach to Session configuration/inheritance; worker idle grace is independently configurable per worker class. Omitted options preserve the approved defaults; each explicit option changes only its own setting. Do not introduce a generic map, mutable live config, or hot reload. Exact exported identifier spelling and final constructor/Session option placement remain API-001 public-surface work.

**Decision:** `CLOSED / OWNER APPROVED / OPTION A` (2026-11-17). Implementation remains NOT AUTHORIZED.

### 4. Defaults already approved and unchanged

| Setting | Approved default | Scope |
|---|---:|---|
| Outgoing Application queue bytes | 4 MiB | Per Session |
| Incoming Application waiting-queue bytes | 4 MiB | Per Session |
| Aggregate queued Application frame bytes | 256 MiB | Per Server, incoming + outgoing |
| Bound Sessions | 1,024 | Per Server |
| Pending pre-Bind connections | 128 | Per listener |
| Absolute pre-Bind handshake deadline | 30s | From TCP acceptance |
| Control-PDU allowance | 4 KiB | Per Session |
| Enquire Link idle interval | 30s | Explicit zero disables outgoing probes only |
| Enquire Link response timeout | 10s | Per Session |

### 5. Zero, ownership and admission rules

- Omitted Functional Options select the approved/recommended positive finite default.
- Zero is never interpreted as Unlimited for worker, queue, event, Request-credit or retained-byte limits.
- Explicit zero/negative values for required positive limits and worker idle grace are rejected. The already-approved EnquireLinkIdleInterval=0 remains the intentional disable switch.
- Per-Session concurrency and owner active-worker ceilings are independent; actual dispatch is bounded by both.
- Every count/byte limit is lazy admission capacity, not eager allocation.
- Queue bytes/counts, active-worker permits, Request credits, OnClose event credits and retained-payload bytes remain distinct ledgers. Do not sum them as if they were separate physical buffers.
- Client reconnect generations share the same Client owner budgets. Independent Client/Server instances remain isolated; no process-global cap is introduced.
- Completion-byte exhaustion must never silently discard a matching Response. The owner-approved policy is bounded Body drain with a fixed scratch buffer, followed by one terminal Resource Capacity Completion callback without Response payload; the Session stays open. A drain/transport failure closes the Session under the transport-error policy. See WORKER-008 above.

### 6. Owner decision sequence

**WORKER-009C is CLOSED / OWNER APPROVED / OPTION A.** Worker Capacity and Resource Accounting design is complete. Next: API-002 lifecycle/timeout design, API-003 package architecture and exact Pinex C++ 1.5.0 maximum-frame compatibility. Exact exported identifiers remain API-001 public-surface work.

**Guardrail:** design/documentation only. No Go/C++ source, tests, benchmarks, builds, dependency changes, CI, deployment or operational changes are authorized.

---

