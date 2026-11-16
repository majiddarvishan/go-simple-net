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


## 9. Worker capacity ownership addendum — WORKER-001 (OWNER APPROVED, 2026-11-16)

The owner approved **Option A**: callback-worker capacity is owned independently by each Client instance and each Server instance. Sessions of one Server share that Server's aggregate worker budget. All reconnect generations of one Client share the same Client budget. Started callbacks from retired Session generations remain charged until return; reconnect cannot reset their resource usage. Independent instances do not share a hidden process-global worker pool or mandatory process-wide active-worker cap.

This decision does not change Q-008/Q-009/Q-010 byte admission or overload behavior and does not approve numeric active-worker/event/payload limits. The waiting-frame byte budgets remain distinct from active callback stacks and payload ownership.

See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). WORKER-002 remains open.

## 10. WORKER-002 — Independent owner/class active-worker ceilings (OWNER APPROVED, 2026-11-16)

Each Client and Server instance has three independently configurable aggregate active-worker ceilings: Application, Completion and OnClose. A Session's per-class concurrency is intersected with the corresponding owner/class remaining active capacity. Active permits cannot be borrowed across callback classes.

These ceilings count callbacks physically executing; waiting Application frames remain subject to Q-009/Q-010 byte admission, queued/running Completion lifecycle remains subject to PERF-003 Request credits, and OnClose events require their own bounded event population. WORKER-001 owner scope applies across Sessions and Client reconnect generations, including already-running callbacks from retired Sessions.

No numeric worker/event limits or public option names are approved. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). WORKER-003 was subsequently resolved: OnClose is skipped when the queued+running event budget is full. WORKER-004 worker-pool topology is next.

## 11. WORKER-003 — OnClose event admission (OWNER APPROVED, OPTION A, 2026-11-16)

The owner approved nonblocking skip on saturation. Each Client/Server has a separate bounded OnClose event budget counting queued + running events. After Done closes, the engine attempts one event-credit reservation. If full, it permanently skips OnClose for that Session; no retry, overflow goroutine or wait. An admitted event retains its event credit until callback return. Done, transport cleanup and reconnect are independent. Reconnect generations share the Client budget.

This event-count ledger is distinct from WORKER-002 active-worker permits and PERF-003 Request credits. Numeric event limits remain open. Next: WORKER-004, worker-pool topology.


## 12. WORKER-004 — class-isolated worker pools (OWNER APPROVED, OPTION A, 2026-11-16)

The owner approved three independent lazy worker pools per Client/Server owner: Application, Completion and OnClose. Each pool has its own scheduler, ready queue, dispatch path, lock domain and wake-up mechanism. Workers execute one callback synchronously and are reused only within their own class; no goroutine per Request, frame or callback.

This worker topology does not change Q-008/Q-009/Q-010 byte admission, queue ownership transfer or overload outcomes. Waiting Application frame bytes, PERF-003 Request credits, OnClose queued+running event credits and WORKER-002 active-worker permits remain distinct ledgers. Idle worker goroutines do not hold active-callback permits but still consume goroutine/stack resources. The internal engine remains independent of user callback pools.

WORKER-005 separately decides idle-worker retirement. Numeric worker/event capacities and payload-byte bounds remain open. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§11–12. No implementation, tests or benchmarks authorized.

## 13. WORKER-005 — finite idle worker retirement (OWNER APPROVED, OPTION B, 2026-11-16)

Worker retirement is a lifecycle policy, not a queue-admission policy. Each class-isolated pool retires an idle worker after a finite configurable grace period, with zero warm-idle workers by default. The timer starts only after callback return and no dispatchable work. Work arriving before expiry reuses the worker; retirement after expiry must atomically update the worker count and preserve queued work plus all byte/Request/event credits. Running or blocked callbacks are never interrupted and retain active permits until return. Numeric timeout defaults/ranges remain for WORKER-009; owner shutdown/drain remains API-002.

WORKER-006 defines fair ready-Session scheduling within each independent class pool. Queue-byte admission, PERF-003 Request credits, OnClose event credits and WORKER-002 active permits remain distinct. No implementation, tests or benchmarks authorized.

 
## 14. WORKER-006 — fair Session scheduling (OWNER APPROVED, OPTION A, 2026-11-16)

