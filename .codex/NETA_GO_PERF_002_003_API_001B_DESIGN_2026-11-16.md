# neta-go V1 — PERF-002 / PERF-003 / API-001b Design Review

**Date:** 2026-11-16
**Repository:** `majiddarvishan/neta-go`, branch `main`
**Baseline HEAD reviewed:** `0fe42d3f0ac634a5a1c9617d1e800b68a00fdf9f`
**Status:** DESIGN PROPOSAL ONLY — PERF-002, PERF-003 and API-001b remain **OPEN / NOT OWNER-APPROVED**
**Implementation:** NOT AUTHORIZED. No Go source, module, tests, benchmarks, builds, network operations or deployments.

## 1. Authority and unchanged prior approvals

This document is a nonbinding follow-up to [DECISION_LOG.md](DECISION_LOG.md), [NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md) and [NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md).

Do **not** reopen: API-004 A (typed Functional Options, validated immutable defaults); API-001a A (nonblocking/callback-first request submission); Q-003 (same-Session callback non-overlap and order); Q-008 (outgoing 4 MiB/Session configurable, fail-fast Queue Full); Q-009 B (incoming queue overflow closes only offending Session); Q-010 A (incoming 4 MiB/Session; Server queued Application 256 MiB; remaining resource defaults configurable); Q-002/Q-006/Q-007 (idle probes); native opaque `[]byte` Pinex wire compatibility.

The approved maximum queued bytes are **not** preallocated, do **not** define the maximum wire PDU and do **not** bound process RSS.

## 2. PERF-002 — byte ownership and copy-cost options (OPEN)

| Choice | Send ownership | Receive ownership | Typical user-space payload copies | Safety / cost |
|---|---|---|---|---|
| **A — Recommended** safe send snapshot + owned direct receive | On successful enqueue, library owns a distinct snapshot; caller may reuse or mutate its input after the call returns | Library reads payload **directly into a fresh, user-retainable owned buffer**, transfers it to callback; never reuses that buffer | Send **one** payload copy into queued frame; receive **zero extra** payload copies after reading into its destination | Predictable semantics and safe async use; allocation/GC for retained inbound buffers and send frame snapshots |
| **B — Borrowed** callback lifetime | Caller must keep payload unchanged until explicit write/release acknowledgement; a return from async enqueue is insufficient | Borrowed pooled payload only valid until callback returns; retaining requires application copy | Potentially zero library-level send snapshot and receive copy, but kernel copies still occur | Lower copy potential, much harder API, accidental mutation and use-after-recycle hazards; complicates error/close |
| **C — Ownership transfer** | Caller hands off exclusive ownership of the backing array; after successful handoff no reads/writes/aliases permitted | Similar to A, or explicitly transferred receive ownership | Potentially zero user-space send payload copies with split header/payload writes | Go slices do not enforce unique ownership; must define failure/rollback and aliasing rules; not a V1 default recommendation |

**Recommendation awaiting owner:** **A** for V1. This does *not* claim end-to-end zero-copy. OS TCP/network copies, framing and allocator effects are outside this count. Direct receive can eliminate the **second** receive-side application copy; it does not eliminate the network-to-user-memory copy.

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

## 3. PERF-003 — bounded pending and completion delivery (OPEN)

The outgoing FIFO byte limit does **not** cap outstanding requests: a sent frame may leave the FIFO while waiting for a response. A stalled callback may also accumulate tiny/empty completion events without consuming material payload bytes. The design therefore requires **count-based** limits in addition to Q-008/009/010 byte caps.

### 3.1 Recommended bounded model — details subject to owner approval

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

## 6. Recommended owner decisions, in order (ALL OPEN)

1. **PERF-002:** choose **A**: send snapshot once, receive directly into owned retainable buffer (recommended), versus B borrowed callback-lifetime or C explicit ownership transfer.
2. **PERF-003:** approve the **shared per-Session completion-credit model + Server aggregate cap + incoming event-count cap**, then review separately proposed numeric defaults (1024 / 65536 are *illustrative only*) and deadline-start rule.
3. **API-001b:** decide **forced-close Option A** prompt cancellation/best-effort callback vs **Option B** deferred bounded callback delivery; choose exact callback registration shape.
4. **API-002:** resolve dial/bind/reconnect/request/Stop defaults and terminal errors; do not silently promote all observed C++ timings to Go approvals.
5. **API-003:** decide Go module path. Library `neta-go` name is already approved; proposed import path `github.com/majiddarvishan/neta-go` and Go package identifier `neta` are still **OPEN**. No `go.mod` yet.
6. **GO-PINEX-001:** pin exact Pinex C++ `develop` HEAD and golden frames **before** cross-language implementation/tests.

## 7. Source references

- Repository baseline: https://github.com/majiddarvishan/neta-go/commit/0fe42d3f0ac634a5a1c9617d1e800b68a00fdf9f
- Go memory model: https://go.dev/ref/mem
- Go `sync.Pool` docs: https://pkg.go.dev/sync#Pool
- Existing .codex contract references linked in section 1.

**All alternatives above are proposals, not approvals; source code and test execution remain prohibited.**
