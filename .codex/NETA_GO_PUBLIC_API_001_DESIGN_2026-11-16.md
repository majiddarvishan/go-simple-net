# neta-go — API-001 Public Client/Server/Session and Callback Design

**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — API-001a APPROVED OPTION A (async callback-first), API-001b OPEN / API-004 APPROVED OPTION A  
**Repository:** `majiddarvishan/neta-go`, branch `main`  
**Implementation:** NOT AUTHORIZED. This is architecture/documentation; no Go source, go.mod, test, build, or repository rename.

## 1. Approved constraints and source authority

- Compatibility target: **Pinex C++ 1.5.0**, branch **develop**, as recorded in [PINEX_WIRE_PROTOCOL_2026-11-16.md](PINEX_WIRE_PROTOCOL_2026-11-16.md). Exact C++ HEAD and golden frames still need pinning before implementation/test.
- Protocol has a 10-byte Big Endian length/ID/status/sequence header; Bind (`0x01/0x81`), Stream (`0x02/0x82`), Unbind (`0x03/0x83`) and Enquire Link (`0x04/0x84`). Responses reuse request sequence; payload remains opaque bytes. Do not change wire layout.
- **Q-001/Q-005:** server Bind Validator is optional; absent means otherwise valid unique Bind accepted; duplicate System IDs rejected without evicting earlier active session. Validator may inspect actual remote TCP address; does not add password/token to wire.
- **Q-002/Q-006/Q-007:** idle-only automatic Enquire Link default 30s, configurable, explicitly 0 disables outbound probes; incoming requests always answered. Response timeout default configurable 10s; only matching sequence satisfies it; session closes on timeout; client reconnect only when explicitly enabled.
- **Q-003:** exported Client/Server/Session APIs are goroutine-safe; callbacks within one Session execute serially and in receipt order, with independent sessions. Concurrent Sends keep entire frames intact. Controlled Close and no duplicate terminal events.
- **Q-004:** lean protocol/messaging core; no business rate limiter, circuit breaker, message deduplicator, priority queue, required Protobuf/SS7, or Prometheus exporter.
- **Q-008/Q-009/Q-010:** outgoing application FIFO 4 MiB/Session configurable and immediate Queue Full error; incoming callback backlog 4 MiB/Session and close only offending Session when a new frame cannot fit; Server aggregate queued application budget 256 MiB; max 1024 bound sessions, 128 pre-bind slots and absolute 30s pre-bind deadline; per Session Control allowance 4 KiB. All independently configurable; global outgoing overflow fails send/keeps Session, global incoming overflow closes offender.
- **API-004 (CLOSED / approved A):** typed Functional Options, defaults internally applied; explicit overrides including idle 0 preserved; all options validated before starting Dial/Listen; resolved configuration privately snapshotted/immutable for each instance. No live hot reload requirement.

Everything after this heading that is marked **proposed** is **not** another owner decision.

## 2. Public surface — suggested minimal concepts, not exported code

| Concept | Responsibility | Approved or proposed |
|---|---|---|
| Client | Dial + Bind lifecycle, optional reconnect, own active Session, explicit Stop | required concept, signatures **proposed** |
| Server | Listen/accept, admission, Bind validator, registry by bound System ID, explicit Stop | required concept, signatures **proposed** |
| Session | Bound peer identity and remote address, stream request/response, Unbind/Close, Done/State | required concept, signatures **proposed** |
| Request | Incoming stream payload, original request sequence, peer Session reference | proposed public shape |
| Response | Incoming stream response payload, header status, original sequence | proposed public shape |
| BindInfo | System ID, bind type and actual peer remote address passed to validator | proposed public shape |
| ClientOption / ServerOption | Typed Functional Options for API-004, parsed before networking | **strategy approved**, exact identifiers proposed |
| Callbacks / Handler | OnBound, OnRequest, OnClose, request completion | proposed callbacks and event names |
| Error categories | invalid option/config, not bound, closing/closed, bind refused, duplicate ID, queue full, protocol invalid, request timeout, heartbeat timeout, remote close | proposed public error names |

Keep serialization, Codec, reader/writer loops, request tracker and system ID map implementation private. Avoid forcing the caller to use raw TCP sockets or codec abstractions.

### Suggested construction order

