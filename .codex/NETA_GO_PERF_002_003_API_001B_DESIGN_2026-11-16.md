# neta-go V1 — PERF-002 / PERF-003 / API-001b Design Review

> **LATEST OWNER REVISION (2026-11-16):** Q-003-R1 supersedes earlier assumptions of strictly serial application callbacks in this document. Configurable parallel Application handlers now **owner-approved**; concurrency=1 is serial. The earlier forced-close alternatives presuppose one serialized user dispatcher and must be **reassessed** under independent internal engine/lifecycle notification lanes. API-001b is OPEN. See [Decision Log](DECISION_LOG.md).


**Date:** 2026-11-16
**Repository:** `majiddarvishan/neta-go`, branch `main`
**Baseline HEAD reviewed:** `0fe42d3f0ac634a5a1c9617d1e800b68a00fdf9f`
**Status:** PERF-002 **CLOSED / OWNER-APPROVED A**; PERF-003 **COUNT-BOUND ADMISSION APPROVED A (NUMBERS/DETAILS OPEN)**; API-001b **OPEN / NOT APPROVED**
**Implementation:** NOT AUTHORIZED. No Go source, module, tests, benchmarks, builds, network operations or deployments.

## 1. Authority and unchanged prior approvals

This document is a nonbinding follow-up to [DECISION_LOG.md](DECISION_LOG.md), [NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md) and [NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md).

Do **not** reopen: API-004 A (typed Functional Options, validated immutable defaults); API-001a A (nonblocking/callback-first request submission); Q-003 (same-Session callback non-overlap and order); Q-008 (outgoing 4 MiB/Session configurable, fail-fast Queue Full); Q-009 B (incoming queue overflow closes only offending Session); Q-010 A (incoming 4 MiB/Session; Server queued Application 256 MiB; remaining resource defaults configurable); Q-002/Q-006/Q-007 (idle probes); native opaque `[]byte` Pinex wire compatibility.

The approved maximum queued bytes are **not** preallocated, do **not** define the maximum wire PDU and do **not** bound process RSS.

## 2. PERF-002 — byte ownership and copy-cost options (OWNER APPROVED A)

| Choice | Send ownership | Receive ownership | Typical user-space payload copies | Safety / cost |
|---|---|---|---|---|
| **A — Recommended** safe send snapshot + owned direct receive | On successful enqueue, library owns a distinct snapshot; caller may reuse or mutate its input after the call returns | Library reads payload **directly into a fresh, user-retainable owned buffer**, transfers it to callback; never reuses that buffer | Send **one** payload copy into queued frame; receive **zero extra** payload copies after reading into its destination | Predictable semantics and safe async use; allocation/GC for retained inbound buffers and send frame snapshots |
| **B — Borrowed** callback lifetime | Caller must keep payload unchanged until explicit write/release acknowledgement; a return from async enqueue is insufficient | Borrowed pooled payload only valid until callback returns; retaining requires application copy | Potentially zero library-level send snapshot and receive copy, but kernel copies still occur | Lower copy potential, much harder API, accidental mutation and use-after-recycle hazards; complicates error/close |
| **C — Ownership transfer** | Caller hands off exclusive ownership of the backing array; after successful handoff no reads/writes/aliases permitted | Similar to A, or explicitly transferred receive ownership | Potentially zero user-space send payload copies with split header/payload writes | Go slices do not enforce unique ownership; must define failure/rollback and aliasing rules; not a V1 default recommendation |

**Owner decision:** **A APPROVED** for V1 (2026-11-16). This does *not* claim end-to-end zero-copy. OS TCP/network copies, framing and allocator effects are outside this count. Direct receive can eliminate the **second** receive-side application copy; it does not eliminate the network-to-user-memory copy.

### 2.1 Proposed outbound hot path under A

1. Validate Bound state, bind-type permission, wire frame length and `10 + len(payload)` with overflow checks.
2. Atomically reserve Sequence (when expecting a response), completion capacity, local outgoing bytes and Server aggregate queued bytes **before** allocating or copying. On failure, promptly return the appropriate typed admission error, with no orphan tracker/timer and no queued frame. Avoid lock-order deadlocks under concurrent producers.
3. Allocate one contiguous private frame buffer of exact needed length (or measured safe internal size-class buffer). Encode the **10-byte header** into that buffer, then copy caller payload **once** directly to the frame body. Publish the fully formed immutable-to-library frame to its Session FIFO; never copy again between queue and single Writer.
4. Ownership changes on **successful return** from the enqueue method. The caller must not concurrently mutate input while the method is taking its snapshot. After return, caller may modify or release it immediately. Unsuccessful admission does not take ownership.
5. Keep writer-active frame memory separately bounded; **conservative proposal**: retain outgoing quota charge until entire frame Write succeeds or fails, so a stalled writer cannot hide in-flight bytes outside the cap. Document exact queue-vs-writer accounting before implementation.
6. A partial TCP Write or Close may leave uncertain remote delivery; never promise transactional delivery or automatic resend.

