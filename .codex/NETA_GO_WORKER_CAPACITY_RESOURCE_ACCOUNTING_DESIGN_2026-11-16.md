# neta-go V1 — Worker Capacity & Resource Accounting Design

**Date:** 2026-11-16  
**Status:** DESIGN IN PROGRESS — WORKER-001/002/003/004/005/006/007 OWNER APPROVED; WORKER-008 OPEN  
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

## 15. WORKER-008 — Resource Accounting and Payload Memory Ceiling (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no owner selection yet.**  
**Implementation:** NOT AUTHORIZED.

### 1. Problem and binding inputs

WORKER-008 defines how library-owned payload memory is reserved, charged, transferred and released. It does not reopen:

- PERF-002 A: outgoing payload is snapshotted into private storage before successful Send return; incoming payload is read into owned storage and may be retained by the application after callback return; the library never recycles user-visible inbound buffers.
- Q-008/Q-009/Q-010: existing 4 MiB per-Session outgoing/incoming byte limits, 256 MiB Server aggregate queued Application-frame byte limit, 4 KiB per-Session Control reserve, and approved overload behavior.
- PERF-003: Request credits cover the entire callback-based Request lifecycle, including waiting for response, queued Completion and running Completion until callback return/cancellation.
- API-001b Forced Close A: unstarted Application/Completion callbacks are cancelled and release queued library-owned state; started callbacks cannot be interrupted and retain their active permits and relevant Request credit until return.
- WORKER-001: all reconnect generations of a Client share Client-owned aggregate capacity; all Sessions of a Server share Server-owned aggregate capacity.
- WORKER-002/004/006: active workers are class-isolated, bounded and fairly scheduled. Worker capacity and payload-byte capacity are different ledgers.
- WORKER-007 A: Application incoming/outgoing frame counts and the Server aggregate queued-frame count are independent of byte budgets; Completion and OnClose reuse their existing count credits.

### 2. Required memory populations

| Population | Library ownership interval | Existing/required bound |
|---|---|---|
| Outgoing Application frame | Successful admission through full Writer completion/failure | Q-008 outgoing bytes + WORKER-007 outgoing count; retained Application bytes proposed |
| Incoming Stream Request | Header validation/body assembly through queue wait and active OnRequest callback return/cancellation | Q-009/Q-010 incoming queue bytes + WORKER-007 incoming count while waiting; retained Application bytes proposed across in-progress/queued/active |
| Incoming Stream Response | Header validation/body assembly through queued/running Completion callback return/cancellation | Existing aggregate queued-frame bytes while queued + PERF-003 Request credit; retained Completion bytes proposed |
| Reader in-progress body | After valid length is known until complete read or abort | Must reserve against the eventual destination ledger before allocation; one Reader/frame per Session |
| Writer in-progress frame | Until full write or definitive failure/discard | Already included in outgoing bytes/count; no second charge |
| Control PDU | Queue/admission through full write; bounded inbound control parsing | Separate Q-010 4 KiB per-Session Control allowance; never consume Application budget |
| Pending Request after request write | Until terminal settlement/cancellation | Metadata/Request credit only; do not retain outgoing payload bytes solely for correlation |
| OnClose | Queued + running event | WORKER-003 event credit; no payload-byte ledger unless API later adds payload |
| User-retained inbound payload | After callback return, if application retained the slice | Outside enforceable library-owned ceiling; never pool/recycle it |

### 3. Option A — owner-scoped retained-payload ledgers per callback class (RECOMMENDED)

Introduce two new finite byte ledgers for library-owned user payloads:

**Application retained bytes**
- A per-Session ceiling and an aggregate per-owner ceiling (per Client or Server).
- Covers incoming Stream Request payloads from the point the Reader has validated the fixed header and reserved the declared frame length, through in-progress body read, queued callback wait and active OnRequest execution until callback return/cancellation.
- Covers outgoing Application frame storage from admission and private snapshot allocation through full Writer completion/failure. Count the complete serialized frame length, including the 10-byte Pinex header, conservatively and consistently with Q-008.
- Existing incoming/outgoing queue byte/count limits remain separate sub-limits. They are not added together to estimate memory. When an incoming callback starts, release only its waiting-queue byte/count charges; keep its retained-payload charge until callback return. When an outgoing frame fully writes, release its retained-payload and outgoing queue charges.
- Client aggregate accounting spans reconnect generations. A callback from an old generation remains charged to the same Client owner after Session teardown/reconnect until it returns.

**Completion retained bytes**
- A separate per-Session and per-owner byte ceiling for Stream Response payloads retained for Completion delivery.
- Reserve before allocating/reading a matching response body; retain through queued and actively executing Completion callback; release on callback return or Forced Close A cancellation.
- PERF-003 Request credit remains the only Completion event/count credit and is held until the complete Request lifecycle ends. The Completion byte ledger is a different dimension, not another event credit.
- A started Completion callback retains its payload-byte charge and Request credit until it returns, including after Done/Session teardown. An unstarted Completion cancelled by Forced Close A releases its payload and Request credit.
- Do not retain the original outgoing request payload after its Writer frame is fully written; correlation state retains only the required metadata.

