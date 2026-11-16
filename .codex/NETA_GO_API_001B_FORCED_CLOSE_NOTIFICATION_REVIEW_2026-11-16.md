# neta-go V1 — API-001b Forced Close, Notification Delivery and Worker Bound Review

**Date:** 2026-11-16  
**Status:** **Forced Close A OWNER APPROVED**; API-001b remains PARTIALLY OPEN for Notification ordering/capacity and Done. Earlier §§3–10 show the historical preapproval design; §11 is the current authority split.  
**Baseline:** `main@893b96629b69b0c67b5aaf456b3e0f178e9fc811`  
**Permission:** documentation review only. No Go/C++ code, go.mod, build, tests, benchmarks or operational changes.

## 1. Binding authority (not choices created by this review)

1. **Q-003-R1 APPROVED:** independently configurable, bounded parallel Application `OnRequest` callbacks per Session; concurrency `1` is serial; N>1 permits overlapping callback execution, with FIFO dispatch selection and no completion-order guarantee.
2. **API-001b-N1 APPROVED:** `OnClose` and Request Completion user callbacks run in a separate **bounded Notification lane**, not in Application workers and not on internal protocol-engine execution paths.
3. **PERF-003 A MODEL APPROVED:** each admitted callback-first Request reserves a count credit per Session and, for Server-owned Sessions, one Server aggregate credit; the credit spans pending response, queued completion and running completion under the still-open forced-close disposition. Capacity rejection is fail-fast, with no send/pending leakage and no Session close.
4. **PERF-002 A APPROVED:** accepted outbound input is snapshotted before successful return; inbound payload is owned and retainable by user code.
5. **API-001a A and API-004 A APPROVED:** callback-first, nonblocking admission and typed Functional Options with immutable validated effective configuration. Q-008/009/010 queue admission, overflow behavior, control headroom, single TCP Writer and Pinex wire compatibility remain unchanged.
6. Existing documents containing unconditional serial *Application* callback wording are historical where they conflict with Q-003-R1. User code, including Notification code, is never forcibly killable in Go.

## 2. Required invariants regardless of forced-close selection

- **Logical completion and physical callback delivery are separate.** Exactly one terminal *internal* outcome wins among response, request deadline, transport error and Close. User callback invocation must be modeled separately as not scheduled / queued / started / returned / cancelled.
- Terminal settlement does not execute arbitrary user code under protocol locks or Reader, Writer, deadline manager, registry or shutdown engine; no goroutine per Request or callback event.
- Session transport may close, release socket/registry/timers/pending entries and signal an optional `Done`-style channel without waiting for Application or Notification callbacks. Whether `Done` is exported and its precise semantics are **OPEN**.
- No duplicate Request terminal result or double `OnClose`. A late response may not resurrect settled pending state. Concurrent response/timeout/Close is resolved by one linearized transition.
- **No promised guaranteed user callback execution if a user function blocks permanently** and teardown must remain bounded. An executing callback may outlive Session closure.
- Application backlog byte caps, global queued-byte caps and separate control reserve do not bound *active* callback payloads, callback stacks or application-retained memory.

## 3. Forced Close policy: historical comparison (A subsequently OWNER APPROVED; see §11)

| Property | **A — Cancel unstarted** (recommended) | **B — Retain bounded terminal callbacks** |
|---|---|---|
| Internal result settlement | Exactly once, independent of user functions | Same |
| Pending callback notifications not yet started at forced Close | Cancel and discharge their owned queue/credit reservations; delivery **not** guaranteed | Retain for conditional later dispatch |
| Already-running user callback | Cannot interrupt; may persist indefinitely | Same |
| Teardown and `Done`-style signal | Do not await user callbacks | Same |
| Reconnect churn with stuck callbacks | Can bound retained running callbacks with **global worker ceiling**, though service may lose notification capacity | A finite per-Session queue is **not sufficient**: retired Sessions accumulate unless a global retired-work cap also applies |
| Observable guarantee | Internal settlement once, user callback at-most-once **invocation**, best-effort upon forced Close | More opportunities for eventual callback invocation, but not unconditional guarantee if code never returns |

