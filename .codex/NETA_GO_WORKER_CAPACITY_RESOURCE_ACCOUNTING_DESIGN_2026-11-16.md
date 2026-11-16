# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001/002/003/004/005/006 OWNER APPROVED; WORKER-007 OPEN  
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
- Separate incoming/outgoing Application frame-count limits are proposed under WORKER-007; they are not yet owner approved.

### 4.2 Completion

- PERF-003 Request credits are reserved before SendRequest acceptance and cover the complete admitted Request lifecycle: response wait, terminal settlement, queued Completion and running Completion.
- Response/timeout/close settlement reuses the already-reserved credit; it must not allocate a second terminal-event credit.
- A physically executing Completion also consumes an active Completion-worker permit. The permit is distinct from the Request credit.
- A queued Completion cancelled by Forced Close A releases its Request credit and library-owned queued payload/metadata. A started Completion retains both Request credit and active-worker permit until it returns.
- Completion payload bytes retained by the library while queued or executing need an explicit bound, unless the final protocol frame-size contract plus count limits prove a sufficiently small hard upper bound. User retention of the owned payload after callback return remains outside library accounting.

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
- WORKER-004 has now selected three independent physical class pools. WORKER-006 has approved per-class round-robin ready-Session scheduling within each pool; N-ORD2's Completion/OnClose isolation remains binding.

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

## 14. WORKER-007 — Queue Capacity and Event Capacity (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no owner selection yet.**  
**Implementation:** NOT AUTHORIZED.

### Scope and existing binding limits

WORKER-007 defines the count dimension and ownership of waiting work/event populations. It does not reopen existing byte caps, active-worker ceilings, PERF-003 Request-credit semantics or WORKER-003 OnClose saturation.

- **Application incoming callback backlog:** Q-009/Q-010 already approve 4 MiB per Session and 256 MiB aggregate queued Application-frame bytes per Server. The incoming overflow action is close only the offending Session.
- **Application outgoing FIFO:** Q-008/Q-010 already approve 4 MiB per Session and the 256 MiB aggregate Server queued-frame byte budget. Byte-cap exhaustion returns Queue Full and keeps the Session connected.
- **Control PDU headroom:** Q-010's separate 4 KiB per-Session byte budget remains isolated from Application traffic; Pinex ordering and the Unbind deadline remain binding.
- **Completion:** PERF-003 already approves count credits per Session and Server aggregate for the complete callback-based Request lifecycle: waiting for response, terminal result queued and Completion callback executing until return/cancellation. A queued Completion is a subset of that lifecycle and must not acquire a second event credit.
- **OnClose:** WORKER-003 already approves a separate owner-scoped event budget counting queued + running OnClose events; at most one event per closed Session; if full, skip permanently.
- **Active execution:** WORKER-002 active-worker permits remain separate from waiting queue/event populations. The internal engine remains outside user callback pools.
- **Scheduler metadata:** WORKER-006's ready queue contains at most one reusable node per eligible Session/class, so its size is bounded by the number of Sessions with eligible work, not callback count; OnClose eligibility is additionally bounded by WORKER-003 event credits.

### The unresolved count-accounting gap

Byte-only Application queues can retain a large number of small/empty frames and their queue metadata. PERF-003 explicitly recommends an incoming callback event-count cap, but that cap and its aggregate scope were not approved. The outgoing Application FIFO also has a byte cap but no explicit frame-count cap. The count dimension should protect queue nodes and event metadata independently of payload bytes.

### Option A — two-dimensional Application queue admission, with existing Completion/OnClose credits reused (RECOMMENDED)

Add independent count ceilings alongside the existing Application byte ceilings:

