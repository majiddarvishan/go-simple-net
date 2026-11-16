# Project Handoff — neta-go / Pinex Compatibility

**Updated:** 2026-11-16  
**Repository:** majiddarvishan/neta-go  
**Branch:** main  
**Implementation status:** NOT STARTED — NOT AUTHORIZED  
**Current work:** PERF-002 A closed; PERF-003 A count-admission model approved (numeric details open); API-001b forced-close/callback semantics OPEN

## 1. Repository state

- Default branch: main
- Latest documentation commit before this handoff update: dbf731f097222d7ebce25ea95889472c028eba24
- Only documentation under .codex has been added/updated for this planning step.
- No Go application source, tests, go.mod, CI, runtime configuration or dependencies were added or changed.

## 2. Source inputs

### Uploaded Go sample

- Archive: ss7.tgz
- SHA-256: 25b3e0cdfed10c170cf1a4aa02945bfe765bfa1378ef979ed5546883dadd8aa3
- 24 Go files total; 14 non-generated package files and generated Protobuf files.
- Treat it as a behavioral/features reference, not as the authoritative current Pinex wire specification.

### C++ Pinex compatibility target

- Repository: github.com/majiddarvishan/pinex
- Branch: develop
- Version: 1.5.0
- High-level API: messaging_client / messaging_server
- Exact develop HEAD SHA must be pinned immediately before implementation and cross-language testing.
- Main is not the compatibility target for this work.

## 3. New planning documents

- PINEX_GO_PORT_TASKS_2026-11-16.md — task breakdown, dependencies, acceptance criteria, open decisions.
- PINEX_WIRE_PROTOCOL_2026-11-16.md — current Pinex 1.5.0 wire contract and differences from the uploaded Go sample.
- NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md — design-only draft of public API, config, session lifecycle, concurrency, idle keepalive, request tracking and backpressure; Q-006 through Q-010 policy approvals are reflected.
- NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md — APPROVED API-004 A: Functional Options, complete internal defaults, independent explicit overrides including idle 0, validated immutable per-instance config.
- NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md — draft Client/Server/Session Public API; API-001a owner approved A (nonblocking/callback-first). API-001b, ownership and callback termination semantics remain OPEN.
- NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md — owner explicitly requires performance as first-class V1 concern; measurement matrix, copy/allocation and bounded completion design work (PERF-001..003), no numeric targets approved.
- NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md — Q-008 outgoing 4 MiB/Fail-Fast, Q-009 B incoming overflow close, and Q-010 A configurable inbound/global/session/pre-bind/control defaults are approved. API-004 design remains open.
- DECISION_LOG.md — authoritative approved decisions Q-001 through Q-010 (as applicable), plus NAM-001; distinguishes remaining open API details.
- SERVER_REVIEW_2026-11-16.md — original static review of the uploaded server sample.

## 4. Confirmed protocol facts

- 10-byte header: total length BE u32, command ID u8, command status u8, sequence BE u32.
- Command IDs: Bind 0x01/0x81, Stream 0x02/0x82, Unbind 0x03/0x83, Enquire Link 0x04/0x84.
- Status: rok 0x00, rfail 0xFF.
- Bind body: bind_type u8 followed by null-terminated system_id; bind_type 0=uni_direction, 1=bi_direction.
- Bind Response uses the same body shape and carries server system_id.
- Stream payload is raw remaining frame bytes, with no extra length prefix or mandatory Protobuf envelope.
- Enquire Link response and Unbind response reuse the request sequence number.
- Sequence is per Session, increments, and skips zero.
- Pinex uses a 10-second Unbind protocol deadline.
- Current Pinex pre-bind defaults: 30-second handshake deadline, 128 pending sessions, optional per-IP quota disabled by default.
- Current Pinex Client defaults documented on develop: 30-second connect timeout, 30-second Bind response timeout, 5-second reconnect delay when auto-reconnect is enabled.
- Pinex default behavior responds to Enquire Link and uses OS TCP Keepalive; it does not send periodic Enquire Link by default.

## 5. Important incompatibilities found in uploaded Go sample

1. Go Bind Request/Response bodies omit the Bind Type byte required by Pinex 1.5.0.
2. Go ConnectionReader drops command_status from the Header.
3. Go ClientProtocolHandler.handleBindResponse incorrectly reads the first Body byte as Status; Pinex status is Header offset 5 and Bind Body begins with bind_type.
4. submit_req/submit_resp names differ from Pinex stream_req/stream_resp, but their numeric wire values match (0x02/0x82).
5. The sample uses random Sequence values and automatic deduplication; Pinex uses per-session increasing nonzero sequence values and does not define Sequence-based deduplication as a protocol feature.