For SendResponse and no-wait sends, use the same safe-copy admission rule but **do not** reserve a response-tracking slot. Response reuses the incoming request Sequence.

### 2.2 Proposed inbound path under A

1. Read/validate the fixed header first, including wire maximum, command/status/length and Session state; reject invalid/oversized frames **before** allocating a declared payload.
2. Allocate one appropriately sized payload destination (or an owned whole-frame buffer) and read the body directly into it. Avoid an intermediate reusable body buffer followed by another payload copy.
3. Count waiting application request/response events against Q-009/Q-010 byte budgets. Add distinct bounded accounting for one partly received frame and the one active callback frame; do not claim the 4 MiB *waiting* queue alone covers these. Admission/checks must happen before an allocation that could exceed the effective budget. A slow partial frame also needs a bounded read/deadline policy.
4. Transfer the owned payload buffer to the serialized callback. The callback may retain it after returning; neta-go does not mutate, recycle, or pool user-visible owned payload memory after delivery. Application retention is outside the library queue-RSS guarantee.
5. For a response, the buffer may become part of one terminal completion event without an additional copy. The original wire Sequence and header status remain intact.

Any direct receive ownership granted to user code must not simultaneously be recycled in an internal pool. `[]byte` is mutable, so immutability can only be documented by convention; user and library must not access the same mutable backing array concurrently.

### 2.3 Pool and zero-copy policy — proposal, not approval

- The **first** implementation should be copy-count-auditable. Candidate optimization: pool only **private outbound frame buffers** after the Writer and all internal owners have finished, potentially with measured size classes and a maximum pooled size.
- Do **not** pool owned inbound callback payload arrays after handoff to user code. Do **not** pool a frame while it is queued, partially written, or accessible to an async callback.
- `sync.Pool` is opportunistic, not a memory limit or a guarantee of reuse; the runtime can discard entries. Pool hit rate, retained heap, p99 and GC must be measured before selecting a pooling implementation. Avoid huge pooled slabs and eager 4 MiB/session allocation.
- Scatter/gather (`net.Buffers` / `writev` where supported) is a possible future optimization for a **separately approved explicit borrowed/transfer API**. It does not make mutable caller memory safe for an asynchronous writer and is not end-to-end zero-copy.
- No `unsafe`, permanent pinning, implicit payload transfer, or unbounded pool as a default V1 shortcut.

**External technical references:** Go memory model https://go.dev/ref/mem and `sync.Pool` https://pkg.go.dev/sync#Pool .

## 3. PERF-003 — bounded pending and completion delivery (OWNER APPROVED OPTION A MODEL; DETAILS OPEN)

The outgoing FIFO byte limit does **not** cap outstanding requests: a sent frame may leave the FIFO while waiting for a response. A stalled callback may also accumulate tiny/empty completion events without consuming material payload bytes. The design therefore requires **count-based** limits in addition to Q-008/009/010 byte caps.

### 3.1 Owner-approved bounded model — exact capacities and lifecycle details still open

