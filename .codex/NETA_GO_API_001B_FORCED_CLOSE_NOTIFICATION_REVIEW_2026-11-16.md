# neta-go V1 — API-001b Forced Close, Notification Delivery and Worker Bound Review

**Date:** 2026-11-16  
**Status:** API-001b POLICY CLOSED / OWNER APPROVED; WORKER-001..009C CLOSED; final public surface, terminal-cause accessor and Close timing are finalized in API-001 §21. Earlier sections are historical and superseded by this authority note.  
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

### 11.2 Notification ordering — N-ORD2 OWNER APPROVED (2026-11-16)

The owner explicitly selected **N-ORD2**. N-ORD1 (one serialized same-Session Completion+OnClose stream) is **not selected**.

| Dimension | N-ORD1 (not selected) | N-ORD2 (OWNER APPROVED) |
|---|---|---|
| Execution classes | Shared serial stream | Separate bounded Completion and OnClose sublanes |
| OnClose vs active Completion | Waits | May overlap; OnClose does not wait for Completion return |
| Capacity isolation | A blocked Completion can occupy the only notification lane | Completion cannot consume reserved OnClose active-worker capacity |
| Cross-class ordering | Serialized | No cross-class start/return order guarantee |
| OnClose liveness | Can starve behind Completion | Independent of Completion congestion, but still best-effort if its own lane is saturated |

**Binding implications:**
- Completion and OnClose must have independently bounded admission/queueing and active-worker capacity. A single unpartitioned worker pool with no per-class reservation is **not** N-ORD2. A shared scheduler is acceptable only if it enforces independent class budgets and reserved execution slots.
- OnClose may execute concurrently with a Completion callback already started for the same Session. Public callback documentation must warn that user state shared by these callbacks needs synchronization.
- OnClose remains best-effort and at-most-once. N-ORD2 does not promise that it will run if its own bounded lane is saturated or its workers are permanently blocked.
- Forced Close A remains binding: unstarted Completion notifications are cancelled; already-started Completion callbacks are not interrupted. OnClose is a distinct close event and remains best-effort.

**Still OPEN:** Completion ordering/concurrency within its own class; worker counts/defaults and caps; event queue limits; fairness; OnClose saturation disposition. Done and its ordering relative to OnClose are now owner-approved by DONE-A. A candidate initial Completion policy is one running callback per Session, FIFO by terminal-settlement enqueue order, with parallel execution across Sessions. This candidate is **not approved**.

### 11.3 Session Done vs OnClose ordering — DONE-A OWNER APPROVED (2026-11-16)

The owner approved **Option A**. Session exposes a read-only **Done() <-chan struct{}** signal, closed exactly once after internal Session teardown and immutable terminal-cause publication. It is an internal-lifecycle signal, never a callback join.

Binding order:

1. Close Session admission; reject new sends/responses and settle each pending Request's internal terminal result exactly once.
2. Under Forced Close A, cancel all not-yet-started Application and Completion callbacks and release their queued library-owned payload/metadata and credits. Already-running callbacks remain active and charged until return.
3. Close/detach transport; stop internal Reader/Writer/control loops and Session timers; remove pending/registry state; release internal queue/accounting reservations not retained by running callbacks.
4. Publish the immutable terminal cause/state, then close Done exactly once.
5. Only after Done is closed may an admitted OnClose event become eligible to start. Under N-ORD2 it may overlap a Completion callback already running for the same Session. If OnClose cannot be admitted under its independent bound, Done still closes; OnClose remains best-effort/at-most-once.

Done does not mean any Application/Completion/OnClose callback has returned, that every Completion ran, or that OnClose started. The internal engine never waits for user callback return. The exact terminal-cause accessor and Close() return behavior remain open; Server/Client Stop aggregation remains API-002. A late callback calling SendResponse must receive a closed-session error and cannot resurrect the Session.

### 11.4 Resource-credit model and worker ceilings (all new defaults OPEN)

