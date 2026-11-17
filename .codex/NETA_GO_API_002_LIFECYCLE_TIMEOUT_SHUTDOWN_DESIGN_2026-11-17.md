# neta-go V1 — API-002 Timeout, Reconnect and Lifecycle/Shutdown Design

**Date:** 2026-11-17  
**Status:** ARCHITECTURE CLOSED — API-002-01 through API-002-08 CLOSED / OWNER APPROVED / OPTION A  
**Repository:** `github.com/majiddarvishan/neta-go`, branch `main`  
**Compatibility baseline:** Pinex C++ 1.5.0, audited `develop` HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`  
**Authority:** [DECISION_LOG.md](DECISION_LOG.md), [API-001](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [API-001b review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md), [API-004](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md), [Worker Capacity Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md), [Protocol Compatibility](PINEX_WIRE_PROTOCOL_2026-11-16.md).

**Guardrail:** documentation/design only. No Go/C++ code, tests, benchmarks, builds, dependencies, CI, deployment or operational changes.

---

## 1. Scope

API-002 completes the public lifecycle and timing contract for Client, Server and Session:

- TCP Dial/Connect, Bind handshake and server Pre-Bind deadlines.
- Request deadline origin, timeout settlement and queue interaction.
- Enquire Link idle/response deadlines.
- Graceful Unbind and forced close.
- Client/Server Stop semantics and context cancellation.
- Optional Client reconnect, retry timing and terminal Stop.
- Session lifecycle, Done and terminal-cause visibility.
- Running/queued user callbacks and resource-accounting behavior during shutdown.

API-002 must not reopen Q-002/Q-006/Q-007, Q-010, API-001b-FC-A, N-ORD2, DONE-A, C-ORD-B or WORKER-001..009C.

## 2. Binding decisions that API-002 must preserve

1. **Q-002/Q-006/Q-007:** idle-only Enquire Link; default idle interval 30s; explicit `EnquireLinkIdleInterval=0` disables automatic probes only; incoming probes are always answered; one local probe may be outstanding; matching-response timeout defaults to 10s and closes the Session.
2. **Pre-Bind admission:** 30s absolute server handshake deadline from TCP acceptance; 128 pending connections per listener by approved Q-010 defaults.
3. **API-001a:** callback-first asynchronous Request completion. Successful SendRequest means local admission only, not peer delivery.
4. **API-001b-FC-A:** on a Forced Close path, not-yet-started Application and Completion callbacks are cancelled; already-running callbacks cannot be forcibly terminated and retain their worker/resource charges until return.
5. **N-ORD2 / DONE-A:** Application, Completion and OnClose use independent bounded lanes. Session `Done() <-chan struct{}` closes after internal teardown and immutable terminal cause publication. Done does not wait for arbitrary user callback return. OnClose becomes eligible only after Done closes and may overlap a running Completion callback.
6. **C-ORD-B:** Completion start selection is FIFO within a Session; callback return order is not guaranteed when concurrency exceeds one.
7. **WORKER-009C:** limits are per Client/Server owner, not process-wide; active callback permits and retained payload charges remain held by already-running callbacks after Done/reconnect until those callbacks return.
8. **PERF-003:** each accepted Request has exactly one terminal settlement. Request credits cover the full admitted Request/Completion lifecycle until Completion callback return or permitted cancellation.

## 3. Pinex C++ 1.5.0 lifecycle evidence (reference, not automatic Go approval)

The audited source snapshot is `f7bd1eb779baff0d10e624f20522df345e6ee747` and must be re-pinned before future implementation or cross-language testing.

| Phase | Pinex 1.5.0 observed behavior |
|---|---|
| TCP Connect | 30s connect deadline |
| Bind response | 30s deadline after TCP establishment |
| Server Pre-Bind | 30s absolute handshake deadline; 128 pending default |
| Request response | caller supplies `timeout_sec`; constructor does not establish a default |
| Enquire Link idle | Go V1 owner-approved default 30s |
| Enquire Link response | Go V1 owner-approved default 10s |
| Unbind | 10s protocol deadline |
| Auto reconnect | 5s fixed delay when enabled; high-level Client Stop disables future retries |
| Client Stop | terminal; disables reconnect and gracefully Unbinds current bound Session |
| Server Stop | closes listener/acceptor and initiates Unbind for every currently bound Session |

C++ uses a single-threaded `io_context` convention. That is not copied into Go: the approved Go public APIs are goroutine-safe and internal lifecycle progress must remain independent of user callbacks.

## 4. Proposed timeout matrix (only approved values are binding)

| Timeout | Current Go contract | API-002 proposal / state |
|---|---|---|
| TCP Connect | **30s approved (API-002-03)** | Absolute per Dial attempt from Dial start; includes DNS + TCP establishment; earlier caller Context wins; Client-only |
| Bind response | Not yet approved | 30s, matching Pinex reference; configurable |
| Server Pre-Bind handshake | 30s approved by Q-010 | Keep 30s; configurable |
| Request response | **30s approved** | Starts at successful local admission; queue, Writer and peer-response time all count; independently configurable per Client/Server owner |
| Enquire Link idle | 30s approved | Keep 30s; zero disables outgoing probes |
| Enquire Link response | 10s approved | Keep 10s; timeout closes Session |
| Graceful Unbind | Not yet approved for Go | 10s, matching Pinex protocol deadline; configurable |
| Reconnect delay | Not yet approved for Go | 5s observed C++ baseline; strategy still open |
| Partial-frame assembly | **30s approved** | Absolute per-Frame deadline from first Header byte on Bound Sessions; no reset on progress; timeout closes only the affected Session |
| Generic socket write | No Go rule | Open technical review; do not silently add a deadline without considering slow peers and queue semantics |

Timeouts are independent. Request timeout is not Enquire Link timeout; Connect/Bind/Pre-Bind and Frame Assembly deadlines do not share a single mutable timer. The approved ConnectTimeout bounds only one Client Dial attempt, including DNS resolution; Bind response has its own independent deadline. Frame Assembly Timeout applies to every inbound Frame on a Bound Session, including partial Header and Body assembly. The already-approved Pre-Bind handshake deadline remains absolute from TCP accept and is not reset or extended by Frame progress. All timers must be cancelled or made generation-safe during close/reconnect so stale callbacks cannot mutate a newer Session.

## 5. API-002-01 — Request deadline origin and default (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

The Request timeout must bound the full lifetime of an accepted callback-first Request, including time spent waiting behind the bounded outbound FIFO. Starting only after a complete TCP write can retain Request credits indefinitely when a Writer is stalled.

**APPROVED OPTION A — start at successful local admission; 30s default**

- Reserve Sequence, Request credit, outgoing frame count/bytes and pending state atomically.
- The admission linearization point is the successful atomic reservation of the Request/Sequence/queue/retained-payload ledgers and insertion into the bounded outbound FIFO. Start the Request deadline immediately after that admission succeeds and before returning successful submission to the caller.
- Default is 30s; independently configurable on Client and Server via typed Functional Options. No per-call timeout override in V1, keeping the hot-path API small.
- Queue delay, Writer delay and peer response time all consume the same deadline.
- If timeout wins before the Writer claims the frame, atomically cancel/remove that queued frame so the Writer cannot transmit it; release its outgoing queue frame/byte charges and queued-payload retention exactly once. The accepted Request still receives its single timeout Completion, and its Request credit remains held through Completion callback return or permitted cancellation under PERF-003. Keep the Sequence/Request entry protected against late-response collision until its lifecycle is fully settled.
- The Writer-claim transition is the send-ownership linearization point. If the Writer has claimed the frame before timeout wins, the library cannot revoke that ownership or promise that no bytes will be sent. Settle the Request once as timed out; a later response is late/unknown and must not resurrect completion. Any Writer-owned buffer remains retained/accounted until Write completes or transport teardown; timeout must never free memory still referenced by Writer.
- Request timeout alone does not close the Session. It delivers exactly one timeout Completion through the existing bounded Completion lane; the Request credit and Completion callback resources remain accounted until callback return or permitted cancellation.
- Timeout/response/close races use one terminal-settlement winner. No duplicate callback and no Sequence reuse while a live Request entry remains.

**Rejected Option B — start after complete TCP Write; 30s default**

- Request admission and queue accounting are the same as A, but the response timer starts only after the entire frame has been written.
- Queue/Writer delay is outside the response timeout. A stalled Writer can therefore retain accepted Request credits and frame memory until a separate write/close condition intervenes.
- This is closer to a narrow interpretation of “response timeout”, but weaker as a total lifecycle bound.

**Rejected Option C — require a per-call timeout; no owner default**

- Each SendRequest call supplies its own positive timeout.
- No default Request timeout is applied by Client/Server.
- This adds a per-call parameter or request-options object to the hot-path API and makes omission impossible only if the language/API enforces it.

**Owner decision:** Option A approved on 2026-11-17. Options B and C were not selected. The 30s Go default is independently configurable on Client and Server through typed Functional Options; no per-call timeout override is part of V1. This is a Go API policy, not a claim that C++ Pinex defines the same default. Request timeout must be positive; zero does not disable it. No additional upper hard maximum has been approved.



### 5.1 API-002-02 — Frame Assembly Timeout (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

**Approved policy:** an absolute **30-second** deadline for assembling each inbound Frame on a Bound Session.

- Start the monotonic deadline when the Reader first observes the first byte of the current Frame Header. The deadline covers the remaining Header and the complete declared Body; it is not a separate 30s allowance for Header plus another 30s for Body.
- Any additional bytes/progress do **not** reset or extend the deadline. This is deliberately not an idle-progress timeout.
- Cancel the timer only after the complete Frame has been read and validated, or when the Session enters terminal close. The Reader processes the ordered byte stream; only the currently incomplete Frame owns an assembly deadline.
- If the deadline expires before the Frame is complete, close only that Session's transport and enter the existing terminal/Forced Close path. Do not skip the incomplete bytes, attempt stream resynchronization, or affect other Sessions. A Client may reconnect only according to the separately approved reconnect policy; this decision does not enable or define reconnect.
- **Partial-header case:** until a complete valid Header is available, retain only bounded parser/Header storage; do not allocate Body storage from an incomplete or unvalidated length. On timeout, release that partial parser state exactly once.
- **Partial-body case:** after validating the Header and declared total Frame length, reserve the full declared-frame retained-byte budget and applicable queue/count admission before Body allocation/read, as required by WORKER-008. If assembly times out, release the declared-frame reservation, partial Body storage, and applicable queue/count reservations exactly once.
- If a matching Response cannot reserve Completion retained bytes and follows the already-approved bounded scratch-buffer Body-drain path, that drain remains under the **same original Frame Assembly deadline**. If the deadline expires before the declared Body is fully drained, close the Session and do not emit a second Resource Capacity Completion; the existing terminal-settlement winner rule remains binding.
- The timer covers Application and protocol/control Frames received on a Bound Session; a partial control Frame is not exempt. No user callback is dispatched for an incomplete Frame.
- Default 30s is independently configurable on Client and Server via typed Functional Options; accepted Server Sessions inherit the Server setting. The setting is positive-only; explicit zero is invalid and does not disable the deadline. No additional upper hard maximum is approved by this decision.
- This is a Go lifecycle/resource-protection policy, not a Pinex wire change. Pinex C++ 1.5.0 has no separate bound-session Frame Assembly deadline.
- **Pre-Bind:** the existing absolute 30s Pre-Bind handshake deadline from TCP acceptance remains authoritative and unchanged. Frame Assembly Timeout does not extend it and is not an additional pre-Bind grace period.

**Future acceptance criteria (not run):** fragmented Header and Body; slow-trickle Body that never resets the deadline; completion just before deadline; simultaneous timeout/full-read race; partial-header and partial-body reservation release exactly once; Completion-budget drain expiry; isolation across Sessions; no callback or pending Request resurrection after close.

**Owner decision:** Option A approved. No implementation or tests authorized.



### 5.2 API-002-03 — TCP Connect Timeout (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

**Approved policy:** an absolute **30-second** deadline for each Client TCP Dial attempt.

- Start the deadline immediately before the Dial operation begins. It covers the complete operation, including DNS/name resolution and TCP connection establishment; DNS is not allowed to run outside the deadline.
- Implement through a derived Dial context / `net.Dialer.DialContext` (or equivalent) so the deadline and caller cancellation propagate through both resolution and socket establishment. Do not implement DNS resolution as an unbounded preliminary step.
- The caller's Context always has precedence when it is cancelled or its deadline is earlier than the configured ConnectTimeout. If the caller has no earlier deadline, the internal 30s ConnectTimeout bounds the attempt. Caller cancellation is not converted into a successful or retryable Session state.
- If the ConnectTimeout wins, cancel the Dial attempt and terminate only that attempt. No Session is created or published, no Bind is sent, and no Session Reader/Writer or callback worker is started. If a connection is returned concurrently after timeout has won, close that socket before it can escape the Dial operation.
- If TCP establishment wins before the deadline, proceed to the separate Bind handshake stage; BindResponseTimeout is independent and is not consumed from the remaining ConnectTimeout budget.
- ConnectTimeout is Client-only, independently configurable through a typed Functional Option. It is a per-attempt deadline, not a total reconnect-loop deadline. This decision does not enable AutoReconnect or authorize retries; reconnect remains a separate API-002 decision.
- Validation: positive finite duration; explicit zero is invalid. No additional hard upper maximum is approved.
- A timeout/connection-success race must have one linearized winner. A late successful socket after cancellation is closed and cannot create a stale Session generation.

**Owner decision:** Option A approved on 2026-11-17. The 30s value matches the observed Pinex C++ 1.5.0 connect reference, but the Go deadline is explicitly owner-approved here. No wire change is introduced. No implementation or tests authorized.

## 6. Proposed graceful shutdown and forced-close contract (design proposal; no new owner decision yet)

### 6.1 Session operations

- Graceful Unbind is distinct from forced Close.
- Graceful Unbind atomically closes new Application admission, preserves FIFO for already-admitted outbound Application frames, queues the Unbind control PDU after those frames, and waits for the matching Unbind response or the finite Unbind deadline.
- While draining, the Reader remains active for matching responses and protocol control. Existing Request deadlines continue to run. A Request settled by a response before terminal Session close completes normally.
- If the Unbind deadline expires, enter Forced Close. Pending Requests are settled once; queued not-yet-started Application/Completion callbacks are cancelled under FC-A; already-running callbacks continue.
- Forced Close immediately prevents new writes/dispatch, closes the transport, stops timers/Reader/Writer/control work and releases queue-only/internal accounting exactly once.
- Session Done closes after internal teardown and immutable terminal cause publication; it is not a callback join.

### 6.2 Client.Stop(ctx)

Proposed semantics:

1. Idempotently transition the Client to terminal Stopping/Stopped; after this point it cannot be restarted.
2. Disable auto reconnect and cancel retry timers, in-progress Dial and in-progress Bind generation.
3. If a Session is Bound, perform graceful Unbind; if only Dial/Bind is in progress, cancel and close that partial transport without emitting a user-visible Bound event.
4. Wait for the affected Session's Done, not for arbitrary Application/Completion/OnClose callback returns.
5. If caller context expires before graceful completion, force-close the remaining transport and wait only for internal Done publication; return the context error. No retry is allowed after Stop.
6. Repeated Stop calls observe the same terminal shutdown; they do not start a second Unbind or duplicate close notification.

### 6.3 Server.Stop(ctx)

Proposed semantics:

1. Idempotently enter terminal Stopping; close the listener first so no new TCP accepts occur.
2. Close all Pre-Bind sessions and cancel their handshake timers; reject any not-yet-completed Bind admission.
3. Initiate graceful Unbind concurrently for all currently Bound Sessions. Each Session retains its own FIFO Writer and Unbind deadline; one slow peer must not serialize shutdown of other peers.
4. Wait until every Session in the Stop snapshot has closed Done. New sessions cannot join after the listener is stopped.
5. If caller context expires, force-close all remaining Sessions and wait for internal Done, then return the context error.
6. Server Stop does not wait for user callbacks to return and does not promise that every OnClose callback starts or returns.

### 6.4 Callback and resource behavior during Stop

- No new Application Request callback may be admitted after the Session/owner begins stopping.
- Forced Close A cancels queued, not-yet-started Application and Completion callbacks. Already-running callbacks are not interrupted.
- OnClose remains best-effort, at-most-once and independently bounded; it becomes eligible only after Session Done. It may start or finish after Client/Server Stop returns.
- Running Application callbacks retain their active-worker permit and library-owned retained-payload charge until actual return. Running Completion callbacks retain their active-worker permit, Request credit and retained Response charge until return. Done/Stop do not release these early.
- A callback that calls Send/SendResponse after its Session starts closing receives the typed closed/closing error; it cannot resurrect Session state.
- A callback that never returns can retain its worker and related callback-lifetime resources indefinitely, but cannot prevent transport teardown, Done or Stop from progressing.
- Stop return means internal network/session/registry/timer teardown is complete, not that all user code has returned.

## 7. Proposed Client reconnect contract (owner approval required)

- Reconnect exists only on Client; accepted Server Sessions never reconnect themselves.
- Default `AutoReconnect=false`. It is explicit opt-in and cannot be re-enabled after terminal Stop.
- Reconnect creates a new Session generation with fresh transport and per-Session sequence state. Client owner worker/resource budgets remain shared across generations.
- Never replay or resend an accepted Request automatically. Requests on the failed generation settle exactly once with their terminal transport/close outcome; the caller decides whether to submit a new Request on the new Session.
- Retry candidates include TCP connect failure, Bind timeout/rejection, pre-Bind EOF and unexpected close of a Bound Session. Bind rejection is a generic wire status in Pinex, so the Client cannot reliably distinguish permanent policy rejection from transient duplicate-ID overlap.
- Pinex uses a fixed 5s retry delay. A fleet-scale Go policy should consider bounded exponential backoff with jitter to avoid synchronized reconnect storms. Whether to preserve fixed 5s or use bounded exponential backoff is an owner decision still open.
- Explicit Client Stop cancels the reconnect timer and prevents every future attempt. A stale callback from an older generation must not start, close or mutate the new generation.

## 8. Remaining API-002 decisions and technical review

1. **API-002-04 — Bind response timeout:** proposed 30s absolute deadline beginning after TCP establishment and before the Bind PDU is sent; not yet owner-approved.
2. **API-002-05 — Graceful Unbind timeout:** proposed 10s total graceful-drain deadline beginning when local Unbinding starts; not yet owner-approved.
3. **API-002-06 — Generic Writer stall timeout:** decide whether every frame Write needs an independent absolute deadline; not yet owner-approved.
4. **API-002-07 — Client reconnect policy:** AutoReconnect default, retryable outcomes, and fixed-vs-jittered backoff remain owner decisions.
5. **API-002-08 — Stop/Close lifecycle:** Client/Server Stop aggregation and graceful-to-forced transition require owner approval; exact exported method/error names and terminal-cause accessor remain API-001 surface work.

## 9. Non-goals

- No transparent Request retry/replay.
- No process-wide worker pool or global resource cap.
- No forced goroutine termination or arbitrary callback cancellation once started.
- No waiting for user callbacks as a prerequisite for Session Done or transport shutdown.
- No new wire commands or changes to the Pinex Frame format.
- No implicit auto-restart after explicit Client/Server Stop.

**Implementation:** NOT AUTHORIZED.

## 10. Final owner-approved API-002 contract (2026-11-17)

**Status:** CLOSED / OWNER APPROVED — OPTION A for API-002-01 through API-002-08. This section is authoritative and supersedes historical proposal language in §§4 and 6–8.

### API-002-04 — Bind Response Timeout
Default **30s absolute**. Starts immediately after TCP establishment and before Bind Request writer ownership. Covers Bind writer wait/ownership, complete Bind Request Write and matching Bind Response wait; progress does not reset it. Earlier caller Context cancellation/deadline wins. On expiry, close the unbound transport; do not publish a Bound Session or start normal Session workers. Client-only, positive finite duration, explicit zero invalid, no additional hard maximum approved. Bind rejection and Bind timeout are non-retryable.

### API-002-05 — Graceful Unbind Timeout
Default **10s absolute**, starting at the atomic transition to Unbinding. Covers new-Application-admission closure, FIFO drain of previously accepted Application Frames, Unbind PDU write and matching Unbind Response. Progress never resets it. Earlier caller Context cancellation/deadline escalates to Forced Close A. On expiry, force-close only that Session, expose ErrUnbindTimeout as its terminal cause and settle Requests exactly once. Applies to Client and Server Sessions through immutable inherited Session settings; positive finite, zero invalid, no additional hard maximum approved.

### API-002-06 — Generic Writer Stall Timeout
Default **30s absolute per Frame**, starting when the Session's single Writer takes ownership. Applies to Application and Control Frame writes. Partial Write/progress does not reset it; queue wait is excluded. Expiry closes only that Session because a partially written TCP Frame cannot be safely resumed. RequestTimeout and UnbindTimeout still bound their complete higher-level lifecycles; BindResponseTimeout bounds the entire earlier Bind phase. Applies to Client and Server Sessions through immutable inherited settings; positive finite, zero invalid, no additional hard maximum approved.

### API-002-07 — Client Reconnect Policy
- AutoReconnect defaults to **false** and is explicit Client opt-in.
- Retryable: transient Dial failure/timeout and unexpected transport loss after a Session reached Bound.
- Non-retryable: Bind rejection and Bind timeout; caller receives the failure.
- One reconnect supervisor and one active retry timer per Client. Exponential backoff with Full Jitter; base window **1s**, cap **30s**. After 30s continuously Bound, reset the next retry window to base.
- Reconnect creates a new Session generation and fresh Session-scoped sequence/pending state. Client owner worker/resource budgets remain shared across generations.
- Never replay/resend an accepted Request. Each old-generation Request settles exactly once; resubmission is an application decision.
- Stop permanently disables reconnect. Server Sessions never self-reconnect.

### API-002-08 — Stop / Close Lifecycle
- Session.Unbind(ctx) is Graceful: stop new Application admission, preserve FIFO for accepted Application Frames, write Unbind after the FIFO and await the matching response, bounded by the absolute 10s UnbindTimeout and caller Context.
- Session.Close() is immediate Forced Close A and returns after initiating close; it does not wait for Done.
- Client.Stop(ctx) disables reconnect, cancels Dial/Bind/retry, Gracefully Unbinds the current Bound Session and waits for internal Done. It is terminal and idempotent.
- Server.Stop(ctx) closes Listener first, closes Pre-Bind connections, starts Unbind concurrently for all Bound Sessions and waits for their Done signals. It is terminal and idempotent.
- If caller Context expires, Stop force-closes remaining Sessions, waits for internal Done publication and returns the caller Context error.
- Stop/Done do not wait for user callbacks. OnClose becomes eligible only after Done and may execute after Stop returns.
- Repeated Stop does not issue another Unbind or duplicate terminal notification. A stopped Client/Server cannot restart.

### Callback/resource interaction
Forced Close A cancels queued, not-yet-started Application and Completion callbacks; started callbacks cannot be terminated. Started Application retains its worker permit and Application retained-byte charge until return. Started Completion retains its worker permit, Request credit and Completion retained-byte charge until return. Done closes after internal teardown and terminal-cause immutability, not callback completion. OnClose remains independently bounded, at-most-once and best-effort under N-ORD2/WORKER-003.

**Decision:** API-002-04, -05, -06, -07 and -08 are all CLOSED / OWNER APPROVED / OPTION A. Combined with API-002-01..03, API-002 is complete. Implementation remains NOT AUTHORIZED.