- Use **one shared completion-credit limit per Session**, covering all accepted requests whose callback has not completed (waiting response, terminal result queued, or currently executing). Reserve one credit **before** accepting each callback-based SendRequest; release after the callback returns, or after explicit terminal cancellation on forced teardown. Do not create one goroutine per Request.
- A separate **Server aggregate completion-credit ceiling** prevents multiplying the per-Session maximum by up to 1024 bound Sessions. Client standalone sessions use their local ceiling.
- **Illustrative, NOT APPROVED defaults:** `MaxOutstandingCompletionSlots=1024` per Session; `MaxAggregateCompletionSlots=65536` per Server. Both configurable typed Functional Options if accepted. Capacity is lazily accounted, not preallocated. Final numbers require workload discussion/measurement.
- On an exhausted completion credit, reject the **new** callback-based request immediately with a distinct typed capacity error (proposed `ErrCompletionCapacity`), do not enqueue/transmit, and leave Session connected. Q-008 `Queue Full` for outgoing **byte** exhaustion remains unchanged. SendNoWait / SendResponse do not use a pending-response completion slot.
- The existing **4 MiB/Session incoming callback backlog** and Server **256 MiB aggregate application-frame queue** continue to bound waiting response/request payload bytes. Add a configurable **event-count cap** to incoming callback backlog to prevent unbounded queues of empty frames; preserve the approved Q-009 closure policy if an incoming application event cannot fit. Do not silently substitute a new overflow behavior.
- Count the **active** callback payload and one in-progress read/write frame separately, with bounded Session lifecycle ownership; avoid double-charge or charge gaps when moving between FIFO, active Writer, callback queue and callback.
- Completion metadata (Sequence/status/cause) and payload reservation must be charged exactly once in their respective capacities; response timeout/close/cancellation race has a single settlement winner. A response that misses its pending entry is late/unknown and cannot resurrect a settled request.
- A request-deadline manager should use bounded per-Session scheduling (heap/wheel design to be selected by measurement), **not** an unbounded goroutine+timer loop for each accepted request. Prefer a single callback dispatcher per Session.
- **Open timer choice:** proposed end-to-end request deadline begins on **local admission**, not only on full socket Write, so a stalled outbound queue cannot leave a request pending indefinitely. If expired before the writer starts, cancel the unsent frame when safe; if writing began, delivery is uncertain and no automatic replay occurs. Exact Go default timeout remains API-002 OPEN.
- Unconditionally preserve the separate Control headroom and single Writer, including Bind, Enquire Link and Unbind deadline/order.

### 3.2 Performance risks to validate after implementation authorization (not measured here)

Compare allocations/accepted request, one vs many concurrent producers, callback contention, completion backlog under sustained responses, p99 request RTT, queue-full reaction time, 1/8/32/128/512 Sessions, GC/RSS, socket churn and blocked callbacks. Include empty-payload events, rejected sends and sequence wraparound. Do not set unmeasured numeric SLA targets.

## 4. API-001b — serialization, termination, Close and Done (OPEN)

### 4.1 Semantics that must be reconciled

Approved Q-003 requires **non-overlapping callbacks within a Session**. If user callback A blocks forever, no subsequent callback B, including `OnClose`, can execute without violating that rule. Go cannot forcibly terminate arbitrary user code.

**Recommended mechanical structure (not an approved guarantee):**

- One per-Session ordered user-event dispatcher; other Sessions progress independently; no transport/global lock while user callbacks run. Incoming application callbacks retain receive order. Timeout and close events join a deterministic serialized event stream; exact tie-break order for concurrent events remains to be specified.
- The Reader/Writer, timers, correlation table and session teardown must continue independent of a blocked user callback, until bounded queues or approved overload policy requires closing **that** Session.
- Terminal settlement is a **single internal state transition** chosen once among reply, deadline, error and Close. Callback execution may be delayed behind earlier callbacks; do not falsely call it delivered when only queued.
- `Done()` is proposed as transport/lifecycle completion notification that closes **promptly after socket, timers, pending registry and quota accounting are released**, not when all user callbacks return. `Stop`/forced `Close` must not wait without bound on arbitrary user code.
- `OnClose` is invoked **at most once**, on the same per-Session dispatcher and only after earlier running callback returns. It cannot be guaranteed to run promptly or at all while user code is permanently blocked.
- Normal operation can deliver each admitted request's completion callback once when callback dispatcher makes progress. When callbacks cannot progress during forced close, the owner must choose between stronger deferred callback retention and prompt release of queued callback state.

### 4.2 Explicit unresolved hard-close choice

**API-001b Option A — recommended for resource isolation:** On forced Close, settle pending requests exactly once internally, close `Done` promptly, cancel any not-yet-started queued user callbacks/completions and release their buffers/credits. A callback already executing is not interruptible. `OnClose` is **at-most-once / best-effort** if the dispatcher can progress. A previously accepted request may therefore **not** receive its user completion callback on forced Close; this is a **material exception** to the normal callback-exactly-once expectation and **needs explicit owner approval** before use. Offer deterministic terminal cause via lifecycle state/`Done` plus optional session close-cause accessor.

**API-001b Option B — deferred stronger notification:** Preserve a finite terminal completion queue for admitted requests after forced transport Close; allow serialized worker to drain it and call `OnClose` once the current user callback returns. This improves eventual callback delivery **if callbacks eventually return**, but closed Sessions can retain queues/workers indefinitely on application stalls; across repeated reconnect/churn this can accumulate. `Done`/transport cleanup still cannot await user callbacks. Not a strict guarantee against a permanently blocked callback.

**Tradeoff:** exactly-once *logical settlement* is achievable; unconditional exactly-once *user callback invocation under arbitrary permanently blocked user code* is incompatible with strictly serialized callbacks and bounded prompt teardown. Do not silently label either alternative approved.

### 4.3 Callback API shape proposal

