# neta-go V1 — Performance Engineering Contract (Design, Not Implementation)

> **LATEST OWNER REVISION (2026-11-16):** Q-003-R1 owner-approved configurable concurrent **Application** handlers per Session, replacing unconditional serial callback execution when `Concurrency > 1`. Internal protocol machinery is independent of user handler return. Older Q-003 wording in this file is historical; see [Decision Log](DECISION_LOG.md). API-001b remains OPEN.


**Date:** 2026-11-16  
**Status:** PERFORMANCE PRIORITY APPROVED; PERF-002 payload ownership **APPROVED OPTION A**; other techniques, numeric targets and PERF-003/API-001b remain DRAFT / OPEN.  
**Source of priority:** Owner explicitly approved API-001a Option A (asynchronous/callback-first) and stated that performance is **very important**.  
**Scope:** neta-go native Go TCP messaging Client/Server wire-compatible with C++ Pinex 1.5.0.  
**Implementation:** NOT AUTHORIZED. No source, Go module, benchmark or operational changes.

## 1. Non-negotiable priority and compatibility

- Performance is a **first-class V1 requirement**, not an optional future optimization. Architecture and Public API reviews must assess throughput, latency distribution, allocation rate, memory pressure, throughput under contention and Session isolation.
- Existing owner-approved contracts remain binding: Q-003 goroutine-safety, ordered non-overlapping callbacks **within each Session**, Session independence; Q-008 outgoing FIFO 4 MiB/Session and immediate Queue Full when saturated; Q-009 incoming overload closes only offender; Q-010 configurable resource ceilings; Q-002/Q-006/Q-007 idle-only Enquire Link.
- Do not speed up the library by violating Pinex framing, exposing a race-prone API, skipping Sequence/timeout bookkeeping, silently dropping frames, removing bounded admission, or changing wire protocol.
- **Asynchronous** means SendRequest admission does not wait for the network response. It does **not** mean it has zero allocations, zero copies, zero lock contention, or that the peer processed the message.
- Avoid numeric performance guarantees before baselines exist. No benchmark has been executed or measured in this design stage.

## 2. API-001a approved completion path

**CLOSED — OWNER APPROVED OPTION A on 2026-11-16:**
- SendRequest-style method reports **local admission** promptly (accepted sequence or local failure), then the **single terminal Request result** reaches the application's completion callback/registered response handler. The exact callback registration shape is still API-001b.
- No blocking Request(ctx) interface is required as the **primary** V1 API. A later optional helper may be considered separately, without replacing the nonblocking core.
- Preserve opaque `[]byte` payload, the original Session-scoped sequence, response `command_status`, and exactly-once local terminal completion.
- Response arrival, response deadline, Queue Full, Session termination and cancellation races must be resolved without double-delivery or orphan pending requests.
- No automatic resend/replay of Application Request on reconnect; a Session identity/reconnect boundary must reset transient pending state safely.

## 3. Performance-sensitive design principles (engineering proposals; not yet owner-approved technique choices)

### 3.1 Memory and copies

- Audit **copies per outgoing/incoming Stream Frame**, per-message allocations, temporary serialization and queued-buffer ownership. A deep copy may be necessary for caller safety; don't assert "zero-copy" while performing one.
- Consider pre-sized fixed header (10-byte Pinex) serialization, reusable bounded scratch buffers and direct network read parsing. Pooling is only safe with a documented borrow lifetime and no data races or use-after-recycle.
- Avoid eager preallocation of 4 MiB queues per Session or 256 MiB for the Server. Limits represent admission ceilings, not reserved memory.
- A user callback retaining an incoming `[]byte` beyond its documented lifetime must not observe unexpectedly reused memory. Explicitly choose either safe owned-copy defaults or borrowed-view rules under **PERF-002 / API-001b**.

### 3.2 Concurrency and completion scheduling