**Reservation and transfer rules**
1. Read the fixed 10-byte header into bounded fixed storage. Validate command, Session state and command_length against the protocol maximum before allocating the body.
2. Once the body length and destination class are known, reserve the appropriate per-Session and owner byte ledgers plus the applicable queue byte/count sub-limits before allocating/reading the body. A partial read holds its full declared reservation conservatively; abort/error releases it.
3. For outgoing Send, validate and reserve count/byte/retained-byte capacity before allocating the private contiguous frame and copying the caller payload once. On any failed admission/allocation, roll back every reservation; no frame or pending tracker becomes visible.
4. Ownership transitions are linearized. Incoming queue → active callback releases queue-only charges but does not release/reacquire the retained-payload charge. Outgoing FIFO → active Writer does not release any charge. Full Write/callback return/cancellation releases exactly once.
5. A response frame that has no live Pending Request is late/unknown; it must not resurrect a settled Request or create a Completion event. Prefer bounded discard after header validation rather than retaining an unowned response payload.
6. Control traffic retains its own finite reserve and ordering. Pre-bind connections remain bounded by Q-010's 128 pending sockets and 30-second absolute handshake deadline; Bind PDU parsing should use its protocol-specific small bound rather than allocating a Stream-sized body.
7. No eager maximum-capacity preallocation, goroutine per payload/event, or pooling of user-visible incoming payloads.

### 4. Frame-size and metadata boundaries

- The Pinex wire header is 10 bytes. The current compatibility reference says a 1 MiB maximum frame, but the exact ceiling still requires cross-language confirmation; reject oversized declared lengths before body allocation. Do not silently infer that the 4 MiB queue cap permits a 4 MiB wire frame.
- One Reader and one Writer per Session bound in-progress transport work. The Writer's current Application frame remains charged to the outgoing FIFO limits until complete.
- Queue/event count ceilings, MaxBoundSessions, pre-bind connection limits and worker ceilings bound metadata populations and goroutine stacks; byte ledgers bound payload/frame storage. These are complementary.
- Go object headers, allocator fragmentation, GC metadata, goroutine stacks, socket/kernel buffers and application-retained payloads are not byte-exactly covered by these payload ledgers. The sum of finite owner/class limits provides a predictable library-owned payload bound, not a strict process RSS cap.
- A callback may retain an incoming []byte after return. The library cannot observe that retention; release its library charge at callback return but never reuse that backing array.

### 5. Option B — separate byte ceiling for every lifecycle stage

Give incoming-in-progress, incoming-queued, active Application, outgoing FIFO/Writer, Completion-queued and Completion-running independent byte ceilings, transferring reservations at every stage boundary.

**Advantages:** maximum per-stage tuning and the ability to isolate a pathological stage.

**Costs:** many more counters and configuration dimensions; every transfer needs a multi-ledger atomic transaction; dispatch can stall when the destination stage is full even while queue capacity remains; double-reserve/rollback logic increases hot-path work and race surface. It is harder to prove no charge gap or deadlock.

### 6. Option C — derive active bounds from frame size, count and worker ceilings

Add no new retained-payload byte ledgers. Rely on a hard maximum frame size, existing queue-byte limits, WORKER-007 counts, PERF-003 Request credits and WORKER-002 active-worker ceilings.

**Advantages:** lowest accounting overhead and fewest configuration options.

**Costs:** worst-case active/Completion memory becomes the product of frame maximum and active/count ceilings. Numeric count/worker limits are not yet approved, and this derived upper bound may be too large. It offers no independent protection against many maximum-sized active payloads and does not make memory tuning explicit.

### 7. Recommendation and unresolved details

Choose **Option A**. It preserves queue-specific overload behavior while giving each callback class a clear owner-scoped retained-payload ceiling. The incoming queue-to-active transition has no accounting gap: the queue sub-limit is released, but the retained-payload reservation stays with the same owned buffer until callback return. Application and Completion byte ledgers are independent so a large Request-handler payload population cannot consume the Completion payload reserve, or vice versa.

Numeric byte ceilings, count defaults, worker ceilings, exact protocol maximum frame size and public configuration names remain for WORKER-009/API-004 and protocol cross-language confirmation. The exact overflow disposition when a valid matching Completion response cannot reserve its Completion byte budget must be resolved before Architecture Freeze; do not silently drop the response or invent a second Request credit. No implementation, tests or benchmarks are authorized.

**Owner decision requested:** choose WORKER-008 Option A (owner-scoped retained-payload ledgers per Application and Completion class), Option B (separate byte limits per lifecycle stage), or Option C (derive bounds only from frame/count/worker limits). No numeric byte defaults are implied.

---

## 16. Explicitly not decided

- Numeric per-Session concurrency defaults and maximums for Application/Completion.
- Numeric owner active-worker ceilings.
- WORKER-005 numeric idle-timeout defaults and valid ranges.
- OnClose event-budget numeric default and public option name.

- WORKER-008 retained Application/Completion payload-byte ceilings, Completion byte-overflow disposition and final maximum frame size.
- PERF-003 numeric Request-credit defaults.
- WORKER-007 numeric count-cap defaults, valid ranges and public configuration names.

- WORKER-009 safe defaults, valid ranges and public configuration limits.
- API-002 timeout, reconnect and Stop/drain semantics; API-003 module/package path.

**Guardrail:** design/documentation only. No Go/C++ source, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.