- Prefer **one per-request completion function passed on accepted SendRequest** to simplify ownership and avoid a competing global response completion handler. Local admission failure returns synchronously and does **not** invoke this callback. Callback completion carries `Sequence`, wire `Status` and opaque owned response payload on success, or a typed terminal error.
- One inbound `OnRequest` callback per Session; application explicitly sends a `stream_resp` with original Sequence and desired Header Status.
- Distinct lifecycle notification `OnClose` and separately readable `Done`; neither is an application Response callback. No implicit auto-reply, automatic retry or dedup.
- Exact exported identifiers, callback args, timestamp and state accessors are **OPEN**; these are conceptual API shapes, not Go source.

## 5. Interface and accounting invariant checklist

1. After successful SendRequest admission, the outgoing wire payload cannot be changed by a subsequent caller mutation (under PERF-002 A); unsuccessful admission produces no transport frame or pending state.
2. At most one terminal logical settlement per admitted request; no sequence reuse while pending, no duplicate timeout/response completion.
3. Every internal allocation has one owner and bounded lifetime except explicitly user-retained callback payload and user-code stalls; do not assert process RSS equals application-frame queue budget.
4. Outgoing byte reservation: full 10-byte header + payload, until full Writer completion under the conservative proposal. Incoming queued bytes: frame-sized application payload accounting consistent with approved Q-010 semantics; separate bounded count for tiny events.
5. A blocked callback never delays independent Session transport or lifecycle cleanup; forced Close cannot invent a simultaneous callback.
6. No extra public payload wrapper, mandatory Protobuf, C++-style event loop or business-layer machinery.
7. No numeric throughput/latency target is approved; benchmark methodology remains a plan only.

## 6. Owner decision sequence (PERF-002 CLOSED; PERF-003 MODEL APPROVED, OTHER DETAILS OPEN)

1. **PERF-002 — CLOSED / APPROVED A:** send snapshot once, receive directly into owned retainable buffer. Borrowed callback-lifetime B and transfer C not selected as V1 public contracts.
2. **PERF-003 — MODEL APPROVED A:** per-Session and Server aggregate count-bounded pending/completion work, configurable, new Request rejected without Session close on saturation. **Still OPEN:** numerical defaults (1024 / 65536 illustrative only), incoming event-count cap, forced-close credit accounting and deadline origin. Incoming separate event-count cap was not explicitly in the owner's final Option A wording and remains a proposal.
3. **API-001b:** decide **forced-close Option A** prompt cancellation/best-effort callback vs **Option B** deferred bounded callback delivery; choose exact callback registration shape.
4. **API-002:** resolve dial/bind/reconnect/request/Stop defaults and terminal errors; do not silently promote all observed C++ timings to Go approvals.
5. **API-003:** decide Go module path. Library `neta-go` name is already approved; proposed import path `github.com/majiddarvishan/neta-go` and Go package identifier `neta` are still **OPEN**. No `go.mod` yet.
6. **GO-PINEX-001:** pin exact Pinex C++ `develop` HEAD and golden frames **before** cross-language implementation/tests.

## 7. Source references

- Repository baseline: https://github.com/majiddarvishan/neta-go/commit/0fe42d3f0ac634a5a1c9617d1e800b68a00fdf9f
- Go memory model: https://go.dev/ref/mem
- Go `sync.Pool` docs: https://pkg.go.dev/sync#Pool
- Existing .codex contract references linked in section 1.

**PERF-002 A and PERF-003 A count-bounded admission MODEL are owner-approved; all PERF-003 numeric settings, incoming count cap, API-001b, timer and pooling choices remain OPEN. Source code and test execution remain prohibited.**


## 8. Owner approval: PERF-002 A (2026-11-16)

The owner explicitly selected **Option A**. This makes the fundamental **public payload lifetime** contract binding, without automatically approving all private implementation details in §§2.1–2.3. The copy should be made during accepted synchronous admission and be complete before successful return. An unsuccessful admission must not retain the input or publish a frame. A caller must not mutate its slice *concurrently with the snapshot operation*, but may reuse it after successful return. The owned received buffer may outlive callback return and must not be returned to an internal pool; try a direct-read path to avoid redundant receive-side copies. Do not label this end-to-end zero-copy. Full authoritative decision text: [DECISION_LOG.md](DECISION_LOG.md).

### 8.1 PERF-003 continued engineering analysis — OPEN (not approved)

**Why incoming event count also matters:** Q-009/010 impose byte budgets, but zero-length application Stream frames still have header and callback metadata. Counting **full serialized 10-byte header + body** where queue budgets specify full frame size helps, but a separate event count cap is recommended to protect handler queue metadata and control churn. The proposed event-count overflow reaction remains Session-local Close consistent with Q-009; the new numerical threshold is OPEN.

