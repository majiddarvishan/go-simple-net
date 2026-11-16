# neta-go V1 — Resource Admission and Incoming Backpressure Design

**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — Q-008 APPROVED / Q-009 OPEN  
**Branch:** `majiddarvishan/go-simple-net` / `main`  
**Implementation:** NOT AUTHORIZED; this file contains no runnable application code.

## 1. Source and approved boundary

This document follows approved decision Q-008 **Option A**. The compatibility target is Pinex C++ 1.5.0 on branch `develop`; see [protocol contract](PINEX_WIRE_PROTOCOL_2026-11-16.md), [task plan](PINEX_GO_PORT_TASKS_2026-11-16.md) and [decision log](DECISION_LOG.md).

### Q-008 — approved

- Outgoing Application Stream frames use a configurable **per-Session FIFO with a default limit of 4 MiB (4,194,304 bytes)**.
- Account full serialized frame bytes including the 10-byte Pinex header.
- New Send calls that exceed free queue capacity **fail immediately with Queue Full** (no blocking, silent drops, partial enqueue or orphaned pending requests).
- Application write bytes are emitted by one serialized writer per TCP connection. Concurrent Send calls must keep frame integrity.
- Control-PDU progress requires a **separate, bounded admission path/headroom**, not a general-purpose Priority Queue and not unlimited reserved memory.
- Existing Pinex-compatible Unbind ordering, response emission and 10-second protocol deadline still apply.
- This does **not** enlarge Pinex's wire-frame limit nor constitute an aggregate/server memory limit.

### Q-003/Q-004 approved constraints

- Session callbacks execute serially in receive order, but unrelated sessions can progress independently.
- Go public methods are goroutine safe.
- Lean V1 does not include a business Rate Limiter, Circuit Breaker, Deduplicator, Priority Queue or Prometheus exporter. Resource admission and bounded backpressure are still Core requirements.

## 2. Accounting model — proposed engineering rules

**Application outgoing queue, per Session:** count fully formed Frame bytes that have been admitted but have **not yet been fully written**. Document whether the current in-progress frame is included in the configured cap. **Recommendation:** count it until full write completion so the bound cannot be bypassed by an indefinitely blocked writer. Admission/reservation is atomic under concurrent Send.

**Application incoming callback backlog, per Session:** count bytes of decoded Application Stream frames waiting to be dispatched into serialized user callbacks; do not use an unbounded channel. One currently executing callback and the transport read buffer require separately accounted/limited memory; an inbound queue cap is not a complete bound on per-Session memory.

**Aggregate server budget:** enforce a separate limit on sum of queued incoming/outgoing Application frame bytes for Sessions belonging to one Server. Count a Frame in precisely one owner bucket at any moment and release charge on completion/cancellation/close; never charge incoming and outgoing simultaneously unless the application explicitly creates an outgoing response. Server-wide queue bytes do not include Go runtime stacks, TLS/kernel socket buffers (if any), reader buffers or application-retained messages. Therefore queue budget alone does **not** guarantee a strict process RSS cap.

**Session admission:** also bound pending pre-bind connections and bound sessions. Bound-session capacity should count the actual bound registry, not the historical number of accepted sockets. Duplicate System ID remains rejected, independent of resource admission. New sessions exceeding the bound-session limit must not evict existing ones.

**Control frames:** avoid starvation under saturated outgoing Application queue, yet all memory remains bounded. For `unbind_resp` preserve the outgoing write ordering documented by Pinex C++, rather than overtaking earlier writes. On stalled draining, the 10-second Unbind deadline enables closure. A control request behind unread TCP payload cannot be magically discovered ahead of earlier bytes; receive strategy must ensure finite buffering and a fail-safe deadline when progress is impossible.

## 3. Q-009 — Incoming overload policy (owner decision pending)

### Option A — pause/resume application dispatch with bounded memory (recommended)

- When a Session's incoming callback capacity becomes saturated, pause **new application callback delivery** for that Session and apply bounded TCP/read-side flow control until the existing callback backlog drains.
- Existing per-Session callbacks remain serialized; other Sessions continue operating independently.
- Keep protocol control handling responsive as far as possible **without unlimited buffering or violating TCP/frame ordering**. Only complete validated frame data may be dispatched or scanned; do not skip an incomplete application frame to search for a control PDU behind it.
- If the peer cannot make bounded forward progress, use a clearly specified per-Session stall/close policy; never allow the reader's retained bytes or pending callback count to grow indefinitely.
- When capacity becomes available, resume application dispatch in original receive order.

### Option B — close saturated Session instead

