# neta-go — API-001 Public Client/Server/Session and Callback Design

> **LATEST OWNER REVISION (2026-11-16):** Q-003-R1 supersedes the older unconditional serial same-Session callback requirement below. Application `OnRequest` callbacks now use **bounded, configurable same-Session parallelism**; concurrency=1 restores serial application processing. This document retains earlier proposals for historical context. For the binding contract, see [Q-003-R1 in Decision Log](DECISION_LOG.md). API-001b lifecycle/notification dispatch remains **OPEN**.


**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — API-001a APPROVED OPTION A (async callback-first), API-001b OPEN / API-004 APPROVED OPTION A  
**Repository:** `majiddarvishan/neta-go`, branch `main`  
**Implementation:** NOT AUTHORIZED. This is architecture/documentation; no Go source, go.mod, test, build, or repository rename.

## 1. Approved constraints and source authority

- Compatibility target: **Pinex C++ 1.5.0**, branch **develop**, as recorded in [PINEX_WIRE_PROTOCOL_2026-11-16.md](PINEX_WIRE_PROTOCOL_2026-11-16.md). Pinex C++ 1.5.0 source baseline was audited at `f7bd1eb779baff0d10e624f20522df345e6ee747`; the 1 MiB total-frame maximum including Header is source-verified. Re-pin before future implementation/tests; golden frames and socket-level Go↔C++ confirmation remain pending.
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
| API-001b | Callback/Close/Done | Callback lanes, Forced Close cancellation and Done semantics | **Policy CLOSED; exact exported names/signatures remain API-001 work** |
| PERF-003 | Outstanding Request/Completion count ceiling | Configurable Session and owner aggregate credits, held through Completion settlement | **CLOSED — model and numeric defaults approved** |
| API-002 | Dial/Bind/Request deadlines, reconnect, graceful Stop and callback/resource shutdown behavior | API-002-01 approved: Request timeout 30s from successful local admission; API-002-02 Frame Assembly Timeout is next | IN PROGRESS |
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


## 13. PERF-003 Option A approved (2026-11-16), API-001b remains OPEN

**OWNER APPROVED:** bounded, configurable *count* of outstanding callback-first requests, including pending response and queued/running terminal completion, per Session plus Server aggregate. New SendRequest failing completion-credit admission is rejected promptly **without** closing the Session and without a queued frame or dangling Pending entry. This is separate from existing byte-based outgoing Queue Full. Numeric ceilings, exported option/error names, inbound event-count cap and request timer origin remain OPEN.

**Do not assume** Option A here also approves callback cancellation on forced Close. API-001b must explicitly resolve the contradiction between permanently blocked callback and immediate serialized `OnClose`/unconditional completion delivery; the recommended model is prompt transport `Done` plus an owner-chosen forced-close policy. See [current design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md) and [Decision Log](DECISION_LOG.md). No code/tests/benchmarks permitted.


## 14. Q-003-R1 — Parallel application callbacks and independent internal engine (owner-approved policy)

**Owner-approved revision:** `OnRequest`-style Application callbacks are dispatched from bounded Session-local FIFO with configurable maximum concurrency per Session; `1` gives serial handler execution, `N>1` permits overlap. Queue-selection order follows received frame order; **callback completion order is not guaranteed** for parallel mode. Methods remain goroutine-safe and TCP writes remain serialized. This **replaces** all earlier statements in this document requiring unconditional same-Session application callback serialization. The exact option identifier and **default** are still OPEN.

**Internal events independent of user callbacks:** protocol decoding, Control response, Bind/Unbind, Enquire Link, correlation, timer/deadline, socket close and once-only internal Request settlement must progress even while a user `OnRequest` handler is blocked. Reader and Writer must not execute arbitrary application handlers inline. No unbounded goroutine-per-message; each Session owns finite dispatch capacity and the existing Q-009/Q-010 incoming waiting queue overflow policy still closes only the offending Session. `N` active callback payloads consume additional bounded memory outside the **waiting** queue budget.

**User-visible lifecycle callbacks:** `OnClose` and per-Request Completion functions remain user code. API-001b-N1 approved a bounded notification domain separate from `OnRequest`; API-001b-NORD2 now approves independent bounded Completion and OnClose sublanes, with OnClose allowed to overlap an already-running Completion callback. C-ORD-B within-Completion concurrency policy is now owner-approved; its numeric default, worker counts, event capacity and payload-byte bounds remain open. Done semantics are resolved by DONE-A. A user callback cannot be forcibly terminated.

**Open constraints to review:** default maximum concurrent Application handlers per Session; optional Server-wide active Application callback limit; whether a listener's max bound Sessions multiplied by configured concurrency needs aggregate active-count protection; what constitutes an active callback's payload memory ceiling; application handler output order vs immutable TCP receipt order. No code or tests.