1. Create a Client or Server by applying typed Functional Options to its complete defaults and validating the effective configuration; invalid options return immediately with no network side effects.
2. Attach/choose handlers before Start/Connect, avoiding callbacks on partially initialized consumers.
3. Start Server Listen or Client Connect/Bind explicitly. The exact synchronous/async start contract and constructor names remain OPEN.
4. Use a bound Session for application sends; Stop/Unbind later closes Sessions and prevents new work/reconnect.

**No actual function signatures are finalized by this proposal.**

## 3. Per-session callback model (draft, derived from Q-003)

- For the **same Session**, user callbacks run **one at a time**, in the order of accepted incoming frames/events. Different Sessions can execute callbacks concurrently.
- Application request handler receives raw bytes and original Session-scoped Sequence; it may call a response send method from inside its callback without waiting for its own callback to return. No global transport lock is held across user callback execution.
- Bind Accepted/Rejected, Request delivery, Response completion and terminal Close must be coordinated through a deterministic event order. **Proposed**: Bound is delivered before first Application callback; Close is terminal and delivered once after already-running callback returns if possible.
- **Important failure case:** a callback blocked forever in user code cannot be killed by the library. It is not possible to promise both (a) exactly sequential callbacks and (b) immediate `OnClose` delivery behind a permanently blocked callback. **Recommended invariant:** promptly close transport/free tracked resources and signal a completion primitive (`Done`/state) even if a user callback is blocked; deliver `OnClose` at most once on the same serialized callback path *only if that path can progress*. Stop/Close must not wait without bound for user callback return. The exact public guarantee remains OPEN for API-001.
- Keep an incoming callback backlog bounded (Q-009/Q-010). On overflow, close **only** the offender; previously queued callback work must follow an explicit terminal/cancellation policy. Do not deliver a user callback concurrently just to force a close notification.
- Request timeouts and local heartbeats are tracked internally independently of user callback execution; ensure that a blocked callback does not prevent TCP shutdown or request bookkeeping release.

## 4. API-001a — Owner-approved asynchronous callback-first Request completion

### Option A — asynchronous, callback-first (APPROVED for V1)

- A nonblocking `SendRequest`-style action returns an immediately known submission outcome, e.g., Session not bound, Queue Full, or accepted Sequence. It must **not** claim the peer received or processed the message merely because local enqueue succeeded.
- The caller supplies a per-request completion handler or uses an agreed Session-level response handler. Exactly **one ownership path** for each request result: response, request deadline, transport failure, or close. Never deliver both per-request and global completion for the same request unless the API contract explicitly separates diagnostics from completion.
- Incoming Application `stream_req` is delivered through a per-Session `OnRequest`-style handler carrying the received sequence. Explicit `SendResponse(originalSequence, payload, status)` returns the response with the original sequence. No auto-response, no Wire change.
- A `SendNoWait`-style operation may emit a standard `stream_req` without registering a pending response. It must not invent a new PDU, guarantee remote processing, or silently deduplicate.
- Callback-based nonblocking requests avoid waiting inside a Session callback and fit the C++ Pinex callback-oriented model, but terminal completion event ordering and callback backlog costs still need a detailed contract.

### Option B — synchronous `Request(ctx, payload) → Response/error` as primary (NOT SELECTED)

- Expose context-aware, blocking request/response result to calling goroutines; retain `SendResponse` for peer-initiated requests. Internally complete the Context wait independently of the serial user callback worker, so a response does not depend on callbacks returning.
- Provide a separate one-way/no-wait send for callers that do not expect a response.
- Risk: a caller that synchronously waits for application progress inside a Session's own serialized `OnRequest` callback can cause self-inflicted deadlock at the application level. Specify and document limitations; the library must still keep transport shutdown bounded.
- A response must have a single terminal completion per request. Do not simultaneously promise a separate response callback for the same completion without a distinct policy.

**Owner decision:** the owner selected **Option A** for API-001a on 2026-11-16. A nonblocking SendRequest-style operation returns the **local admission result** without waiting for the network response; the later response, error or timeout completes exactly once through an agreed callback. The synchronous Context-first primary API (B) is **not selected** for V1. Exact callback shape and payload lifetime remain API-001b/PERF-002 OPEN. Both designs would preserve Pinex wire compatibility, but only A is approved.

## 5. Proposed request lifecycle and wire constraints

