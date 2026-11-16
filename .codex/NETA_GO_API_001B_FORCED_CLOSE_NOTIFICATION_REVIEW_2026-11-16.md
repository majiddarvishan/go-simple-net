# neta-go V1 — API-001b Forced Close, Notification Delivery and Worker Bound Review

**Date:** 2026-11-16  
**Status:** DESIGN PROPOSAL ONLY — **API-001b remains OPEN; owner has NOT selected forced-close A or B**  
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

## 3. Forced Close policy: material owner decision (OPEN)

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