**Recommendation, not approval:** choose **API-001b A** for V1. It prevents accumulating old unstarted Notification payloads after Session churn and keeps transport cleanup independent from user code. This deliberately allows a previously admitted Request completion callback not to execute after forced Close; the API must state this plainly. `Done` plus a close-cause accessor can report Session transport termination, **not** all per-Request results that were never delivered to user code.

**Definition under discussion:** "Forced Close" includes unrecoverable transport failure, fatal protocol/heartbeat error, resource-overload Session closure and explicit immediate Close. A bounded, *graceful* Stop/drain policy is a distinct API-002 choice; it must never wait forever on callbacks. Do not silently apply Option A cancellation to ordinary live-session response or timeout delivery.

## 4. Candidate three-domain lifecycle / ordering contract (NOT APPROVED)

### 4.1 Internal state machine

Conceptual Request states: `admitted -> pending -> settled(result) -> notification(queued | started | returned | cancelled)`. Internal settlement is once-only; Notification state is not a second Request outcome. The Request's **existing** completion credit is reused to represent its terminal event, never a newly demanded credit that may deadlock at saturation.

Proposed forced-close sequence if A is approved:

1. Atomically mark Session closing, refuse new SendRequest/SendResponse admissions and stop admitting further Application callback work. Preserve already-linearized internal results.
2. Settle each still-pending Request once with a typed Session-close cause; reject any later response. Cancel **unstarted** Application callbacks and Notification completions, releasing their library-owned buffers/queued-event references; never attempt to terminate an executing user function.
3. Stop Session-owned timers and network operations, close/detach transport and pending registry, release non-active quota reservations, and publish immutable Session terminal cause. Report `Done` once after this internal cleanup; it must not mean all callbacks returned.
4. Queue at most one best-effort `OnClose` event **only if** a separately bounded close-event budget admits it. The optional event can be invoked after `Done`, outside engine locks; if it cannot be admitted, skip it rather than growing unlimited close events.
5. Keep capacity occupied by **already-running** user callbacks in a process/Server-scoped *active-worker* accounting domain **until they actually return**, even if original Session is retired. A closed Session must not reset the global active limit via reconnect.

Proposed ordinary closure (separate from forced cancellation) may drain previously queued completion notifications up to a finite grace deadline; exact semantics are **API-002 OPEN**. Transport `Done` still does not await indefinitely slow callbacks.

### 4.2 Proposed ordering

- Request admission order is **not** guaranteed completion order. Terminal results enter Notification scheduling in the order in which their outcomes are linearized; concurrently settled results need an explicit serialized enqueue point.
- **Candidate, not approval:** at most **one running Notification callback per Session** to simplify `OnClose`/Completion order. Keep FIFO scheduling of accepted Completion notifications within that Session; there is no callback order guarantee between independent Sessions.
- **Candidate `OnClose` barrier:** the lifecycle cause is final and `Done` is closed before `OnClose` starts. `OnClose` follows any *already-running* Notification callback of that Session; on forced Close A, unstarted completions are cancelled and do not need to run before `OnClose`. Therefore a stuck Completion callback can make `OnClose` never execute; this is explicitly *best effort*.
- A blocked **Application** callback never gates Notification delivery or `Done`. A blocked **Notification** callback can gate later same-Session Notification events in this candidate model, but must not gate internal cleanup.
- No interleaving or cross-Session ordering promises are implied. An Application handler calling `SendResponse` after Session closure receives a terminal closed/admission error, never silently revives that Session.

## 5. Notification worker and memory bounds (OPEN numerical choices)

**Candidate scheduler:** a finite shared Notification worker pool for Server-owned Sessions, with an independently bounded Client-side domain. Session-local FIFO notification metadata uses the **already-reserved PERF-003 completion credits** for Request callbacks; schedule active Sessions fairly (e.g., ready-Session round robin), lazily, without one permanently allocated worker per Session or one goroutine per event. A callback that blocks forever consumes one **global** Notification worker until process termination or user return; with all workers blocked, delivery can stall. This is an unavoidable bounded-scheduling limitation, *not* an internal-engine stall.