| Limit | Why it is independent | Lifetime / recommended treatment |
|---|---|---|
| `MaxOutstandingCompletionSlotsPerSession` | Count admitted Request response/terminal notification work, including small or zero payloads | Reserve before Request admission; hold through pending, queued and actively running Completion; on forced A release if never started, otherwise upon callback return |
| `MaxAggregateCompletionSlotsPerServer` | Prevent thousands of Sessions from multiplying pending/retired Completion work | Keep active callback charges until return **even if Session is closed** |
| `MaxConcurrentApplicationCallbacksPerSession` | Owner-approved configurable Q-003-R1; one is serial | Count active handler after dispatch start until return; *default remains OPEN* |
| `MaxActiveApplicationCallbacksPerServer` | `MaxBoundSessions × SessionConcurrency` alone is not a safe bound across reconnect churn | Finite active goroutine permit, retained across retired Session generations; when full, work waits in existing bounded ingress queue, overflow follows Q-009/Q-010 |
| `MaxActiveCompletionNotificationWorkers` | Bound physically running arbitrary Completion user functions | Finite worker/pool budget, charged until callback returns even if close cancelled unstarted tasks |
| `MaxActiveOnCloseWorkers` (required by owner-approved N-ORD2) | Avoid Completion callbacks consuming all OnClose execution capacity | Finite independent worker reserve; a permanently stalled OnClose can exhaust this class |
| `MaxOutstandingOnCloseEvents` | OnClose is **not** a Request and must not require a Request completion slot, especially when that quota is full | Global active+queued event credits, at most one OnClose per closed Session; best-effort overflow disposition **OPEN**, preferably skip rather than hang |
| `MaxQueuedIncomingCallbackEvents` (optional proposal) | Small/empty Application frames occupy metadata even under 4 MiB byte cap | Bounded count alongside approved incoming bytes; exceeding it would use existing Session-only Q-009 close if independently approved |

These are **semantic dimension names only, not exported API names or approved knobs/defaults**. Prior numeric examples 1024/Session and 65536/Server outstanding completion credits remain **unapproved**. Explicit zero/unlimited for a required bounded worker/event cap should not silently disable limits; validation details are open.

**Do not mis-account**: Request credits and running Notification worker slots are different obligations, not two replacements for one another. The response/timeout/close handler reuses an admitted Request's completion credit and may not demand a second terminal-result credit. After forced A, cancelled queued completions release credits promptly; a running callback keeps its credit and worker permit until it actually returns. A retired Session should not hold full incoming FIFO/circular buffers merely because a running callback retains a user-visible payload.

**Scheduler concept — PROPOSAL:** shared, bounded, lazily activated workers with a finite per-Session queue and a bounded ready-Session structure; rotate among Sessions rather than spawning a goroutine per event or locking a global mutex while invoking user code. Under owner-approved N-ORD2, the OnClose sublane has a separate worker reserve. Avoid eager preallocation to configured maxima. No numerical performance SLA or worker count has been measured.

### 11.5 Resource and liveness failure cases (design-only review; no tests executed)

1. **All Completion workers permanently blocked:** internal response settlement, TCP control handling and Forced Close still progress. Completion events remain counted until cancelled/returned; new Request admission rejects at respective approved caps. Under owner-approved N-ORD2, available OnClose workers may continue independently.
2. **All OnClose workers blocked:** subsequent OnClose callbacks are best-effort within finite OnClose event credits; do not create unlimited goroutines or delay `Done`. No unconditional delivery guarantee.
3. **Repeated reconnect with stalled Application callbacks:** old callbacks still occupy the global active Application ceiling; new Session cannot reset those charges. Global saturation may reduce other Sessions' callback throughput, but must not make internal state machine block on user code.
4. **Completion vs Forced Close:** outcome settlement has a unique winner; callback-start vs cancellation has a unique winner. Cancelled callbacks never later start, and each credit releases once.
5. **Close-event queue full while Request slots full:** OnClose admission does not consume Request slots; if its own bounded resource is full, apply a separately approved best-effort congestion policy rather than deadlocking cleanup.
6. **A Completion is already executing when the socket closes:** it may continue; under owner-approved N-ORD2, OnClose can overlap it; `Done` does not mean the running Completion has returned.
7. **Graceful Stop:** no unbounded await for user callbacks. Stop grace, which work can drain and the transition to Forced Close A at deadline are API-002 OPEN.

