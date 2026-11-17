# Project Handoff — neta-go / Pinex Compatibility

> **CANONICAL REVISION (2026-11-17):** Q-003-R1, PERF-002 A, PERF-003 A model, API-001b-FC-A, N-ORD2, DONE-A, C-ORD-B and WORKER-001..009C are owner approved. The three callback-class pools, fairness, lazy retirement, bounded admission and retained-byte accounting remain as frozen in the design records. PROTOCOL-001 fixes Pinex's maximum Frame at 1 MiB total including the 10-byte Header. **Phase 2 Wire Codec is now implemented and verified:** Go CI and a compiled C++ 1.5.0 production-serializer golden compatibility workflow both PASS against pinned source commit `f7bd1eb779baff0d10e624f20522df345e6ee747`. The source snapshot and fixture generator are retained under `testdata/pinex-1.5.0/`. Live TCP Go↔C++ interoperability was not included in Phase 2 and requires a separately authorized integration phase. Phase 3 transport/runtime, workers, reconnect, benchmarks and PERF-004 remain future work and are not yet authorized.

**Updated:** 2026-11-17  
**Repository:** majiddarvishan/neta-go  
**Branch:** main  
**Implementation status:** PHASE 1 COMPLETE / CI PASS; PHASE 2 WIRE CODEC COMPLETE / GO CI + PINEX C++ GOLDEN CI PASS; PHASE 3 SESSION TRANSPORT/LIFECYCLE AUTHORIZED / IN PROGRESS  
**Current work:** Phase 2 Wire Codec is COMPLETE. Verified implementation commit: `b026dadd9ad0b33118b05f49477e1412a4255978`. Go CI `44044273742` and Pinex C++ compatibility CI `44044273818` both PASS. The compiled C++ fixture uses pinned production PDU serializer headers; its header writer mirrors the audited Pinex header function. Live TCP Go↔C++ interoperability has not been run. **Phase 3 Session Transport & Lifecycle Engine is now owner-authorized (2026-11-17) and in progress.** Phase 4 resource ledgers, Phase 5 worker scheduler, Phase 6 Client/Server orchestration, live socket interoperability, benchmarks, release and deployment remain outside this authorization.

## 1. Repository state

- Default branch: main
- Verified remote `main` HEAD at Phase 3 authorization: `ebf269dc9ba1a8f5d88922cf1e6ab29c99782a05`.
- `.codex/AGENT_CONTEXT.md`: not present on `main` at the verified HEAD.
- Verified baseline before WORKER-001 documentation updates: 525487737c1b75c62d021a391a395e03c1aadb1e
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
- Source-audited develop HEAD on 2026-11-17: `f7bd1eb779baff0d10e624f20522df345e6ee747` (CMake version 1.5.0); re-pin/re-audit before implementation or cross-language testing.
- Main is not the compatibility target for this work.

## 3. New planning documents

- PINEX_GO_PORT_TASKS_2026-11-16.md — task breakdown, dependencies, acceptance criteria, open decisions.
- PINEX_WIRE_PROTOCOL_2026-11-16.md — current Pinex 1.5.0 wire contract and differences from the uploaded Go sample.
- NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md — design-only draft of public API, config, session lifecycle, concurrency, idle keepalive, request tracking and backpressure; Q-006 through Q-010 policy approvals are reflected.
- NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md — APPROVED API-004 A: Functional Options, complete internal defaults, independent explicit overrides including idle 0, validated immutable per-instance config.
- NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md — draft Client/Server/Session Public API; API-001a owner approved A (nonblocking/callback-first). API-001b, ownership and callback termination semantics remain OPEN.
- NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md — owner explicitly requires performance as first-class V1 concern; measurement matrix, copy/allocation and bounded completion design work (PERF-001..003), no numeric targets approved.
- NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md — Q-008 outgoing 4 MiB/Fail-Fast, Q-009 B incoming overflow close, and Q-010 A configurable inbound/global/session/pre-bind/control defaults are approved. API-004 design remains open.
- NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md — WORKER-001 owner-approved Client/Server instance-scoped worker ownership; WORKER-002 active-worker ceiling topology is the next open decision.
- DECISION_LOG.md — authoritative approved decisions Q-001 through Q-010 (as applicable), plus NAM-001; distinguishes remaining open API details.
- SERVER_REVIEW_2026-11-16.md — original static review of the uploaded server sample.

## 4. Confirmed protocol facts

- 10-byte header: total length BE u32, command ID u8, command status u8, sequence BE u32.
- Maximum accepted inbound Frame in pinned Pinex 1.5.0: 1,048,576 bytes total including the header; maximum Body 1,048,566 bytes. Go must enforce this maximum in both directions.
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

- GO-PINEX-001: protocol source baseline pinned to Pinex develop `f7bd1eb779baff0d10e624f20522df345e6ee747`; Maximum Frame Size verified as 1 MiB total including Header; owner approved Q-001 Option A. Golden-frame artifacts and socket-level cross-language confirmation remain open.
- GO-PINEX-002 through GO-PINEX-014: open.
- Cross-language tests have not been run.
- No implementation has been started.

## 7. Owner decisions

- **Q-001 — APPROVED (Option A, 2026-11-16):** Pinex-compatible Bind Validator callback based on System ID and actual remote address. No password/token on the wire, no change to Bind PDU layout. Accepted Bind => rok 0x00; rejected => rfail 0xFF. This is not cryptographic identity authentication. See DECISION_LOG.md.
- **Q-005 — APPROVED (Option B, 2026-11-16):** If no Bind Validator callback is supplied, allow an otherwise-valid Bind by default, matching Pinex C++. Duplicate `system_id`, malformed Bind, invalid state and admission limits are still enforced. If a callback is provided, enforce its allow/reject result. This is permissive admission, not authenticated peer identity. See DECISION_LOG.md.
- **Q-002 — APPROVED (2026-11-16):** Configurable, idle-only Enquire Link on Go Client and Server. Send automatically only after no successful frame exchange in either direction for a positive idle interval; `0` disables automatic sending, not replies to peer Enquire Link requests. Never send a second probe while one is outstanding; protocol bytes unchanged. Numeric idle default is **30s** (Q-006); matching-probe response timeout default **10s** and failure closure are approved (Q-007). See DECISION_LOG.md.
- **Q-003 — REVISED BY Q-003-R1 (2026-11-16):** Goroutine-safe Client/Server/Session APIs; **configurable bounded parallel Application callbacks per Session (1 = serial)** with FIFO dispatch selection; callback completion may be out-of-order at >1. Cross-Session isolation, frame-safe concurrent sends and controlled shutdown remain. Internal protocol/timer/Close engine proceeds independently of user callbacks. Exact concurrency default and user-facing notification callback lanes remain OPEN. Implementation details (queue capacity, deadline, API signatures) still need design. See DECISION_LOG.md.
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

## 21. Q-003-R1 owner revision: parallel callbacks + independent internal engine (2026-11-16)

**Repository:** `github.com/majiddarvishan/neta-go` / `main`. **Last verified HEAD BEFORE this documentation sequence:** `4f12ce80e28e8fd2fb56a626058bbc7132441207`.

**Owner action:** explicitly rejected old Q-003 mandatory serial same-Session processing as wrong and selected **Option B: configurable parallel Application callbacks**, with `Concurrency=1` providing serial application handling. Asked whether the library's *internal* callbacks/events can remain independent of blocked user callbacks. **Answer: yes** — isolate internal protocol and lifecycle state transitions from arbitrary user handler execution.