1. **Incoming Application callback backlog:** count queued frames per Session and across the Server aggregate. Count only frames waiting to start an Application callback; do not count an active callback as a queued event. On dispatch, transfer ownership to the active callback under one atomic accounting transition. If either incoming count ceiling cannot admit the next valid frame, close only the originating Session under the existing Q-009/Q-010 overload outcome.
2. **Outgoing Application FIFO:** count admitted frames per Session and across the Server aggregate, including the frame currently being written until the full frame is written. If either outgoing count ceiling is full, fail the new Send promptly with Queue Full; do not close an otherwise healthy Session and do not publish pending state for a rejected SendRequest.
3. **Server aggregate count:** one aggregate queued-Application-frame count covers incoming + outgoing queued frames, matching the existing combined MaxAggregateQueuedFrameBytes scope. A frame is charged to exactly one queued owner bucket at a time. The incoming queue-to-active transition must not create a charge gap; its active payload accounting is finalized under WORKER-008.
4. **Completion:** reuse PERF-003 Request credits. They already bound queued + running Completion work and the preceding pending-response lifecycle; do not add a second Completion event-count cap or reserve a new credit on terminal settlement.
5. **OnClose:** reuse WORKER-003's queued+running event credit. No second OnClose queue cap. A full budget permanently skips the new OnClose event.
6. Keep the Control PDU byte headroom separate. Do not merge Control or callback events into the Application frame-count budget or a shared cross-class event pool.

**Advantages:** protects against tiny-frame/metadata floods; keeps inbound and outbound overload reactions consistent with already approved Q-009/Q-010 behavior; provides an aggregate Server bound across many Sessions; avoids duplicate Completion/OnClose credits and cross-class interference.

**Costs:** adds count reservations to Application ingress/egress admission and an aggregate Server counter. These counters must be atomic/race-safe and must not become a long-held global lock. Exact numeric values and public option names remain for WORKER-009/API-004.

### Option B — per-Session Application frame-count caps only

Add incoming and outgoing frame-count ceilings per Session, but no Server aggregate frame-count ceiling. Keep the existing 256 MiB aggregate byte budget and existing Completion/OnClose credits.

**Advantages:** simpler aggregate hot path and per-Session isolation; one noisy Session cannot exceed its own event count.

**Costs:** up to MaxBoundSessions Sessions can each consume their full count allowance. Aggregate queue metadata can therefore grow to the per-Session cap multiplied by the Session limit, even while aggregate payload bytes remain within 256 MiB. This is a weaker owner-wide metadata bound.

### Option C — retain byte-only Application queue admission

Do not add Application incoming/outgoing frame-count ceilings. Keep Q-008/Q-009/Q-010 byte limits, PERF-003 Request credits and WORKER-003 OnClose event credits as they are.

**Advantages:** no new count reservation or configuration dimension for Application frame queues.

**Costs:** byte limits alone are not a useful practical bound on queue-node/event metadata for very small frames; a burst of tiny frames can create a large number of queued objects and scheduler operations. This leaves the incoming count-cap gap already identified by PERF-003 unresolved.

### Recommendation

Choose **Option A**. Add per-Session incoming/outgoing frame-count limits plus one aggregate Server queued-Application-frame count covering both directions. Keep Completion Request credits and OnClose event credits as their existing independent ledgers; do not double-count them or introduce a shared budget across callback classes. Count only waiting Application events in the incoming queue; active callback execution is already bounded by WORKER-002 and active payload memory is WORKER-008.

Numeric defaults, practical maxima and public configuration names are not part of WORKER-007; defer them to WORKER-009 and API-004. Do not preallocate queue capacity.

**Owner decision requested:** choose WORKER-007 Option A (per-Session incoming/outgoing frame counts plus aggregate Server frame count), Option B (per-Session counts only), or Option C (keep Application queues byte-only). No numeric count defaults are implied.

---

## 15. Explicitly not decided

- Numeric per-Session concurrency defaults and maximums for Application/Completion.
- Numeric owner active-worker ceilings.
- WORKER-005 numeric idle-timeout defaults and valid ranges.
- OnClose event-budget numeric default and public option name.
- Application incoming/outgoing queued-frame count caps and aggregate Server frame-count cap (WORKER-007 pending).
- Completion payload-byte budget and final maximum frame size.
- PERF-003 numeric Request-credit defaults.
- WORKER-007 numeric count-cap defaults and public configuration names (pending owner selection on policy).
- WORKER-008 resource accounting and payload-memory ceilings.
- WORKER-009 safe defaults, valid ranges and public configuration limits.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.