### 11.6 Decisions still awaiting owner approval

**N-ORD2, DONE-A and C-ORD-B are OWNER APPROVED**; cross-class Notification separation, Done-before-OnClose eligibility and within-Completion concurrency policy are closed. Remaining work: numeric worker/event ceilings and defaults, aggregate scope, fairness, OnClose saturation, payload-byte/frame-size accounting and numeric PERF-003 limits. Then review API-002 timeouts and Stop/reconnect, and API-003 module name. **No unapproved choice above is final.**

### 11.7 Worker-capacity architecture and Done contract — N-ORD2 / DONE-A APPROVED; capacities remain PROPOSALS

This section follows owner-approved N-ORD2 and DONE-A. Done semantics are binding; Worker counts, queue/event limits, aggregate scope and public capacity option names remain unapproved.

#### A. Three independent user-code execution budgets

| Domain | Active execution budget | Queued-work admission | Lifetime / isolation |
|---|---|---|---|
| Application OnRequest | Per-Session configurable concurrency (Q-003-R1) plus a proposed Server aggregate active-worker ceiling | Existing Q-009/Q-010 Session + Server queued-frame byte budgets; optional incoming event-count cap remains open | Started handlers retain active permits until return, even after Session close |
| Completion notifications | Separate bounded Completion worker budget; proposed Server aggregate ceiling | Reuse PERF-003's already-reserved per-Session + Server Request completion credits; do not allocate a second slot on response settlement | A queued callback cancelled by Forced Close A releases its credit; a started callback retains both its credit and worker permit until return |
| OnClose notifications | Independent bounded OnClose worker budget, reserved separately from Completion | Separate active+queued OnClose event budget; at most one event per Session; does not consume Request completion credits | A started OnClose callback retains its worker/event permit until return; saturation is best-effort skip, never unbounded spawn |

The active worker limit is **not** interchangeable with Request completion credits. Credits bound the lifecycle population of admitted callback-first Requests; worker permits bound how many arbitrary user functions are physically executing. Q-010's 4 MiB / 256 MiB incoming queue byte caps count waiting frames, not active callback stacks or all user-retained payload memory.

#### B. Candidate scheduler shape

- Use finite, lazily activated worker loops per callback class and per owning Client/Server resource domain. Do not allocate one permanent goroutine per Session or spawn one goroutine per event.
- Keep Session-local FIFO metadata and a bounded ready-Session structure. A fair ready-Session round-robin is preferred over draining one busy Session indefinitely. Do not hold registry/global hot-path locks while calling user code.
- Completion queue items consume the existing PERF-003 Request credit from SendRequest admission through response pending, queued notification and callback return/cancellation. Settlement reuses that credit; no second terminal-result slot is allocated.
- **Owner-approved C-ORD-B:** configurable positive per-Session Completion concurrency; 1 is serial and values greater than 1 permit overlap. FIFO start selection follows serialized per-Session terminal-settlement enqueue order, not Request admission order. With concurrency greater than 1, callback return order is not guaranteed. The per-Session limit is intersected with a finite aggregate active Completion-worker ceiling; numeric limits/default remain open.
- OnClose queue/event admission has its own bounded budget, at most one event for a Session. If no event credit is available, recommended policy is to skip that OnClose invocation and record an internal diagnostic/counter; never block transport cleanup or create an overflow goroutine. The skip policy is **not yet owner-approved**.
- The Server aggregate should account for active callbacks from **retired Session generations** until their functions return, so reconnect cannot reset worker usage. Client reconnect generations share their Client's budget. Whether separate Client/Server objects also share a process-wide singleton cap is open; the V1 recommendation is per owning Client/Server instance to avoid hidden cross-instance coupling.
- No numeric worker count is selected. Counts should be chosen only after representative concurrency/latency and memory measurements are authorized. Zero/unlimited must not silently disable a required bounded class.

#### C. Completion payload and memory accounting

PERF-003's count cap bounds the number of accepted Completion lifecycles, not their aggregate bytes. A Completion event may retain a response payload while queued or executing. Add a design-level byte budget for library-retained Completion payloads per Session and aggregate owner (or prove a strict frame-size ceiling makes a separate cap unnecessary). Charge while the library retains the payload; release its library-owned charge when the callback returns or is cancelled. A user retaining the provided owned []byte beyond callback return owns that additional memory, outside a library-enforceable RSS cap. Exact byte cap and frame-size relationship remain open.