**Proposed credit lifetime:** a successful callback-based SendRequest holds one counted credit from admission through response/timeout/close settlement and, in normal running, until its serialized completion callback returns. This keeps response-waiter/queued-completion growth bounded. A terminal response reuses its reserved credit instead of attempting a fresh credit allocation, avoiding deadlock under full capacity. Internal once-only state transition can precede delayed user callback execution. The credit cannot be released at wire-write completion because response tracking and callback delivery remain outstanding.

**Proposed credit overflow:** reject new callback-based SendRequest synchronously with a distinguishable completion-capacity error and **do not enqueue or transmit**. This is separate from byte-based Queue Full; never alter the approved Q-008 error for an outgoing byte overrun. SendResponse and no-wait operations consume outgoing-byte quota but not response-tracking credits.

**Proposed quota topology:** per-Session count limit for Client/Server Session, and separate Server-aggregate count limit for its admitted Session requests. No max-size preallocation; atomic admission with coherent rollback across Sequence/pending, byte quota and count quota. Avoid single global mutex around normal Session operations. Configurability uses approved typed Functional Options if the added knobs are approved.

**Deadline choice still open:** start at local admission (recommended to bound time spent queued) vs start after complete TCP Write (better excludes queue delay but can defer forever behind blocked writer). No deadline numeric default has been approved. If an expiry occurs while frame is waiting, ensure cancellable queue ownership; if Write began, remote delivery status is ambiguous.

**Blocked callback/cancellation caution:** internal state can settle and transport can close promptly, but forced-close treatment of queued callbacks and when credit charges are released must follow **API-001b owner choice**, not an implicit exactly-once user callback delivery claim.

**Illustrative only:** per-Session 1024 and Server-aggregate 65536 completion credits are provisional discussion figures and **not approved defaults**. Any target should be calibrated against expected inflight ≈ offered requests/s × observed response latency, and validated with memory/GC/tail-latency tests after separate authorization.

## 9. API-001b next review — OPEN

Both forced-close Option A (cancel queued user callbacks for promptly bounded teardown) and Option B (defer finite completions until earlier callbacks return) remain owner choices. Neither can guarantee that `OnClose` runs promptly behind a permanently blocked callback while preserving Q-003 non-overlap. Recommend lifecycle `Done` report transport/accounting closure independent of arbitrary user callback return, **subject to owner approval**. Response/timeout/close can settle internally once; physical user callback invocation needs explicit lifecycle guarantees. Detailed exported method names and timing remain OPEN.

**No code, tests, benchmarks or operational actions were performed.**


## 10. PERF-003 owner approval and API-001b bounded terminal sequence review (2026-11-16)

**Owner confirmed:** **PERF-003 Option A**, i.e. bounded count admission for pending and completion work per Session AND across Server; all ceilings configurable; reject NEW SendRequest promptly on exhaustion without closing otherwise healthy Session. This approves the policy, **not** the previously illustrative `1024` / `65536` defaults. See [Decision Log](DECISION_LOG.md). The incoming callback **event-count** cap remains a technical proposal, not silently included in owner approval.

### 10.1 Proposed reservation and failure order (not new owner choices)

- Check Session State/Bind capability and frame size before expensive payload copy. Before making any request visible to Writer, reserve one Session completion credit + one Server aggregate completion credit (if Server-owned), outgoing Session byte quota + Server queued-byte quota (if applicable), and unique nonzero Sequence/Pending registry entry. Must rollback every reservation atomically/coherently on any rejection or construction failure. No user callback is invoked for an unadmitted request.
- After successful admission, reserve credit **through response wait, internal settlement, queued terminal callback and executing completion callback**; in ordinary operation, release only after completion callback returns. The terminal Response reuses the Request's completion credit: do not require a *second* completion reservation on reply, which could deadlock a full Session.
- Response vs timeout vs close selects a single logical terminal outcome. Internal settlement may occur while another user callback is running; dispatch still uses Q-003 per-Session serialized order. A late response cannot create a second completion. No global Server mutex may be held while invoking user code.
- On count quota exhaustion, reject promptly, keep Session alive, no write/sequence leak; return a **distinct typed capacity cause** (exported name OPEN). On byte quota exhaustion, preserve approved Q-008 `Queue Full` policy. Application `SendResponse` and no-wait sends consume outgoing byte quota but **do not** consume Request/Response Completion credits.
- All quota ceilings remain lazy, finite and separately configured through API-004 Functional Options; no eager per-Session table/queue allocation proportional to max capacity. Performance evaluation (when authorized) must cover contention and memory for high Session counts.

