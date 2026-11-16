# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001/002/003 OWNER APPROVED  
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
- Physical worker-pool layout (three dedicated pools versus a shared scheduler with hard class reservations) is not yet an owner-approved implementation choice. It must preserve Application/Notification isolation and N-ORD2's Completion/OnClose independent active capacity.

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
- Bounded worker loops avoid per-event goroutine creation and reduce allocation/scheduler churn. The precise physical scheduler remains open.
- Independent class budgets prevent blocked Application callbacks from consuming Completion/OnClose capacity and prevent Completion saturation from consuming OnClose's reserved capacity.
- Fair scheduling improves tail latency and prevents a high-volume Session from monopolizing available workers, but fairness cannot overcome a class whose workers are all permanently blocked.
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
- No numeric defaults, public option names, physical pool layout or event capacities are selected.

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

## 11. WORKER-004 — Logical execution-pool topology (OPEN; RECOMMENDATION ONLY)

WORKER-001/002 establish owner scope and strict per-class active ceilings; WORKER-003 establishes bounded OnClose event admission. The remaining architecture choice is how to execute the three callback classes without unbounded goroutines or cross-class head-of-line blocking.

### Option A — three class-isolated worker pools (RECOMMENDED)

Each Client/Server owner has a logically and physically separate Application, Completion and OnClose worker pool. Each pool:
- Has its own per-Session FIFO/ready-session scheduler and owner-scoped active limit.
- Starts workers lazily on demand, never above its WORKER-002 class ceiling; idle workers may be reused within that class.
- Uses no per-Request/per-frame goroutine. A worker executes one callback synchronously, then returns to that class's scheduler.
- Has independent queue/accounting locks and wakeups; no callback-class global lock is required.
- Retains WORKER-002 active permits and class-specific event/Request credits until the callback returns.

**Advantages:** strongest isolation and simplest reasoning; a blocked callback class cannot consume another class's execution goroutines; lower cross-class scheduler contention; independent queue and wake-up hot paths.

**Costs:** idle workers cannot be reused across classes; three schedulers/worker groups per owner add implementation structure; if worker creation is eager it would waste goroutines, so lazy creation is mandatory.

### Option B — one shared owner worker pool with strict class reservations

Each Client/Server has one shared execution pool whose maximum worker count is at least the sum of its three class ceilings. A shared scheduler dispatches from separate class queues only when that class has an available active permit. Workers may execute any class, but each callback retains only its own class permit.

**Advantages:** idle worker goroutines can be reused across callback classes; one worker lifecycle implementation; potentially lower idle-goroutine count when workloads are imbalanced.

**Costs:** a shared scheduler/wakeup/dispatch path can create cross-class lock contention and tail-latency coupling; fairness must be enforced both across classes and across Sessions; sizing below the sum of ceilings would violate WORKER-002 isolation; scheduler bugs could cause head-of-line blocking.

### Recommendation

Choose **Option A** for V1. User callbacks may block indefinitely, and predictable class isolation is more valuable than cross-class reuse of idle goroutines. Lazy worker creation avoids paying the maximum goroutine cost up front. A common internal implementation can still share worker-loop code without sharing runtime pools, queue locks or active capacity.

**Owner decision requested:** choose WORKER-004 Option A (three class-isolated pools) or Option B (one shared owner pool with strict per-class reservations). This is a logical execution architecture decision; numeric ceilings and idle-worker retirement remain separate.

## 11. Explicitly not decided

- Numeric per-Session concurrency defaults and maximums for Application/Completion.
- Numeric owner active-worker ceilings.
- WORKER-004 physical/logical pool topology and idle-worker lifecycle.
- OnClose saturation/skip policy and event-budget numbers.
- Incoming callback event-count cap.
- Completion payload-byte budget and final maximum frame size.
- PERF-003 numeric Request-credit defaults.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.