OnClose metadata is small and payload-free; count admission is the primary bound. Application active payloads similarly need a separate active-count/frame-size analysis beyond waiting queue bytes.

#### D. Session Done semantics — OWNER APPROVED (DONE-A)

**Approved public shape:** Done() <-chan struct{} is a read-only, idempotently observable Session lifecycle signal. The channel closes exactly once after internal Session teardown is complete and after the terminal cause/state is immutable. The terminal-cause accessor name/signature remains open.

The binding order is: (1) close Session admission and settle pending Request outcomes exactly once; (2) on a Forced Close path, cancel unstarted Application/Completion callbacks under Forced Close A and release their queued library-owned references/credits; (3) close/detach transport, stop timers and internal Reader/Writer/control loops, remove pending/registry state and release internal accounting not retained by running callbacks; (4) publish immutable terminal cause/state; (5) close Done; (6) only after Done closes may an admitted OnClose event become eligible to start. Under N-ORD2 it may overlap a Completion callback already running for that Session.

Done never waits for Application, Completion or OnClose callback return. A running callback remains charged to its active-worker budget until return; a running Completion also retains its PERF-003 credit. Done does not guarantee that every Completion ran, that OnClose was admitted/started/returned, or that application-retained payload memory was freed. Done closes even when the OnClose event cannot be admitted under its independent bound.

**Why this boundary:** callers can wait for internal transport/registry/timer cleanup without hanging forever on arbitrary user code. A late callback calling SendResponse receives a closed-session error and cannot resurrect the Session.

**Still open:** terminal-cause accessor name/signature; whether Close() returns before, at, or after Done; OnClose saturation/admission policy; Client/Server graceful Stop aggregation under API-002.

#### E. Failure behavior under finite capacity

1. All Completion workers block forever: internal protocol and Done still progress; Completion credits remain charged for started callbacks; unstarted callbacks are cancelled by Forced Close A. New SendRequest admissions reject when PERF-003 count limits are exhausted. OnClose remains independently schedulable.
2. All OnClose workers block forever: Done still progresses; OnClose event capacity remains bounded; subsequent close events may be skipped under the proposed saturation policy.
3. Application workers block: existing bounded ingress queue absorbs work only up to Q-009/Q-010; a blocked Application callback cannot consume Completion or OnClose worker slots.
4. Reconnect after Session close: any old running callback continues to consume the owning instance's active-worker permit; a new Session cannot manufacture extra capacity.
5. Completion and OnClose for one Session: OnClose may start before a running Completion returns. This overlap is owner-approved by N-ORD2; shared user state must be synchronized by the application.

### 11.8 Completion within-lane concurrency — C-ORD-B OWNER APPROVED (2026-11-16)

The owner selected **C-ORD-B**. This closes the same-Session Completion concurrency policy while leaving numeric capacity/default choices open.

**Binding V1 contract:**
1. Completion concurrency is a configurable **positive per-Session maximum**.
2. 1 means at most one Completion callback executes at a time for that Session; values greater than 1 allow Completion callbacks from the same Session to overlap.
3. The per-Session queue preserves FIFO callback **start selection** by the order in which the internal engine serializes terminal-result settlement/enqueue for that Session. This is not original SendRequest admission order: responses may arrive and settle out of order.
4. With concurrency greater than 1, callback return/completion order is not guaranteed to match FIFO start selection. Applications sharing mutable state across Completion callbacks must synchronize it.
5. The configured per-Session maximum is intersected with a finite aggregate active Completion-worker ceiling for the owning Client/Server domain. It cannot create a goroutine per Request or bypass that ceiling.
6. PERF-003 Request credits continue to bound the full admitted Completion lifecycle from before SendRequest acceptance through pending, queued and running callback until return/cancellation. A separate active-worker permit bounds physical callback execution and is held until the callback returns.
7. Under N-ORD2, Completion and OnClose remain independently bounded; OnClose may overlap a running Completion after Done closes. Under Forced Close A, not-yet-started Completion callbacks are cancelled on a Forced Close path, while already-running callbacks continue and retain their Request credit and worker permit until return.

