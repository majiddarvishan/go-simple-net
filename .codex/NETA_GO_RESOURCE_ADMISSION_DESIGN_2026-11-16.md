# neta-go V1 — Resource Admission and Incoming Backpressure Design

> **LATEST OWNER REVISION (2026-11-16):** Q-003-R1 replaces older unconditional serial application callbacks below with **configurable bounded same-Session parallel handlers** (concurrency=1 yields serial). Q-009/Q-010 queue capacities and overload outcomes remain approved. See [Decision Log](DECISION_LOG.md).


**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — Q-008 / Q-009 B / Q-010 A APPROVED; exact Go API design remains OPEN  
**Branch:** `majiddarvishan/neta-go` / `main`  
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

- Incoming Application Stream callbacks for a Session run serially in receive order; the waiting callback backlog has **4 MiB default configurable capacity per Session** (Q-010 A APPROVED).
- If the next decoded valid Application frame does **not fit** the remaining callback backlog capacity, **close only the responsible Session** promptly with a resource-limit/queue-overload reason. Reaching exactly the limit without a new non-fitting frame is not itself overflow.
- No silent drop, indefinite wait, unbounded queue, or Pause/Resume recovery on this overload path. Stop admitting Application work and release all session-owned queue reservations, timers and Pending Requests; Close must not be reported twice.
- Other Sessions remain independent; no global lock may be held during a user callback or blocking teardown.
- If Client Auto Reconnect is enabled, reconnect is allowed through the normal configured retry policy; Server never reconnects accepted peers.
- Control frames already decoded **before** the overflow are processed by the normal ordered state machine. After an overload-triggered socket close there is no obligation/possibility to process future frames on that closed Session.
- Go cannot forcibly stop a callback blocked in user code. Transport teardown must not block indefinitely waiting for it; the exact close-event/callback ordering/cancellation guarantee must be resolved separately under API-001/GO-PINEX-004.

### Option A — pause/resume (not selected)

The owner rejected the Pause/Resume overload policy for V1. Do not use it as the fallback when the incoming callback backlog is full.

**Approved scope:** Q-009 B defines the Session-local incoming overflow reaction. Q-010 A subsequently also approves the separate **global** overload reaction and all configurable numerical defaults. See [Decision Log](DECISION_LOG.md).

## 4. Q-010 — Approved configurable capacities and global overload policy

**Status:** CLOSED — APPROVED OPTION A, with all settings independently configurable (2026-11-16).

| Config | Approved default | Scope and result |
|---|---:|---|
| `MaxQueuedIncomingBytes` | **4 MiB = 4,194,304 bytes** | Application callback waiting backlog per Session; failed incoming admission closes only that Session (Q-009 B) |
| `MaxQueuedOutgoingBytes` | **4 MiB = 4,194,304 bytes** | Separate Application outgoing FIFO per Session (Q-008 A); failed outgoing admission returns Queue Full, keeps Session |
| `MaxAggregateQueuedFrameBytes` | **256 MiB = 268,435,456 bytes** | Combined admitted queued Application frames per Server; not a whole-process RSS cap |
| `MaxBoundSessions` | **1024** | Bound Sessions per Server; refuse new Bind at capacity without evicting earlier Sessions |
| `MaxPendingPreBindSessions` | **128** | Pending pre-bind connections per listener; close excess sockets before Session allocation |
| `PreBindHandshakeTimeout` | **30s** | Absolute timeout from TCP accept; partial frames do not extend it |
| `MaxQueuedControlBytes` | **4 KiB = 4,096 bytes** | Separate finite Control PDU headroom per Session; unrecoverable control congestion closes the affected Session |

**Every limit and deadline above must be independently configurable, not hardcoded.** These approved numbers define V1 defaults only; deployments may override them. Allocation must be lazy/charge-based rather than preallocating every Session's maximum. The 128/30s values match the reviewed Pinex C++ pre-bind baseline, but any numeric default must still be verified under realistic workloads.

### Approved global overload and admission behavior

