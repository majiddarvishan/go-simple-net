# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001 OWNER APPROVED OPTION A; WORKER-002 OPEN  
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
| Application | Q-003-R1: positive configurable per-Session concurrency; 1 is serial, >1 permits overlap; FIFO dispatch selection | Default, owner aggregate active ceiling, event-count and active-payload bounds |
| Completion | C-ORD-B: positive configurable per-Session concurrency; 1 is serial; FIFO start selection by terminal-settlement enqueue order; return order not guaranteed | Default, owner aggregate active ceiling, queue metadata and payload-byte bounds |
| OnClose | N-ORD2: independent from Completion; eligible only after Done closes; may overlap a running Completion; best-effort/at-most-once | Owner active-worker ceiling, active+queued event budget and saturation disposition |
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
- It needs its own bounded active-worker capacity and bounded active+queued event population per owning instance.
- A started OnClose retains its active-worker and event charges until return.
- If the independent OnClose capacity is exhausted, Done still closes and internal teardown still completes. Whether the new OnClose event is skipped or otherwise handled remains an owner decision.

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
| OnClose active/event capacity full | Done and internal cleanup still proceed; OnClose remains best-effort; exact skip/admission policy is OPEN |
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

## 9. WORKER-002 — owner aggregate active-worker ceiling topology (OPEN)

The next decision is whether active callback execution should have explicit owner-wide ceilings per callback class or be derived from Session limits.

### Option A — explicit independent owner ceilings per class (RECOMMENDED)

Each Client/Server instance has three independently bounded aggregate active-worker ceilings:

- Application active callbacks.
- Completion active callbacks.
- OnClose active callbacks.

Each per-Session concurrency limit is intersected with its class's owner-wide ceiling. The three class budgets cannot consume each other's reserved active capacity. Numeric defaults are a later decision; selecting this policy does not select values.

**Advantages:** strict and understandable upper bounds; strong isolation under permanently blocked callbacks; straightforward accounting across reconnect generations; no dependence on the current number of live Sessions.

**Costs:** three additional capacity dimensions per owner and possible idle reserved capacity; more configuration surface. This does not require eagerly creating all worker goroutines.

### Option B — derive aggregate ceilings from existing Session limits

Derive Application/Completion active limits from maximum Session count multiplied by per-Session concurrency; derive OnClose limits from the Session/event admission ceiling, while still counting retired generations.

**Advantages:** fewer independent configuration knobs.

**Costs:** a high MaxBoundSessions multiplied by a high per-Session concurrency can create a very large theoretical active-worker ceiling; capacity becomes coupled to unrelated Session admission settings; Client reconnect generations do not map cleanly to Server MaxBoundSessions; retired callbacks make event and worker sizing less intuitive.

### Owner decision requested

Choose whether V1 requires explicit, independently configurable owner-wide active-worker ceilings for Application, Completion and OnClose (Option A), or derives them from existing Session admission/concurrency settings (Option B). This decision is about the **capacity model**, not numeric defaults or exported option names.

## 10. Explicitly not decided

- Numeric per-Session concurrency defaults and maximums for Application/Completion.
- Numeric owner active-worker ceilings.
- Physical worker-pool layout and idle-worker lifecycle.
- OnClose saturation/skip policy and event-budget numbers.
- Incoming callback event-count cap.
- Completion payload-byte budget and final maximum frame size.
- PERF-003 numeric Request-credit defaults.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.
