# neta-go V1 — API-002 Timeout, Reconnect and Lifecycle/Shutdown Design

**Date:** 2026-11-17  
**Status:** DESIGN IN PROGRESS — owner decisions required; no implementation authorization  
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
| TCP Connect | Not yet approved | 30s, matching Pinex reference; configurable |
| Bind response | Not yet approved | 30s, matching Pinex reference; configurable |
| Server Pre-Bind handshake | 30s approved by Q-010 | Keep 30s; configurable |
| Request response | No Go default approved | **API-002-01 owner decision required; see §5** |
| Enquire Link idle | 30s approved | Keep 30s; zero disables outgoing probes |
| Enquire Link response | 10s approved | Keep 10s; timeout closes Session |
| Graceful Unbind | Not yet approved for Go | 10s, matching Pinex protocol deadline; configurable |
| Reconnect delay | Not yet approved for Go | 5s observed C++ baseline; strategy still open |
| Partial-frame assembly | No Go rule | Open technical review; C++ source has no separate bound-session frame-assembly deadline |
| Generic socket write | No Go rule | Open technical review; do not silently add a deadline without considering slow peers and queue semantics |

Timeouts are independent. Request timeout is not Enquire Link timeout; Connect/Bind/Pre-Bind deadlines do not share a single mutable timer. All timers must be cancelled or made generation-safe during close/reconnect so stale callbacks cannot mutate a newer Session.

## 5. API-002-01 — Request deadline origin and default (OPEN; OWNER DECISION REQUIRED)

The Request timeout must bound the full lifetime of an accepted callback-first Request, including time spent waiting behind the bounded outbound FIFO. Starting only after a complete TCP write can retain Request credits indefinitely when a Writer is stalled.

**Option A — start at successful local admission; 30s default (RECOMMENDED)**

- Reserve Sequence, Request credit, outgoing frame count/bytes and pending state atomically.
- Only after the complete local admission succeeds, start the Request deadline.
- Default is 30s; independently configurable on Client and Server via typed Functional Options. No per-call timeout override in V1, keeping the hot-path API small.
- Queue delay, Writer delay and peer response time all consume the same deadline.
- If timeout wins before the Writer claims the frame, remove/cancel that queued frame, release its queue and retained-payload charges exactly once, release Request tracking as defined by PERF-003, and never transmit it.
- If Writer has already claimed/started the frame, the library cannot retract bytes. Settle the Request once as timed out; any later response is late/unknown and must not resurrect completion. Do not claim that the peer did not receive it.
- Request timeout alone does not close the Session. It delivers exactly one timeout Completion through the existing bounded Completion lane; the Request credit remains held until callback return or permitted cancellation.
- Timeout/response/close races use one terminal-settlement winner. No duplicate callback and no Sequence reuse while a live Request entry remains.

**Option B — start after complete TCP Write; 30s default**

- Request admission and queue accounting are the same as A, but the response timer starts only after the entire frame has been written.
- Queue/Writer delay is outside the response timeout. A stalled Writer can therefore retain accepted Request credits and frame memory until a separate write/close condition intervenes.
- This is closer to a narrow interpretation of “response timeout”, but weaker as a total lifecycle bound.

**Option C — require a per-call timeout; no owner default**

- Each SendRequest call supplies its own positive timeout.
- No default Request timeout is applied by Client/Server.
- This adds a per-call parameter or request-options object to the hot-path API and makes omission impossible only if the language/API enforces it.

**Decision requested:** choose A, B or C. The 30s value in A/B is a proposed Go default, not a C++ default (C++ receives it as a constructor argument).

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

## 8. Remaining API-002 technical review items

1. **API-002-01:** Request timeout origin/default — owner decision in §5.
2. **Frame assembly deadline:** decide whether a bound Session that receives a valid header and then stalls mid-Body must be closed after a configurable finite deadline. Resource credits bound memory, but not time; Pinex C++ 1.5.0 has no separate bound-frame assembly timeout.
3. **Write stall deadline:** decide whether a stalled TCP Writer should close a Session after a configurable deadline, beyond the existing bounded Unbind deadline and Request timeout.
4. **Reconnect policy:** fixed 5s compatibility delay versus bounded exponential backoff with jitter; AutoReconnect default false is recommended.
5. **Stop API signatures and terminal cause accessor:** exact exported method/error names remain API-001 surface work; lifecycle semantics should be frozen before implementation.

## 9. Non-goals

- No transparent Request retry/replay.
- No process-wide worker pool or global resource cap.
- No forced goroutine termination or arbitrary callback cancellation once started.
- No waiting for user callbacks as a prerequisite for Session Done or transport shutdown.
- No new wire commands or changes to the Pinex Frame format.
- No implicit auto-restart after explicit Client/Server Stop.

**Implementation:** NOT AUTHORIZED.