1. Sending an Application Stream request requires a **Bound** Session and valid Bind Type rights. Stream before Bound and Send during Unbinding/Closed return bounded, typed local errors; do not write invalid protocol frames.
2. A Session allocates a **nonzero**, monotonically increasing `uint32` request sequence and prevents reusing an in-flight value on wraparound.
3. Reserve pending request state before the frame becomes visible to the writer; if local admission fails, remove/rollback pending entry and do not leave a response timer.
4. On a valid matching `stream_resp`, correlate by sequence and deliver only the once-only completion. Status comes from header byte 5; preserve raw payload including binary zero bytes. Unknown/late replies need a defined diagnostic/drop policy, not automatic retry or dedup.
5. On request deadline, complete once with timeout; a late response must not resurrect completion. Close/timeout/response races have a single winner.
6. Inbound `stream_req` never triggers an implicit application response. The app may send `stream_resp` with same sequence and explicit status.
7. Queue Full means **not admitted locally**; accepted/enqueued means **not guaranteed delivered or remotely processed**. Error taxonomy must distinguish local admission, wire response status and transport failures.
8. The C++ `uni_direction` mode has requester-only enforcement at binding Client. Preserve this exact compatibility boundary and document public Send/Receive restrictions; do not reinterpret it as global server-level bans.

## 6. Proposed payload ownership and memory safety

- Receiving: Decode/copy payload into a user-readable `[]byte` whose lifetime covers the callback. Public docs must say whether caller can retain it; **recommended**: caller gets a distinct immutable-by-convention snapshot that it may retain. Avoid exposing pooled mutable buffers that are recycled before the callback ends or after return without warning.
- Sending: **recommended**: snapshot/copy user `[]byte` before returning successful queue admission, so later caller mutation cannot corrupt an on-wire frame; include resulting serialized bytes in the approved 4 MiB byte accounting. Alternatively explicit ownership transfer could be used, but must be documented. Neither exact ownership rule is approved yet.
- Slow callback/large payload must not bypass the approved incoming/global queue caps; include retention/working buffers in later capacity analysis beyond the queue-budget limit.
- No mandatory serialization format; the API should work with empty and arbitrary binary payloads.

## 7. Proposed Client/Server and Session lifecycle rules

| State | Operation | Proposed result |
|---|---|---|
| Created | Register options / handlers | Configure once before Start; then immutable |
| Connecting / Binding | Application Send | Error NotBound, no enqueue |
| Bound | Send Request or Response | Atomic quota admission; write serialized frame |
| Bound | Inbound Request | Serialized callback with sequence and payload |
| Bound | Inbound Control | Internal state machine responds with Pinex-compatible PDU |
| Unbinding | New Application Send | Error Closing; no enqueue |
| Closed | Send | Error Closed; no implicit reconnect from a call |
| Closed, Auto Reconnect enabled Client | Background reconnect | Only under separately approved/configured retry policy, not after explicit Stop |
| Any Session with unreadable/stalled TCP | Close/Stop | Transport and tracking close promptly; bounded drain/deadline, not contingent on arbitrary callback termination |
| Server Stop | New Accept/Bind | Refuse new work and close owned sessions within specified lifecycle deadline |

The exact method names, callback registration style, `Done` signaling, timer defaults, and reconnection event/session identity model remain **API-001/API-002** open details.

## 8. Proposed acceptance criteria (NOT tested)

- Two goroutines concurrently invoking Session Sends cannot corrupt frames, exceed per-session/global quotas or overwrite a pending sequence.
- Same-session callbacks execute in frame-receipt order without overlap; a slow callback in Session A does not block Session B.
- Callback can queue SendResponse safely; overflowing callback input closes only offender; output Queue Full returns promptly without close.
- No orphan pending request or duplicate completion after Queue Full, timeout, close or race; raw payload and status preserved over Go↔C++ TCP interoperability.
- Stop closes network/registries even with blocked user callback. It does not guarantee forced callback cancellation. Event/Done semantics documented.
- Client reconnect does not duplicate an already completed request or reuse old Session request tracking; no external resend implied.
- All approved configurations have default and override tests; user-visible exported API examples only after authorization to implement.

## 9. Decision ledger — what remains open

