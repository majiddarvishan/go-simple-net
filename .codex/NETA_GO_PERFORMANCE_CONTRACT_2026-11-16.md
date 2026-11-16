# neta-go V1 — Performance Engineering Contract (Design, Not Implementation)

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