**Still OPEN:** numeric per-Session concurrency default and exported option name/range; active Completion worker counts and aggregate scope; event/queue capacities; OnClose saturation disposition; Completion payload-byte/frame-size bounds. No numeric defaults are inferred.


## 12. WORKER-001 — Owner-scoped worker capacity (OWNER APPROVED, 2026-11-16)

The owner selected **Option A**.

- Every Client instance and every Server instance owns an independent callback-worker resource domain.
- Sessions of one Server share that Server's worker budgets. All reconnect generations of one Client share the same Client budget.
- A started callback from a retired Session remains charged to its owning instance until it returns; reconnect cannot reset the charge.
- Independent Client/Server instances do not share a hidden process-global worker pool or mandatory process-global active-worker cap.

This decision defines only the **owner boundary**. It does not approve physical worker-pool layout, per-class aggregate-cap configuration, numeric worker counts/defaults, queue/event limits, payload-byte bounds, public option names or OnClose saturation behavior.

**Next owner decision:** WORKER-002 — explicit independent owner-wide active-worker ceilings per callback class versus aggregate ceilings derived from Session limits. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §9.

No code, tests, benchmarks, builds, dependencies or operational changes.

## 13. WORKER-002 — Independent owner-wide active-worker ceilings (OWNER APPROVED, 2026-11-16)

The owner selected **Option A**.

Each Client and Server instance has three independently configurable aggregate active-worker ceilings:

- Application active callbacks.
- Completion active callbacks.
- OnClose active callbacks.

A Session's configured per-Session concurrency is intersected with the remaining capacity of its corresponding owner/class ceiling. The three classes cannot borrow or consume one another's active-worker permits. Queued/event populations are bounded separately and are not counted as physically active workers.

WORKER-001 ownership applies: Server Sessions share their Server's three ceilings; all Client reconnect generations share the Client's ceilings; started callbacks from retired generations remain charged until return. Independent instances do not share a process-global ceiling.

No numeric defaults, public option names, physical worker-pool layout or event capacities are selected. The next open decision is **WORKER-003**, OnClose event-budget saturation. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§9–12.

No code, tests, benchmarks, builds, dependencies or operational changes.

## 14. WORKER-003 — OnClose saturation (OWNER APPROVED, OPTION A, 2026-11-16)

The owner selected **Option A: skip a new OnClose event when the owner event budget is full**.

The owner-scoped OnClose event budget counts queued plus running events. After Session Done closes, the engine makes one nonblocking reservation attempt. On success, the event is queued and holds its event credit until callback return. On saturation, the callback is permanently skipped: no retry, overflow goroutine, hidden queue or wait. Done, transport teardown, terminal cause and reconnect are unaffected. The event budget is shared across Client reconnect generations and remains charged for callbacks from retired generations.

This implements the already-approved best-effort/at-most-once OnClose contract. No numeric budget, public option name or diagnostic API is selected.

**Next:** WORKER-004 — logical execution-pool topology. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.


## 15. WORKER-004 — Three class-isolated worker pools (OWNER APPROVED, OPTION A, 2026-11-16)

WORKER-004 Option A is owner approved. Each Client and Server owns three independent callback pools: Application, Completion and OnClose. Each class has its own scheduler, ready queue, dispatch path, lock domain and wake-up mechanism. Workers are created lazily, execute one callback synchronously and are reusable only within their own class. No goroutine is created per Request, frame or callback.

A blocked callback class cannot consume another class's worker capacity or dispatch resources. The internal protocol/transport engine remains outside all user callback pools. Shared worker-loop implementation code is permitted, but shared execution pools, queues, scheduler locks or wake-up state across classes are not.

The number of worker goroutines in a class is bounded by that class's WORKER-002 ceiling. Idle workers do not hold active-callback permits but retain goroutine/stack resources. WORKER-001 owner scope, WORKER-002 independent active ceilings, WORKER-003 OnClose event budget and retired-generation accounting remain binding. There is no process-global pool or ceiling.

