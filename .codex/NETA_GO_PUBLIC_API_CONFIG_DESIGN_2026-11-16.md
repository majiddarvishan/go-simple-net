# neta-go — Public API, Configuration & Session Lifecycle Design (Draft)

**Date:** 2026-11-16  
**Status:** DESIGN DRAFT — NOT APPROVED AS A COMPLETE API CONTRACT  
**Repository:** majiddarvishan/go-simple-net / main  
**Implementation:** NOT AUTHORIZED. No Go application code, go.mod, tests or runtime changes.  
**Compatibility target:** Pinex C++ 1.5.0, develop branch, unchanged wire protocol.

## 1. Scope and traceability

This draft advances GO-PINEX-002 and GO-PINEX-011 (and informs GO-PINEX-004/005/006/007/010) after Q-004 Option A was approved. It does not replace the existing protocol contract, task plan or decision log.

### Already approved by the owner

- Q-001 A: No password/token field on the wire; Go Server has an optional Bind Validator.
- Q-005 B: When that validator is absent, otherwise valid unique System IDs are accepted; duplicate System IDs are still rejected.
- Q-002: Configurable **idle-only** Enquire Link; `0` disables automatic sending, not replies to peer requests.
- Q-003: Goroutine-safe Client/Server/Session public APIs, ordered non-overlapping callbacks within one session, cross-session isolation, concurrency-safe writes and controlled shutdown.
- Q-004 A: Lean Core (TCP client/server, protocol, session, request tracking, backpressure, resource limits, reconnect, idle Enquire Link). Defer application Circuit Breaker, Rate Limiter, Priority Queue, Prometheus exporter and business protocol integration; no automatic sequence deduplication.
- NAM-001: `neta-go` is the approved project/library name; repository/module/package renames have NOT been authorized.

### Current C++ behavior used as compatibility evidence (not automatically Go decisions)

- Pinex `docs/architecture.md` (develop): 10-byte wire header; session framing, control PDUs remain processable under backpressure; Bind Type `uni_direction` means requester-only enforcement on the binding client.
- Pinex `docs/lifecycle.md` (develop): client Connect and Bind timeout documentation; 5s reconnect delay when enabled; 10s Unbind deadline; default server pre-bind 30s handshake deadline and 128 pending sessions; close notification once; response correlation by sequence.
- Pinex `docs/thread-safety.md` (develop): C++ objects require a **single thread per io_context**; this is *not* Go's API contract. The owner's Q-003 Go thread-safety choice intentionally differs in programming model while preserving **wire** compatibility.

Source references:
- https://github.com/majiddarvishan/pinex/blob/develop/docs/architecture.md
- https://github.com/majiddarvishan/pinex/blob/develop/docs/lifecycle.md
- https://github.com/majiddarvishan/pinex/blob/develop/docs/thread-safety.md
- [Wire protocol contract](PINEX_WIRE_PROTOCOL_2026-11-16.md)
- [Decision Log](DECISION_LOG.md)

## 2. Proposed public API surface (design, not Go source code)

| Public concept | Planned responsibilities | Status |
|---|---|---|
| Client | Connect, Bind, optional reconnect, SendRequest/SendResponse, Stop, status reporting | Proposed |
| Server | Listen, Accept, pre-bind admission, duplicate-ID registry, optional Bind Validator, Stop | Proposed |
| Session | Bound peer identity, Bind Type, remote address, state, SendRequest/SendResponse, Unbind/Close | Proposed |
| ClientConfig | Identity, dial/bind deadlines, request deadline, reconnect, TCP and Enquire Link policy, queue bounds | Proposed |
| ServerConfig | Listener identity, bind policy, pre-bind limits, bound-session limits, TCP/Enquire Link policy, queue bounds | Proposed |
| SessionCallbacks | Bound, incoming request, incoming response (if exposed), timeout, closed and errors | Proposed |
| Error categories | Configuration, malformed frame, bind rejected, duplicate ID, transport closed, timeout, queue full / backpressure, shutdown | Proposed |

