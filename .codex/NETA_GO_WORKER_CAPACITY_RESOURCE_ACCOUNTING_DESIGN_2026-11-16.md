# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001/002/003/004/005 OWNER APPROVED; WORKER-006 OPEN  
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
- A separate incoming event-count limit for zero/small frames remains a proposal, not an approved extension to Q-009/Q-010.

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
- WORKER-004 has now selected three independent physical class pools. WORKER-006 will define the fair ready-Session scheduling policy within each pool; N-ORD2's Completion/OnClose isolation remains binding.

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
- Bounded class-isolated worker loops avoid per-event goroutine creation and reduce allocation/scheduler churn. WORKER-004 fixes the pool topology; WORKER-006 will refine fair ready-Session selection.
- Independent class budgets prevent blocked Application callbacks from consuming Completion/OnClose capacity and prevent Completion saturation from consuming OnClose's reserved capacity.
- WORKER-006 proposes per-class round-robin over eligible Sessions. Fair dispatch improves tail latency and prevents a high-volume Session from monopolizing future worker starts, but cannot preempt running callbacks or overcome a class whose workers are all permanently blocked.
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

## 13. WORKER-006 — Fair Scheduling Between Sessions (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no owner selection yet.**  
**Implementation:** NOT AUTHORIZED.

WORKER-004 fixes three class-isolated pools. WORKER-006 defines how each pool chooses among Sessions with dispatchable work; it does not merge pools or alter WORKER-001/002 ownership and capacity contracts.

### Binding constraints inherited from prior decisions

- Fairness is scoped to eligible Sessions within one owner and one callback class. Client reconnect generations share the Client owner scheduler; queued work from a retired generation remains visible until its own lifecycle ends.
- Preserve FIFO work order within each Session. For Completion, C-ORD-B remains binding: FIFO callback-start selection follows serialized terminal-result settlement/enqueue order, not SendRequest admission order. Parallel callback return order remains unspecified.
- A Session is eligible while it has queued work and an available per-Session concurrency slot. Owner/class active-worker ceilings remain binding.
- Application, Completion and OnClose have independent ready-session state; scheduling turns cannot cross callback classes.
- Callbacks are non-preemptive. Fairness governs dispatch opportunities/start selection, not CPU-time equality or callback completion deadlines.
- No scheduler lock is held while user code runs. No goroutine or heap allocation is required per callback turn.

### Option A — per-class round-robin, one callback per Session turn (RECOMMENDED)

Each class pool maintains a FIFO rotation of ready Sessions. A Session appears at most once in that class's ready set. A worker dispatches one callback from the head Session's FIFO queue, then moves that Session to the tail if it still has queued work and an available per-Session slot. If the Session reaches its concurrency limit, it becomes eligible again only when a callback returns and frees a slot.

**Advantages:** explicit and predictable fairness; a continuously busy Session cannot drain its entire queue before other ready Sessions receive a turn; O(1) ready-queue rotation is possible; one reusable ready node per Session/class avoids per-callback scheduling allocations; per-Session FIFO start order is preserved.

**Costs:** more bookkeeping than a single global FIFO; a shared ready-queue lock may contend at very high worker counts if held too long. Keep its critical section short and isolated per class. Do not add lock-free or sharded scheduling without measured evidence.

**Fairness boundary:** each continuously eligible Session receives one dispatch opportunity per rotation. This does not guarantee equal CPU time when callback durations differ, and cannot preempt callbacks already running.

### Option B — weighted round-robin / configurable per-Session quantum

Each Session receives a configured weight or a multi-callback quantum per turn.

**Advantages:** can improve throughput for intentionally prioritized Sessions and reduce scheduling overhead for homogeneous high-volume workloads.

**Costs:** adds weight/quantum state and likely public configuration; equal service is no longer the default; large quanta increase tail latency for low-volume Sessions and can approximate starvation. Callback-count weights cannot guarantee CPU-time fairness because callback durations are unknown and non-preemptive.

### Option C — global FIFO by enqueue time

All work items from all Sessions enter one FIFO per class and workers take the oldest item.

**Advantages:** simple enqueue ordering and comparatively little ready-session bookkeeping.

**Costs:** no Session fairness. A busy Session can fill the queue with a burst and push sparse Sessions far back; fairness depends on arrival volume rather than Session opportunity.

### Recommendation

Choose **Option A** for V1: per-class round-robin over eligible Sessions, one callback dispatch per Session turn, with FIFO preserved inside each Session. It provides an understandable noisy-neighbour boundary with bounded scheduling metadata and low per-turn work. Keep scheduler critical sections short and per-class; defer lock-free/sharded implementation choices until measurements justify them.

**Owner decision requested:** choose WORKER-006 Option A (one callback per eligible Session turn), Option B (weighted/quantum scheduling), or Option C (global FIFO by enqueue time). No weights, quantum sizes or numeric capacity defaults are implied.

## 14. Explicitly not decided

- Numeric per-Session concurrency defaults and maximums for Application/Completion.
- Numeric owner active-worker ceilings.
- WORKER-005 numeric idle-timeout defaults and valid ranges.
- OnClose event-budget numeric default and public option name.
- Incoming callback event-count cap.
- Completion payload-byte budget and final maximum frame size.
- PERF-003 numeric Request-credit defaults.
- WORKER-006 fair scheduling between Sessions (pending owner selection).
- WORKER-007 queue and event capacities.
- WORKER-008 resource accounting and payload-memory ceilings.
- WORKER-009 safe defaults, valid ranges and public configuration limits.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.
