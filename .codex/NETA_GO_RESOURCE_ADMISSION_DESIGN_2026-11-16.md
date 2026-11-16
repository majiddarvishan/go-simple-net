# neta-go V1 — Resource Admission and Incoming Backpressure Design

**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — Q-008 APPROVED / Q-009 OPTION B APPROVED / Q-010 OPEN  
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

## 3. Q-009 — Incoming overload policy (APPROVED OPTION B)

### Option B — close saturated Session (owner approved 2026-11-16)

- Incoming Application Stream callbacks for a Session run serially in receive order; the waiting callback backlog has a **bounded**, configurable capacity. Numeric capacity is Q-010, still OPEN.
- If the next decoded valid Application frame does **not fit** the remaining callback backlog capacity, **close only the responsible Session** promptly with a resource-limit/queue-overload reason. Reaching exactly the limit without a new non-fitting frame is not itself overflow.
- No silent drop, indefinite wait, unbounded queue, or Pause/Resume recovery on this overload path. Stop admitting Application work and release all session-owned queue reservations, timers and Pending Requests; Close must not be reported twice.
- Other Sessions remain independent; no global lock may be held during a user callback or blocking teardown.
- If Client Auto Reconnect is enabled, reconnect is allowed through the normal configured retry policy; Server never reconnects accepted peers.
- Control frames already decoded **before** the overflow are processed by the normal ordered state machine. After an overload-triggered socket close there is no obligation/possibility to process future frames on that closed Session.
- Go cannot forcibly stop a callback blocked in user code. Transport teardown must not block indefinitely waiting for it; the exact close-event/callback ordering/cancellation guarantee must be resolved separately under API-001/GO-PINEX-004.

### Option A — pause/resume (not selected)

The owner rejected the Pause/Resume overload policy for V1. Do not use it as the fallback when the incoming callback backlog is full.

**Approved scope:** the **per-Session** incoming-queue overflow reaction, not numeric capacities or the global-overload reaction. See the [Decision Log](DECISION_LOG.md).

## 4. Q-010 — suggested numeric defaults (NOT APPROVED)

These are sizing illustrations to guide review, not verified production requirements:

| Config idea | Proposal | Purpose and caveat |
|---|---|---|
| `MaxQueuedIncomingBytes` | **4 MiB per Session** | Bounded waiting callback frames; exact choice depends on frame-size cap and callback behavior |
| `MaxAggregateQueuedFrameBytes` | **256 MiB per Server** | Cap combined queued incoming+outgoing application frame bytes; not a hard process-memory limit |
| `MaxBoundSessions` | **1024 per Server** | Bound registry/connection overhead; needs workload-specific sizing |
| `MaxPendingPreBindSessions` | **128 per listener** | Matches currently documented C++ Pinex baseline; separate from bound limit |
| `PreBindHandshakeTimeout` | **30s** | Existing C++ reference value; Go default not yet approved |
| Bounded control reserve | **size TBD** | Needs separate count/bytes policy; cannot be unlimited |
| Inbound overload close | **Immediate when next Frame cannot fit** | Reaction approved under Q-009 B; callback teardown timing details remain under API-001 |

**Important:** The numbers 4 MiB inbound, 256 MiB global and 1024 bound Sessions are **proposals only** under Q-010. The owner approved **4 MiB outgoing per Session under Q-008** and **close-on-incoming-overflow under Q-009 B**, not these additional numerical limits. Capacity must be configurable; avoid eagerly allocating the maximum byte limit for every Session.

## 5. Resource and lifecycle invariants

1. Atomic admission happens before pending request becomes visible to the writer. On Queue Full, no queued frame, leaked pending request or response timer remains.
2. On **failed admission** of a decoded incoming Application frame to the Session's bounded callback backlog, **close that Session promptly** (Q-009 B), without silent drop or pause/resume recovery; preserve prior callback order and release resources.
3. A caller's slow callback never holds the global Session Registry lock or blocks unrelated Session readers/writers.
4. Go shutdown and `unbind_req` must remain progress-capable under backpressure, respecting serialized full-frame writes and Pinex drain/deadline semantics.
5. Enquire Link defaults approved separately: idle-only 30s, response deadline 10s. A Session stalled behind an un-drainable TCP buffer must eventually close under a bounded policy; do not infer that an application-layer heartbeat can bypass TCP's ordered byte stream.
6. Dropped/late/unexpected response PDUs must not accidentally complete pending request state more than once.
7. A rejected/closed Session releases all queue reservations and timers, cannot continue writing old frames, and generates no duplicate terminal notifications.
8. Backpressure is not a security or authentication replacement. Bind validation and duplicate-ID policy remain unchanged.

## 6. Tests to plan (NOT executed)

- Stress simultaneous `SendRequest`/`SendResponse` calls around 4 MiB boundary: no over-admission, no Frame interleaving, Queue Full is prompt.
- Wire interoperability Go Client ↔ C++ Server and C++ Client ↔ Go Server under send and receive pressure.
- Callback slowed indefinitely: bounded incoming bytes, **only the overflowing Session closes**, unaffected other Sessions, no double-close. A stuck user callback cannot be forcibly terminated; do not promise indefinite Stop waits.
- Global pressure across many Sessions: aggregate queue ceiling respected and admission effects isolated.
- TCP fragmentation and partial frame reads: no blind skip of application data; control PDU order preserved.
- Incoming Enquire Link/Unbind already decoded before overflow follow normal state rules; after overflow closes Session, do not skip incomplete frames or promise control processing through a closed transport.
- Session teardown while a writer is stalled: all reserved bytes and pending sequences released exactly once.
- Race detector and bounded-churn/soak suite for Go Session state and callbacks.

## 7. Next design actions

- **Q-009 CLOSED / APPROVED B:** close only Session whose incoming callback backlog cannot admit a new Application frame; other Sessions are unaffected.
- **Q-010 OPEN:** proposed numerical ingress/global/session admission defaults and what happens when **global** capacity cannot admit a frame. These cannot be derived from Q-008 or Q-009 and require independent approval.
- **API-004 OPEN:** Go configuration must distinguish **unset** from explicit `EnquireLinkIdleInterval=0`. A plain zero-valued `time.Duration` field cannot simultaneously implement approved **default 30s** and explicit **0 disables**. Prefer a constructor+functional-option override or pointer/optional type; choose before implementation.
- Existing **API-001..003** also remain open: exported types, callbacks, capacity config validation, module/package naming.
- Pin the exact C++ develop HEAD before any cross-language test execution.

**No implementation permission is implied by this document.**