Exact exported Go identifiers, whether to expose synchronous request/wait helpers alongside callbacks, and whether to use Options or Config structs remain **unapproved design details**. Proposed package name `neta` must be approved separately at implementation setup; the selected library name is `neta-go`.

### Proposed callback rules based on Q-003

- Each Session has its own serial callback execution order; callbacks for different sessions can progress independently.
- Public API calls from callbacks, including a send-response operation, must be safe. No global mutex should be held while executing user code.
- A slow callback may delay **its own session's** application callbacks; bounded queues and backpressure protect resources, while protocol control PDUs such as Unbind and Enquire Link must remain responsive.
- A Session close event is terminal and delivered once; define deterministic behavior for pending request deadlines and final notifications before implementation.
- No implicit auto-response to an application Stream Request: the application decides whether and what to reply, using the original sequence number.

## 3. Proposed Client/Server configuration matrix

| Setting | Client | Server | Approval / baseline |
|---|---|---|---|
| System ID | local ID | local ID for Bind Response | Protocol required |
| Bind Type | configurable; default bi_direction | accept requested type; honor wire | C++ baseline, Go default proposed |
| Connect timeout | yes | N/A | C++ reference 30s; Go default not yet approved |
| Bind response timeout | yes | N/A | C++ reference 30s; Go default not yet approved |
| Pre-bind handshake timeout | N/A | yes, absolute from TCP accept | C++ reference 30s; Go default not yet approved |
| Max pending pre-bind sessions | N/A | yes | C++ reference 128; Go default not yet approved |
| Optional Bind Validator | N/A | yes | Q-001/Q-005 approved; absent => valid unique Bind allowed |
| Max bound sessions | N/A | yes, bounded policy | capacity to define |
| Request response timeout | yes | yes | configurable, default to define |
| Auto reconnect | yes | N/A | optional; retry policy to define |
| Reconnect delay/backoff | yes | N/A | C++ reference 5s when enabled; Go default not yet approved |
| Enquire Link idle interval | yes | yes | **Default 30s, Q-006 Option B approved**; positive => idle-only; explicit 0 => automatic sending disabled, inbound replies still active |
| Enquire Link response deadline | yes | yes | **Default 10s, configurable; Q-007 APPROVED**. Starts after full probe write, matching response by Sequence only; timeout closes Session |
| TCP keepalive | configurable | configurable | independent of application Enquire Link |
| Outgoing queue byte bound | per connection | per connection | **Default 4 MiB** for serialized Application Stream Frames, configurable; **Queue Full immediately** on saturation — Q-008 APPROVED |
| Max protocol frame length | yes | yes | must interoperate with C++ 1 MiB receive capacity; effective bound/definition to pin |
| Graceful Unbind deadline | yes | yes | C++ reference 10s; Go default proposed to match |
| Stop/shutdown deadline | yes | yes | exact API and duration OPEN |

The proposed Go defaults should be distinguished from *observed C++ defaults*. Do not present any unapproved number as an owner decision.

## 4. Idle-only Enquire Link policy: executable design intent, not code

1. The idle measurement is per **bound** session and uses monotonic elapsed time. Initialize activity on successful Bind.
2. When a full Pinex frame is actually received or fully handed to the TCP write operation as completed, record actual inbound/outbound activity respectively. Enqueueing alone is not a write and does not count.
3. With configured idle interval `0`, do not autonomously create Enquire Link Requests. Reply to incoming requests normally with their sequence number.
4. With a positive interval, when **no frame was successfully exchanged in either direction** for the entire interval, send one Enquire Link Request.
5. While a locally initiated Enquire Link is outstanding, do not send another. The matching `enquire_link_resp` clears the pending probe; normal incoming stream frames do not by themselves satisfy sequence correlation.
6. Re-arm idle measurement from actual activity; do not run fixed-rate pings in the presence of traffic.
7. The deadline for a missing matching response is **10s by default**, configurable separately from idle interval (Q-007). Once a sent probe times out without its matching response, close Session; Client reconnects only if enabled.

**Boundary under review:** What exactly counts as 'successful outgoing frame' should follow the Writer's completed full-frame write, not just queued bytes; this needs a concrete test under partial writes or disconnects.