- Once an incoming Application frame cannot be admitted to its bounded callback queue, close that Session promptly with a resource-limit error.
- Simpler failure path, but a temporary slow callback causes a disconnection and could trigger reconnect churn.
- Even on close, perform a single terminal callback, free buffers and avoid other Sessions being affected.

**Scope of approval requested for Q-009:** choose A (pause/resume) or B (close on saturation); neither is authorized merely by approving Q-008.

## 4. Suggested numeric defaults for Q-009 — **not approved**

These are sizing illustrations to guide review, not verified production requirements:

| Config idea | Proposal | Purpose and caveat |
|---|---|---|
| `MaxQueuedIncomingBytes` | **4 MiB per Session** | Bounded waiting callback frames; exact choice depends on frame-size cap and callback behavior |
| `MaxAggregateQueuedFrameBytes` | **256 MiB per Server** | Cap combined queued incoming+outgoing application frame bytes; not a hard process-memory limit |
| `MaxBoundSessions` | **1024 per Server** | Bound registry/connection overhead; needs workload-specific sizing |
| `MaxPendingPreBindSessions` | **128 per listener** | Matches currently documented C++ Pinex baseline; separate from bound limit |
| `PreBindHandshakeTimeout` | **30s** | Existing C++ reference value; Go default not yet approved |
| Bounded control reserve | **size TBD** | Needs separate count/bytes policy; cannot be unlimited |
| Inbound stall deadline | **TBD** | Defines when stalled callback/peer forces close under Option A |

**Important:** The numbers 4 MiB inbound, 256 MiB global and 1024 bound sessions are **proposals only**. The owner has so far approved **only 4 MiB outgoing per Session under Q-008**. The queue cap and session cap are independently configurable deployment tradeoffs. Avoid pre-allocating full configured queue capacity for every accepted Session.

## 5. Resource and lifecycle invariants

1. Atomic admission happens before pending request becomes visible to the writer. On Queue Full, no queued frame, leaked pending request or response timer remains.
2. If a decoded input Frame is blocked by callback backlog, retain only bounded data, use TCP backpressure and do not dispatch application frames out of order.
3. A caller's slow callback never holds the global Session Registry lock or blocks unrelated Session readers/writers.
4. Go shutdown and `unbind_req` must remain progress-capable under backpressure, respecting serialized full-frame writes and Pinex drain/deadline semantics.
5. Enquire Link defaults approved separately: idle-only 30s, response deadline 10s. A Session stalled behind an un-drainable TCP buffer must eventually close under a bounded policy; do not infer that an application-layer heartbeat can bypass TCP's ordered byte stream.
6. Dropped/late/unexpected response PDUs must not accidentally complete pending request state more than once.
7. A rejected/closed Session releases all queue reservations and timers, cannot continue writing old frames, and generates no duplicate terminal notifications.
8. Backpressure is not a security or authentication replacement. Bind validation and duplicate-ID policy remain unchanged.

## 6. Tests to plan (NOT executed)

- Stress simultaneous `SendRequest`/`SendResponse` calls around 4 MiB boundary: no over-admission, no Frame interleaving, Queue Full is prompt.
- Wire interoperability Go Client ↔ C++ Server and C++ Client ↔ Go Server under send and receive pressure.
- Callback slowed indefinitely: bounded incoming bytes, unaffected other Sessions, deterministic recovery or close.
- Global pressure across many Sessions: aggregate queue ceiling respected and admission effects isolated.
- TCP fragmentation and partial frame reads: no blind skip of application data; control PDU order preserved.
- Incoming Enquire Link and Unbind when application delivery is paused: reply when reachable without violating byte order/limits; bounded exit otherwise.
- Session teardown while a writer is stalled: all reserved bytes and pending sequences released exactly once.
- Race detector and bounded-churn/soak suite for Go Session state and callbacks.

## 7. Next design actions

- **Q-009 OPEN:** owner choice for inbound overload handling: Option A pause/resume vs Option B immediate close.
- **Q-010 OPEN:** proposed numerical ingress/global/session admission defaults. These cannot be derived from Q-008 and need workload sizing/approval.
- **API-004 OPEN:** Go configuration must distinguish **unset** from explicit `EnquireLinkIdleInterval=0`. A plain zero-valued `time.Duration` field cannot simultaneously implement approved **default 30s** and explicit **0 disables**. Prefer a constructor+functional-option override or pointer/optional type; choose before implementation.
- Existing **API-001..003** also remain open: exported types, callbacks, capacity config validation, module/package naming.
- Pin the exact C++ develop HEAD before any cross-language test execution.

**No implementation permission is implied by this document.**