Account for **three distinct resources**, none implied to be interchangeable:

| Resource | Scope | Candidate invariant |
|---|---|---|
| Request completion credits | Session + Server aggregate | One per admitted callback-based Request from admission through queued/running callback; on forced A release cancelled/unstarted credits, retain a running callback credit until return |
| Active user callback worker slots | Per Session + Server/process aggregate, per callback class | Bound active Application and Notification callbacks even across recently closed Session generations; a reconnect may not bypass the aggregate ceiling |
| `OnClose` events | One maximum per Session, plus **global active+queued close-event budget** | `OnClose` must not need a new Request completion credit, because a closing Session may have exhausted all Request credits; if close-event budget is full, best-effort skip |

Request completion count ceilings do **not** by themselves bound the number of active Application callbacks, so add a separate active-Application guard. Q-009/010 waiting bytes similarly do not include active payloads. A separate count cap for tiny/empty incoming Application events is recommended to prevent metadata pressure, but **not approved**.

**Performance design preference (not a frozen implementation):** reuse bounded readiness/queue metadata; avoid per-event goroutines, huge preallocated arrays, global hot-path locks, mandatory copied response payloads and any indefinite wait in Reader/Writer/Close. Worker pools, queue fairness and timer structure must be measured only after explicit benchmark authorization.

**Unapproved numeric examples:** previous `1024` pending credits/Session and `65536` Server aggregate are **illustrative**, never defaults. Worker counts, queue-event caps, active-callback global ceilings and `OnClose` reserve are all **OPEN**. Size pending capacity using peak admitted requests/s × worst permitted combined queue+peer RTT+notification latency, with a safety factor, subject to memory/GC measurements. No numeric SLA was validated.

## 6. Race and failure matrix (test plan only; nothing executed)

| Situation | Required invariant / proposed outcome |
|---|---|
| Response and timeout race | One terminal result; no duplicate completion or credit release |
| Response vs forced Close | One internal winner; in proposed A, even an already-settled but unstarted callback may be cancelled after Close |
| Application `OnRequest` hangs | Only its bounded Application worker slot occupied; engine and separate Notification lane can continue |
| Notification Completion hangs | Running worker/credit retained; `Done` still settles; later same-Session `OnClose` may never be invoked |
| All Notification workers hang | No unbounded new goroutines; admitted completion work remains counted; new sends reject at caps; `OnClose` may be skipped on capacity exhaustion |
| Repeat reconnect/Close with hung callbacks | Old workers remain globally charged; new Session cannot create unlimited permanently blocked work |
| Empty incoming Application frames | Separate event-count admission needs decision; byte quotas alone may be insufficient for metadata |
| Client calls send during/after Close | Synchronous typed rejection, no frame/pending entry/credit leak |
| Timed-out Request still queued before TCP Write | If API-002 admission-time deadline selected, cancel unsent frame safely; after partial Write delivery is uncertain, no automatic resend |

## 7. PERF-003 remaining choices (NO owner approval here)

- Set **numeric** defaults and maximum-validation rules for pending completion credits per Session and Server aggregate. Prior 1024/65536 are discussion examples only. Preserve independent typed Functional Option overrides and fail-fast admission rejection without Session closure.
- Decide independent **incoming Application event-count cap** and its numeric default (recommended yes, Q-009 Session-only overload outcome unchanged); ensure active payload ownership and one in-progress frame are accounted.
- Decide active Application callback global ceiling, Notification active worker ceiling, global `OnClose` event budget and pending Notification scheduling bounds. Exact default of `MaxConcurrentApplicationCallbacks` remains **OPEN**; setting `1` remains explicitly supported.
- Confirm forced-close credit release on cancellation vs already-running callbacks, and a finite close-event reserve, together with API-001b policy.