Therefore, copying the sample's protocol handling verbatim would not interoperate with current Pinex.

## 6. Task status

See PINEX_GO_PORT_TASKS_2026-11-16.md for the complete backlog.

- GO-PINEX-001: protocol analyzed; owner approved Q-001 Option A on 2026-11-16; golden frames and further contract details are still open.
- GO-PINEX-002 through GO-PINEX-014: open.
- Cross-language tests have not been run.
- No implementation has been started.

## 7. Owner decisions

- **Q-001 — APPROVED (Option A, 2026-11-16):** Pinex-compatible Bind Validator callback based on System ID and actual remote address. No password/token on the wire, no change to Bind PDU layout. Accepted Bind => rok 0x00; rejected => rfail 0xFF. This is not cryptographic identity authentication. See DECISION_LOG.md.
- **Q-005 — APPROVED (Option B, 2026-11-16):** If no Bind Validator callback is supplied, allow an otherwise-valid Bind by default, matching Pinex C++. Duplicate `system_id`, malformed Bind, invalid state and admission limits are still enforced. If a callback is provided, enforce its allow/reject result. This is permissive admission, not authenticated peer identity. See DECISION_LOG.md.
- **Q-002 — APPROVED (2026-11-16):** Configurable, idle-only Enquire Link on Go Client and Server. Send automatically only after no successful frame exchange in either direction for a positive idle interval; `0` disables automatic sending, not replies to peer Enquire Link requests. Never send a second probe while one is outstanding; protocol bytes unchanged. Numeric idle default is **30s** (Q-006); matching-probe response timeout default **10s** and failure closure are approved (Q-007). See DECISION_LOG.md.
- **Q-003 — APPROVED (2026-11-16):** Goroutine-safe Client/Server/Session APIs; non-overlapping, receipt-ordered callbacks per Session; isolation across Sessions; frame-safe concurrent sends; controlled shutdown for callbacks and pending operations. Implementation details (queue capacity, deadline, API signatures) still need design. See DECISION_LOG.md.
- **Q-004 — CLOSED / APPROVED OPTION A (2026-11-16):** Lean V1 Core retains Pinex interoperability, sessions, backpressure, bounded FIFO, reconnect/timeout and idle-only Enquire Link. Defer Circuit Breaker, business Rate Limiter, Priority Queue, Prometheus exporter and SS7/Protobuf integrations; exclude automatic sequence dedup. Transport admission/backpressure remains mandatory. See DECISION_LOG.md.

## 8. Next action

Q-001, Q-002, Q-003, Q-004, Q-005, Q-006, Q-007, Q-008, Q-009, Q-010 and NAM-001 are **CLOSED** as design decisions. Q-010 Option A (2026-11-16) approved **configurable** initial capacities and global-overload rules: inbound 4 MiB per Session, outgoing 4 MiB per Session (Q-008), aggregate queued Application 256 MiB per Server, 1024 bound Sessions, 128 pre-bind, 30s pre-bind timeout, 4 KiB Control reserve per Session. These are defaults, not preallocated memory or process RSS limits.

**Next design-only work:** API-004 A and API-001a A are CLOSED. Performance is an owner-declared first-class V1 priority. Next: design PERF-002 / API-001b payload ownership/copy lifetime and PERF-003 bounded pending/completion; then API-002 timeouts/reconnect/close and API-003 module/package path. Pin C++ Pinex develop HEAD and golden-wire test vectors before implementation. No code, tests, module/repository renaming or operational action authorized.

## 9. Approved Go library name

**NAM-001 CLOSED (2026-11-16):** The owner selected `neta-go` as the name of the native Go TCP Messaging library. NETA was originally proposed as "Network Easy Transport API" but the selected name itself, rather than the expansion, is the approved decision. The project owner subsequently renamed the GitHub repository to `majiddarvishan/neta-go` (confirmed accessible on `main`). No Go module/package-path change or source implementation has been performed or approved in this documentation phase. Pinex C++ stays named Pinex. No code has been written. See DECISION_LOG.md.


## 10. Enquire Link default decision