## 15. API-001b-N1 — user notification lane separation (OWNER APPROVED, 2026-11-16)

- **APPROVED:** `OnClose` and per-Request Completion user callbacks are dispatched through a **bounded notification domain separate from the bounded Application `OnRequest` worker domain**. The internal protocol/timeout/transport lifecycle engine must not wait for either domain to finish.
- **APPROVED EXISTING:** Q-003-R1 Application callbacks have configurable bounded same-Session concurrency (1 = serial), while PERF-003 A puts configurable count ceilings on outstanding Request + Completion work.
- Blocking `OnRequest` must not by itself stall internal Close/result settlement or occupy the notification workers. An arbitrary blocking `OnClose`/Completion user function can still stall its own bounded notification domain; it is not forcibly cancelable.
- **API-001b PARTIALLY OPEN:** Forced Close A and N-ORD2 sublane separation/overlap are owner-approved. Still open: numeric Completion concurrency default, worker/event capacities/defaults, terminal-cause accessor, Close() return timing, OnClose saturation policy and numerical PERF-003 limits. C-ORD-B policy is owner-approved. OnClose remains best-effort/at-most-once; forced Close A cancels unstarted Completion notifications.
- Read [DECISION_LOG.md](DECISION_LOG.md) § API-001b-N1 for the binding structure; older universal serialization notes are historical/superseded by Q-003-R1.

## 16. API-001b-NORD2 — independent Completion / OnClose sublanes (OWNER APPROVED, 2026-11-16)

- **APPROVED:** Completion and OnClose have distinct, independently bounded Notification sublanes. OnClose may execute concurrently with an already-running Completion callback for the same Session; it does not wait for Completion return. N-ORD1 shared serial Notification stream was not selected.
- Completion and OnClose must not consume each other's reserved active-worker capacity. A common scheduler is acceptable only with distinct class queues/admission and reserved execution slots; one unpartitioned shared worker pool is not N-ORD2.
- **Still open:** numeric Completion concurrency default; worker counts/defaults and aggregate limits; OnClose event cap/saturation; fairness; terminal-cause API and Close return behavior. C-ORD-B policy is closed. Done visibility and Done-before-OnClose eligibility are resolved by DONE-A.
- **DONE-A OWNER APPROVED:** Session.Done() <-chan struct{} closes exactly once after internal teardown and immutable terminal-cause publication, never waits for user callbacks, and gates OnClose eligibility. OnClose may then overlap an already-running Completion under N-ORD2. See [API-001b review §§11.3 and 11.7](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md).
- Implementation remains unauthorized.

## 17. API-001b-DONE-A — Session Done signal (OWNER APPROVED, 2026-11-16)

- **APPROVED public signal:** Done() <-chan struct{}, read-only and closed exactly once after internal Session teardown and immutable terminal-cause publication.
- Before Done closes: stop admission; settle pending Request outcomes exactly once; on a Forced Close path, cancel unstarted Application/Completion callbacks under Forced Close A; stop transport and internal Reader/Writer/control loops; clean timers, registry and internal accounting not retained by already-running callbacks.
- Done never joins or waits for running Application, Completion or OnClose callbacks. Those callbacks retain active-worker permits until return; running Completion retains its PERF-003 credit.
- An admitted OnClose event becomes eligible only after Done closes. Under N-ORD2 it may overlap a Completion callback already running for that Session. Done does not guarantee OnClose admission/start/return.
- **Still open:** terminal-cause accessor name/signature, Close() return timing, OnClose event saturation, numeric Completion concurrency default and all numeric worker/event/PERF-003 capacities.
- Implementation remains unauthorized.

## 18. API-001b-CORD-B — Configurable Completion concurrency (OWNER APPROVED, 2026-11-16)

- **APPROVED:** positive configurable per-Session maximum Completion callback concurrency; 1 means serial and values greater than 1 permit same-Session overlap.
- FIFO applies to callback start selection by serialized per-Session terminal-result settlement/enqueue order, not SendRequest admission order. With concurrency greater than 1, callback return order is not guaranteed.
- The per-Session limit is bounded by a finite aggregate active Completion-worker ceiling. PERF-003 Request credits cover the full admitted Completion lifecycle; active-worker permits cover physical callback execution.
- On a Forced Close path, Forced Close A cancels not-yet-started Completion callbacks; running callbacks retain their Request credit and worker permit until return. N-ORD2 OnClose independence and DONE-A ordering remain unchanged.
- **Still open:** numeric concurrency default and option name/range; worker counts/aggregate limits; event and payload-byte/frame-size bounds; OnClose saturation; terminal-cause and Close return API; API-002/API-003.
- Implementation remains unauthorized.


## 19. WORKER-001 — owner-scoped callback capacity (OWNER APPROVED, 2026-11-16)