## 8. API-002 — proposed questions and nonbinding recommendation

1. **Request deadline origin:** recommended **successful local admission**, including outbound queue delay. Starting after full TCP Write can retain an outstanding Request indefinitely behind a stalled writer. Choose duration/default separately. Timeouts racing with a started Write cannot assert "not delivered".
2. **Connect/Bind/Unbind:** C++ Pinex-observed 30s Connect, 30s Bind, 10s Unbind and 5s reconnect delay are **reference values, not approved Go defaults**. Specify deadlines and contexts per phase; do not silently inherit all C++ timers.
3. **Reconnect:** recommended explicit opt-in, Client only, bounded backoff/jitter and cancellation on explicit Close; Server never auto-reconnects accepted Sessions. Default enabled/disabled and delay policy **OPEN**.
4. **Stop vs Close:** recommend clear **graceful bounded Stop** vs **immediate forced Close**, with `Done` representing internal transport lifecycle completion, not a callback join. Grace period, pending Request terminal errors, OnClose state access and event cancellation remain **OPEN**.
5. **Typed causes:** distinguish admission full (byte vs completion count), request timeout, peer close, local stop, overload and invalid protocol. Exact exported names/signatures **OPEN**.

## 9. API-003 — module and package (OPEN)

Proposed public module import path: `github.com/majiddarvishan/neta-go`; proposed package identifier: `neta` (a Go identifier cannot contain the hyphen). Keep Client/Server/Session in a focused root public package, keep wire/internal machinery private; avoid tying public package to required Protobuf schemas. **Neither proposal has owner approval**. No `go.mod` was created or renamed. Pin exact C++ Pinex `develop` HEAD and golden wire frames before later authorized implementation/interoperability work.

## 10. Approval gate / next handoff

**Only next essential owner choice:** API-001b forced-close **A** (cancel not-yet-started notification callbacks) versus **B** (retain bounded deferred callbacks). A is recommended on performance/resource-isolation grounds. The independent Notification lane (API-001b-N1) and Application callback parallelism (Q-003-R1) are **already approved** and must not be conflated with the forced-close letter choice.

After that choice, seek approval for Notification ordering/`Done` contract and finite worker/close-event budgets; review PERF-003 numeric defaults and inbound event count; then API-002 deadlines/reconnect/Stop and API-003 import path. **Do not close any OPEN ID without explicit owner selection; do not implement, test or benchmark.**

## 11. OWNER-APPROVED API-001b-FC-A and follow-up Notification ordering/capacity design (2026-11-16)

**Authority split:** the owner has **explicitly approved Forced Close Option A** in [Decision Log](DECISION_LOG.md) § `API-001b-FC-A`. Sections 3 and 10 were written before the approval; their A/B choice and "not approved" statements are historical. Everything **below marked PROPOSAL or OPEN is NOT approved** by this selection. Parent API-001b remains partly open.

### 11.1 Forced Close A: concrete event classification and race constraints (APPROVED POLICY, detailed mechanism still design)

- Internal settlement is a single winner among matching response, deadline, fatal transport outcome and Close; its logical outcome is stored/observed exactly once.
- Forced Close **cancels all not-yet-started Application handlers and Completion callbacks** for that Session. A queued Completion can be cancelled even if the response already won and was recorded internally. Do not invoke callbacks simply to report their cancellation; that would defeat option A.
- A callback that *has started* cannot be forcibly stopped, including if its underlying transport is closed. Continue charging its active-worker budget (and running Request Completion credit where applicable) until it returns, even after Session removal/reconnect.
- A callback about to start and a concurrent Close need one linearization boundary: a worker commits the event from `queued` to `started` only if Session policy still permits it; otherwise cancel and release the owned charge exactly once. A worker must not keep starting cancelled events after Forced Close.
- `OnClose` remains best-effort/at-most-once, not necessarily executed if Notification scheduling is saturated. Internal engine cleanup and a possible `Done` do not wait for user callbacks. **Exporting `Done` and its exact position in ordering remain OPEN.**
- The exact catalogue of Forced Close causes versus graceful Stop deadline/drain and type names is **API-002 OPEN**, not approved implicitly by selecting A.