### Current approved semantics

1. **Q-003-R1 CLOSED / OWNER APPROVED:** per-Session, bounded, configurable positive max simultaneous Application callbacks; `1` serial and >1 concurrent; incoming queue dispatch selection FIFO, completion ordering unconstrained in parallel mode. TCP Wire and single Writer remain unchanged.
2. Internal Control (Bind/Unbind/Enquire Link), Reader/Writer, Request/Response correlation, Timers, internal once-only terminal state and transport Close/Done settlement **cannot depend on return of Application callback**, regardless of concurrency. This does not mean arbitrarily blocked **user** callbacks can be killed.
3. Q-009/Q-010 remain active: per-Session 4 MiB waiting incoming queue + 256 MiB aggregate Server queued Application budget; saturation closes only offending Session. A maximum of N executing user callbacks creates extra active-payload/stack resource cost beyond those waiting queues; separate accounting and lifecycle-safe global concurrency caps are **OPEN** design.
4. PERF-002 A snapshot/owned receive and PERF-003 A bounded outstanding Request/Completion **model** remain approved. PERF-003 counts (1024 Session / 65536 Server) remain unapproved examples.

### Open API-001b design choices — do not finalize without owner

- `OnClose` and per-Request completion handlers are **user callbacks**, not internal engine logic. Consider a **separate, bounded user notification dispatcher** isolated from Application callback workers, so `OnRequest` blockage does not automatically stall these notifications. A stuck notification callback still cannot be forcibly stopped, and resource bounds/forced-close behavior must be explicit.
- Old API-001b Forced Close Option A (cancel not-yet-started callbacks) versus B (retain bounded terminal callbacks for eventual delivery if work progresses) remains **OPEN** and must be reconsidered in the parallel-handler design. Owner's **Q-003-R1 Option B** is NOT an approval of forced-close Option B.
- Choose callback concurrency **default**, max validation, optional global running-handler budget and behavior if previous Session's user callback remains blocked after forced Close / reconnect; avoid accumulated orphan workers. Choose event count cap for empty incoming frames (still open).
- Decide whether `Done()` should signal prompt internal transport closure separately from `OnClose` callback, callback lane ordering, forced-close terminal delivery guarantee, and API-002 Request timeout start. All remain **OPEN**.
- Library name and repository path approved; Go module/package path still API-003 OPEN.