- Single serialized Writer per TCP connection preserves entire frame integrity while allowing multiple producer goroutines to enqueue concurrently.
- Prefer per-Session contention boundaries: no global mutex in the hot path for ordinary Session sends/callbacks. Server-wide byte quota accounting may require an atomic counter or equivalent; evaluate actual cost under contention before specifying an implementation.
- The requirement that callbacks for **one Session** not overlap still applies to completion callbacks and incoming Application callback ordering as specified by API-001b. Avoid one goroutine per message without compelling measured need.
- A blocked user callback cannot be forcibly killed. Transport close and accounting cleanup must not wait unboundedly for it; terminal callback delivery guarantee needs explicit design without violating Q-003.
- A bounded completion/pending registry is essential: successful nonblocking submits can otherwise accumulate unlimited response-completion state even if outgoing bytes are bounded. Specify cap and overflow action separately under **PERF-003**.

### 3.3 Network path, backpressure and control

- Do not perform blocking network Write inside arbitrary user SendRequest caller goroutines; serialize writer ownership, preserve FIFO and apply Q-008 admission before queue visibility.
- Unbind and Enquire Link Control PDUs must maintain bounded independent headroom and required ordering. They cannot bypass bytes already in TCP's ordered stream; don't promise priority inversion elimination via a separate socket-less lane.
- Coalesced/fragmented incoming TCP reads must remain correct, including full-frame limits. Batch operations only if benchmarked and without changing callback order or frame correctness.
- Avoid logging and exported metrics in the per-message hot path by default; instrumentation should be low-cost or opt-in when exposed. Q-004 still excludes mandatory Prometheus integration.
- Don't hide queue saturation via unbounded retry, spin loops or silent memory growth.

## 4. Benchmark matrix to define before implementation

**Measurement requirement:** compare the Go library to a realistic Go↔C++ Pinex wire interoperability workload and an internally consistent Go baseline when available. Hardware, GOMAXPROCS, Go/compiler versions, TCP loopback vs LAN topology, CPU frequency/load, payload profile, connection count and offered load must be recorded.

| Workload | Variables to exercise | Measurements |
|---|---|---|
| Steady single Session | 64B, 256B, 1KiB, 4KiB, and near protocol maximum, each validated against actual C++ frame limit | msgs/s, MiB/s, allocations/op, B/op, p50/p95/p99 completion latency, CPU |
| Parallel Session scaling | 1, 8, 32, 128, 512 Sessions, bounded by configured Server cap | aggregate throughput, per-Session fairness, lock contention, goroutine count, memory |
| High producer contention | 1, 4, 16, 64 concurrent sending goroutines per Session | submission p99 latency, queue accounting, race-free sequence/callback completion |
| Request/Response | Go↔Go and Go↔Pinex C++; matching response and error status | completed ops/s, p95/p99 RTT, pending registry size, callback delay |
| Fire/no-wait | Standard stream_req without pending response, no new Wire PDU | accepted submission rate, allocations/op, queue utilization |
| Slow callbacks and peers | delayed receiving peer, blocked callback, near/full FIFO | Queue Full speed, inbound only-offender closure, affected vs unaffected Session latency |
| Reconnect/close | churn, abrupt peer close, timeout, Bind reject | recovery overhead, goroutine/FD leaks, pending cleanup, tail latency |
| Idle control | 30s no-traffic idle, 10s unanswered probe | timer wakeups, bounded overhead on active Session, correct close |
| Memory stability | 5-min+ bounded soak under mixed size/load, retained callback cases | heap live bytes, heap allocations, GC cycles/pause, RSS, outstanding buffers |

**Note:** listed sizes/counts/durations are **proposed test workload samples**, not guaranteed capacity, benchmark results or performance SLAs. Pinex C++ 1.5.0 source confirms a 1 MiB total-frame receive maximum including Header; Go enforces the same bound in both directions. Cross-language confirmation remains pending.

### Measurement methodology

1. Compare like-for-like with/without optional instrumentation, and disclose whether transport RTT, scheduler delay or application callback time is included in latency.
2. Record distributions (p50/p95/p99), not mean only; include time under queue congestion and GC pressure.
3. Measure total system CPU/RSS and per-operation allocations; never infer performance from one synthetic microbenchmark alone.
4. Test with `-race` for correctness **separately** from performance numbers; race instrumentation is not a production throughput baseline.
5. Compare alternative payload ownership/copy strategies under exactly the same workload before freezing API-001b semantics.
6. Explicitly define measurable performance acceptance thresholds later from real data and owner workload requirements. **No threshold is approved yet**.