### 11.2 Two Notification ordering architectures (both OPEN; recommend N-ORD-2)

| Dimension | `N-ORD-1`: one per-Session serialized notification stream | `N-ORD-2`: distinct bounded Completion + OnClose sublanes (RECOMMENDED) |
|---|---|---|
| Completion callback execution | At most one at a time per Session; FIFO by terminal-result enqueue/settlement linearization, **not** original Request admission order | Completion concurrency and per-Session ordering separately configurable/proposed; maintain once-only event admission |
| OnClose | After any already-running same-Session Completion returns; it can be blocked forever | After internal Session closure (proposed: after `Done`), **without waiting for any running Completion** |
| Callback overlap | OnClose never overlaps a Completion in the same Session | OnClose **may overlap** a previously started Completion callback from the closed Session; user code must tolerate that |
| Permanently blocked Completion | Holds Session's notification execution lane and can starve OnClose; engine remains healthy | Cannot occupy OnClose worker reserve; can still prevent future Completion delivery within its own class |
| Ordering guarantee | FIFO **dispatch starts** for Completion notifications admitted before Close; upon Forced Close A all unstarted Completions are cancelled | No cross-class callback ordering guarantee; `OnClose` signals internal closure, not that all Completion callbacks returned |
| Cost | Smaller scheduler, simpler user-visible ordering | Extra finite scheduling/bookkeeping and worker reservation, better OnClose independence under stalled Completion code |

**Recommendation, not owner decision:** `N-ORD-2` better matches the high-performance and independent-user-callback goals. It needs an explicit public contract that `OnClose` may execute *concurrently with an already-running Completion*. This tradeoff is MATERIAL and requires owner approval. A global shared pool **without dedicated class capacity** does not deliver meaningful class isolation when all workers are stuck.

### 11.3 Proposed `Done` vs `OnClose` event ordering (OPEN)

Suggested internal sequence is:

1. Close admission gate, settle pending Request outcomes, cancel unstarted Application/Completion callbacks under approved A.
2. Close transport, detach registry and timers, discharge queued metadata/byte reservations and all non-active completion credits; retain credits/worker slots for any running user callback.
3. Publish immutable close reason and close a potential `Done()` signal to indicate **internal Session/transport termination only**, NOT that any user callback has finished.
4. Best-effort enqueue a single `OnClose` event using a separately bounded OnClose-event budget, independent of Request completion-credit exhaustion. If budget admission fails, do not block the engine or grow a queue indefinitely. If N-ORD-2 selected, OnClose may start before a previously running Completion returns.

The core Engine must remain nonblocking with respect to both callback pools. A best-effort `OnClose` cannot be used as a resource-cleanup barrier. A late Callback calling `SendResponse` must receive a closed-session error; it cannot resurrect the transport. **Order `Done` before `OnClose`, event drop/admission policy and exported method names are not approved.**

### 11.4 Resource-credit model and worker ceilings (all new defaults OPEN)