Each callback-class pool independently schedules eligible Sessions with a FIFO round-robin ready-Session queue. A Session has at most one reusable ready node per class. Each turn dispatches one callback from that Session's FIFO; if work remains and its per-Session concurrency slot is available, it moves to the tail. A Session at its concurrency ceiling becomes temporarily ineligible and is reconsidered when a callback returns. Completion preserves C-ORD-B start-selection order by terminal-result settlement/enqueue, not SendRequest admission order. This is dispatch fairness, not CPU-time equality or preemption. Pools/locks/wakeups remain class-isolated, and no lock is held during user code.

WORKER-006 does not change Q-008/Q-009/Q-010 byte admission, PERF-003 Request credits, WORKER-002 active permits or WORKER-003 OnClose event credits. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §13.

## 15. Historical WORKER-007 draft (SUPERSEDED)

The earlier recommendation-only alternatives in this section are superseded by the owner-approved Option A recorded in §16 and DECISION_LOG.md. They are retained only as history; no open WORKER-007 policy remains.

## 16. WORKER-007 — Queue Capacity and Event Capacity (OWNER APPROVED, OPTION A, 2026-11-16)

The owner approved per-Session incoming and outgoing Application frame-count limits plus one Server-aggregate queued Application frame-count limit across both directions. Incoming count saturation closes only the offending Session under Q-009/Q-010; outgoing count saturation returns Queue Full and keeps the Session. Outgoing count/bytes remain charged through full Writer completion. Incoming Request count is released at callback start. Completion reuses PERF-003 Request credits and OnClose reuses WORKER-003 queued+running event credits; no duplicate event credits or shared cross-class budget. Existing Q-008/Q-009/Q-010 byte limits and the separate 4 KiB Control reserve remain unchanged. Numeric count defaults/ranges and public names remain WORKER-009/API-004.

## 17. WORKER-008 — retained payload memory (OWNER APPROVED, OPTION A, 2026-11-17)

Two independent retained-payload byte ledgers are approved: Application and Completion, each with per-Session and aggregate Client/Server owner ceilings. Application covers inbound Request storage from validated-header reservation through OnRequest callback return/cancellation and outbound private frames through full Writer completion. Completion covers matched Response storage from before body allocation through queued/running Completion callback return or Forced Close A cancellation. Queue byte/count limits remain separate admission sub-limits; they are not additive physical-memory estimates. Queue-to-active releases queue-only charges while retained-byte accounting follows the same buffer. Reconnect generations share Client owner accounting; started callbacks remain charged after Done until actual return. User-retained inbound payload after callback return is outside the library ceiling, and user-visible buffers are never recycled.

**Completion-byte exhaustion — OWNER APPROVED OPTION A (2026-11-17):** when a syntactically valid, in-limit Response matches a live Pending Request but its full declared frame cannot reserve either the Session or owner Completion retained-byte budget, the Reader does not allocate a Response Body buffer. It drains exactly the declared Body length through one small fixed-size scratch buffer, bounded independently of Body length. If drain succeeds and the Request has not already been terminally settled, deliver one normal Completion callback with no Response payload and a typed terminal Resource Capacity error; keep the Session open. The existing PERF-003 Request credit remains held until callback return or Forced Close A cancellation; no second Completion/event credit is added. If timeout/close/another terminal event wins during drain, it remains the sole settlement and no duplicate Resource Capacity callback is emitted. If drain fails because of EOF, timeout/read error or concurrent transport close, close the Session under the existing transport-error/Forced Close policy. Malformed or over-maximum frames follow protocol-error handling, not this path. Exact timeout details remain API-002 work.

The exact C++ Pinex 1.5.0 maximum frame size and Go/C++ cross-language confirmation remain open. No silent Response drop is allowed.

## WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits

**Status:** WORKER-009A, WORKER-009B1 and WORKER-009B2 CLOSED / OWNER APPROVED; WORKER-009B3 and WORKER-009C OPEN.  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 establishes zero-configuration defaults and finite validation boundaries without weakening the bounded-resource contracts.

- **WORKER-009A — CLOSED:** derive the three owner active-worker defaults from GOMAXPROCS; keep other resource quotas fixed and finite.
- **WORKER-009B1 — CLOSED / OWNER APPROVED:** per-Session Application/Completion callback concurrency defaults and independent idle-retirement grace defaults for all three worker classes.
- **WORKER-009B2 — CLOSED / OWNER APPROVED:** OnClose event-credit, Application frame-count and outstanding Request-credit defaults; see §3.2 for the six approved V1 defaults.
- **WORKER-009B3 — OPEN:** retained Application and Completion payload-byte defaults.
- **WORKER-009C — OPEN:** explicit configuration hard maxima, valid ranges, cross-field constraints and final Functional-Option mapping.