### Approved default policy (Q-006 Option B, 2026-11-16)

- **Default `EnquireLinkIdleInterval=30s`** on neta-go Client and Server.
- Send automatically only on session inactivity for this interval, measured using actual incoming and completed outgoing Pinex frames; do not send at fixed intervals during normal traffic.
- An explicit `0` disables automatic sending but never disables responding to a peer's incoming Enquire Link.
- The C++ Pinex implementation currently does not automatically send Enquire Link by default; this Go default remains compatible at the wire level.
- The response timeout and action on a missing response are **approved in Q-007** (10s default, configurable; Session close; conditional Client reconnect).

### Approved missing-probe response policy (Q-007, 2026-11-16)

- The Enquire Link response deadline is **independently configurable**, with a **10-second default** on Client and Server.
- Start the response deadline after the entire local Enquire Link Request has actually been written; a queued but unwritten probe does not start it.
- Clear the pending probe **only on an `enquire_link_resp` with the matching sequence number**. Ordinary `stream_req`, `stream_resp`, or other traffic may still count as activity for Q-002 idle measurement but cannot satisfy this pending probe.
- If the matching response does not arrive before deadline, **close the Session once** and fail/complete pending operations through the Session close path. Auto Reconnect applies only to a Client explicitly configured for reconnect.
- Handle close/timeout/late-response races with one terminal completion and no duplicate callback.
- Preserve the single locally outstanding probe rule and existing Pinex 1.5.0 wire layout.
- The exact default capacity and bounded failure of outgoing queues remain open under Q-008.

## 5. Request/Response tracking and callback interactions

- Per-session nonzero increasing sequence values, wrapping without collision with still-pending request entries; responses copy request sequence.
- Outstanding request registry is bounded; publish/record pending before a corresponding frame becomes eligible for writing.
- Stream response timeout removes the pending entry exactly once; response after timeout must not resurrect a completed request or trigger duplicate completion.
- If the public API exposes both response callbacks and blocking/waiting helpers, define which entity owns completion to avoid double delivery. Favor a single completion source.
- Sending from inside a receive callback (especially SendResponse) must not depend on that callback returning before the send can be queued.
- Set explicit application payload ownership/copy rules. Do not assume caller keeps `[]byte` immutable without documentation.

## 6. Session lifecycle and concurrency boundaries

### Proposed lifecycle

- Client: Created → Connecting → Binding → Bound → Unbinding → Closed; optional ReconnectWaiting → Connecting if auto reconnect enabled.
- Server accepted peer: PreBind → Bound → Unbinding → Closed; pre-bind exhaustion or malformed PDU can close before Bind.
- All network write operations are serialized per TCP connection and preserve frame integrity.
- Bind rejection and duplicate `system_id` must not evict a previously bound peer.
- On Unbind, flush required queued control output and enforce a bounded close deadline. An incoming `unbind_req` receives matching `unbind_resp` before TCP close when possible.
- Controlled Stop prevents new work, suppresses new auto reconnect attempts, and provides one final close signal for bound sessions.
- Session ownership, callbacks, timers, pending requests and queued buffers must eventually release after close; race/churn tests must verify.

### Backpressure and control frames

- Normal outgoing application messages go through bounded FIFO admission (bounded by bytes and/or count; precise policy OPEN).
- Exceeding capacity must produce a documented bounded outcome. Initial recommendation: fail fast with a dedicated queue-full error; optional context-aware waiting needs separate API discussion.
- Incoming per-session callback queue also needs a bound or an equivalent pause strategy; do not let callback slowness grow memory without limit.
- Protocol control work (Enquire Link, Unbind) must be able to progress under application queue congestion. Preserve order where the C++ protocol requires it; do not reorder ordinary application messages.
- Unbind protocol deadline must still protect sessions that cannot drain application backlog.


### Q-008 — Queue capacity and overload behavior: APPROVED OPTION A (2026-11-16)

**Goal:** Protect per-session and aggregate memory usage while avoiding a complex rate-control framework in the approved lean Core.