**Docs updated:** [Decision Log](DECISION_LOG.md), [Public API design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [Public API/Config draft](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md), [Resource Admission draft](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md), [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md), [PERF-002/PERF-003/API-001b](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [Tasks](PINEX_GO_PORT_TASKS_2026-11-16.md).

**No implementation authorization:** documentation-only. No Go/C++ source code, `go.mod`, tests, benchmarks, CI or operations. Further owner decision only needed for API-001b user notification/forced-close semantics and callback concurrency defaults/aggregate resource caps.

## 22. API-001b-N1 owner-approved notification isolation (2026-11-16)

**GitHub initial checked HEAD:** `d68b04a7e11bfa53ca8d355b7b2bb8513b743ff8` on `main` before this documentation-only sequence.

**Owner confirmation:** explicitly approved making **`OnClose` and per-Request Completion user callbacks independent from Application `OnRequest` workers through a separate bounded user Notification dispatch lane**. Binding decision in [DECISION_LOG.md](DECISION_LOG.md) § `API-001b-N1`.

### Current architecture authority

- **Internal Protocol/Lifecycle:** independent of all arbitrary user callbacks: TCP codec Reader/Writer, Bind/Unbind/Enquire Link, pending response matching, timeout, internal once-only Request settlement and prompt resource/accounting close. No network/protocol goroutine blocks while invoking user functions.
- **Application callback lane:** `OnRequest` with **owner-approved Q-003-R1** configurable bounded concurrency per Session. `1` = serial; >1 permits parallel callbacks and non-ordered completion. Bounded inbound waiting queue preserves Q-009/Q-010 overflow (Session-only close).
- **Notification callback lane:** `OnClose`, per-Request Completion user functions dispatched separately from Application workers, with **bounded work and concurrency**, cannot prevent internal engine teardown. A stuck notification callback cannot be forcibly terminated and can stall its own worker; not a promise of guaranteed notification delivery.
- **Owner-approved other contracts:** API-001a A nonblocking callback-first request, API-004 A functional options, PERF-002 A independent outbound snapshot + incoming owned retainable payload, PERF-003 A configurable bounded pending/completion counts per Session and Server, Q-008/009/010 admission. No change to Pinex 1.5.0 wire.

**Historical snapshot:** the following open-question list predates owner approvals API-001b-FC-A, N-ORD2, DONE-A and C-ORD-B; it is retained only as a record of the earlier state. Current open items are summarized at the top and in §§25–27.

### Explicitly OPEN — next chat should continue here

1. **API-001b forced-close choice:** A (cancel queued/unstarted notification callbacks promptly) vs B (defer bounded terminal events until user callbacks return, at potential memory/worker retention cost). User did **not** select either. Reevaluate in light of independent notification lane; decide exact semantics for admitted Request callback delivery when Session closes, and `OnClose` lifecycle ordering.
2. **Notification execution contract:** one shared bounded lane vs further lanes, notification ordering/fairness, worker/concurrency and queue/count budgets, blocked notification-worker behavior. Only domain separation and boundedness approved; **no numeric default** or exact public signature approved.
3. **Q-003-R1 concurrency default:** user approved configurable `1` serial or >1 parallel; default and any Server-wide active callback guard still OPEN. Must bound active payloads separately from waiting queue bytes.
4. **PERF-003** numeric completion ceilings: earlier 1024/Session and 65536/Server illustrative **not approved**; inbound callback event-count cap proposal OPEN.
5. **API-002** Request timeout origin (admission vs Write), connect/bind/reconnect/stop defaults OPEN; **API-003** Go module/package path OPEN. Exact Pinex C++ `develop` SHA and golden vectors to pin before authorized implementation.

**Updated docs this turn:** `.codex/DECISION_LOG.md`, `NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md`, `NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md`, `NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md`, `PINEX_GO_PORT_TASKS_2026-11-16.md`, and `HANDOFF.md`.

**Strict guardrail:** Design and documents only; no source code, Go module, tests, benchmarks, builds, deployment or operational changes. Never silently finalize the remaining API-001b options; ask owner only for essential material choice.

## 23. Historical API-001b-R2 forced-close / Notification capacity review (superseded by §§24–25)

**Historical status at the time:** the GitHub baseline was `main@893b96629b69b0c67b5aaf456b3e0f178e9fc811`. The comparison below predates the subsequent owner approvals of Forced Close A and N-ORD2 recorded in §§24–25. Full engineering matrix and race/credit/worker discussion: [NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md). Decision-log trace: [DECISION_LOG.md](DECISION_LOG.md) § API-001b-R2.

**Historical note:** this statement described the state before the owner later approved API-001b-FC-A and API-001b-NORD2. The other owner-approved contracts listed here remain authoritative.

**Historical recommendation:** Forced Close A was recommended here and was subsequently approved in §24. The B comparison is no longer an open choice. N-ORD2 was subsequently approved in §25; Done semantics were subsequently resolved in §26; C-ORD-B is subsequently resolved in §27. Numeric capacities remain open.

**Historical next-decision prompt:** A/B and N-ORD1/N-ORD2 have since been resolved by owner approval. Done and within-Completion concurrency were subsequently resolved in §§26–27. Numeric worker/event caps, PERF-003 values, API-002 and API-003 remain open.

**Guardrail unchanged:** no code/go.mod, tests, benchmarks, CI, builds, deployments, operational actions or new owner decision inferred from this document.

## 24. Historical API-001b-FC-A approval and N-ORD1/N-ORD2 comparison (superseded by §25)

**Verified baseline before documentation commit:** `main@4183d182264d768f8c2856d8cead22b37f758c63`. Owner **APPROVED API-001b Forced Close Option A**; previous `API-001b-R2` comparison was historical and A/B selection is no longer open. Binding authority: [Decision Log](DECISION_LOG.md) § `API-001b-FC-A`.

**On Forced Close:** internal Request result settles exactly once; not-yet-started queued `OnRequest` and per-Request Completion callbacks are cancelled, with queued library-owned payload/metadata/credits released. A Completion already queued after a successful response can still be cancelled if its invocation has not started. Already-running user callbacks cannot be interrupted and may outlive Session closure. Transport/pending/timer/registry cleanup must not wait. Completion user callback execution is at-most-once, **not** unconditionally guaranteed for every admitted Request. `OnClose` is best effort/at-most-once. Exact `Done`/OnClose ordering and callback overlap are still **OPEN**.

**Historical note:** this section was written before the owner selected N-ORD2. The A/B Forced Close policy remains binding, but its N-ORD1/N-ORD2 comparison and next-decision prompt are superseded by [§25](#25-api-001b-nord2-approval-and-worker-done-design-handoff-2026-11-16).

**Implementation guardrail:** documentation only; no Go/C++ code, go.mod, test, build, benchmark, CI, or operations. Continue from actual GitHub main HEAD, not historical handoff SHAs.

## 25. API-001b-NORD2 approval and Worker / Done design handoff (2026-11-16)

**Verified baseline before documentation commit:** main@45485f7c23390c2689469c959a24ee4588b4a800. Owner explicitly **APPROVED N-ORD2**. Binding authority: [Decision Log](DECISION_LOG.md) § API-001b-NORD2.

**Approved Notification contract:** Completion and OnClose execute in distinct, independently bounded sublanes. OnClose may execute concurrently with an already-running Completion callback for the same Session; it does not wait for Completion return. A single unpartitioned Notification worker pool with no reserved OnClose capacity would violate this decision. OnClose remains best-effort/at-most-once. Forced Close A still cancels not-yet-started Completion events and does not terminate already-running callbacks.

**Worker design (proposal only):** separate bounded Completion and OnClose worker capacity per owning Server/Client instance; Completion event queue uses the already-reserved PERF-003 per-Session + aggregate Request completion credits; OnClose has its own at-most-one-per-Session event and independent global active+queued event budget. Retain active-worker charges for callbacks surviving Session closure/reconnect. Prefer bounded lazy worker pools and fair ready-Session scheduling; never one goroutine per event. Historical candidate (superseded by §27): at most one active Completion callback per Session. The owner subsequently approved configurable same-Session concurrency under C-ORD-B; see §27. FIFO start selection remains by terminal-settlement enqueue order, while callback return order is not guaranteed for concurrency greater than 1. Numeric worker counts, event caps, fairness and per-Server vs process-wide aggregate are **OPEN**.

**Done design:** the preceding Done proposal has been approved as API-001b-DONE-A; see §26. The read-only Session Done() <-chan struct{} closes after internal teardown and immutable terminal-cause publication, never waits for user callbacks, and gates OnClose eligibility. A running Completion retains its worker and PERF-003 credit until return; OnClose may overlap it under N-ORD2. The exported terminal-cause accessor and Close return semantics remain OPEN.

**Next work:** finalize numeric Completion concurrency default and worker/event capacity dimensions, aggregate scope, OnClose saturation, Completion payload-byte/frame-size bounds, PERF-003 count/event limits, API-002 timeout/Stop/reconnect and API-003 package/module. Do not infer numeric values from earlier examples.

**Guardrail:** no Go/C++ code, go.mod, tests, builds, benchmarks, CI or operations. All worker values remain proposals; no numeric defaults were invented.

## 26. API-001b-DONE-A — Owner-approved Session Done contract and next Worker design (2026-11-16)

**Owner decision:** Option A is **APPROVED**. Binding source: [DECISION_LOG.md](DECISION_LOG.md) § API-001b-DONE-A.

**Contract:** Session exposes a read-only **Done() <-chan struct{}** signal. It closes exactly once after admission stops, pending Request outcomes settle once, on a Forced Close path, unstarted Application/Completion callbacks are cancelled under Forced Close A; transport and internal Reader/Writer/control loops stop, timers/registry/internal accounting are cleaned up, and terminal cause is immutable. Done never joins or waits for any running Application, Completion or OnClose callback. An admitted OnClose event is eligible only after Done closes; under N-ORD2 it may overlap a Completion callback already running for the same Session. OnClose remains best-effort/at-most-once.

**Still open:** terminal-cause accessor name/signature; Close return semantics; numeric Completion concurrency default; worker/event counts and aggregate scope; OnClose event saturation; Completion payload-byte/frame-size bounds; PERF-003 numeric limits and optional incoming event-count cap; API-002 and API-003. C-ORD-B policy itself is closed.

**Worker design recommendation (not approved as API defaults):** three separate bounded callback classes (Application, Completion, OnClose); per-Session FIFO work queues feeding finite bounded owner-instance worker capacity; fair ready-Session scheduling; no goroutine per event; no global/registry lock while invoking user code. Keep the resource ledgers distinct:
- Application waiting frames: Q-009/Q-010 byte caps; active handlers: Q-003-R1 per-Session concurrency plus a still-open Server aggregate active-worker cap.
- Completion lifecycle: PERF-003 per-Session + aggregate Request credits reserved before SendRequest acceptance and held through pending, queued, and running callback; active Completion Worker permit is a separate physical-execution cap.
- OnClose: at most one event per Session, independent event admission and active Worker capacity; it never consumes Request completion credits. Done closes even if OnClose cannot be admitted.
- Running callbacks from retired Session generations continue consuming the owning Client/Server worker capacity until return, including after reconnect.

**Next decision:** select the within-Completion concurrency/ordering contract. Recommended alternatives and consequences are recorded in [the detailed review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md) §11.8. All numeric limits remain OPEN.

## 27. API-001b-CORD-B approval — configurable Completion concurrency (2026-11-16)

**Owner decision:** C-ORD-B is **APPROVED**. Binding authority: [DECISION_LOG.md](DECISION_LOG.md) § API-001b-CORD-B.

**Contract:** positive configurable maximum concurrent Completion callbacks per Session; 1 means serial, values greater than 1 permit overlap. The Session dispatcher preserves FIFO start selection by serialized per-Session terminal-result settlement/enqueue order, not SendRequest admission order. Callback return order is not guaranteed for concurrency greater than 1. Shared mutable state used by concurrent Completion callbacks is the application's synchronization responsibility. Per-Session concurrency is intersected with a finite aggregate active Completion-worker ceiling; no goroutine-per-Request.

**Still open:** numeric default and public option name/range; active worker ceilings and aggregate scope; Completion queue/event and payload-byte/frame-size bounds; OnClose saturation; terminal-cause accessor and Close return behavior; API-002 Stop/drain/reconnect and API-003 module/package.

**Guardrail:** design/docs only. No Go/C++ implementation, tests, benchmarks, builds, CI, dependencies or operations.


## 28. WORKER-001 — Owner-scoped Worker capacity and next design decision (2026-11-16)

**Verified baseline before this documentation sequence:** `main@525487737c1b75c62d021a391a395e03c1aadb1e`.  
**Owner decision:** WORKER-001 **Option A APPROVED**.

### Binding contract

- Each Client instance and each Server instance owns an independent callback-worker resource domain.
- All Sessions of one Server share its Server-owned budgets.
- All reconnect generations of one Client share its Client-owned budgets.
- A callback started by a retired Session generation remains charged to the owning instance until it returns; reconnect cannot reset the charge.
- Independent Client/Server instances do not share a hidden process-global worker pool or mandatory process-global active-worker cap.

This decision fixes only the worker **ownership scope**. It does not select numeric worker counts, physical pool layout, per-class aggregate-cap policy, queue/event limits, payload-byte bounds or OnClose saturation behavior.

### Synchronized design records

- [DECISION_LOG.md](DECISION_LOG.md) — binding WORKER-001 entry.
- [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) — consolidated resource ledgers; WORKER-001 and WORKER-002 are approved; WORKER-003 is the next open decision.
- [API-001b review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md), [PERF-002/003 design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [Public API design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md) and [Resource Admission design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md) updated consistently.

### Next decision at that time — WORKER-002 (SUPERSEDED / RESOLVED BY §29)

The owner subsequently approved explicit independent owner-wide active-worker ceilings for Application, Completion and OnClose. See §29 and the binding Decision Log entry.

**Guardrail:** documentation/design only. No Go/C++ code, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.

## 29. WORKER-002 — Independent owner-wide active-worker ceilings (OWNER APPROVED, 2026-11-16)

**Owner selection:** WORKER-002 Option A.

### Binding contract

Each Client and Server instance has three independently configurable aggregate active-worker ceilings:

- Application callbacks.
- Completion callbacks.
- OnClose callbacks.

Per-Session concurrency is intersected with the remaining owner-wide ceiling for the same class. Classes cannot borrow or consume one another's active permits. Ceilings count physically executing callbacks; queued/event populations are separately bounded.

WORKER-001 owner scope remains binding across all Sessions and Client reconnect generations. A started callback from a retired Session remains charged to the same owner/class ceiling until it returns. There is no mandatory process-global active-worker ceiling.

### Synchronized documents

- [DECISION_LOG.md](DECISION_LOG.md) — binding WORKER-002 entry.
- [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) — detailed capacity model and WORKER-003 proposal.
- API-001b review, PERF-002/003 design, Public API design, Performance Contract and Resource Admission design updated.

### Next decision — WORKER-003

Choose the OnClose saturation policy: skip a new OnClose callback when the owner's bounded active+queued event budget is full (recommended, consistent with best-effort delivery), or reserve an event slot at Session admission and apply backpressure to new Session/reconnect admission when no reservation is available.

Numeric defaults and public option names remain open. No implementation, tests, benchmarks, builds or operational changes.

## 30. WORKER-003 — OnClose event-budget saturation (OWNER APPROVED, OPTION A, 2026-11-16)

The owner selected Option A: when the owner's bounded OnClose event budget is full, skip the new OnClose invocation permanently.

### Binding behavior

- The owner-scoped event budget counts queued + running OnClose events; at most one event per Session.
- After Session Done closes, the engine makes one nonblocking reservation attempt.
- If full, skip OnClose with no retry, overflow goroutine, hidden queue or wait.
- An admitted event holds its event credit until callback return.
- Done, terminal cause, transport cleanup and reconnect are unaffected.
- Client reconnect generations share the same OnClose event budget.

### Next decision at that time — WORKER-004 (RESOLVED BY §31)

This was the pending choice when WORKER-003 was first recorded. The owner subsequently approved Option A; the binding result is recorded in §31 and DECISION_LOG.md.

No numeric event capacity or implementation authorized. No code, tests, benchmarks, builds or operational changes.


## 31. WORKER-004 — Three class-isolated worker pools (OWNER APPROVED, OPTION A, 2026-11-16)

The owner-approved WORKER-004 Option A is now registered in the Decision Log and synchronized design records. Initial registration commit: `fd3b4bdbe9ab424a444c237dd6017b99b60c3193`.

### Binding contract

- Every Client and Server owner has three independent pools: Application, Completion and OnClose.
- Each pool owns its own scheduler, ready-work queue, dispatch path, lock domain and wake-up mechanism.
- Worker creation is lazy. A worker is reused only within its own class. No goroutine is created per Request, frame or callback.
- A worker executes one callback synchronously. A blocked class cannot consume another class's workers or scheduler resources.
- Worker goroutine count per class is bounded by the corresponding WORKER-002 ceiling. Idle workers do not hold active-callback permits, but their goroutines/stacks remain resource usage.
- Internal protocol and transport processing remains outside all user callback pools.
- Shared internal worker-loop code is allowed; shared pools, queues, dispatch locks and wake-up state across callback classes are not.
- WORKER-001 owner scope, WORKER-002 independent class ceilings and WORKER-003 OnClose event saturation remain binding. No process-global pool or ceiling is introduced.

### Synchronized documents

- [DECISION_LOG.md](DECISION_LOG.md) — binding WORKER-004 approval.
- [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) — WORKER-004 contract and WORKER-005 proposal.
- [API-001b review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md).
- [PERF-002/003 design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md).
- [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md).
- [Public API design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md).
- [Resource Admission design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md).

### Next decision at that time — WORKER-005 (RESOLVED BY §32)

The owner subsequently approved finite idle retirement (Option B), with a zero warm-idle floor. The binding result is recorded in §32 and DECISION_LOG.md.

**Guardrail:** documentation/design only. No Go/C++ code, go.mod, tests, benchmarks, builds, dependencies, CI or operational changes.


## 32. WORKER-005 — Finite idle worker retirement (OWNER APPROVED, OPTION B, 2026-11-16)

The owner approved Option B: idle workers retire after a finite configurable grace period.

### Binding contract

- Worker pools remain class-isolated per WORKER-004 and workers are created lazily.
- After callback return, a worker with no dispatchable work may enter Idle and start its class-specific finite grace timeout.
- Work arriving before expiry reuses the idle worker. If the timeout expires with no work, the worker atomically removes itself from the pool count and exits.
- The default warm-idle floor is zero. A class pool may return to zero idle goroutines after sufficient inactivity.
- Application, Completion and OnClose idle timeouts are independently configurable. Numeric defaults/ranges are deferred to WORKER-009.
- A running or permanently blocked callback is never retired by the idle timeout; its active permit remains charged until actual return.
- Retirement never discards queued work or releases queue-byte, Request or event credits.
- Enqueue versus retirement has one linearization point; no lost wakeup, stranded ready Session or worker-count overshoot is permitted.
- No goroutine per timeout and no process-global reaper. Reusable timer/wait state is preferred.
- Owner shutdown/drain behavior remains API-002.

### Synchronized documents

- [DECISION_LOG.md](DECISION_LOG.md) — binding WORKER-005 approval.
- [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) — WORKER-005 contract and WORKER-006 proposal.
- API-001b review, PERF-002/003 design, Performance Contract, Public API design and Resource Admission design — lifecycle and scheduling constraints.

### Next decision at that time — WORKER-006 (RESOLVED BY §33)

The owner subsequently approved Option A: per-class round-robin, one callback per eligible Session turn. The binding result is recorded in §33 and DECISION_LOG.md.

**Guardrail:** documentation/design only. No Go/C++ code, tests, benchmarks, builds, dependencies, CI or operational changes.


## 33. WORKER-006 — Fair Scheduling Between Sessions (OWNER APPROVED, OPTION A, 2026-11-16)

The owner approved per-class round-robin scheduling, with one callback dispatch per eligible Session turn.

### Binding contract

- Application, Completion and OnClose each have an independent FIFO ready-Session rotation.
- A Session has at most one reusable ready node per class pool.
- Each turn dispatches one callback from the head Session's own FIFO. If work remains and a per-Session concurrency slot is free, that Session moves to the tail.
- If no work remains, its ready state is cleared. If its concurrency limit is full, it is temporarily ineligible and is reconsidered at the tail when a callback returns.
- Per-Session FIFO callback-start order is preserved. Completion follows C-ORD-B terminal-result settlement/enqueue order, not SendRequest admission order; parallel return order is unspecified.
- Fairness applies to continuously eligible Sessions within one owner/class. It does not promise equal CPU time or preempt callbacks.
- Pools, queues, locks and wakeups remain class-isolated. No lock is held during user code. No per-callback goroutine or per-turn allocation is required.
- Forced Close/start linearization and WORKER-001..005 remain unchanged.

### Synchronized documents

- [DECISION_LOG.md](DECISION_LOG.md) — binding WORKER-006 approval and WORKER-007 proposal.
- [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) — approved scheduler contract and WORKER-007 alternatives.
- [Resource Admission design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md) and [PERF-002/003 design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) — queue and credit accounting.
- [API-001b review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md), [Public API design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md) and [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md).

### Next decision at that time — WORKER-007 (RESOLVED BY §34)

The owner subsequently approved Option A: per-Session incoming/outgoing Application frame-count caps plus one aggregate Server count covering both directions. Completion and OnClose reuse their existing independent credits.

**Guardrail:** documentation/design only. No Go/C++ code, tests, benchmarks, builds, dependencies, CI or operational changes.


## 34. WORKER-007 — Queue Capacity and Event Capacity (OWNER APPROVED, OPTION A, 2026-11-16)

The owner approved per-Session incoming and outgoing Application frame-count ceilings plus one Server-aggregate queued Application frame-count ceiling across both directions. Incoming count saturation closes only the offending Session under Q-009/Q-010; outgoing count saturation returns Queue Full and keeps the Session. Outgoing frames remain counted through full Writer completion. Incoming Request count is released at callback start. Completion reuses PERF-003 Request credits; OnClose reuses WORKER-003 queued+running event credits. No duplicate Completion/OnClose event credit or shared cross-class budget is added. Existing byte limits and 4 KiB Control reserve remain independent. Numeric count defaults/ranges and public option names remain for WORKER-009/API-004.

### Next decision at that time — WORKER-008 (RESOLVED BY §35)

The owner subsequently approved Option A: separate owner-scoped retained-payload byte ledgers for Application and Completion classes. The binding contract is recorded in DECISION_LOG.md and Worker Capacity Design §15.

**Guardrail:** documentation/design only. No Go/C++ code, tests, benchmarks, builds, dependencies, CI or operational changes.


## 35. WORKER-008 — Resource Accounting and Payload Memory Ceiling (OWNER APPROVED, OPTION A, 2026-11-17)

The owner approved two independent retained-payload byte ledgers: Application and Completion. Each has per-Session and aggregate per-owner ceilings; Client reconnect generations share the same Client owner ledger. Application covers inbound Request storage from validated-header reservation through active OnRequest return/cancellation and outgoing private frame storage through full Writer completion. Completion covers matched Response storage from before body allocation through queued/running Completion return or Forced Close A cancellation. Queue byte/count limits remain independent sub-limits and are not added to retained-payload bytes as if they were separate physical allocations. Queue-to-active releases queue-only charges while the retained-byte charge follows the same buffer. Started callbacks remain charged until actual return; unstarted cancelled callbacks release once. No user-visible inbound buffer recycling.

**Completion-byte exhaustion — CLOSED / OWNER APPROVED OPTION A (2026-11-17):** when a valid, in-limit Response matches a live Pending Request but its full declared frame cannot reserve Session or owner Completion bytes, do not allocate its Body. Drain exactly the declared Body through one small fixed-size scratch buffer. If drain completes and no competing terminal event has won, invoke the normal Completion callback once with no Response payload and a typed terminal Resource Capacity error; keep the Session open and retain its existing Request credit until callback return/cancellation. If timeout/close/another terminal event wins during drain, it remains the sole settlement; do not invoke a duplicate callback. If drain fails due to EOF, timeout/read error or concurrent transport close, close the Session under the existing transport-error/Forced Close policy. Malformed or over-maximum frames use protocol-error handling. Exact read-timeout behavior remains API-002 work.

**Still open before Architecture Freeze:** golden-frame artifacts and Go/C++ cross-language confirmation against pinned Pinex HEAD; API-002/API-003 decisions. WORKER-009 numeric limits/defaults and hard validation ranges are closed. Maximum Frame Size is source-verified as 1 MiB total. No silent Response drop is allowed.

### Next decision — WORKER-009

WORKER-009A, WORKER-009B1 and WORKER-009B2 are CLOSED / OWNER APPROVED. B1 defaults: Application and Completion callback concurrency=1 per Session; Application, Completion and OnClose idle grace=30s independently. B2 count/event and Request-credit defaults are recorded in §38. Next owner decision: WORKER-009B3 retained-payload byte defaults.

**Guardrail:** documentation/design only. No Go/C++ code, tests, benchmarks, builds, dependencies, CI or operational changes.


## 36. WORKER-009A — CPU-derived worker defaults (OWNER APPROVED OPTION A, 2026-11-17)

The owner approved Option A. At Client/Server effective-configuration construction, capture P=max(1,runtime.GOMAXPROCS(0)) once. Resolve owner-wide active-worker defaults as Application=min(max(4,4P),128), Completion=min(max(2,2P),64), and OnClose=min(max(2,P),32). These are lazy worker ceilings, not eager goroutine counts. The resulting values remain immutable for the owner lifetime; later CPU quota/GOMAXPROCS changes do not resize a live instance. All other queue, count and retained-byte quotas remain fixed finite defaults, independently configurable. The formula caps are not hard maxima for explicit overrides; WORKER-009C will decide those.

**Next:** WORKER-009B3 — review retained Application and Completion payload-byte defaults. WORKER-009B2 is CLOSED / OWNER APPROVED; its six count/event defaults and saturation rules are recorded in §38 and DECISION_LOG.md.

**Guardrail:** documentation/design only. No code, tests, benchmarks, builds, dependency or operational changes.


## 37. WORKER-009B1 — per-Session concurrency and idle retirement (OWNER APPROVED, 2026-11-17)

The owner approved all five defaults: Application callback concurrency=1 per Session; Completion callback concurrency=1 per Session; Application idle grace=30s; Completion idle grace=30s; OnClose idle grace=30s. All are independently configurable. Concurrency 1 preserves serial callback execution by default; higher positive values remain configurable subject to the existing per-Session and owner-wide ceilings. Each idle grace starts only after callback return and when no dispatchable work exists; it never interrupts running callbacks. WORKER-005's zero warm-idle floor, class isolation and atomic enqueue/retirement rules remain binding. Explicit zero is not Unlimited or an immediate-retirement shorthand and is invalid for these positive settings.

**Next:** WORKER-009C — proposed hard maxima, valid ranges, cross-field constraints and semantic Functional-Option mapping. WORKER-009B2 and WORKER-009B3 are CLOSED / OWNER APPROVED.

**Guardrail:** documentation/design only. No code, tests, benchmarks, builds, dependency or operational changes.


## 38. WORKER-009B2 — count/event and Request-credit defaults (OWNER APPROVED OPTION A, 2026-11-17)

The owner approved all six proposed defaults as official V1 defaults, independently configurable through typed Functional Options:

| Setting | Approved default | Scope |
|---|---:|---|
| OnClose event credits | 1,024 | Per Client/Server owner; queued + running |
| Incoming Application frames | 256 | Per Session |
| Outgoing Application frames | 256 | Per Session, including Writer-owned frame until full Write/discard |
| Aggregate queued Application frames | 65,536 | Per Server, incoming + outgoing |
| Outstanding Requests | 128 | Per Session, full async lifecycle through Completion callback return/cancellation |
| Aggregate outstanding Requests | 32,768 | Per Client/Server owner; Client reconnect generations share the same budget |

Incoming local or aggregate count saturation closes only the offending Session. Outgoing local or aggregate count saturation returns Queue Full and keeps the Session open. OnClose saturation permanently skips the new event. Request-credit saturation rejects a new callback-based SendRequest before enqueue/transmission with a typed capacity error; it does not close the Session. SendNoWait and SendResponse do not consume Request credits. Completion reuses the PERF-003 Request credit; OnClose reuses the WORKER-003 event credit. Queue bytes/counts, active-worker permits, Request/Event credits and retained-payload bytes remain distinct admission ledgers, not additive physical-memory totals. Aggregate frame-count and Request-credit defaults are intentionally lower than the sum of their theoretical per-Session ceilings; cross-field validation must not force the aggregate values to equal or exceed those products. These values are defaults, not WORKER-009C hard maxima.

**Next:** WORKER-009C — owner review of proposed hard maxima, valid ranges, cross-field constraints and semantic Functional-Option mapping.

**Guardrail:** documentation/design only. No code, tests, benchmarks, builds, dependency or operational changes.

## 39. WORKER-008 Completion retained-byte exhaustion — OWNER APPROVED OPTION A (2026-11-17)

The owner selected Option A. For a syntactically valid, in-limit Response matched to a live Pending Request, if the declared full frame cannot reserve the Session or owner Completion retained-byte budget, the Reader must not allocate a Body-sized payload buffer. It drains exactly the declared Body through a small fixed-size scratch buffer. After successful drain, if the Request has not already been terminally settled, invoke its normal Completion callback once with no Response payload and a typed terminal Resource Capacity error; keep the Session open. The existing PERF-003 Request credit remains held through callback return or Forced Close A cancellation; no second Completion/event credit is created. A timeout/close/other terminal event that wins during drain remains the sole settlement. Drain/transport failure closes the Session under the existing transport-error/Forced Close policy and does not also emit the Resource Capacity completion. Malformed or over-maximum frames use protocol-error handling. The scratch buffer is bounded independently of Body length; exact read-timeout behavior remains API-002 work.

**Next:** WORKER-009B3 — review the four proposed retained-payload byte defaults; then WORKER-009C.

**Guardrail:** documentation/design only. No code, tests, benchmarks, builds, dependency or operational changes.

## 40. WORKER-009B3 — retained-payload byte defaults (OWNER APPROVED OPTION A, 2026-11-17)

The owner approved all four values as official V1 defaults:

| Setting | Approved default | Scope |
|---|---:|---|
| Retained Application bytes | 16 MiB | Per Session |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner |
| Retained Completion bytes | 16 MiB | Per Session |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner |

The limits are independent, finite and lazily enforced; they do not preallocate memory or define process RSS. Client owner budgets are shared across reconnect generations. Queue byte/count limits remain independent admission sub-limits and are not added to retained-payload bytes as separate physical allocations. Queue-to-active transitions release queue-only charges while retained-byte accounting follows the same payload. Started callbacks remain charged until actual return; user-retained inbound data after callback return is outside the library-owned ledger.

**Next:** WORKER-009C owner review.

## 41. WORKER-009C — hard maxima and validation (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

The owner approved all proposed WORKER-009C hard maxima, valid ranges, cross-field validation rules and semantic Functional-Option mapping.

| Setting | Hard maximum |
|---|---:|
| Application Workers / Owner | 512 |
| Completion Workers / Owner | 256 |
| OnClose Workers / Owner | 128 |
| Application Concurrency / Session | 32 |
| Completion Concurrency / Session | 32 |
| Worker Idle Grace | 10m (minimum 1ms) |
| OnClose Event Credits / Owner | 4,096 |
| Incoming Frames / Session | 1,024 |
| Outgoing Frames / Session | 1,024 |
| Aggregate Frames / Server | 262,144 |
| Outstanding Requests / Session | 512 |
| Outstanding Requests / Owner | 131,072 |
| Application Retained Bytes / Session | 64 MiB |
| Application Retained Bytes / Owner | 2 GiB |
| Completion Retained Bytes / Session | 64 MiB |
| Completion Retained Bytes / Owner | 1 GiB |

Count and ordinary byte limits have minimum 1; retained-byte limits have minimum 10 bytes. Validate after applying every Functional Option, and complete validation before creating sockets or goroutines. `EnquireLinkIdleInterval=0` remains the only intentional zero exception. Owner and Session limits are independent; aggregate limits may be lower than theoretical Session sums. Session concurrency intersects owner worker ceilings at dispatch. Queue, retained-byte, Request, OnClose and Worker Permit ledgers remain independent with no payload double-counting. All calculations/counters are overflow-safe. Limits are per Client/Server owner, not process-wide. Ten bytes covers only a header-sized frame. The 1 MiB total-frame maximum is source-verified against pinned Pinex C++ 1.5.0; it is a separate Protocol Compatibility fact, not a WORKER-009C setting.

**Typed Functional Options:** one independent option per semantic setting, mapped to Client/Server owner or Session scope as appropriate; aggregate queued Application frame count is Server-only; worker idle grace is separately configurable per class. Defaults remain when omitted; explicit values override only their own setting. Exact exported identifiers and constructor/Session placement remain API-001 work.

**Status:** CLOSED / OWNER APPROVED / OPTION A. Implementation remains NOT AUTHORIZED.


## 42. PROTOCOL-001 — Pinex 1.5.0 source baseline and Maximum Frame Size (SOURCE VERIFIED, 2026-11-17)

Pinned source snapshot: `majiddarvishan/pinex` branch `develop`, HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`; CMake declares version 1.5.0. In `session.hpp`, `receive_buf_` is `flat_buffer<uint8_t, 1024 * 1024>`; `command_length` includes the 10-byte Header and values above the fixed buffer capacity are rejected. Exact maximum accepted inbound Frame: **1,048,576 bytes total**, maximum Body **1,048,566 bytes**. The bound applies before Bind and after Bind. Pinex's send path does not preflight the same cap, but the receiving Pinex Session rejects over-limit frames; neta-go must enforce the limit on both inbound and outbound frames. Re-pin/re-audit the branch before future implementation or cross-language tests. Golden-frame and real socket-level Go↔C++ confirmation remain OPEN; no tests or code were run.


## 43. API-002 — Lifecycle, Timeout, Reconnect and Shutdown (DESIGN IN PROGRESS, 2026-11-17)

Detailed draft: `NETA_GO_API_002_LIFECYCLE_TIMEOUT_SHUTDOWN_DESIGN_2026-11-17.md`. It preserves all approved Done/Forced Close/Notification/Worker contracts and proposes context-aware terminal Client/Server Stop that waits for internal Done but never joins arbitrary callbacks. **API-002-01 CLOSED / OWNER APPROVED / OPTION A:** RequestTimeout=30s from successful local admission, including Queue/Writer/peer time. **API-002-02 CLOSED / OWNER APPROVED / OPTION A:** FrameAssemblyTimeout=30s absolute from first byte of each Frame Header on a Bound Session, not reset by progress; expiry closes only that Session. **API-002-03 CLOSED / OWNER APPROVED / OPTION A:** ConnectTimeout=30s absolute per Client Dial attempt, including DNS and TCP establishment; earlier caller Context deadline/cancellation wins; timeout creates no Session and does not enable reconnect. These settings are independently configurable and positive-only. Pre-Bind remains 30s absolute from TCP accept. Next owner decision: API-002-04 Bind response timeout. Unbind, write-stall, reconnect, Stop aggregation and API-001 surface names remain open. No implementation or tests authorized.


## 44. API-002-01 — Request Timeout (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- Default: **30s**, independently configurable per Client/Server owner with typed Functional Options; positive only; zero does not disable.
- Start: immediately after successful atomic local admission (Sequence + Request credit + queue frame/bytes + retained-payload reservation + FIFO insertion). Queue wait, Writer wait/Write and peer response all consume the deadline.
- If timeout wins before Writer claim: cancel queued frame; release queue and queued-payload charges exactly once; never transmit. Deliver one timeout Completion; retain Request credit/Sequence protection until Completion callback return or permitted cancellation.
- If Writer claim wins first: send ownership cannot be revoked; late Response is ignored; Writer-owned memory stays accounted until Write completion or transport teardown.
- Timeout alone does not close Session; timeout/Response/Close race has one terminal settlement winner.
- Rejected: start-after-full-Write and per-call-required timeout.

**Next owner decision:** API-002-04 Bind response timeout. Proposal: 30s absolute deadline from TCP establishment, before Bind Write, covering Bind Write and matching Bind Response wait. No implementation/tests authorized.


## 45. API-002-02 — Frame Assembly Timeout (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- **Default:** 30s absolute deadline per inbound Frame on a Bound Session.
- **Start:** first byte of that Frame's Header observed by Reader; one deadline covers remaining Header and full declared Body. Incremental progress never resets it.
- **Expiry:** close only the affected Session via existing terminal/Forced Close path. No byte skipping or stream resynchronization; other Sessions remain unaffected.
- **Partial Header:** retain only bounded parser/header storage until a complete valid Header exists; no Body allocation from incomplete/unvalidated length. Release partial parser state once on timeout.
- **Partial Body:** after Header validation, WORKER-008 full declared-frame retained-byte and applicable queue/count reservations remain held until full read or terminal cleanup. On timeout, release reservation and partial storage exactly once.
- Completion retained-byte failure's bounded scratch-buffer Body drain remains under the same original Frame Assembly deadline. If it expires, close Session and do not emit Resource Capacity Completion.
- Applies to Application and protocol/control Frames on Bound Sessions. No callback for an incomplete Frame.
- Independently configurable per Client/Server owner via typed Functional Options; positive-only, explicit zero invalid; accepted Server Sessions inherit the Server value. No additional hard maximum approved.
- The absolute 30s Pre-Bind handshake deadline from TCP accept remains unchanged and is not extended by Frame progress. Pinex C++ 1.5.0 has no separate equivalent deadline; no wire change.

**Next:** API-002-04 Bind response timeout. Proposal: 30s absolute from TCP establishment, covering Bind Write and response wait. No implementation/tests authorized.


## 46. API-002-03 — TCP Connect Timeout (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- **Default:** 30s absolute per Client Dial attempt.
- **Start/scope:** immediately before Dial; includes DNS/name resolution and TCP establishment. No DNS lookup outside the bounded context.
- **Context precedence:** caller cancellation always applies; an earlier caller deadline wins. If the caller has no earlier deadline, internal ConnectTimeout bounds the attempt.
- **Timeout outcome:** cancel only the current Dial attempt. No Session is created/published, no Bind is sent, and no Session Reader/Writer or callback worker starts. Close a socket returned concurrently after timeout has won.
- **Success:** if TCP establishment wins, proceed to the separate Bind handshake and its own deadline; ConnectTimeout is not reused.
- **Configuration:** Client-only typed Functional Option; positive finite duration; zero invalid; no additional hard maximum approved.
- This does not enable AutoReconnect or authorize retries. No wire change; no implementation/tests authorized.

**Next:** API-002-04 Bind response timeout. Proposal: 30s absolute from TCP establishment, before Bind Write, including Bind Write and matching response wait.


## 47. Consolidated Architecture Freeze decision register (2026-11-17)

All remaining owner choices are collected in [NETA_GO_ARCHITECTURE_FREEZE_DECISION_REGISTER_2026-11-17.md](NETA_GO_ARCHITECTURE_FREEZE_DECISION_REGISTER_2026-11-17.md): API-002-04..08, final API-001 public surface, API-003 module/package boundaries and PROTOCOL-001 adoption of the source-verified 1 MiB total-frame ceiling. The register is a proposal, not an approval. No implementation, tests, benchmarks, builds or dependency changes are authorized.

## 48. Architecture Freeze completed — 2026-11-17

**Owner approval:** Option A for API-002-04..08, API-001, API-003 and PROTOCOL-001.  
**Status:** ARCHITECTURE FROZEN — all fundamental owner decisions CLOSED / OWNER APPROVED.  
**Implementation:** NOT AUTHORIZED.

Binding final records: DECISION_LOG.md; NETA_GO_API_002_LIFECYCLE_TIMEOUT_SHUTDOWN_DESIGN_2026-11-17.md; NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md; NETA_GO_API_003_MODULE_PACKAGE_ARCHITECTURE_2026-11-17.md; NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md; PINEX_WIRE_PROTOCOL_2026-11-16.md; NETA_GO_ARCHITECTURE_FREEZE_AND_IMPLEMENTATION_ROADMAP_2026-11-17.md.

No fundamental owner decision remains open. Golden-frame vectors, bidirectional Go↔C++ socket verification, correctness/race/fuzz/soak tests, baseline benchmarks and evidence-driven PERF-004 are implementation validation tasks, not architecture blockers. Begin none of them until explicit implementation authorization. The previous §47 decision register is now CLOSED / OWNER APPROVED / OPTION A for all eight entries.


## Implementation Authorization — Phase 1 (2026-11-17)

Owner explicitly authorized Phase 1 in this conversation: «آره، اجازه داری». This supersedes the previous global NOT AUTHORIZED boundary only for Phase 1.

- **Authorized:** module skeleton (go.mod, root package neta), typed Client/Server/Session Functional Options, immutable effective configuration, approved defaults, apply-all-then-validate construction, WORKER-009C validation and checked arithmetic, public configuration errors/state declarations required by API-001, focused tests and CI.
- **Baseline before this authorization:** f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- **Current Phase 1 test-hardening commit:** 18bec96bb13cbdfa5a7819301b23d61e3fb3db75 (Server-side WORKER-009C maximum/minimum and final-option precedence tests; CI pending at time of this handoff update).
- **Explicitly not authorized:** Phase 2 Wire Protocol, transport/runtime, worker scheduler, reconnect, cross-language integration, benchmarks and deployment. Obtain separate owner authorization before starting Phase 2.
- No approved architecture decision is reopened.


## Phase 1 completion evidence — 2026-11-17

**Status:** COMPLETE / CI PASS.

- Module path and Go module: github.com/majiddarvishan/neta-go, Go 1.23.
- Sole public package: root package neta.
- Typed ClientOption, ServerOption and SessionOption with independent functional settings.
- Immutable effective configuration; all options are applied before final validation.
- Approved defaults and WORKER-009C maxima/ranges are enforced before any socket or goroutine is created.
- EnquireLinkIdleInterval=0 is the sole approved duration-zero exception.
- Owner and Session budgets remain independent; checked uint64 add/subtract/multiply helpers reject overflow/underflow.
- Nil typed options are rejected; Session option slices are snapshotted.
- TCP endpoint syntax is validated without opening sockets.
- Focused tests cover defaults, GOMAXPROCS worker defaults, zero rules, hard maxima, inclusive boundaries, Client and Server owner budgets, independent cross-field limits, final option precedence and arithmetic guards.
- Latest verified commit: cb3bdbc2964c1a0444cd40684117e1987f4cf092.
- GitHub Actions run 44040206052: PASS — gofmt, go vet, go test, go test -race.

**Next:** Phase 2 — pure-Go internal/wire and Pinex-compatible frame codecs. This phase remains NOT AUTHORIZED until the owner explicitly approves it. No Phase 2 source has been started.


## Phase 2 Authorization — Wire Protocol (2026-11-17)

The owner explicitly approved beginning Phase 2 by replying «ادامه بده» after being asked whether Phase 2 (Wire Protocol) was authorized. This approval is scoped to protocol implementation only.

**Authorized scope:** pure-Go `internal/wire` package; Pinex 1.5.0 10-byte frame header and Big Endian fields; frame encode/decode and size validation; Bind Request/Response with bind_type and C-octet system_id; Stream Request/Response raw body; Unbind and Enquire Link control PDUs; sequence fields/semantics; unit and golden-vector tests; documentation and CI.

**Compatibility reference:** Pinex C++ develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Re-pin/re-audit if the develop branch moves before interoperability testing.

**Hard protocol maximum:** 1,048,576 bytes total including the 10-byte header; maximum Body 1,048,566 bytes. The limit is fixed, not configurable, and applies inbound and outbound.

**Not authorized:** TCP transport/read-write loops, Client/Server runtime integration, lifecycle/shutdown/reconnect, worker scheduler, resource-admission runtime, cross-language socket test harness requiring C++ execution, benchmarks, release or deployment. Those belong to later phases and require separate owner approval.

No frozen architecture decision is reopened. Phase 1 remains complete with CI PASS. Phase 2 implementation must not import root package `neta` from `internal/wire`; the package uses only the Go standard library.


## Phase 2 — Wire Protocol implementation authorization (2026-11-17)

Owner approval was explicitly granted in this conversation by replying «ادامه بده» to the question asking permission to start Phase 2. This approval supersedes the earlier Phase-2-not-authorized wording, only for the scope below.

**Scope:** pure-Go `internal/wire`; Pinex 1.5.0 frame Header and complete Frame codec; Bind Request/Response; raw Stream Request/Response; empty Unbind and Enquire Link Request/Response; per-Session nonzero Sequence generator; fixed 1 MiB total frame bound; focused golden/boundary/concurrency tests.

**Compatibility source:** Pinex develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Wire fields are BE u32 length, u8 command, u8 status, BE u32 sequence; Bind body is u8 bind_type followed by NUL-terminated system_id (maximum 19 bytes); Stream body is raw bytes.

**Out of scope / not authorized:** TCP transport, read/write loops, Client/Server runtime integration, Session lifecycle, worker scheduling, resource admission runtime, reconnect, socket-level cross-language test execution, benchmarks, release and deployment. Those require later phase authorization.

Implementation must stay inside `internal/wire`, use only the standard library, and must not import root package `neta`. Root `neta` may depend on `internal/wire`.



## Phase 2 implementation evidence — 2026-11-17

**Phase 2 Wire Codec: COMPLETE** within the explicitly owner-authorized scope.

- Verified implementation commit and code/test baseline: `b026dadd9ad0b33118b05f49477e1412a4255978`.
- Go CI run `44044273742`: PASS — gofmt, go vet, go test, go test -race.
- C++ compatibility CI run `44044273818`: PASS — compile pinned C++ PDU serializer snapshot, regenerate/diff golden frames, run Go golden tests.
- Pinned Pinex source: version 1.5.0, commit `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Corrected Pinex's omitted/empty Bind Response Body semantics; Go now returns default `bi_direction` and empty system ID.
- Source snapshot provenance and original Git blob hashes: `testdata/pinex-1.5.0/source/SNAPSHOT.md`.

The C++ fixture generator uses actual production PDU serializer templates; its Header writer mirrors the audited private `session::serialize_header` implementation. The private Pinex repo cannot be checked out by neta-go Actions without a cross-repository credential, so the exact pinned source headers are snapshotted in this repo. This is compiled C++ golden compatibility, not live TCP/socket interoperability.

**Not authorized:** Phase 3 TCP transport/read/write loops, runtime integration, Session lifecycle, worker scheduler, resource admission runtime, reconnect, live socket interoperability, benchmarks, release and deployment. Obtain separate owner authorization before starting Phase 3.


## 20. Phase 3 authorization — Session Transport & Lifecycle Engine (2026-11-17)

The owner explicitly authorized Phase 3 in the continuation of chat pg_07 by replying «اره هست و مجوز داری» to the question asking permission to begin Phase 3.

**Authorized scope:** implement the private Session transport/lifecycle engine against the frozen API and protocol contracts:
- one Reader and one serialized Writer per Session; no user callback executes inline in either loop;
- internal Bind/Pre-Bind state transitions and handshake primitives, including the absolute Pre-Bind/Bind deadlines;
- absolute Frame Assembly and per-Frame Writer deadlines, plus idle-only Enquire Link state and matching response timeout;
- Request Sequence allocation/correlation, 30s deadline from successful local admission, exactly-once internal settlement, and late-response suppression;
- Graceful Unbind, Forced Close A, immutable terminal cause, Done publication, and exact internal teardown.

**Implementation boundary:** Phase 3 may use the already-approved per-Session FIFO/queue limits as the minimal bounded transport admission needed by the Writer. It must not implement the full independent queue/count/retained-byte/owner ledgers of Phase 4, the three-class worker scheduler of Phase 5, or Client/Server Dial/Listen/reconnect/session-generation orchestration of Phase 6. Callback dispatch is behind a private interface so Phase 5 can supply the approved isolated bounded workers. No public API expansion, new protocol behavior, third-party dependency, benchmark, release or deployment is authorized.

**Baseline:** remote main HEAD ebf269dc9ba1a8f5d88922cf1e6ab29c99782a05; Phase 1 and Phase 2 remain complete. Pinex compatibility baseline remains version 1.5.0 at f7bd1eb779baff0d10e624f20522df345e6ee747.

**Exit evidence required:** focused net.Pipe transport/lifecycle tests, fragmented and slow-trickle Frame tests, request timeout/late response/close races, partial writes and Writer timeout, Unbind FIFO/timeout, Bind/Pre-Bind handshake tests, gofmt, go vet ./..., go test ./..., and go test -race ./... in GitHub Actions. Live Pinex TCP interoperability remains Phase 7 evidence, not a Phase 3 gate.