**Q-006 CLOSED, Option B (2026-11-16):** neta-go Client/Server use an idle-only, configurable Enquire Link interval with a **30-second default**. It is not a fixed 30-second periodic probe: actual incoming/outgoing messages reset idle time. Explicit `0` disables automatic requests but never disables responding to incoming Enquire Link. Probe response policy is approved in Q-007: **10-second configurable default**, matching response Sequence required; timeout closes the Session and Client reconnect is conditional on Auto Reconnect.


## 11. Enquire Link response timeout decision

**Q-007 CLOSED (2026-11-16):** Enquire Link response timeout is separately configurable, **10 seconds by default** on Go Client and Server. It begins after the full Request is written. Only matching `enquire_link_resp` (same Sequence) satisfies it; unrelated traffic cannot satisfy it. On expiry, Session closes once and pending operations terminate; Client retries only if Auto Reconnect was enabled. No Pinex wire changes or implementation were authorized.


## 12. Outgoing resource decision and ingress design

**Q-008 CLOSED / approved Option A (2026-11-16):** configurable 4 MiB outgoing Application Stream FIFO per Session, including serialized Frame headers, immediate Queue Full when full; no partial enqueue, silent drops, or orphan request trackers. Control path capacity must be separate and bounded, without violating single-writer framing or Pinex Unbind ordering/deadline.

**Q-009 CLOSED / APPROVED B (2026-11-16):** if a new incoming Application Frame cannot fit the bounded Callback queue, promptly close **that Session only** with an overload cause; no silent drop or pause/resume fallback. Other Sessions continue unaffected. Go cannot forcibly terminate a blocked user callback; avoid indefinite shutdown waits and finalize terminal callback semantics in API design.

**Q-010 CLOSED / APPROVED A (2026-11-16):** ALL defaults are configurable: incoming 4 MiB/Session, aggregate queued Application 256 MiB/Server (NOT process RSS), 1024 bound Sessions, 128 pre-bind, absolute 30s pre-bind deadline, and 4 KiB Control reserve/Session. Outgoing failed local/global admission => Queue Full, keep Session. Incoming failed local/global admission => close only offender. At bound limit reject new Bind without eviction; at pre-bind capacity refuse socket. Outgoing 4 MiB/Session from Q-008 also remains configurable. See resource design.

**API-004 CLOSED / OWNER APPROVED A:** typed Functional Options over complete internal defaults, independently applied explicit overrides including idle 0, validation before Dial/Listen, immutable effective per-instance snapshots. No hot reload approved.


## 13. Incoming overload decision

**Q-009 CLOSED — approved Option B (2026-11-16):** A new Application Stream Frame which cannot fit the bounded Session-local incoming Callback backlog triggers a prompt **close of that Session**, not Pause/Resume. No impact on other Sessions, no silent drop, no duplicate terminal event; pending state and queue charges are released. In the presence of blocked user callback code, transport shutdown must not wait indefinitely for arbitrary user logic. Exact callback delivery/close ordering awaits API design. Q-010 configurable default capacities and global admission rules are now APPROVED. No code changes authorized.

## 14. Q-010 final owner decision and configurability handoff

**Q-010 CLOSED — OPTION A WITH ALL SETTINGS CONFIGURABLE (2026-11-16).** Approved defaults and outcomes are defined in [Decision Log](DECISION_LOG.md), [Resource Design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md) and [API/Config Design](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md). An omitted setting selects the approved default; an explicit valid override is honored. API-004 A specifies Functional Options, internal defaults, and immutable validated per-instance config. Explicit `EnquireLinkIdleInterval=0` disables automatic probes.

**No implementation is authorized.** API-004 A is owner-approved; public method names, callback termination policy and request completion style are not yet approved.

## 15. API-004 closure and API-001 follow-up

**API-004 CLOSED / OWNER APPROVED OPTION A (2026-11-16):** Go-native typed Functional Options, complete defaults, independent explicit overrides including idle=0, pre-network validation, immutable effective configuration. [Approved configuration contract](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md).

**API-001a CLOSED / OWNER APPROVED A:** [Public API draft](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md) defines callback-first asynchronous request submission, with prompt local admission feedback and later one-shot completion via callback. Option B synchronous Context-first as primary API not selected. Other open design details: callback/terminal close/Done behavior, outgoing/incoming payload lifetime, error categories, startup/Stop and reconnect defaults, exact package/module path. Preserve approved Q-003 per-session callback serialization, do not promise forced termination of indefinitely blocked user callbacks. **No source code, module changes, tests or execution.**

## 16. API-001a closure and performance-first V1 constraint