| Limit | Why it is independent | Lifetime / recommended treatment |
|---|---|---|
| `MaxOutstandingCompletionSlotsPerSession` | Count admitted Request response/terminal notification work, including small or zero payloads | Reserve before Request admission; hold through pending, queued and actively running Completion; on forced A release if never started, otherwise upon callback return |
| `MaxAggregateCompletionSlotsPerServer` | Prevent thousands of Sessions from multiplying pending/retired Completion work | Keep active callback charges until return **even if Session is closed** |
| `MaxConcurrentApplicationCallbacksPerSession` | Owner-approved configurable Q-003-R1; one is serial | Count active handler after dispatch start until return; *default remains OPEN* |
| `MaxActiveApplicationCallbacksPerServer` | `MaxBoundSessions × SessionConcurrency` alone is not a safe bound across reconnect churn | Finite active goroutine permit, retained across retired Session generations; when full, work waits in existing bounded ingress queue, overflow follows Q-009/Q-010 |
| `MaxActiveCompletionNotificationWorkers` | Bound physically running arbitrary Completion user functions | Finite worker/pool budget, charged until callback returns even if close cancelled unstarted tasks |
| `MaxActiveOnCloseWorkers` (if N-ORD-2) | Avoid Completion callbacks consuming all OnClose execution capacity | Finite independent worker reserve; a permanently stalled OnClose can exhaust this class |
| `MaxOutstandingOnCloseEvents` | OnClose is **not** a Request and must not require a Request completion slot, especially when that quota is full | Global active+queued event credits, at most one OnClose per closed Session; best-effort overflow disposition **OPEN**, preferably skip rather than hang |
| `MaxQueuedIncomingCallbackEvents` (optional proposal) | Small/empty Application frames occupy metadata even under 4 MiB byte cap | Bounded count alongside approved incoming bytes; exceeding it would use existing Session-only Q-009 close if independently approved |

These are **semantic dimension names only, not exported API names or approved knobs/defaults**. Prior numeric examples 1024/Session and 65536/Server outstanding completion credits remain **unapproved**. Explicit zero/unlimited for a required bounded worker/event cap should not silently disable limits; validation details are open.

**Do not mis-account**: Request credits and running Notification worker slots are different obligations, not two replacements for one another. The response/timeout/close handler reuses an admitted Request's completion credit and may not demand a second terminal-result credit. After forced A, cancelled queued completions release credits promptly; a running callback keeps its credit and worker permit until it actually returns. A retired Session should not hold full incoming FIFO/circular buffers merely because a running callback retains a user-visible payload.

**Scheduler concept — PROPOSAL:** shared, bounded, lazily activated workers with a finite per-Session queue and a bounded ready-Session structure; rotate among Sessions rather than spawning a goroutine per event or locking a global mutex while invoking user code. An independent OnClose sublane could have a separate small worker reserve (if N-ORD-2 is approved). Avoid eager preallocation to configured maxima. No numerical performance SLA or worker count has been measured.

### 11.5 Resource and liveness failure cases (design-only review; no tests executed)

1. **All Completion workers permanently blocked:** internal response settlement, TCP control handling and Forced Close still progress. Completion events remain counted until cancelled/returned; new Request admission rejects at respective approved caps. If N-ORD-2 is chosen, available OnClose workers may continue independently.
2. **All OnClose workers blocked:** subsequent OnClose callbacks are best-effort within finite OnClose event credits; do not create unlimited goroutines or delay `Done`. No unconditional delivery guarantee.
3. **Repeated reconnect with stalled Application callbacks:** old callbacks still occupy the global active Application ceiling; new Session cannot reset those charges. Global saturation may reduce other Sessions' callback throughput, but must not make internal state machine block on user code.
4. **Completion vs Forced Close:** outcome settlement has a unique winner; callback-start vs cancellation has a unique winner. Cancelled callbacks never later start, and each credit releases once.
5. **Close-event queue full while Request slots full:** OnClose admission does not consume Request slots; if its own bounded resource is full, apply a separately approved best-effort congestion policy rather than deadlocking cleanup.
6. **A Completion is already executing when the socket closes:** it may continue; if N-ORD-2 selected, OnClose can overlap it; `Done` does not mean the running Completion has returned.
7. **Graceful Stop:** no unbounded await for user callbacks. Stop grace, which work can drain and the transition to Forced Close A at deadline are API-002 OPEN.

### 11.6 Decisions still awaiting owner approval

**Next essential question:** `N-ORD-1` same-Session serialized Completion/OnClose vs `N-ORD-2` Completion/OnClose bounded sublanes with possible callback overlap (recommended N-ORD-2). Once selected, determine `Done` visibility/order, Completion concurrency and fairness, separate worker/close-event quotas and numerical defaults. Then review PERF-003 slots/event caps, API-002 timeouts and Stop/reconnect, and API-003 module name. **No unapproved choice above is final.**