### 10.2 Unresolved after model approval

- **Capacity numbers:** 1024 Session / 65536 Server are unapproved examples only. No numeric default approved, no benchmark data.
- **Inbound event-count cap:** add independent count limit for tiny/empty inbound application events? Suggested yes, but not approved; must not confuse this with owner-approved outbound pending/completion cap. Existing inbound byte overflow => Session close remains binding.
- **API-001b forced-close:** a terminal result can be settled internally even when its Callback cannot execute behind a permanently blocked user callback. Decide cancellation vs deferred delivery; this also determines release timing for remaining completion credits and terminal event metadata. Never claim user callback exactly-once if it can be deliberately dropped after forced Close.
- **API-002 timer:** request deadline starts on local successful admission (recommended, includes outgoing queue time) vs full TCP Write (alternate). Exact deadlines and Stop grace remain OPEN.

### 10.3 API-001b follow-up recommendation: separate lifecycle plane from callbacks (not approved)

Use an internally once-only terminal state and a `Done`-like transport closure notification that does **not** require user callbacks to complete. Maintain Q-003 strict non-overlap within one Session's user callbacks. A real application callback cannot be cancelled forcibly. Two alternatives still require owner choice:

| Item | API-001b A: Cancel unstarted callbacks at forced Close (recommended for bounded resources) | API-001b B: Defer bounded terminal callbacks after transport close |
|---|---|---|
| Socket/registry/pending cleanup | Prompt, independent of user callback | Prompt, independent of user callback |
| Running callback | Cannot be forcibly stopped | Cannot be forcibly stopped |
| Queued callbacks | Cancel without invoking, release queued library-owned memory/credits | Keep finite queue until prior callback returns, release after dispatch |
| `Done` | Prompt; means transport/internal work closed, NOT callback completion | Same |
| `OnClose` | At most once/best effort on serialized lane; may never run if blocked | At most once, can run after existing callback returns; never guaranteed if stuck forever |
| Normal reply/timeout before forced Close | Dispatch terminal completion once if dispatcher progresses | Same |
| Resource risk | Bounded teardown, callback not guaranteed after forced close | Closed worker/queued data may live indefinitely while user callback blocks |

**Suggested API clarity:** If Option A wins, explicitly document that success means Request **admitted**, not that its completion callback is guaranteed to run after forced transport Close. `Done` and a terminal cause/state accessor expose the transport outcome independently from notification callbacks. If the application requires guaranteed results, it must use its own durable state/reconciliation layer; TCP library does not provide delivery guarantees. If B wins, promises are still conditional on callbacks returning.

No code/test/benchmark run. `API-001b` and these detailed callback semantics remain OPEN.


## 11. Callback-concurrency owner revision and API-001b reframing (2026-11-16)

**Q-003-R1 OWNER APPROVED:** Bounded parallel **Application** handler dispatch per Session with configurable maximum concurrency; setting `1` restores serial application handling. All previously stated unconditional serial application callback requirements here are superseded. Handler dispatch accepts in TCP-received FIFO queue order, but handlers may complete out of order. The bounded waiting ingress queue and Q-009 overload/Session closure remain unchanged.

**Internal engine independence (binding intent):** no transport Reader, Writer, Control response, timeout/pending resolution, once-only internal completion settlement or Close/Done transport cleanup path may depend on arbitrary application handler completion. Never hold lifecycle or global locks through callback invocation. A stuck handler occupies one of the configured worker slots, not the engine.

**Proposed three-domain scheduler (NOT API-001b APPROVED):**
1. **Internal protocol engine, not user callbacks:** single Writer, receive parser, correlation, timers, state transitions, predictable terminal cleanup. Bounded, event-driven, independent of user handlers.
2. **Application handler lane:** concurrency-configurable Session-local dispatch with finite queue, no goroutine per inbound Frame, Q-009/Q-010 overflow reaction. Multiple SendResponse calls may complete out of order and still use each request's original Sequence/Status; outgoing serialized Writer preserves frame integrity.
3. **User notification lane(s):** bounded delivery of per-Request completion callbacks, OnClose and other user-supplied functions, **distinct from internal settlement**. Recommended separate from Application handler lane so an OnRequest stall does not necessarily starve notifications; user notification handler itself can stall, so finite worker/queue and forced-close cancellation/defer still require **owner choice**.

**API-001b competing alternatives STILL OPEN:** A — forced close cancels unstarted callback notifications/application work for bounded teardown; B — retain finite pending completion notifications for conditional future dispatch (may retain closed Session resources indefinitely if a user callback never returns). New parallelism means neither alternative should be treated as selected by the owner's **Q-003-R1 Option B**; these are separate decision IDs.