| ID | Decision | Recommendation | State |
|---|---|---|---|
| API-004 | Config representation | Functional Options + private effective defaults, validated before network use | **CLOSED — owner approved A** |
| API-001a | Request/Response completion style | **A: asynchronous/nonblocking submission + callback completion**; B synchronous primary API not selected | **CLOSED — owner approved A, 2026-11-16** |
| PERF-002 | Payload ownership | Outgoing snapshot before successful admission return; inbound directly received buffer retainable beyond callback | **CLOSED — OWNER APPROVED A** |
| API-001b | Callback/Close/Done | Serial callbacks, decide forced-close notification/cancellation policy | **OPEN — performance-critical design** |
| API-002 | Dial/Bind timeout defaults, request deadline, reconnect, graceful stop | C++ reference values are input, not automatically Go approvals | OPEN |
| API-003 | Go package/module path | Library name `neta-go` approved; no repo/module rename authorized | OPEN |
| GO-PINEX-001 | Pin exact C++ develop HEAD/golden frames | Ensure current baseline before implementation/tests | OPEN |

**Next design question:** **PERF-002 / API-001b** — `[]byte` ownership/lifetime (safe owned copies vs explicit callback-lifetime borrowed buffers) and bounded callback completion dispatch. Performance is a first-class V1 requirement; benchmark acceptance methodology is in [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md). No public API code has been created or authorized.

## 10. Performance-first requirement and follow-on design (2026-11-16)

The owner explicitly stated **performance is very important** while selecting API-001a **Option A**. This raises throughput, latency distribution (especially p99), allocations/op, copies per Frame, memory/GC pressure and contention across concurrent Sessions to **first-class V1 engineering evaluation criteria**, not guaranteed numerical targets.

**Detailed draft:** [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md). It covers reproducible benchmarks for varied payload sizes, single vs many Sessions, concurrent senders, Go↔C++ interoperability, callback stalls, queue saturation, churn and memory stability; none have been run.

### Performance-safe API-001a details

- Fast `SendRequest` submission must return a local queue-admission outcome, not pretend the peer acknowledged anything. Local Queue Full must fail promptly without pending tracker leaks.
- A response completion callback must have exactly one owner and be driven independently of the calling goroutine's wait state. Do not promise immediate callback delivery behind a user callback that never returns.
- Avoid a goroutine per Request or unbounded completion events; investigate bounded pending capacity and callback dispatch cost under **PERF-003**.
- Allocation/copy cost depends on `[]byte` lifetime. Under **PERF-002**, owner has **not** approved either safe-copy or borrowed buffers. Do not silently optimize into data races, use-after-recycle or an API that retains mutable caller memory without a contract.
- Reuse buffers or batch work only when validated by Go benchmark/race/soak tests and without changing Pinex frame format or per-Session callback order.
- The exact exported method names remain proposals; API-004 typed Functional Options is already approved and unaffected.

**API-001a CLOSED — Option A. API-001b/PERF-002 and PERF-003 remain OPEN. No implementation authorization.**


## 11. PERF-002 / PERF-003 / API-001b follow-up (2026-11-16; DRAFT ONLY)

See [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) for the nonbinding comparison of safe-copy/owned-receive, borrowed and transfer ownership; proposed count-bounded pending/completion admission; and the explicitly unresolved choice between prompt forced-close callback cancellation versus deferred bounded terminal events. Approved API-001a callback-first submission and Q-003 same-Session callback non-overlap remain unchanged. No API-001b/PERF-002/PERF-003 option or exported signature is approved by this cross-reference.


## 12. PERF-002 owner approval and API-001b next work (2026-11-16)

The owner **APPROVED PERF-002 Option A**: independent snapshot of caller's bytes for an accepted async Stream send before returning success; direct receive into an owned retainable callback-visible `[]byte` where possible, never recycled by the library after handoff. Caller may reuse outgoing input after successful Send return but must not race an in-progress snapshot. No end-to-end zero-copy promise. Exact private buffer pooling strategy remains PERF-004 OPEN. Source of truth: [DECISION_LOG.md](DECISION_LOG.md).

**API-001b stays OPEN.** An admitted request logically settles once via response/timeout/close; serialized callback invocation after forced Close is not yet specified. A permanently blocked same-Session callback precludes both forced immediate `OnClose` and strict callback serialization. See [follow-up design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) for explicit options A/B; do not finalize without the owner. Pending/completion credit count limits and their numbers (**PERF-003**) remain OPEN. No implementation authorized.