## 5. Open technical decisions (no silent implementation choice)

| ID | Subject | Alternatives / concern | Status |
|---|---|---|---|
| PERF-001 | Performance measurement policy | V1 benchmarks and allocation/latency/soak gates required; numeric pass/fail targets after measurement | DRAFT follow-up |
| PERF-002 | `[]byte` lifetime and ownership | A: one outbound snapshot and owned, directly received retainable inbound buffer; no borrowed default | **CLOSED — APPROVED A (2026-11-16)** |
| PERF-003 | Pending/completion count admission | **OWNER-APPROVED Option A model**: configurable per-Session + Server aggregate completion credits, fail-fast reject new Request without Session close; numerical defaults and forced-close details OPEN | **MODEL APPROVED A; DETAILS OPEN** |
| PERF-004 | Allocation/pooling policies | choose after benchmark; pools must never leak borrowed buffer data after recycle | OPEN — engineering validation |
| API-001b | Callback execution + Close/Done semantics | must reconcile per-Session callback ordering and blocked user callback on shutdown | OPEN |
| API-002 | Timeout/reconnect defaults | compatibility and production performance impacts | OPEN |

## 6. Guardrails for implementation authorization (still absent)

No coding or benchmarks are authorized at this stage. When the owner later authorizes implementation, measure a minimal baseline early, document reproducible commands and environment, and keep optimizations evidence-driven. New benchmarks and tests should validate performance without sacrificing Q-003/Q-008/Q-009/Q-010 or wire compatibility.

**Next architectural topic:** decide the public payload ownership model (PERF-002) with explicit performance/correctness tradeoffs. Do not silently select zero-copy or extra copies merely because performance was emphasized.


## 7. Follow-up design review (2026-11-16; ALL NEW CHOICES OPEN)

A detailed copy/ownership/retention matrix, bounded pending/completion-credit proposal, callback lifecycle impossibility boundary, and owner decision list are recorded in [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md). Recommended **PERF-002 A** is one outbound snapshot copy on successful async admission and zero *extra* receive payload copies by decoding directly into an owned user-retainable buffer. This is a proposal, not an approval. PERF-003 numerical example limits and API-001b forced-close tradeoffs also remain OPEN; no benchmark or implementation was run.


## 8. Owner approval notice (2026-11-16)

**PERF-002 CLOSED / Option A approved**: outgoing asynchronous sends snapshot the caller's payload before successful return, once in the intended hot path; receiving reads directly into a new owned user-retainable buffer where possible, without a redundant second payload copy. The library never pools an incoming buffer after handing its ownership to user code. The caller cannot concurrently write to its source during the copy; after a successful Send return it may reuse the source. This is not an end-to-end zero-copy guarantee.

Earlier sections describing PERF-002 as OPEN are historical and superseded by [the approved Decision Log](DECISION_LOG.md) and [updated design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md). **PERF-003**, **API-001b**, numerical targets, optional buffer pooling and all performance validation remain OPEN. No benchmark, test or implementation authorized.


## 9. PERF-003 approval notice (2026-11-16)

**Owner approved PERF-003 Option A at the model level**: count-bounded outstanding Request + queued/running completion per Session and Server aggregate; independently configurable with API-004 Functional Options; promptly reject new Request on exhausted count while keeping Session connected and without publishing a frame or leaking pending state. No goroutine-per-Request or unbounded completion queue. This is in addition to Q-008/Q-010 queued byte caps, not a replacement.

**Not approved:** 1024 per Session/65536 Server are merely illustrative numbers, separate incoming event-count cap is still only a proposal, and API-001b forced-close callback behavior and credit release semantics need owner choice. Earlier PERF-003 OPEN statements in this document refer to historic *preapproval* status; refer to [Decision Log](DECISION_LOG.md) and [current design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) for the latest authoritative split. No implementation/test/benchmark authorization.