1. If a **new outgoing Application Frame** cannot fit either its local outgoing FIFO (Q-008) or the Server's aggregate queued-Application budget, promptly return **Queue Full**, without sending the rejected Frame, leaking a pending sequence, or closing an otherwise healthy Session.
2. If a **new incoming Application Frame** cannot be admitted to its local callback backlog (Q-009) **or** the Server's aggregate queued-Application budget, promptly **close only that offending Session**, with an overload reason. Do not drop silently, bypass the callback ordering contract, pause/resume, or evict unrelated Sessions.
3. At `MaxBoundSessions`, reject the new valid Bind using the existing Pinex `rfail` response when feasible. Preserve the existing System ID registration and do not replace a valid active connection.
4. At `MaxPendingPreBindSessions`, refuse the excess socket before creating a full Session. Enforce the configured absolute pre-bind deadline.
5. Keep Control PDU backlog **bounded separately** by `MaxQueuedControlBytes`. If it cannot progress within its admission/Unbind deadline constraints, close the affected Session. Do not reorder previously queued Application Frames, interleave bytes or violate the Pinex 10s Unbind deadline.
6. Track queued incoming and outgoing Application frame charges in a Server-wide budget with race-free atomic reservations and exact release on write, dispatch ownership transfer, timeout or close. The charge for an in-progress frame and currently executing callbacks needs precise API/runtime accounting to avoid counting gaps; this detail remains for the next engineering contract.
7. Do not confuse the **256 MiB frame-queue ceiling** with a process RSS guarantee: reader buffers, callback-owned payloads, Control PDUs, stacks, sockets and other allocations still consume memory.

### Configurability design boundary (NOT YET APPROVED AS PUBLIC API)

- The exact Go API mechanism for default/override handling is still **API-004 OPEN**, not settled by the owner's approval of values. The design must make omitted config fields use defaults, while honoring explicit overrides.
- `EnquireLinkIdleInterval`: omitted => approved default **30s**; explicitly `0` => disable autonomous Enquire Link (Q-002/Q-006). A plain zero-valued `time.Duration` field cannot distinguish these cases without an additional representation.
- Safety bounds should not become unlimited accidentally when a setting is explicitly zero; validation rules for each resource field must be documented, with positive finite limits as the recommended policy.
- Apply configuration at component construction/start (recommended initial behavior). **Live hot-reconfiguration has not been requested or approved**; avoid silently implying runtime mutation.
- Preserve Pinex wire framing and maximum protocol frame size separately from queue capacities. A 4 MiB queue does not authorize a 4 MiB PDU on a 1 MiB-compatible peer.

### Unselected Option B

The owner chose Option A defaults, **not** a workload-specific replacement default set. Custom values are still supported through the approved configurability requirement.

### Planned acceptance tests (NOT EXECUTED)

- Omitted vs explicit valid override for **each** of the seven listed configuration settings; invalid numeric values must fail deterministically.
- Exactly-at-limit vs non-fitting frame tests for inbound, outgoing, global and Control byte caps.
- Multi-session simultaneous ingress/egress and Bind admission must not oversubscribe quotas; no premature eviction or orphan Pending Request.
- Overrun of local/global outgoing budget: Queue Full, Session stays bound. Local/global incoming overrun: only offending Session closes.
- Pre-bind 128/30s default test with custom overrides; Control under saturation remains bounded and respects Pinex Unbind ordering.
- No eager worst-case memory allocation, duplicate Close, or unbounded callbacks/goroutines.
- Go↔C++ Pinex real-socket interoperability across saturation conditions.

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
- **Q-010 CLOSED / APPROVED A:** all numeric ingress/global/session/pre-bind/control defaults are configurable and the global overload reactions are approved, as recorded above.
- **API-004 OPEN:** Go configuration must distinguish **unset** from explicit `EnquireLinkIdleInterval=0`. A plain zero-valued `time.Duration` field cannot simultaneously implement approved **default 30s** and explicit **0 disables**. Prefer Functional Options with complete defaults and immutable validated snapshot; optional/pointer Config remains an alternative. See [API-004 detailed configuration contract](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md); no choice has been approved.
- Existing **API-001..003** also remain open: exported types, callbacks, capacity config validation, module/package naming.
- Pin the exact C++ develop HEAD before any cross-language test execution.

**No implementation permission is implied by this document. Approved numeric defaults are not measured production sizing guarantees.**


## 8. Callback parallelism accounting addendum — Q-003-R1 (2026-11-16)

**Owner-approved:** same Session may execute up to a *configured* number of Application callbacks at once; `1` retains serial processing. Internal protocol/timeout/Close logic progresses independently of user handler return. The previous single-active-callback assumption in older sections is **superseded**.

**Queue semantics unchanged:** Q-009/Q-010's 4 MiB per Session incoming limit and 256 MiB Server aggregate queue limit are for **queued/waiting** Application frames, not entire process RSS. A work item moving from waiting queue to a callback's active ownership must release waiting-queue charges in the preexisting coherent accounting model; do not double charge or silently lose ownership. Separate capacity bounds for **N simultaneously active** payloads/callback stacks must be evaluated before implementation. Avoid admitting an unbounded number of users' active callbacks across 1024 Sessions.

**Still open:** choice of default application callback concurrency, global active callback budget, separate count budget for tiny queued events, whether user notification callbacks use independent bounded dispatch, and forced close of blocked active callbacks. Overflow of **waiting** incoming backlog still closes offending Session only. A newly received frame must never bypass the bound by launching unlimited new handlers. No new resource numbers silently approved.