**Option A (APPROVED for V1):**
- `MaxQueuedOutgoingBytes` per session: **4 MiB APPROVED default (4,194,304 bytes)** (count complete serialized Application Stream Frames, including the 10-byte header). The limit is configurable per Session. This is not a global server memory cap or a maximum individual frame size.
- Outgoing application Stream frames are admitted into a bounded FIFO. If a new frame would exceed capacity, `SendRequest`/`SendResponse` fail promptly with a dedicated **queue-full** error: do not block indefinitely and do not silently drop or partially enqueue the frame.
- Request pending/sequence tracking must be rolled back or left unadmitted when enqueue fails; a rejected frame must not later time out as if it was sent.
- Keep control frames (`enquire_link_req/resp`, `unbind_req/resp`) operable even when the application FIFO is full via separate **bounded** control capacity/coordination; never interleave bytes, bypass invalid protocol states or reorder application messages. On incoming Unbind, honor the existing Pinex-compatible queued-write drain plus 10s deadline contract.
- Bound incoming callback backlog as well. Its byte/count limit and pause/resume behavior need a separate technical contract; do not let slow callbacks grow memory without bound.
- A per-session cap alone is insufficient if unlimited sessions can be accepted. A global/server memory budget or maximum bound-session admission must also be specified; no specific numerical global value is currently approved.
- No additional business Rate Limiter, Circuit Breaker, Priority Queue, or automatic Deduplicator is added.

**Option B (not selected):**
- A bounded FIFO with context-aware waiting for capacity instead of immediate rejection (cancellation and wait fairness must be defined), while retaining all resource/control safety requirements.
- This adds API complexity and risk of blocking inside application callbacks. Not recommended for initial lean Core.

**Future acceptance tests (not run):** concurrent producers cannot exceed byte accounting; full queue returns bounded error (A) without missing sequence cleanup; small Frame admission after drain works; incoming Enquire Link and Unbind progress under saturation; closed session releases all held buffers; many sessions remain within the configured aggregate budget.

**Decision recorded:** Owner explicitly selected **A** on 2026-11-16: 4 MiB default per-session Application output FIFO, immediate queue-full failure, and bounded independently progressing control work. Q-008 is CLOSED. Numeric server-wide budget, inbound callback capacity and exact control reserve are **not approved** and are tracked under Q-009.

## 7. Open decisions and next steps

| ID | Decision | Recommendation | Status |
|---|---|---|---|
| Q-006 | Numeric default for Enquire Link idle interval | **30s by default (Option B); 0 disables automatic sending** | CLOSED — owner approved 2026-11-16 |
| Q-007 | Enquire Link response timeout and action | **Default 10s, configurable; close Session on missing matching reply, Client reconnect iff enabled** | CLOSED — owner approved 2026-11-16 |
| Q-008 | Capacity defaults and behavior at queue saturation | **Option A: 4 MiB per-session outgoing Application FIFO, immediate Queue Full, bounded control path** | CLOSED — approved 2026-11-16 |
| Q-009 | Incoming callback backlog, global queued memory and bound-session admission | Bounded ingress and global resource policy; detailed proposal in resource design | OPEN — awaiting review |
| API-001 | Exported public types, callbacks, send/wait semantics, []byte ownership | Minimal Go-native API, explicit lifecycle and one completion source | DRAFT |
| API-002 | Deadline/reconnect and shutdown configuration defaults | Prefer verified C++ baseline where appropriate; decide Go defaults explicitly | DRAFT |
| API-003 | Package/module path | Library brand `neta-go` approved; package `neta` proposed; no rename | OPEN |

### Proposed task sequence (documentation only)

1. Q-006 approved: default idle interval **30s**, explicit **0** disables autonomous sending.
2. Q-007 approved: configurable **10s** default response deadline, close on missing matching response; Client reconnect only when enabled.
3. Q-008 approved: per-session 4 MiB outgoing Application FIFO and Fail-Fast. Next review Q-009: inbound/global resource limits and overload behavior.
4. Pin exact C++ develop HEAD and golden-wire test vectors in GO-PINEX-001.
5. Obtain explicit owner approval **before any** coding, go.mod/module/repository changes, tests or deployment.

**No implementation is authorized or started.**