**API-001a CLOSED / OPTION A APPROVED (2026-11-16).** Nonblocking/callback-first SendRequest-style API is primary. Successful queue admission does not guarantee remote delivery or processing. Matching response, timeout and close all compete for one terminal callback completion; API-001b must define precise delivery and Stop/Done guarantees.

**PERFORMANCE OWNER PRIORITY:** high throughput/low tail latency/low allocation overhead and scalability are important for V1. Propose reproducible measurements of msgs/s, p50/p95/p99 latency, allocation and copy counts, GC/RSS, contention and multi-Session load without precommitting unsupported numeric SLAs. See [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md).

**Next open decisions:** PERF-002 payload `[]byte` lifetime (safe-copy vs borrowed), PERF-003 pending/completion bounds and API-001b callback/Close/Done semantics. No coding or tests permitted.

## 17. GitHub repository path changed by owner (2026-11-16)

**Current canonical repository:** `majiddarvishan/neta-go`  
**SSH URL:** `git@github.com:majiddarvishan/neta-go.git`  
**Branch:** `main` — existing `.codex` documents verified accessible via the GitHub connector.

The owner **already renamed the GitHub repository** from `majiddarvishan/go-simple-net`; this update only synchronizes documentation references. No repository rename, source code edit, `go.mod` edit, test, or build is performed by the assistant. A `go.mod` file was **not found at repository root on main**; do not presume a Go module path is configured. Keep API-003 (package/module path) open. C++ Pinex remains unchanged.

**Next:** continue PERF-002 payload ownership/copy-lifetime and PERF-003 bounded async completions; performance remains first-class V1 requirement. No coding authorization.


## 18. PERF-002 / PERF-003 / API-001b follow-up review (2026-11-16)

**Baseline inspected:** `main` HEAD was exactly `0fe42d3f0ac634a5a1c9617d1e800b68a00fdf9f` before these documentation changes. `.codex/AGENT_CONTEXT.md` was not found (404); all requested remaining design documents were read.

**New design draft:** [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md). This contains **proposals, not new owner approvals**:

- **PERF-002 CLOSED / OWNER APPROVED A:** outbound independent snapshot before accepted async send returns, receiving directly into owned user-retainable payload with no extra receive copy. Borrowed callback-lifetime B and transfer C were not selected. Pooling only for proven-safe private internal buffers after evidence; no end-to-end zero-copy claim.
- **PERF-003 OWNER APPROVED A MODEL:** independently configurable per-Session and Server aggregate count ceiling for pending + queued + active request completions; new Request immediately rejected on exhaustion without closing Session. Independent incoming event-count cap remains an unapproved proposal. Suggested `1024` / `65536` are illustrative **unapproved** counts, not defaults. Existing Q-008/009/010 byte ceilings stay unchanged.
- **API-001b OPEN:** blocked callback makes prompt `OnClose` and strict per-Session callback serialization mutually incompatible. Proposed `Done()` reflects transport/accounting termination independent of arbitrary callback return; `OnClose` at-most-once on serial dispatcher. Forced-close Option A cancels unstarted callbacks promptly but creates an explicit exception to user-visible completion delivery; Option B retains bounded deferred completions but may retain closed workers indefinitely. **Owner must choose**; do not silently claim callback-delivery exactly once in all failure modes.
- **API-002 / API-003 OPEN:** timeouts/reconnect/Stop defaults and module path `github.com/majiddarvishan/neta-go` / package `neta` proposed only; no `go.mod` added.

**Next owner decision:** API-001b forced-close callback-delivery policy A vs B; then PERF-003 numeric caps and potential inbound event-count cap. PERF-002 A is CLOSED; PERF-003 A count-admission model is APPROVED. Existing owner decisions Q-001..Q-010, NAM-001, API-004 A and API-001a A remain unchanged. No Go/C++ source, test, benchmark, build, CI, deployment or operational change performed. Main has documentation commits only for this review.


## 19. PERF-002 owner approval and remaining work (2026-11-16)

**Owner approval just recorded:** **PERF-002 CLOSED / Option A (Safe Snapshot + Owned Receive)**; authoritative entry in [DECISION_LOG.md](DECISION_LOG.md). This is an **API semantics** decision, not implementation authorization. For accepted asynchronous Stream sends, one independent outbound payload snapshot is complete before successful return; the caller can then reuse its source. A callback-visible received payload is separately owned, directly filled where possible, may be retained after callback return and is never returned to a library pool. No end-to-end zero-copy promise; no concurrent mutation during the outbound snapshot.