This supersedes the earlier open WORKER-004 references in §§11.6–11.7. Idle-worker lifecycle and retirement are separated into WORKER-005; numeric defaults remain open. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§11–12.

No implementation, tests, benchmarks or operational changes.

## 16. WORKER-005 — Finite idle worker retirement (OWNER APPROVED, OPTION B, 2026-11-16)

The owner approved finite idle retirement. A worker becomes eligible for retirement only after its callback returns and no dispatchable work exists in its class pool. The class-specific finite idle grace timeout begins at that point. Work arriving before expiry reuses/wakes the worker; expiry with no work atomically removes the worker from the pool count and exits it. The default warm-idle floor is zero.

Application, Completion and OnClose idle timeouts are independently configurable; numeric defaults and ranges are deferred to WORKER-009. A running or permanently blocked callback is never interrupted or retired by the timeout and retains its active permit until actual return. Retirement cannot discard queued work or release Request/event/queue credits. Enqueue-versus-retirement must have one linearization point, with no lost wakeup, stranded ready Session or worker-count overshoot. No goroutine per timeout or process-global reaper. Owner shutdown/drain remains API-002.

**Next:** WORKER-006 — fair scheduling between Sessions within each class-isolated pool. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§12–13. No implementation, tests or benchmarks authorized.


## 17. Historical WORKER-006/007 handoff (SUPERSEDED)

The WORKER-006 and WORKER-007 policies are resolved by the owner-approved decisions recorded in §§17–18 below and in DECISION_LOG.md. No open WORKER-007 policy remains.

## 18. WORKER-007 / WORKER-008 — queue counts and retained payload accounting (OWNER APPROVED, 2026-11-17)

WORKER-007 Option A is approved: per-Session incoming/outgoing Application frame-count ceilings plus one Server-aggregate queued Application frame-count ceiling across both directions. Incoming count saturation closes only the offending Session; outgoing saturation returns Queue Full and keeps it. Completion reuses PERF-003 Request credits; OnClose reuses WORKER-003 queued+running event credits.

WORKER-008 Option A is approved: separate per-Session and aggregate-owner retained-payload byte ledgers for Application and Completion. Application covers incoming Request and outgoing private frame storage through callback return/full Writer completion. Completion covers matched Response payload through queued/running Completion return or Forced Close A cancellation. Queue-only charges are released at queue transitions; retained-byte charges follow the same buffer. Started callbacks remain charged until return, including after Done/reconnect. Completion-byte exhaustion behavior is closed under WORKER-008 Option A. Pinex 1.5.0's 1 MiB total-frame maximum is source-verified; golden-frame/cross-language confirmation remains open. No silent Response drop is permitted.

## WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits

**Status:** WORKER-009A CLOSED / OWNER APPROVED; WORKER-009B1 CLOSED / OWNER APPROVED; WORKER-009B2/B3 and WORKER-009C OPEN.  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 establishes zero-configuration defaults and finite validation boundaries without weakening the bounded-resource contracts.

- **WORKER-009A — CLOSED:** derive the three owner active-worker defaults from GOMAXPROCS; keep other resource quotas fixed and finite.
- **WORKER-009B1 — CLOSED / OWNER APPROVED:** per-Session Application/Completion callback concurrency defaults and independent idle-retirement grace defaults for all three worker classes.
- **WORKER-009B2 — OPEN:** OnClose event, Application frame-count and outstanding Request-credit defaults.
- **WORKER-009B3 — OPEN:** retained Application and Completion payload-byte defaults.
- **WORKER-009C — OPEN:** explicit configuration hard maxima, valid ranges, cross-field constraints and final Functional-Option mapping.

The existing Q-008/Q-010 defaults remain binding and are not reopened. Candidate numbers in B1–B3 are not approved until the owner selects them.

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

### 3. WORKER-009B — numeric defaults (PROPOSAL ONLY)

The following matrix distinguishes approved WORKER-009B1 defaults from the still-proposed B2/B3 values. The formulas in §2 and all values in §3.1 are owner approved; B2/B3 remain recommendations only.

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