**PERF-003 unchanged:** Request completion slot counts already owner-approved at model level, numeric defaults remain unapproved. Completion callback stuck in the user notification lane must not prevent *internal* response settlement or transport close. The Request credit lifetime on forced close depends on A/B and remains OPEN.

**Further design question:** should OnClose/user-completion callback execution use a bounded independent lane, and what ordering/at-most-once guarantee is desired on forced Close? Recommend separate control-plane settlement and bounded user notification dispatcher, but do not silently implement it.


## 12. Historical API-001b-N1 snapshot — separate bounded notification lane (2026-11-16; superseded by §§14–17)

**APPROVED STRUCTURE:** `OnClose` and per-Request Completion user callbacks execute on a **bounded notification lane independent of the bounded parallel Application `OnRequest` dispatcher**. Neither user callback domain may block internal TCP Protocol/Control/Pending/Timeout/Close machinery. A slow application handler must not occupy the notification workers. Internal request result settlement remains once-only and proceeds irrespective of user callback execution.

**Important limit:** This is separation between the **Application** and **Notification** user callback classes, not approval of one goroutine per notification, an unlimited queue, or one guaranteed worker per event. A blocked notification callback can still stall its own notification worker, even though it cannot stall the internal engine. Use PERF-003's bounded completion-credit accounting and finite scheduling/memory budgets.

**Historical open-question snapshot at N1 approval time:** notification worker count/defaults, queue/memory limits, sublane organization, OnClose/Completion order, forced-close A/B and Done semantics were still open at that point. Subsequent owner approvals in §§14–17 resolve Forced Close A, N-ORD2, DONE-A and C-ORD-B; numeric capacity and saturation questions remain open.

**Authoritative source:** [DECISION_LOG.md](DECISION_LOG.md) § API-001b-N1. No implementation, tests or benchmarks authorized.

## 13. Historical API-001b forced-close, ordering and worker bounds review (superseded by §§14–17)

A separate **historical nonbinding** owner-choice review covered forced-close A vs B, delivery and `OnClose`/`Done` ordering proposals, active-worker caps across closed Session generations, the need for a separate bounded `OnClose` event budget, PERF-003 numeric/event-count choices, API-002 deadlines and API-003 module name: [NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md). **Historical status only:** Forced Close A, N-ORD2, DONE-A and C-ORD-B were subsequently owner-approved in §§14–17. Remaining numeric capacities and saturation policies remain open.

## 14. API-001b Forced Close Option A — OWNER APPROVED (2026-11-16)

The owner explicitly selected **API-001b Forced Close A**, after the historical analysis in §§4, 9–13. This supersedes the earlier "forced-close A/B OPEN" statements *only for the forced-close disposition*. The owner subsequently selected **N-ORD2** (see §15): Completion and OnClose use independent bounded sublanes and OnClose may overlap a running Completion callback. **Policy:** all not-yet-started Application callbacks and per-Request Completion notifications are cancelled on forced closure, even if the terminal result was already internally settled; their queued library-owned payload/metadata and completion credits release. Already-started user callbacks cannot be interrupted and retain active-worker capacity (including any running Completion credit) until return. Internal Session resource cleanup and once-only Request settlement never wait. `OnClose` is best-effort/at-most-once. At the time of this Forced Close approval, exact Done semantics and within-Completion ordering/dispatcher bounds were still open; DONE-A subsequently resolved Done semantics, while within-Completion ordering and worker bounds remain open.

Binding approval: [DECISION_LOG.md](DECISION_LOG.md) § `API-001b-FC-A`. Further nonbinding ordering/worker-capacity proposals: [NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md) §11. No code, tests, benchmarks or operational changes.

## 15. API-001b-NORD2 owner approval and Worker / Done proposal (2026-11-16)

**OWNER APPROVED:** Completion and OnClose execute in distinct, independently bounded Notification sublanes. OnClose may run concurrently with a Completion callback already executing for the same Session; it does not wait for that callback to return. N-ORD1 shared serial Completion+OnClose stream was not selected. Binding detail: [DECISION_LOG.md](DECISION_LOG.md) § API-001b-NORD2.