**Synced docs:** [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md), [PINEX_GO_PORT_TASKS_2026-11-16.md](PINEX_GO_PORT_TASKS_2026-11-16.md).

**Remaining design questions (not approved):**

1. **PERF-003:** approve count-bounded completion-credit admission for all pending/queued/active callback-based requests, Server aggregate cap, and incoming callback event-count cap. Rejection of *new* SendRequest on count exhaustion leaves Session connected, separate from Q-008 byte-based Queue Full. Incoming application event overflow follows already-approved Session-only close policy, but proposed count limit and numeric defaults require review. Illustrative 1024 per Session / 65536 Server are **NOT APPROVED**; no benchmark has run.
2. **API-001b:** choose forced-close Option A (prompt cancel of not-yet-executing user callbacks with at-most-once `OnClose`, `Done` closes independently) vs B (defer bounded terminal completion to worker, possible unbounded worker lifetime for permanently blocked user code), and callback registration API shape. Internal request result settlement once is distinct from invocation of user callback; do not claim both immediate `OnClose` and same-Session serialization under permanent user block.
3. **API-002:** decide request deadline origin (admission vs wire Write), timeout/reconnect/Stop defaults, pending cancellation and terminal errors. Preserve approved Q-006/007 idle heartbeat and Pinex wire deadline.
4. **API-003:** Go module path still undecided; no `go.mod` creation. C++ develop HEAD and golden-wire vectors must be pinned before authorized implementation/testing.

**Current guardrail:** docs only. No Go/C++ code, tests, benchmarks, builds, live operations or module changes performed. Latest source HEAD before this documentation sequence was `08fb5df8f582d7d91c53308988f31906a68ac557`.


## 20. PERF-003 A owner-approved model and API-001b follow-up (2026-11-16)

**Baseline verified before these documentation edits:** `main@52a126617fbe194c171926cf21fe2fb13893238b`. The owner chose **PERF-003 Option A** in the subsequent conversation. **Authoritative approved policy:** [DECISION_LOG.md](DECISION_LOG.md) section PERF-003. Previous historical notes describing the entire PERF-003 model as OPEN are superseded.

### Current contract truth

- **PERF-002:** CLOSED, owner-approved A: snapshot outgoing before successful async Send return; direct owned/retainable incoming payload, no reuse after callback handoff.
- **PERF-003:** **Option A MODEL APPROVED**. Bound by count: active outstanding Request waiting for response, queued completion callback, running completion callback; independently configurable per Session and Server aggregate with API-004 Functional Options. New request exceeding either limit rejected promptly, without sending, orphan pending state or disconnect. No per-Request goroutine or unbounded Completion queue. The existing Q-008 outgoing byte Queue Full and Q-009/Q-010 incoming/global queue and overload policy remain binding.
- **PERF-003 details still OPEN:** final numeric defaults (the sample 1024 Session / 65536 Server are NOT approved), optional separate **incoming callback event-count cap** and precise error identifiers, memory ownership of active/partial frames, forced-close credit release and timing. Pooling/benchmarks not authorized.
- **API-001b still OPEN:** the same-Session serialization rule makes unconditional immediate `OnClose` impossible behind an indefinitely blocked user callback. A proposed `Done` signal indicates prompt internal transport/resource closure independently of callback progress, **not owner approved**. **Choice A:** cancel not-yet-running callback work at forced Close and best-effort `OnClose`, favoring bounded teardown but allowing a request completion not to invoke the user callback. **Choice B:** defer a bounded set of terminal callbacks, improving delivery if callback progresses but permitting closed worker/queue retention indefinitely when blocked. Neither approved. Settle outcome internally once, regardless of callback delivery. Avoid conflating API-001b forced-close A/B with PERF-003 A (already approved).
- **API-002:** request deadline start (admission vs full Write), Connect/Bind/Reconnect/Stop defaults OPEN. **API-003:** module/package path OPEN. Go/C++ source untouched.

### Design records synchronized

- [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) §10 includes quota reservation, release, race handling, and API-001b alternatives.
- [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md), [NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), and [PINEX_GO_PORT_TASKS_2026-11-16.md](PINEX_GO_PORT_TASKS_2026-11-16.md) reflect approval.

**Next owner decision:** choose **API-001b forced-close Option A vs Option B** after explicit tradeoff explanation. Do not finalize without approval. Afterward review PERF-003 numerical count defaults and inbound event-count cap. No code, go.mod, tests, benchmarks, builds or operational changes. Branch `main` docs only.