- **APPROVED Option A:** each Client instance and each Server instance owns an independent callback-worker resource domain.
- Sessions within one Server share its aggregate budgets. Reconnect generations of one Client share its budget; already-running callbacks from retired Sessions remain charged until return.
- Independent instances do not share a hidden process-global worker pool or mandatory process-global active-worker ceiling.
- This is an ownership-scope decision only. Numeric capacities, physical pool layout, per-class aggregate-cap policy, public option names and saturation policies remain open.

See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md). WORKER-002 is next.

## 20. WORKER-002 — owner-wide active-worker ceilings (OWNER APPROVED, 2026-11-16)

- **APPROVED Option A:** every Client and Server instance has three independent aggregate active-worker ceilings: Application, Completion and OnClose.
- Per-Session concurrency is intersected with the corresponding owner/class active ceiling.
- Active capacity is isolated by class; no class may borrow or consume another class's permits.
- Ceilings count physically executing callbacks; queued events remain subject to separate count/byte admission.
- WORKER-001 owner scope applies across all Sessions and Client reconnect generations, including callbacks still running from retired Sessions.
- Numeric defaults, public option names, physical pool layout and event capacities remain open.

WORKER-003 was subsequently resolved: OnClose is skipped on event-budget saturation. Next is WORKER-004, worker-pool topology. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.

## 21. WORKER-003 — OnClose saturation (OWNER APPROVED, OPTION A, 2026-11-16)

OnClose event admission is best-effort/at-most-once. The owner-scoped event budget counts queued + running events. After Done closes, the engine attempts one nonblocking reservation; if the budget is full, that Session's OnClose invocation is skipped permanently. No retry, overflow goroutine or teardown wait is allowed. An admitted event holds its event credit until callback return, including while queued. Done and transport cleanup are unaffected.

Numeric event capacity and public option name remain open. Next: WORKER-004, worker-pool topology. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.


## 22. WORKER-004 — isolated callback worker pools (OWNER APPROVED, OPTION A, 2026-11-16)

- Each Client and Server owner has three independent pools: Application, Completion and OnClose.
- Each class pool owns separate scheduling, ready-work queue, dispatch, locks and wake-up state. Shared internal worker-loop code is permitted, but the execution pools and their scheduler resources remain isolated.
- Workers are created lazily, run one callback synchronously and are reusable only within their own class. No goroutine per Request/frame/callback.
- A blocked callback class cannot occupy another class's workers; the internal engine remains independent.
- Per-class worker goroutine count is bounded by the corresponding WORKER-002 ceiling. Idle workers do not hold active-callback permits but still consume goroutine/stack resources.
- WORKER-001 owner scope, WORKER-002 class ceilings and WORKER-003 OnClose event admission remain binding.

WORKER-004 Option B (one shared owner pool with per-class reservations) is not selected. WORKER-005 is next for idle-worker lifecycle and retirement. Numeric defaults, queue/event capacities and payload-byte bounds remain open. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§11–12. No implementation authorized.

## 23. WORKER-005 — finite idle worker retirement (OWNER APPROVED, OPTION B, 2026-11-16)

- Idle workers retire after a finite, independently configurable grace period for their callback class.
- The grace period starts only after callback return and when no dispatchable work exists.
- A worker is reused if work arrives before expiry; otherwise it atomically leaves the pool count and exits.
- The default warm-idle floor is zero. Running/blocked callbacks are never retired by the idle timeout and retain their active permits until return.
- Retirement never drops queued callbacks or releases their independent queue, Request or event credits.
- Enqueue/retirement has one linearization point; no lost wakeup or stranded ready work. No goroutine per timeout or global reaper.
- Numeric idle-timeout defaults/ranges remain for WORKER-009; Stop/drain semantics remain API-002.

WORKER-006 now defines the scheduling fairness policy. See [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§12–13. No implementation authorized.


## 24. Historical WORKER-006/007 configuration handoff (SUPERSEDED)

WORKER-006 and WORKER-007 are owner-approved as recorded in DECISION_LOG.md. The following section records the later WORKER-007 and WORKER-008 resource handoff.

## 25. WORKER-007 / WORKER-008 — resource configuration handoff (OWNER APPROVED, 2026-11-17)

WORKER-007 Option A is approved: per-Session incoming/outgoing Application frame-count limits plus one aggregate Server queued-frame count across both directions. Completion reuses PERF-003 Request credits and OnClose reuses WORKER-003 event credits; no duplicate count options are added.

WORKER-008 Option A is approved: separate per-Session and aggregate Client/Server retained-payload byte ceilings for Application and Completion classes. Queue byte/count limits remain separate admission dimensions; payload charges follow owned buffers through queue/active transitions. Completion byte-cap exhaustion disposition remains open and must not silently drop a matching Response.

Public option names remain under API-001/API-004. Numeric defaults and hard maxima are the subject of WORKER-009; no public identifiers are selected here.

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

