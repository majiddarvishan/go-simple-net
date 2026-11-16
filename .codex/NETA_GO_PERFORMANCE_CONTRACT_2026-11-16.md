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

**Note:** listed sizes/counts/durations are **proposed test workload samples**, not guaranteed capacity, benchmark results or performance SLAs. Pinex 1 MiB receive capacity is a compatibility clue; the exact validated maximum is a separate protocol contract.

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

The Completion-byte exhaustion disposition and exact maximum frame size remain open; no silent Response drop is allowed.

## 20. WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no numeric defaults or defaulting strategy approved yet.**  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 is the umbrella for safe zero-configuration defaults and finite validation boundaries. To avoid combining unrelated choices into one oversized approval, split it into:

- **WORKER-009A:** default derivation model for worker ceilings versus fixed resource quotas. This is the current owner decision.
- **WORKER-009B:** approve the numeric default matrix for worker/concurrency/idle settings, event and Request credits, Application queue counts and retained-payload bytes.
- **WORKER-009C:** approve hard validation ranges, cross-field constraints and the configuration-to-public-Functional-Options mapping.

The existing Q-008/Q-010 approved defaults remain unchanged. The proposed values below are engineering candidates only, not approved limits or performance SLAs.

### 2. WORKER-009A — default derivation model

**Option A — CPU-derived worker ceilings, fixed finite resource quotas (RECOMMENDED)**

At Client/Server effective-config construction, capture P = max(1, runtime.GOMAXPROCS(0)) once. Resolve only the three owner active-worker defaults from P, with finite caps:

- Application workers: min(max(4, 4 × P), 128); explicit configuration hard maximum proposed as 256.
- Completion workers: min(max(2, 2 × P), 64); explicit configuration hard maximum proposed as 128.
- OnClose workers: min(max(2, P), 32); explicit configuration hard maximum proposed as 64.

All other capacities remain fixed finite defaults, independently configurable and independent of P. The resolved settings are immutable for the lifetime of the Client/Server instance; no live adaptation, automatic resizing or hot reload is implied. Worker pools remain lazy, class-isolated and have zero warm-idle floor.

**Advantages:** scales callback execution with the available CPU budget without requiring user configuration; avoids one fixed worker count being excessive on small hosts or too restrictive on larger hosts; retains deterministic, independently bounded byte/count quotas.

**Costs:** the same program configuration resolves to different worker ceilings on hosts with different GOMAXPROCS values. The resolved values must be inspectable/documented, and deployment reproducibility requires recording the effective configuration.

**Option B — fixed worker defaults independent of CPU**

Use one fixed positive Application/Completion/OnClose worker default on every host; users explicitly tune for larger or smaller systems.

**Advantages:** identical defaults across machines and simpler capacity prediction.

**Costs:** a fixed conservative value can underuse large hosts; a high fixed value can retain too many goroutines/stacks on small hosts. It does not adapt to the execution budget already exposed by Go.

**Option C — require explicit worker settings**

Do not provide worker defaults; require every Client/Server to configure all class ceilings.

**Not recommended:** violates Zero-Configuration by Default and makes a basic Client/Server unnecessarily difficult to construct.

### 3. Candidate numeric matrix for WORKER-009B (NOT APPROVED)

| Setting | Candidate default | Scope | Notes |
|---|---:|---|---|
| Application callback concurrency | 1 | Per Session | Preserves serial default; positive configurable values permit overlap |
| Completion callback concurrency | 1 | Per Session | C-ORD-B; higher values opt into overlap |
| Application active workers | min(max(4, 4P), 128) | Per Client/Server | Owner-wide; class-isolated |
| Completion active workers | min(max(2, 2P), 64) | Per Client/Server | Owner-wide; class-isolated |
| OnClose active workers | min(max(2, P), 32) | Per Client/Server | Owner-wide; one OnClose event maximum per Session |
| Idle timeout per class | 30s | Per class, per owner | Finite grace; zero warm-idle floor |
| OnClose event credits | 1024 | Per Client/Server | Queued + running; skip permanently on saturation |
| Incoming Application frame count | 256 | Per Session | Includes Reader reservation and waiting Request callback frame |
| Outgoing Application frame count | 256 | Per Session | Includes current Writer frame until full Write |
| Aggregate queued Application frame count | 65,536 | Per Server | Incoming + outgoing; WORKER-007 scope |
| Outstanding Request credits | 128 | Per Session | Entire Request lifecycle through Completion return/cancellation |
| Aggregate outstanding Request credits | 32,768 | Per Client/Server owner | No duplicate Completion event credit |
| Retained Application bytes | 16 MiB | Per Session | Incoming Request + outgoing private frame storage |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner | Includes in-progress, queued and active library-owned Application frames |
| Retained Completion bytes | 4 MiB | Per Session | Matched Response through Completion callback return/cancellation |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner | Independent from Application retained bytes |

For context, the already-approved defaults remain: outgoing queue 4 MiB/Session; incoming queue 4 MiB/Session; aggregate queued Application frame bytes 256 MiB/Server; 1024 bound Sessions/Server; 128 pre-bind connections/listener; 30s absolute pre-bind deadline; 4 KiB Control reserve/Session; Enquire Link idle 30s (explicit zero disables outgoing probes); matching Enquire Link response timeout 10s.

The 1 MiB maximum frame in the Pinex wire reference is provisional pending exact C++ HEAD and cross-language confirmation. It is not made a frozen WORKER-009 default by this proposal.

### 4. Zero and validation principles

- Omitted Functional Options select the documented positive finite default.
- Zero is never interpreted as Unlimited for worker, queue, event, Request-credit or retained-byte limits.
- Explicit zero/negative values for required positive limits and worker idle timeouts are rejected. The already-approved EnquireLinkIdleInterval=0 remains the one intentional zero-valued disable switch.
- Per-Session concurrency and owner active-worker ceilings are independent; effective dispatch is bounded by both.
- Every count/byte limit is lazy admission capacity, not eager allocation.
- Queue bytes/counts, active-worker permits, Request credits, OnClose event credits and retained-payload bytes remain distinct ledgers. They are not summed as if they represented separate physical buffers.
- CPU-derived worker defaults are captured once during effective-config construction; all other defaults are fixed and independently overrideable.
- The exact hard maxima and cross-field validation matrix are WORKER-009C, not approved by WORKER-009A.

### 5. Owner decision requested — WORKER-009A

Choose the default derivation model:

- **Option A:** CPU-derived, capped worker ceilings; fixed finite queue/count/byte quotas. Recommended.
- **Option B:** fixed worker ceilings independent of CPU.
- **Option C:** require explicit worker settings, with no worker defaults.

No numeric value in the candidate matrix is owner-approved by this proposal. If A is approved, the next decision is the numeric matrix in WORKER-009B; valid ranges and hard maxima follow in WORKER-009C.

---