| Setting | Recommended default | Scope | Accounting and saturation |
|---|---:|---|---|
| OnClose event credits | 1,024 | Per Client/Server owner | Queued + running; on saturation permanently skip the new OnClose per WORKER-003 |
| Incoming Application frame count | 256 | Per Session | Reader reservation + waiting Request callback frames; if admission fails, close only that Session |
| Outgoing Application frame count | 256 | Per Session | Includes current Writer-owned frame until full Write/discard; fail fast with Queue Full |
| Aggregate queued Application frame count | 65,536 | Per Server | Combined incoming + outgoing queued frames; incoming overflow closes offender, outgoing overflow returns Queue Full |
| Outstanding Request credits | 128 | Per Session | Full async Request lifecycle through Completion callback return/cancellation |
| Aggregate outstanding Request credits | 32,768 | Per Client/Server owner | Same Request credits; no duplicate Completion event credit |

Rationale: count limits protect tiny/empty-frame metadata that byte budgets alone do not bound. The 1,024 OnClose event budget matches the currently approved 1,024 bound-Session default for a single close wave, while remaining finite across churn and permanently blocked callbacks. The 65,536 Server frame ceiling limits the aggregate population independently of the per-Session 256 ceilings. PERF-003 credits remain the sole Completion event-count mechanism.

#### 3.3 WORKER-009B3 — retained-payload byte defaults

| Setting | Recommended default | Scope | Accounting |
|---|---:|---|---|
| Retained Application bytes | 16 MiB | Per Session | Inbound Request payload from header reservation through callback return/cancellation, plus outbound private frame through full Write/discard |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner | All library-owned Application payloads across that owner's Sessions and reconnect generations |
| Retained Completion bytes | 16 MiB | Per Session | Matched Response payload from header reservation through Completion callback return/cancellation |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner | All library-owned Completion payloads for that owner |

Rationale:

- The 16 MiB Application Session ceiling provides headroom beyond the independent 4 MiB incoming and 4 MiB outgoing queue-byte limits for active callbacks and in-progress frames. Those queue limits remain independent admission sub-limits; they are not added to the retained-byte ledger as separate physical buffers.
- The 512 MiB Application owner ceiling is twice the existing 256 MiB Server queued-Application-byte default, leaving bounded headroom for active/in-progress Application payloads. It is not an RSS limit.
- The earlier 4 MiB Completion-per-Session draft is superseded by this 16 MiB recommendation. Against the source-verified 1 MiB total-frame maximum, 4 MiB would permit only four maximum-size Response payloads for one Session. 16 MiB is a less restrictive bounded burst allowance; the 256 MiB owner ceiling still caps aggregate Completion payload retention.
- These values are admission ceilings, not preallocations. User-retained inbound payload after callback return remains outside the library-owned retained-byte ledger.

The pinned Pinex C++ 1.5.0 source confirms a 1 MiB total-frame maximum including the 10-byte header. Cross-language confirmation remains pending; WORKER-009B does not set this protocol limit.

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
- Completion-byte exhaustion must never silently discard a matching Response. Its exact terminal policy remains open and must be decided before Architecture Freeze.

### 6. Owner decision sequence

**Next: WORKER-009B2.** Review the candidate defaults for OnClose event credits, incoming/outgoing Application frame counts, Server aggregate queued Application frame count, and per-Session/owner outstanding Request credits. WORKER-009B3 retained-payload byte defaults follow; WORKER-009C establishes hard maxima and cross-field validation.

**Guardrail:** design/documentation only. No Go/C++ source, tests, benchmarks, builds, dependency changes, CI, deployment or operational changes are authorized.

---

## 19. Final authority reconciliation — 2026-11-17

API-001b policy is fully CLOSED: Forced Close A, N-ORD2, DONE-A and C-ORD-B. WORKER-001 through WORKER-009C, including numeric defaults, hard maxima, valid ranges, worker/resource ownership and all ledgers, are CLOSED / OWNER APPROVED. The terminal-cause accessor is Session.Err(); Session.Close() initiates Forced Close and returns without waiting for Done. Exact public API signatures and callback registration are finalized in API-001 §21. Stop aggregation and all timeout/reconnect decisions are finalized in API-002 §10. Historical "Still open" lists in this file are superseded by these decisions and DECISION_LOG.md. Implementation remains NOT AUTHORIZED.