## 10. Parallel application callback performance implications (Q-003-R1)

**Owner-approved choice:** bounded Application callback parallelism within the same Session, independently configurable per Session, `Concurrency=1` serial, >1 parallel. No unconditional callback return ordering in parallel mode; incoming queue dispatch still respects FIFO selection. Internal Reader/Writer/state machine, Control PDU, Pending/Timer/Close and resource cleanup are not blocked by a slow user Application callback. Do not permit goroutine-per-frame fanout.

**Performance plan (NOT EXECUTED):** compare 1/2/4/8/16 concurrent handlers per Session and large Session counts; monitor p50/p95/p99 handler scheduling delay, completed callbacks per second, active/queued payload memory, GC, goroutine counts, starvation, fairness and Control response latency under permanently blocked handlers. Test callbacks producing `SendResponse` out-of-order: wire reply Sequence must always match its request and writer remains single-frame serialized, though response *time order* may differ from request arrival. This is not permission to reorder received wire parsing.

**Open decisions:** choose default concurrency and potential Server-wide active callback guard; select separate bounded user lifecycle/completion callback dispatcher vs unified worker admission in API-001b; decide forced Close policy and callback completion guarantees. The 256 MiB Server queued Application cap does not cover N running user callbacks or user-retained payloads. No numeric SLA or new default approved.


## 11. API-001b-N1 approved notification isolation (2026-11-16)

**APPROVED:** separate **bounded** user Notification dispatch (`OnClose` and Request Completion) from concurrent bounded Application `OnRequest` dispatch. Both are external user code, isolated from the internal TCP/Timeout/Pending/Close engine. The latter must progress without waiting for either user callback class. This enables application stalls not to automatically occupy the notification workers; it does not guarantee completion of an arbitrarily stuck notification callback.

PERF-003 count ceilings remain binding at the model level; exact numeric default, dispatcher-worker count and forced-close cancel/defer semantics are **OPEN**. Benchmark plan (not executed) should compare notification latency under blocked `OnRequest`, slow completion callbacks, many sessions, and queue saturation. See [Decision Log](DECISION_LOG.md) § API-001b-N1. No code/tests/benchmarks.


## 12. WORKER-001 owner approval — instance-scoped callback capacity (2026-11-16)

**APPROVED Option A:** each Client and Server instance owns an independent callback-worker resource domain. Sessions belonging to one Server share that Server's budgets; all reconnect generations of one Client share its budget. Started callbacks from retired Session generations remain charged until return. Independent instances do not share a hidden process-global callback pool or mandatory process-global active-worker ceiling.

This ownership boundary avoids hidden cross-instance contention and a mandatory process-global hot-path admission point. It does not itself select worker counts, physical pool layout, numeric defaults, event limits or payload-byte ceilings.

See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). WORKER-002 will decide explicit per-class owner-wide active ceilings versus deriving them from Session limits. No implementation, tests or benchmarks authorized.

## 13. WORKER-002 owner approval — independent class active ceilings (2026-11-16)

**APPROVED Option A:** each Client and Server instance owns three independently configurable aggregate active-worker ceilings: Application, Completion and OnClose. Per-Session concurrency is intersected with the remaining ceiling for its own class. Active permits cannot be borrowed across classes. These ceilings count physically executing callbacks; queued work is governed by separate event/queue budgets.

WORKER-001 owner scope applies to all Sessions of one Server and all reconnect generations of one Client. Started callbacks from retired Sessions remain charged until return. Independent instances have no mandatory process-global active-worker ceiling.

No numeric defaults, public option names, pool layout or event-budget values are approved. WORKER-003 was subsequently resolved by approval of nonblocking skip on saturation. Next: WORKER-004, logical worker-pool topology. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11. No implementation or performance validation authorized.

## 14. WORKER-003 owner approval — bounded best-effort OnClose events (2026-11-16)

