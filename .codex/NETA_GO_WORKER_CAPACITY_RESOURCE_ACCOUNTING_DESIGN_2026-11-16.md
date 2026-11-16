# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001/002/003/004/005/006/007/008 OWNER APPROVED; WORKER-009 OPEN  
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

### Rationale and performance consequences

Two class-separated ledgers provide a direct finite bound for payload memory retained by user callback work without introducing a counter for every lifecycle stage. Per-Session limits isolate a single noisy Session; aggregate Client/Server limits prevent reconnect generations or many Sessions from multiplying the owner's budget. Queue-to-active transfer is a release of queue-only admission, not a release/re-reservation of the underlying payload bytes.

The hot path adds a bounded reservation/release alongside queue/count admission. Session-local and owner counters must be updated with short race-safe operations; no owner lock may be held during user code or network I/O. Any future sharding must preserve a strict aggregate ceiling. These ledgers bound library-owned frame/payload storage, not process RSS, allocator fragmentation, Go metadata/stacks, kernel buffers or memory retained by the application.

### Explicitly unresolved; must close before Architecture Freeze

- If a valid matching Stream Response cannot reserve Completion retained bytes, the engine must not silently drop it, corrupt the TCP stream or create an unbounded overflow path. The exact policy (e.g. bounded discard plus one terminal resource error versus closing the offending Session and settling its pending requests) remains a separate owner decision.
- The compatibility reference currently records a 1 MiB maximum frame, but the exact C++ Pinex 1.5.0 maximum and Go/C++ cross-language confirmation are not yet frozen.
- Numeric retained-byte ceilings, count defaults, worker ceilings, valid ranges and public option identifiers remain WORKER-009/API-004 design work.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §15.  
**Next:** WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits.

---

## 16. Explicitly not decided

- WORKER-009A default derivation model; WORKER-009B numeric defaults; WORKER-009C hard validation ranges and cross-field constraints.
- WORKER-008 Completion retained-byte exhaustion disposition; no silent Response drop is allowed.
- Exact maximum Pinex frame size and Go/C++ cross-language confirmation.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.
- Final exported option identifiers and constructor signatures under API-001/API-004.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.

## 17. WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no numeric defaults or defaulting strategy approved yet.**  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 is the umbrella for safe zero-configuration defaults and finite validation boundaries. To avoid combining unrelated choices into one oversized approval, split it into:

- **WORKER-009A:** default derivation model for worker ceilings versus fixed resource quotas. This is the current owner decision.
- **WORKER-009B:** approve the numeric default matrix for worker/concurrency/idle settings, event and Request credits, Application queue counts and retained-payload bytes.
- **WORKER-009C:** approve hard validation ranges, cross-field constraints and the configuration-to-public-Functional-Options mapping.

The existing Q-008/Q-010 approved defaults remain unchanged. The proposed values below are engineering candidates only, not approved limits or performance SLAs.

### 2. WORKER-009A — default derivation model

**Option A — CPU-derived worker ceilings, fixed finite resource quotas (RECOMMENDED)**

At Client/Server effective-config construction, capture P = max(1, runtime.GOMAXPROCS(0)) once. Resolve only the three owner active-worker defaults from P, with finite caps:

- Application workers: min(max(4, 4 × P), 128); explicit configuration hard maximum proposed as 256.
- Completion workers: min(max(2, 2 × P), 64); explicit configuration hard maximum proposed as 128.
- OnClose workers: min(max(2, P), 32); explicit configuration hard maximum proposed as 64.

All other capacities remain fixed finite defaults, independently configurable and independent of P. The resolved settings are immutable for the lifetime of the Client/Server instance; no live adaptation, automatic resizing or hot reload is implied. Worker pools remain lazy, class-isolated and have zero warm-idle floor.

**Advantages:** scales callback execution with the available CPU budget without requiring user configuration; avoids one fixed worker count being excessive on small hosts or too restrictive on larger hosts; retains deterministic, independently bounded byte/count quotas.

**Costs:** the same program configuration resolves to different worker ceilings on hosts with different GOMAXPROCS values. The resolved values must be inspectable/documented, and deployment reproducibility requires recording the effective configuration.

**Option B — fixed worker defaults independent of CPU**

Use one fixed positive Application/Completion/OnClose worker default on every host; users explicitly tune for larger or smaller systems.