The existing Q-008/Q-010 defaults remain binding and are not reopened. B1 and B2 defaults are owner approved; B3 remains a recommendation pending owner approval. None of these defaults establishes WORKER-009C hard maxima.

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

The following matrix distinguishes owner-approved WORKER-009B1/B2 defaults from the still-proposed B3 values. The formulas in §2 and all values in §§3.1–3.2 are owner approved; B3 remains a recommendation only.

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

| Setting | Approved default | Scope | Accounting and saturation |
|---|---:|---|---|
| OnClose event credits | 1,024 | Per Client/Server owner | Queued + running; on saturation permanently skip the new OnClose per WORKER-003 |
| Incoming Application frame count | 256 | Per Session | Reader reservation + waiting Request callback frames; if admission fails, close only that Session |
| Outgoing Application frame count | 256 | Per Session | Includes current Writer-owned frame until full Write/discard; fail fast with Queue Full |
| Aggregate queued Application frame count | 65,536 | Per Server | Combined incoming + outgoing queued frames; incoming overflow closes offender, outgoing overflow returns Queue Full |
| Outstanding Request credits | 128 | Per Session | Full async Request lifecycle through Completion callback return/cancellation |
| Aggregate outstanding Request credits | 32,768 | Per Client/Server owner | Same Request credits; no duplicate Completion event credit |

Rationale: count limits protect tiny/empty-frame metadata that byte budgets alone do not bound. The 1,024 OnClose event budget matches the approved 1,024 bound-Session default for a single close wave, while remaining finite across churn and permanently blocked callbacks. The 65,536 Server frame ceiling limits the aggregate population independently of the per-Session 256 ceilings. PERF-003 Request credits remain the sole Completion lifecycle/count mechanism; no second Completion event credit is created.

**Owner approval (2026-11-17):** the owner approved all six defaults in §3.2 (Option A). OnClose credits count queued + running events per Client/Server owner and saturation permanently skips the new OnClose event under WORKER-003. Incoming count reserves after a valid fixed header and before body allocation/read; it covers the in-progress Reader reservation plus waiting Request callback frames and is released at callback start. If either the Session or Server incoming count cannot admit the frame, close only that Session. Outgoing count includes the Writer-owned frame until full Write or definitive discard; local or Server aggregate exhaustion returns Queue Full without closing the Session or creating Pending/Sequence state. The Server aggregate frame count covers incoming + outgoing queued Application frames, not a second physical population. Outstanding Request credits are reserved before accepting callback-based SendRequest and remain through response wait, terminal settlement, queued Completion and active Completion callback until callback return or permitted cancellation. The per-Session limit is 128; the aggregate limit is 32,768 per Client/Server owner, shared by all Server Sessions and all Client reconnect generations. A saturated Request-credit admission rejects the new request before enqueue/transmission with a typed capacity error; it does not close the Session. SendNoWait and SendResponse do not consume Request credits. All six limits are independently configurable, lazy admission ceilings (not preallocation), and explicit zero is invalid. The aggregate frame-count and Request-credit defaults are intentionally lower than the sum of their theoretical per-Session ceilings; no cross-field requirement forces aggregate capacity to equal or exceed the product of Session count and local capacity. These defaults do not set WORKER-009C hard maxima.

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
- The earlier 4 MiB Completion-per-Session draft is superseded by this 16 MiB recommendation. Against the still-provisional 1 MiB maximum-frame reference, 4 MiB would permit only four maximum-size Response payloads for one Session. 16 MiB is a less restrictive bounded burst allowance; the 256 MiB owner ceiling still caps aggregate Completion payload retention.
- These values are admission ceilings, not preallocations. User-retained inbound payload after callback return remains outside the library-owned retained-byte ledger.

The Pinex reference's 1 MiB maximum frame remains provisional until the exact C++ Pinex 1.5.0 baseline and cross-language behavior are confirmed. WORKER-009B does not freeze that frame maximum.

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
- Completion-byte exhaustion must never silently discard a matching Response. The owner-approved policy is bounded Body drain with a fixed scratch buffer, followed by one terminal Resource Capacity Completion callback without Response payload; the Session stays open. A drain/transport failure closes the Session under the transport-error policy. See WORKER-008 above.

### 6. Owner decision sequence

**Next: WORKER-009B3.** Review retained Application and Completion payload-byte defaults. WORKER-009C follows to establish hard maxima, valid ranges, cross-field validation and public Functional-Option limits.

**Guardrail:** design/documentation only. No Go/C++ source, tests, benchmarks, builds, dependency changes, CI, deployment or operational changes are authorized.

---