**APPROVED Option A:** after Session Done closes, make one nonblocking reservation from the owning Client/Server OnClose event budget. The budget counts queued + running events. If full, skip that OnClose invocation permanently; no retry, overflow goroutine, hidden queue or teardown wait. Admitted events hold their event credit until callback return. Done, transport cleanup and reconnect never depend on OnClose callback progress. Client reconnect generations share the same budget.

No numeric event limit or public option name is approved. Next: WORKER-004, logical worker-pool topology. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.


## 15. WORKER-004 owner approval — isolated callback-class worker pools (2026-11-16)

**APPROVED Option A:** Application, Completion and OnClose each use an independent lazy worker pool per Client/Server owner. Each pool has a separate scheduler, ready queue, dispatch path, lock domain and wake-up mechanism. Workers are reused only within their class; no goroutine is created per Request, frame or callback. A worker executes one callback synchronously.

A permanently blocked callback can consume only its own class's worker capacity. It cannot occupy another class's workers or dispatch path, and the internal protocol/transport engine remains independent. Worker goroutine count per class is bounded by its WORKER-002 ceiling. Idle workers retain goroutine/stack memory but do not hold active-callback permits. Shared worker-loop code is allowed; shared pool resources are not.

This supersedes earlier references to WORKER-004 as an open topology choice. WORKER-005 covers idle-worker retirement. Numeric limits and performance validation remain open; no benchmark, test or implementation was authorized.

## 16. WORKER-005 owner approval — finite idle worker retirement (2026-11-16)

**APPROVED Option B:** workers that return from callbacks and find no dispatchable work enter Idle for a finite, independently configurable class-specific grace period, then atomically retire if still idle. The default warm-idle floor is zero. Work arriving before expiry reuses the worker. Running or permanently blocked callbacks are never interrupted and retain active permits until return. Retirement preserves queued work and all independent queue/Request/event credits. Enqueue/retirement races must not lose wakeups or strand work. No goroutine per timeout or process-global reaper. Numeric timeout defaults/ranges are deferred to WORKER-009; owner shutdown/drain remains API-002.

**Next:** WORKER-006 fair scheduling between Sessions. No code, tests or benchmarks authorized.


## 16. WORKER-006 — per-class fair Session scheduling (OWNER APPROVED, OPTION A, 2026-11-16)

Each callback-class pool uses a FIFO round-robin ready-Session queue. One callback is dispatched per eligible Session turn, preserving FIFO inside that Session. One reusable ready node per Session/class bounds scheduler metadata by Session count. Completion preserves C-ORD-B terminal-result settlement/enqueue order. Fairness does not mean equal CPU time or callback preemption. Class pools remain isolated and no lock is held while user code runs.

## 17. Historical WORKER-007 queue-count proposal (SUPERSEDED)

The recommendation-only proposal is superseded by the owner-approved WORKER-007 Option A in §18.

## 18. WORKER-007 — Application queue count ceilings (OWNER APPROVED, OPTION A, 2026-11-16)

Per-Session incoming/outgoing Application frame-count limits and one Server-aggregate queued Application frame-count limit across both directions are approved. Incoming Request count releases at callback start; outgoing count remains until full Writer completion. Incoming overflow closes only the offending Session; outgoing overflow returns Queue Full. Completion and OnClose retain their existing independent credits.

## 19. WORKER-008 — payload memory ceiling (OWNER APPROVED, OPTION A, 2026-11-17)

Separate owner-scoped retained-payload byte ledgers for Application and Completion are approved, each with per-Session and aggregate Client/Server ceilings. Application covers inbound Request payload from header reservation through callback return and outgoing private frame through full Writer completion. Completion covers matching Response payload from before body allocation through Completion callback return/cancellation. Queue byte/count caps remain independent sub-limits, not additive physical buffers. Queue-to-active releases only queue charges; retained-byte charges follow the same payload. Started callbacks remain charged until return; user-retained payload after callback return is outside the library ceiling.

Completion-byte exhaustion is closed under WORKER-008 Option A. Pinex 1.5.0's 1 MiB total-frame maximum is source-verified; golden-frame/cross-language confirmation remains open. No silent Response drop is allowed.

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