**Advantages:** identical defaults across machines and simpler capacity prediction.

**Costs:** a fixed conservative value can underuse large hosts; a high fixed value can retain too many goroutines/stacks on small hosts. It does not adapt to the execution budget already exposed by Go.

**Option C — require explicit worker settings**

Do not provide worker defaults; require every Client/Server to configure all class ceilings.

**Not recommended:** violates Zero-Configuration by Default and makes a basic Client/Server unnecessarily difficult to construct.

### 3. Candidate numeric matrix for WORKER-009B (NOT APPROVED)

| Setting | Candidate default | Scope | Notes |
|---|---:|---|---|
| Application callback concurrency | 1 | Per Session | Preserves serial default; positive configurable values permit overlap |
| Completion callback concurrency | 1 | Per Session | C-ORD-B; higher values opt into overlap |
| Application active workers | min(max(4, 4P), 128) | Per Client/Server | Owner-wide; class-isolated |
| Completion active workers | min(max(2, 2P), 64) | Per Client/Server | Owner-wide; class-isolated |
| OnClose active workers | min(max(2, P), 32) | Per Client/Server | Owner-wide; one OnClose event maximum per Session |
| Idle timeout per class | 30s | Per class, per owner | Finite grace; zero warm-idle floor |
| OnClose event credits | 1024 | Per Client/Server | Queued + running; skip permanently on saturation |
| Incoming Application frame count | 256 | Per Session | Includes Reader reservation and waiting Request callback frame |
| Outgoing Application frame count | 256 | Per Session | Includes current Writer frame until full Write |
| Aggregate queued Application frame count | 65,536 | Per Server | Incoming + outgoing; WORKER-007 scope |
| Outstanding Request credits | 128 | Per Session | Entire Request lifecycle through Completion return/cancellation |
| Aggregate outstanding Request credits | 32,768 | Per Client/Server owner | No duplicate Completion event credit |
| Retained Application bytes | 16 MiB | Per Session | Incoming Request + outgoing private frame storage |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner | Includes in-progress, queued and active library-owned Application frames |
| Retained Completion bytes | 4 MiB | Per Session | Matched Response through Completion callback return/cancellation |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner | Independent from Application retained bytes |

For context, the already-approved defaults remain: outgoing queue 4 MiB/Session; incoming queue 4 MiB/Session; aggregate queued Application frame bytes 256 MiB/Server; 1024 bound Sessions/Server; 128 pre-bind connections/listener; 30s absolute pre-bind deadline; 4 KiB Control reserve/Session; Enquire Link idle 30s (explicit zero disables outgoing probes); matching Enquire Link response timeout 10s.

The 1 MiB maximum frame in the Pinex wire reference is provisional pending exact C++ HEAD and cross-language confirmation. It is not made a frozen WORKER-009 default by this proposal.

### 4. Zero and validation principles

- Omitted Functional Options select the documented positive finite default.
- Zero is never interpreted as Unlimited for worker, queue, event, Request-credit or retained-byte limits.
- Explicit zero/negative values for required positive limits and worker idle timeouts are rejected. The already-approved EnquireLinkIdleInterval=0 remains the one intentional zero-valued disable switch.
- Per-Session concurrency and owner active-worker ceilings are independent; effective dispatch is bounded by both.
- Every count/byte limit is lazy admission capacity, not eager allocation.
- Queue bytes/counts, active-worker permits, Request credits, OnClose event credits and retained-payload bytes remain distinct ledgers. They are not summed as if they represented separate physical buffers.
- CPU-derived worker defaults are captured once during effective-config construction; all other defaults are fixed and independently overrideable.
- The exact hard maxima and cross-field validation matrix are WORKER-009C, not approved by WORKER-009A.

### 5. Owner decision requested — WORKER-009A

Choose the default derivation model:

- **Option A:** CPU-derived, capped worker ceilings; fixed finite queue/count/byte quotas. Recommended.
- **Option B:** fixed worker ceilings independent of CPU.
- **Option C:** require explicit worker settings, with no worker defaults.

No numeric value in the candidate matrix is owner-approved by this proposal. If A is approved, the next decision is the numeric matrix in WORKER-009B; valid ranges and hard maxima follow in WORKER-009C.

---