**Worker design proposal (not numeric approval):** three user-code domains remain distinct — Application, Completion, OnClose. Completion queue admission reuses PERF-003 Request completion credits; active Completion worker permits are separate and remain charged until callback return. OnClose has its own active worker reserve and an independent active+queued event budget (one event maximum per Session), never consumes Request completion credits, and is best-effort if its own capacity is exhausted. Retired Session callbacks continue consuming their owning Client/Server worker budget across reconnect. Prefer finite lazy worker pools and fair ready-Session scheduling; no goroutine per event. The earlier one-at-a-time FIFO Completion candidate was not approved and has since been superseded by owner-approved configurable C-ORD-B; see §17. FIFO start selection remains by per-Session terminal-settlement enqueue order; callback return order is not guaranteed for concurrency greater than one.

**Done-A OWNER APPROVED:** Session exposes a read-only Done() <-chan struct{} signal. It closes exactly once after internal teardown and immutable terminal-cause publication; it never joins Application, Completion or OnClose callbacks. An admitted OnClose event becomes eligible only after Done closes and may overlap an already-running Completion under N-ORD2. Running callbacks retain active-worker permits (and a running Completion retains its PERF-003 credit) until return. Terminal-cause accessor and Close return semantics remain **OPEN**.

See [Forced Close / Notification / Done / Worker Capacity review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md) §11.7. No implementation, tests, benchmarks or operations.

## 16. API-001b-DONE-A — Session Done semantics (OWNER APPROVED, 2026-11-16)

The owner selected Option A. The public Session signal is Done() <-chan struct{}, closed exactly once after internal admission stops, pending Request outcomes settle, on a Forced Close path, unstarted Application/Completion callbacks are cancelled under Forced Close A; transport and internal Reader/Writer/control loops stop, timers/registry/internal accounting are cleaned up, and terminal cause becomes immutable. **Done never waits for arbitrary user callbacks.** Only after Done closes may an admitted OnClose event become eligible; under N-ORD2 it may overlap a Completion callback already running for that Session. OnClose remains best-effort/at-most-once.

The terminal-cause accessor and Close() return timing remain open. Server/Client Stop aggregation remains API-002. Worker/event capacities and Completion within-class ordering/concurrency remain open; see [detailed review §§11.7–11.8](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md).

No code, tests, benchmarks or operational changes.

## 17. API-001b-CORD-B — Configurable Completion concurrency (OWNER APPROVED, 2026-11-16)

The owner selected C-ORD-B: a positive configurable per-Session Completion concurrency limit. 1 gives serial execution; values above 1 permit callbacks from the same Session to overlap. FIFO start selection follows the order in which the internal engine serializes terminal-result settlement/enqueue for that Session, not SendRequest admission order. Callback return order is not guaranteed when concurrency exceeds one, and shared mutable application state must be synchronized by the application.

The per-Session limit is intersected with a finite aggregate active Completion-worker ceiling. PERF-003 Request credits bound the complete admitted Completion lifecycle, while active-worker permits bound callbacks physically executing. On a Forced Close path, Forced Close A cancels callbacks that have not started; running callbacks retain their credit and worker permit until return. N-ORD2's independent OnClose lane and DONE-A's Done-before-OnClose eligibility remain unchanged.

**Still open:** numeric concurrency default and public option name/range; worker counts and aggregate scope; event/queue and payload-byte/frame-size limits; OnClose saturation; terminal-cause/Close return API; API-002 and API-003.

No implementation, tests, benchmarks or operational changes.


## 18. WORKER-001 — owner-scoped Worker capacity (OWNER APPROVED, 2026-11-16)

The owner approved Option A: each Client and each Server instance owns an independent callback-worker resource domain. Sessions belonging to one Server share its aggregate budgets; all reconnect generations of one Client share its budget. A callback started in a retired Session remains charged until return. Independent instances do not share a hidden process-global worker pool or active-worker ceiling.

This closes only the capacity **ownership boundary**. It does not approve numeric worker counts, physical pool layout, per-class aggregate cap configuration, event/queue limits, payload-byte limits or saturation policy.

See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). Next: WORKER-002, explicit per-class owner-wide active ceilings versus ceilings derived from Session limits. No implementation or performance validation authorized.

## 19. WORKER-002 — Independent per-class owner active ceilings (OWNER APPROVED, 2026-11-16)

The owner selected Option A: each Client and Server instance has three independently configurable aggregate active-worker ceilings for Application, Completion and OnClose. Per-Session concurrency is intersected with the corresponding owner/class remaining capacity. The three classes cannot borrow or consume one another's active permits.

The ceilings count physically executing callbacks; waiting queues/event populations remain separately bounded. WORKER-001 ownership applies across all Sessions and reconnect generations, including callbacks still running from retired Sessions. There is no mandatory process-global active-worker cap.

Numeric values, public option names, physical worker-pool layout and event/queue capacities remain open. Next: WORKER-003, OnClose event saturation.

See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). No implementation, tests or benchmarks authorized.
