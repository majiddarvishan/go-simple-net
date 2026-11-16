# Decision Log — neta-go / Pinex Compatibility

## Q-001 — Bind validation compatible with Pinex (2026-11-16)

**Status:** APPROVED — Option A  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-005, GO-PINEX-008, GO-PINEX-012  
**Implementation:** NOT AUTHORIZED

### Decision

- Preserve the Pinex C++ 1.5.0 Bind wire format unchanged: `bind_type` (one byte) followed by the null-terminated `system_id`. No credential, password, token, extra header or new field is introduced.
- Design the Go Server with a **Bind Validator callback**. It receives at least the peer's `system_id` and the **actual transport remote address**; `bind_type` can be included to support validation policy. The callback determines whether to accept or reject the Bind.
- For accepted binds, return the normal Pinex-compatible `bind_resp` with status `rok = 0x00`; for rejected binds, use `rfail = 0xFF` in the existing header and follow the Pinex close/unbind lifecycle.
- A duplicate bound `system_id` must be rejected without removing or replacing the currently active session.
- Go Client should interoperate with Pinex C++ Server without added authentication fields.

### Explicit boundary

Checking a `system_id` and remote IP/address is **admission/authorization policy**, not cryptographic authentication or proof of peer identity. This mechanism does not prevent source-address spoofing across every threat model, shared-NAT identity confusion, network interception, or a hostile peer within the trusted network. Strong authentication would require a separate mechanism or compatibility-impacting protocol change and is **not part of this decision**. Transport and network policy remain deployment responsibilities.

### Follow-up resolution

Q-005 was resolved by the project owner on 2026-11-16: **Option B**, accepting a valid Bind when no custom Bind Validator is configured. See the Q-005 decision below.

### References

- [Port tasks](PINEX_GO_PORT_TASKS_2026-11-16.md)
- [Wire protocol contract](PINEX_WIRE_PROTOCOL_2026-11-16.md)
- [Project handoff](HANDOFF.md)

---

## Q-005 — Default Bind Validator policy (2026-11-16)

**Status:** APPROVED — Option B (Pinex-compatible permissive default)  
**Approved by:** Project owner  
**Depends on:** Q-001 — Bind validation Option A  
**Applies to:** GO-PINEX-002, GO-PINEX-005, GO-PINEX-008, GO-PINEX-012  
**Implementation:** NOT AUTHORIZED

### Decision

- If the Go Server is constructed **without a Bind Validator callback**, it will accept otherwise-valid Bind requests by default, matching the C++ Pinex behavior.
- This permissive default does **not** waive protocol validation, session-state checks, existing connection limits, or the duplicate-`system_id` check.
- If a Bind Validator callback **is configured**, its accept/reject decision is enforced. It may inspect at least the `system_id` and the actual TCP peer address; `bind_type` can also be provided.
- Duplicate `system_id` Bind attempts are rejected regardless of whether the validator is absent or present, without evicting the previously bound connection.
- Successful Bind uses the existing `bind_resp` with `rok=0x00`; rejection uses `rfail=0xFF`. Do not change the wire format.
- The public Go API must make the optional validator behavior explicit in documentation and examples.

### Security boundary

The no-validator default is permissive admission, **not** cryptographic authentication or a security boundary. It must not be represented as verified identity. Operators who need restricted admission should configure a validator and appropriate network controls; the protocol itself remains unchanged to preserve C++ Pinex compatibility.

### Acceptance criteria (planned, not implemented)

1. No validator + otherwise valid, nonduplicate Bind => accepted.
2. No validator + duplicate `system_id` => rejected; existing bound session stays registered.
3. Configured validator returning reject => rejected.
4. Configured validator returning allow + nonduplicate, valid Bind => accepted.
5. Protocol-invalid or disallowed-state Bind => rejected/closed per the Pinex protocol contract, independent of validator presence.
6. All four cases must be exercised against the matching C++ Pinex baseline, without adding custom fields to Bind frames.

Q-005 is **CLOSED**. No implementation, tests, or build commands were run as part of this decision.

---

## Q-002 — Idle-only Enquire Link (2026-11-16)

**Status:** APPROVED — configurable idle interval, `0` disables automatic sending  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-006, GO-PINEX-011, GO-PINEX-012, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Owner requirement

"Enquire Link همیشه استفاده نمی شه. فقط وقتی که بسته ای رد و بدل نمی شه، ارسال می شه. قابل پیکربندی باشه. اگر 0 بود، ارسال نمی کنیم."

### Decision

- Expose a configurable **Enquire Link idle interval** on Go Client and Go Server connections. A positive interval enables idle-only automatic Enquire Link; **`0` disables automatic sending**.
- An Enquire Link Request may be sent only after no TCP protocol frame has been successfully exchanged **in either direction** on that bound session for the configured idle interval. This is inactivity detection, **not** a fixed-rate periodic ticker that sends pings while normal traffic is flowing.
- Any successfully received or sent Pinex frame constitutes session traffic and resets the inactivity measurement. The measurement is per connection/session and begins after successful Bind. Local pending work that has not been written on the wire is not proof of traffic.
- A new Enquire Link Request must not be initiated while another locally initiated Enquire Link is pending on the same session.
- Receiving an Enquire Link Request must still trigger the compatible Enquire Link Response with the same sequence number, **even when automatic sending is disabled (`0`)**.
- Preserve the Pinex 1.5.0 PDU encoding: `enquire_link_req=0x04`, `enquire_link_resp=0x84`, header-only frames and unchanged sequence correlation. This is a configurable Go-side keepalive policy, **not a wire format change**.
- Both client and server require tests with active traffic, inactive traffic, zero interval, a pending probe, and cross-language peers.

### Not specified by this approval

- The owner has not selected a numeric positive default interval. Document the exact configuration default at the public API/configuration design step; do **not** silently assert a default of `0` or any positive duration as part of this decision.
- The unanswered-probe response deadline, failure threshold and resulting session close behavior require a separate reviewed specification under GO-PINEX-011. Do not invent them while implementing.
- TCP keepalive is independent of this application-layer probe.

### Planned acceptance tests (not yet run)

1. Positive interval, ordinary inbound/outbound traffic at intervals shorter than the threshold => no automatic Enquire Link Request.
2. Positive interval, no traffic in either direction for the threshold => an Enquire Link Request is sent.
3. Any real incoming/outgoing Pinex frame before expiry => timer is rearmed from actual activity; it cannot send prematurely.
4. Interval `0` => no automatic Enquire Link Request, regardless of how long the session stays idle.
5. Interval `0` and peer sends Enquire Link Request => matching Enquire Link Response is still sent.
6. Locally initiated Enquire Link pending => do not send a second outstanding probe.
7. Go Client ↔ C++ Pinex Server and C++ Pinex Client ↔ Go Server => valid control PDU exchange.

Q-002 is **CLOSED** at the policy level. No code, tests or build actions were authorized or performed.

---

## Q-003 — Concurrency Safety and Callback Ordering (2026-11-16)

**Status:** HISTORICAL APPROVAL, **CALLBACK SERIALIZATION SUPERSEDED** by Q-003-R1 (2026-11-16); goroutine-safety, Session isolation, frame-safe sends and controlled shutdown remain binding  
**Applies to:** GO-PINEX-002, GO-PINEX-004, GO-PINEX-007, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Confirmed contract

1. **Goroutine-safe public APIs:** Go Client, Server and Session methods must support concurrent calls from multiple goroutines.
2. **Per-session callback ordering — SUPERSEDED by Q-003-R1:** Original rule required serial execution in receipt order; the owner subsequently replaced it with bounded, **configurable parallel application callback execution**. At a configured concurrency of 1, application callbacks are serial. See Q-003-R1 below; do not implement the original unconditional serial rule.
3. **Session independence:** A slow or blocked callback for one Session must not stall callbacks or network handling for unrelated Sessions. The owner approved independence, not unlimited background goroutines.
4. **Concurrent sends:** Multiple goroutines may send over a single Session concurrently; outgoing Frame bytes must not interleave or corrupt. A session-local serialized writer must preserve the integrity of each frame.
5. **Controlled shutdown:** On disconnect or shutdown, callback completion/cancellation and outstanding operations must be handled deterministically without duplicate terminal notifications, goroutine leaks, hangs or corrupted state.

### Engineering constraints for later API design

- The precise callback signatures, queue capacities, backpressure strategy, shutdown deadline and pending-request error taxonomy remain to be designed within GO-PINEX-002/004/007/010 and must not be silently assumed from this policy.
- Keep blocking user callback execution off unrelated Session readers/writers. Do not hold global/session transport locks across user callbacks.
- Race tests, concurrent-send frame-integrity tests, per-session order tests, cross-session isolation tests, and shutdown/churn tests are part of the planned acceptance work.
- These are requirements to implement later; **no application code is approved**.

Q-003 is **CLOSED as REVISED by Q-003-R1**; the original callback-serialization clause above is not current.

---

## NAM-001 — Go library naming (2026-11-16)

**Status:** CLOSED — APPROVED: `neta-go`  
**Approved by:** Project owner  
**Implementation:** NOT AUTHORIZED; the repository was subsequently renamed by the owner

### Final owner decision

- The selected name for the **native Go TCP Messaging Client/Server** is **`neta-go`** (NETA Go).
- The name **NETA** had been proposed as a short form of *Network Easy Transport API*; the owner explicitly selected the name `neta-go` (not a separately approved expansion).
- It must remain interoperable with the C++ Pinex 1.5.0 wire protocol. Choosing a Go library name does not rename or change C++ Pinex.
- NAM-001 originally settled the **library branding** only. The owner subsequently renamed the GitHub repository to `neta-go`; the proposed code package name `neta` and Go module path remain unapproved/undetermined.
- The current, owner-renamed GitHub repository is `majiddarvishan/neta-go` on branch `main` (verified 2026-11-16); the prior repository name was `majiddarvishan/go-simple-net`.
- The owner's GitHub repository rename does not authorize changes to code, module paths, imports, README branding outside documentation, further repository settings, or C++ Pinex naming.
- Naming collision, module-path availability, and trademark review have not been performed; verify as needed before public release.

### Next planning work

Continue Q-004 and the outstanding configuration/API details. No application implementation is authorized.

---

## Q-004 — Optional features scope for neta-go V1 (2026-11-16)

**Status:** CLOSED — APPROVED OPTION A (lean V1 Core), 2026-11-16  
**Applies to:** GO-PINEX-002, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-011, GO-PINEX-014  
**Implementation:** NOT AUTHORIZED

### Background from the existing project task plan

The uploaded Go sample includes several application-oriented features that are not part of Pinex C++ 1.5.0's wire contract. Go V1 must remain a general-purpose, interoperable TCP messaging library rather than inherit SS7-specific or sample-only policy.

### Approved V1 Core (retain)

- Shared Pinex-compatible codec, TCP client/server, session and Bind/Unbind state machine.
- Opaque binary payload, stream request/response, sequence correlation and request deadlines.
- Concurrent-safe public API, per-session ordered callbacks, frame-safe concurrent writes and independent sessions per approved Q-003.
- Bounded queues, backpressure and connection admission for predictable resource behavior; FIFO by default.
- Configurable client reconnect, transport timeouts, TCP keepalive and approved idle-only Enquire Link (Q-002), including `0` to disable automatic sending.
- Controlled closure, terminal event notifications, and direct Go↔C++ Pinex interop tests.

### Approved V1 exclusions / future backlog

| Feature | Proposal | Rationale |
|---|---|---|
| Generic Circuit Breaker | Defer to a later optional component | Application policy and failure thresholds should not obscure TCP/session behavior |
| Business/application Rate Limiter | Defer to caller or middleware | Do not conflate with required transport backpressure and resource limits |
| Sequence-based message Deduplicator | Exclude from Core, no implicit dedup | Pinex sequence is for correlation, not a global deduplication identity |
| Priority Queue | Defer; use bounded FIFO in Core | Preserve simple, deterministic traffic flow and control frames' ability to progress |
| Prometheus metrics exporter/dependency | Defer; consider minimal neutral hooks in Go API design | Avoid coupling networking Core to a metrics backend |
| Protobuf/SS7 business handlers | Out of scope | Payload must remain opaque; application owns serialization |
| Multi-client load-balancing Group | Separate future task | Not required for the initial client/server compatibility milestone |

**Important distinction:** Transport-level queue bounds, per-session admission and backpressure **remain mandatory in Core** even though business Rate Limiter and Circuit Breaker are proposed for deferral. Control PDUs must not starve behind regular payload backlog.

### Approved acceptance checks (to verify during implementation)

- V1 can perform complete Bind, Stream, Enquire Link and Unbind interoperability against C++ Pinex with only the Go Core.
- No optional application feature is needed to get a usable client/server.
- No automatic sequence deduplication; repeated valid sequence-like payload content is not dropped.
- Overloaded sessions use defined bounded behavior and do not cause unbounded memory growth.
- Optional application policies can be layered outside the public API without wire changes.

### Approval resolution

The owner explicitly selected **Option A** on 2026-11-16. The lean V1 scope above is **approved** and Q-004 is **CLOSED**.

The approval does **not** authorize code implementation, module/repository renaming, tests, deployments, or changes in the C++ Pinex repository. Detailed API signatures and numerical limits remain open for design.

---

## Design handoff after Q-004 approval (2026-11-16)

The next **design draft** has been recorded in [NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md). It distinguishes approved owner contracts from suggested (unapproved) exported API structure, numerical defaults, queue capacity and missing Enquire Link response behavior.

**Resolved after this handoff:** Q-006 default idle interval **30s** (Option B, approved) and Q-007 response timeout **10s**, close on missing matching reply, reconnect only when enabled (approved). **Resolved:** Q-008 Option A (4 MiB per-session outgoing FIFO and fail-fast) is now approved. **Resolved:** Q-009 Option B (close only the saturated Session on incoming callback queue overflow). **Still open:** Q-010 (inbound/global/session capacity defaults and control reserve), API-001..004 (exact Go API and configuration semantics). No code work is authorized.

---

## Q-006 — Default Enquire Link idle interval (2026-11-16)

**Status:** CLOSED — APPROVED OPTION B  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-006, GO-PINEX-011, GO-PINEX-012  
**Implementation:** NOT AUTHORIZED

### Decision

- The default value of `EnquireLinkIdleInterval` for **neta-go Client and Server** is **30 seconds**.
- This is an *inactivity threshold*, not a fixed-period heartbeat. Automatic Enquire Link is sent only if no valid Pinex frame has actually been exchanged in **either direction** on the bound session for 30 seconds, or for the configured positive interval.
- Each completed outgoing/incoming Pinex frame refreshes the per-session activity timestamp; the idle timer begins once Bind succeeds.
- Setting the interval explicitly to `0` disables **automatic sending**; responding to a peer's incoming Enquire Link remains mandatory.
- At most one locally initiated Enquire Link can be pending for a session. Requests and responses retain the Pinex 1.5.0 control PDU wire format and sequence behavior.
- The default differs from the current C++ Pinex implementation's lack of automatically initiated Enquire Link, but is **wire compatible** with that implementation.

### Unresolved, not part of Q-006

**Q-007** remains open: response deadline for an outstanding Enquire Link and whether to close the session if the matching reply is missing. No default deadline or failure action was approved by selecting Q-006 B.

### Future tests (not run)

- Default no-traffic session after Bind sends a probe on 30s idle, not on a 30s fixed ticker during active messaging.
- A recent inbound or outbound frame prevents a premature probe.
- Explicit zero disables automatic probes but responses to peer probes still work.
- One pending local probe does not generate another, and Go↔C++ Pinex control frames interoperate.

**No code, tests, module changes or repository renames are authorized by this decision.**

---

## Q-007 — Enquire Link response timeout and session failure policy (2026-11-16)

**Status:** CLOSED — APPROVED (10s configurable default; close on unanswered probe)  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-006, GO-PINEX-007, GO-PINEX-009, GO-PINEX-011, GO-PINEX-012, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Decision

1. For neta-go Client and Server, the Enquire Link **response timeout** is independently configurable, with **10 seconds as the default**.
2. The deadline starts **after the complete Enquire Link Request has actually been written** to TCP, not when the frame is merely queued.
3. A valid reply must be an `enquire_link_resp` with the **exact sequence number** of the outstanding local Enquire Link Request. Other traffic, including normal `stream_req` and `stream_resp`, must **not** substitute for or clear this pending probe.
4. When the configured response deadline expires without a matching response, consider the Session unhealthy and **close it**. The terminal close notification must happen only once, and pending application operations must be completed/failed according to the Session close contract.
5. **Client reconnect** after this close is permitted **only if Auto Reconnect is configured/enabled**. Server does not initiate client-style reconnect.
6. Preserve the existing policy of **no more than one outstanding locally initiated Enquire Link per Session**.
7. An incoming peer-originated Enquire Link must still be answered, including with automatic Enquire Link disabled (`EnquireLinkIdleInterval=0`). A zero idle interval prevents initiating probes and thus there is no local probe to time out.
8. No new status, header field, opcode, token, or other wire change is authorized; the design remains compatible with C++ Pinex 1.5.0.

### Race and failure cases to test later

- Response arrives before deadline: clear only the matching probe; no close caused by that deadline.
- Only unrelated traffic arrives while probe is pending: probe still times out and session closes.
- Matching response arrives simultaneously with timeout: use a single atomic/serialized completion winner; no double close or double event.
- TCP write fails or socket closes before the full probe is written: transport close/error handling applies; do not start a phantom response deadline.
- Reconnect disabled: close terminally; reconnect enabled: obey the separately configured retry policy.
- Peer sends a request while waiting: always send compatible Enquire Link response regardless of local outbound probe state, subject to the bounded transport/close rules.

### Still open

- Q-008 (subsequently CLOSED / Option A): 4 MiB outgoing queue per Session, fail-fast on saturation. Remaining inbound/global admission details are tracked separately in Q-009.
- Detailed numerical/default/retry settings beyond Q-007, public API signatures and close/pending error taxonomy remain part of GO-PINEX-002/011 design. This decision did **not** authorize any code, tests or operational changes.

---

## Q-008 — Bounded outgoing queue and Fail-Fast (2026-11-16)

**Status:** CLOSED — APPROVED OPTION A  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-004, GO-PINEX-007, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-012, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Approved requirements

1. `MaxQueuedOutgoingBytes`: **4 MiB (4 * 1024 * 1024 bytes)** by default, separately configurable **per Session**.
2. This queue is bounded FIFO for **Application Stream PDUs**, counted by their full serialized frame length, **including the 10-byte Pinex header**. It is an outgoing queue limit, **not** the server's total memory budget and **not** an increased maximum wire frame size.
3. On an application frame not fitting the remaining capacity, `SendRequest` or `SendResponse` must **fail immediately** with a distinct **Queue Full** error. No indefinite waiting for capacity, silent dropping, partial admission, or unexpected deferred sends.
4. A request rejected at admission must have **no orphaned pending sequence/response timer**, and no corresponding application PDU may be transmitted. Concurrent goroutines' admission/accounting must remain race-free under Q-003.
5. Preserve a **bounded, separately admissible control-PDU path/headroom** for Bind/Unbind and Enquire Link responses/requests as appropriate to the lifecycle, so a full Application FIFO does not silently starve protocol control. This does **not** permit unlimited control buffering, interleaved bytes, or reordering of already queued Application Stream PDUs.
6. Specifically retain Pinex-compatible Unbind ordering: a queued `unbind_resp` must be sent after earlier writes are drained when possible, with the established **10-second Unbind protocol deadline** protecting an undrainable connection.
7. No generic Circuit Breaker, application Rate Limiter, message Deduplicator or priority queue is introduced by Q-008.

### Explicit non-decisions

- The numeric **incoming callback queue** limit, incoming pause/overflow behavior, control headroom in bytes/count, global/server queued-memory budget, and bound-session capacity are **not** covered by the 4 MiB approval.
- Those belong to **Q-009** and must remain visibly open until specified/reviewed.
- Public API method signatures and whether additional send modes are supported remain under API-001.

### Planned acceptance tests (not run)

- Concurrent sends never exceed 4 MiB of admitted queued Application frame bytes per Session; byte accounting includes each full header.
- An over-capacity send returns Queue Full promptly, neither transmits nor leaves an outstanding Sequence entry, and a later send succeeds after capacity drains.
- Incoming Enquire Link/Unbind works while Application queue is full; control buffering stays bounded.
- The single-writer framing invariant and Pinex Unbind's required ordering/deadline hold even during backpressure and TCP stalls.
- A large number of sessions cannot grow unchecked because Q-009 must additionally define aggregate admission.

**Q-008 is CLOSED; implementation is still not authorized.**

---

## Q-009 / Q-010 — Follow-on resource design (2026-11-16)

**Status:** Q-009 CLOSED / APPROVED OPTION B; Q-010 subsequently CLOSED / APPROVED OPTION A with all defaults configurable (2026-11-16).  
**Reference:** [NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md)

- **Q-009 APPROVED B:** close the specific Session promptly on an incoming Application frame that cannot be admitted to its bounded callback backlog; no pause/resume recovery policy for queue overflow in V1. See the Q-009 decision below.
- **Q-010 APPROVED OPTION A (2026-11-16):** configurable initial defaults: inbound Callback queue 4 MiB per Session, server-wide queued Application bytes 256 MiB, max 1024 bound Sessions, 128 pre-bind connections with 30s handshake deadline and 4 KiB control reserve per Session. Approved global overload: outgoing => Queue Full without close; incoming => close offending Session; new Bind at bound limit => reject without eviction. The 256 MiB quota is not process RSS. See the specific Q-010 decision below.
- **API-004 OPEN:** choose a Go configuration representation distinguishing **omitted** `EnquireLinkIdleInterval` (approved default 30s) from explicitly provided `0` (disable). Ordinary zero-valued `time.Duration` alone cannot represent both states.

Numeric limits now approved: **outgoing 4 MiB/Session (Q-008)** plus **incoming 4 MiB/Session, aggregate 256 MiB/Server, 1024 bound Sessions, 128 pre-bind, pre-bind timeout 30s and control reserve 4 KiB/Session (Q-010)**. These are configurable defaults, not hardcoded constants or a full-process RSS cap. No implementation is authorized.

---

## Q-009 — Incoming callback queue overflow (2026-11-16)

**Status:** CLOSED — APPROVED OPTION B (close saturated Session)  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-004, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-012, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Approved behavior

- Deliver Application Stream callbacks for each Session serially in receipt order (Q-003), using a **bounded** incoming callback backlog. The byte/count limit is separately configurable, with the numeric default pending under Q-010.
- If a valid incoming Application frame **cannot be admitted** without exceeding its Session's callback backlog capacity, **close that Session promptly** with a distinct resource-limit/overload cause. An exactly full queue is not by itself a failure until a new frame cannot fit.
- **Do not** silently drop or skip the overflow frame, accept it into unbounded memory, wait indefinitely for callback capacity, or switch to a pause/resume recovery mechanism under Q-009. The overflowing Session has a terminal transport close instead.
- The triggering Session must stop accepting new application work, release owned transport/buffer/timer/queue reservations and pending requests, and emit **no duplicate terminal notification**. Do not evict other Sessions, cancel their callbacks, or hold a global registry lock while invoking any user Callback.
- An explicitly configured Client Auto Reconnect policy may reconnect after closure, following its separate retry rules; a Server does not attempt to reconnect its accepted peers.
- Incoming Control PDUs already decoded **before** overflow remain governed by the existing protocol/state rules. Once the Session closes due to overflow, do not assume that a later control frame can be processed through a closed TCP connection. Preserve per-frame order and Pinex Wire compatibility.
- **Q-003 boundary:** a user callback currently blocked indefinitely cannot be forcibly killed by the Go library. Transport shutdown must not wait forever on such a callback; exact terminal-callback delivery and waiting semantics need a separate API/Lifecycle contract. Never promise guaranteed completion of arbitrary user callback code.

### Separate, not approved by this decision

- **Q-010 (subsequently CLOSED / APPROVED OPTION A):** default inbound queue capacity, global/server queue budget, max bound Sessions, pre-bind limits and bounded control reserve. Exact callback completion/active-frame accounting details are still part of the API/runtime design.
- Whether the *same close-on-exhaustion* policy should apply to a global queue budget (as opposed to Session-local queue exhaustion) belongs to Q-010 and is **not** silently decided here.
- API error names, Session Close event ordering, and cancellation behavior remain under GO-PINEX-002/004 design.

### Planned tests (not run)

- A slow Session callback fills the incoming backlog: next non-fitting Application frame closes **only that Session**, returns/fires the specific overload cause once, and releases its buffer/timer/request registrations.
- Two other Sessions receiving traffic concurrently remain responsive and preserve callback order, even when the overloaded Session's callback is blocked.
- No hidden message drop, no pause/resume recovery on overflow, no double terminal callback.
- Boundary accounting at exactly-full versus one additional non-fitting frame; fragmentation, disconnect and callback completion races; client reconnect if explicitly enabled.

Q-009 is **CLOSED**, and no code, tests, deployments or module/repository renames are authorized.

---

## Q-010 — Configurable resource defaults and global overload (2026-11-16)

**Status:** CLOSED — APPROVED OPTION A WITH EXPLICIT CONFIGURABILITY  
**Approved by:** Project owner  
**Applies to:** GO-PINEX-002, GO-PINEX-004, GO-PINEX-005, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-012, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Approved defaults (ALL configurable)

| Setting | Default | Scope |
|---|---:|---|
| `MaxQueuedIncomingBytes` | **4 MiB = 4,194,304 bytes** | Waiting Application callback Frames, per Session |
| `MaxAggregateQueuedFrameBytes` | **256 MiB = 268,435,456 bytes** | Aggregate admitted queued Application Frames, per Server |
| `MaxBoundSessions` | **1024** | Bound Sessions, per Server |
| `MaxPendingPreBindSessions` | **128** | Pending pre-bind TCP connections, per listener |
| `PreBindHandshakeTimeout` | **30 seconds** | Absolute deadline from TCP acceptance |
| `MaxQueuedControlBytes` | **4 KiB = 4,096 bytes** | Bounded Control PDU allowance, per Session |

- **All six parameters must be independently configurable** in the public Go Client/Server/Session configuration as applicable. They are **defaults**, not immutable values. Validation and representation of omitted vs explicit configuration are API-design work, not permission to hardcode policy.
- The **outgoing Application FIFO** separately retains its **configurable 4 MiB default per Session** from Q-008. The incoming limit and outgoing limit are **distinct**, even though both have a 4 MiB default.
- No eager per-Session allocation of the full configured capacity. Use dynamic, accounted, bounded allocations.
- The 256 MiB aggregate value caps queued Application frame bytes for one Server, **not process RSS, network kernel buffers, goroutine stacks or all application-owned memory**. Pre-bind buffers, active callbacks, in-progress frames and Control PDUs need separate bounded accounting in the subsequent API/implementation contract.

### Approved overload outcomes

1. **Outgoing new Application Frame:** if its Session-local or Server-wide aggregate limit would be exceeded, return a prompt **Queue Full** error; **keep the Session connected**. Do not enqueue or transmit a partial Frame or leave orphan Pending Request state.
2. **Incoming new Application Frame:** if admission would exceed either its Session-local incoming callback queue or the Server-wide aggregate queued-frame budget, **close only that originating Session**, with a distinct overload cause. Do not evict healthy unrelated Sessions, silently discard the frame or turn this into pause/resume handling.
3. **New Bind at bound-Session limit:** reject that new Bind with the existing Pinex-compatible status (`rfail`) where protocol state/available resources permit; do not evict existing bound peers or replace existing System IDs.
4. **Too many pre-bind connections:** refuse excess sockets before allocating a full Session. Absolute **30s** pre-bind deadline remains enforced even with partial frames.
5. **Control queue exhausted:** its distinct **4 KiB per-Session** allowance remains bounded. If required Control PDU progress cannot be achieved within limits, close **that** Session in a controlled way instead of permitting unbounded memory growth. Do not interleave frame bytes, reorder already queued Application frames, or override the Pinex-compatible Unbind drain/10s deadline.

### Important integration and follow-up

- All above behaviors must preserve Q-003 callback isolation, Q-007 Enquire Link response handling, Q-008 outgoing Fail-Fast and Q-009 incoming close-on-overflow. No change to the C++ Pinex wire protocol.
- **API-004 remains OPEN:** the Go Config must distinguish *omitted* `EnquireLinkIdleInterval` (default 30s) from an explicitly configured `0` (disable automatic probes); similarly distinguish omitted limits from invalid explicit zero where required. Propose a concrete config representation and validation before code.
- **API-001/002:** determine exact Session Callback ordering on forced Close, byte accounting for currently executing/in-progress frames, source of global budget ownership and cancellation/pending completion semantics.
- A configured maximum is an admission policy, not pre-allocated memory and not a guarantee about total resident memory. Deployment-specific sizing may override all defaults.

### Future acceptance tests (planned, NOT run)

- For every configuration key: omission selects documented default; explicit valid override is honored; invalid settings fail deterministically.
- Test exactly-at-limit vs one-frame-over for inbound/global/control budgets and bound/prebind admission; simultaneous goroutines must not over-admit.
- Failed outgoing admission leaves Session open and request-tracking clean; failed incoming/global admission closes only offender.
- Pre-bind flood and duplicate ID behavior preserve existing Sessions; Control/Unbind remain bounded and wire-compatible.
- No eager full-capacity allocations, no duplicate Close event, and no unbounded goroutine/buffer retention.

**Q-010 is CLOSED. No code, tests, deployment or repository/module rename is authorized.**

---

## API-004 — Go configuration representation (2026-11-16)

**Status:** CLOSED — OWNER APPROVED OPTION A (Functional Options), 2026-11-16  
**Detailed draft:** [NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md)  
**Implementation:** NOT AUTHORIZED

Q-010 requires all resource ceilings, pre-bind deadline and Control reserve to be independently configurable, and Q-002/Q-006 require omitted `EnquireLinkIdleInterval` to use 30s while an **explicit** `0` disables outgoing Enquire Link.

- **APPROVED Option A:** Go-native typed Functional Options applied over complete defaults, validated as a whole into immutable effective per-instance config. Omitted means default; explicit override (including idle interval zero) remains distinguishable. Invalid resource zero/negative is rejected, not interpreted as unbounded.
- **Option B not selected:** public Config structs using optional/pointer fields as the primary API. Internal/private structs and resolved configuration snapshots are still allowed.
- No live hot-reload semantics are approved; no released exported identifier, package/module path or source changes have been chosen.

**Owner approved Option A:** API-004 is **CLOSED**. Client/Server use typed Functional Options with internally applied complete defaults, independent explicit overrides, validation before starting Dial/Listen, and immutable effective per-instance settings. Caller omission of `EnquireLinkIdleInterval` means 30s; explicit `0` disables outgoing auto-probes, while inbound requests are still answered. No hot reload requirement. Exact exported option/constructor names remain under API-001. See [API-004 contract](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md). API-001, API-002 and API-003 remain OPEN. No code or tests authorized.

---

## API-001 — Public Client/Server/Session design follow-up (2026-11-16)

**Status:** DRAFT / OWNER CHOICE OPEN — no API-001 completion model approved  
**Design:** [NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md)

- Approved API-004 functional options shape is carried into the Client/Server construction and configuration plan.
- Proposed public concepts: Client, Server, Session, Session-bound incoming Request and explicit Response with original Sequence and header Status, per-Session ordered callbacks, and predictable Close/Done behavior under Q-003/Q-008/Q-009.
- **API-001a CLOSED / owner approved A (2026-11-16):** nonblocking request submission with callback completion is V1's primary interface; synchronous Context-first Request (B) was not selected. Pinex 1.5.0 wire compatibility and raw binary payload remain mandatory.
- Do not confuse successful enqueue with network delivery or remote application processing. Completion and transport shutdown guarantees need separate, testable contracts, particularly when arbitrary user callbacks never return.
- Exact method/handler names, Go package/module path, error taxonomy, byte ownership (PERF-002), callback completion bounds (PERF-003), timer/reconnect details remain OPEN. **Performance is a first-class V1 requirement**: evaluate measured allocations/op, copy counts, p95/p99 latency, throughput, GC and concurrency scaling; no numeric performance SLA has yet been approved. No source changes are authorized.

---

## API-001a and PERF-001 — Asynchronous Request model and performance priority (2026-11-16)

**API-001a Status:** CLOSED — OWNER APPROVED OPTION A  
**Performance priority:** OWNER REQUIREMENT — performance is **very important**, first-class V1 engineering criterion  
**Implementation:** NOT AUTHORIZED

- Go Client/Server/Session API must expose a **nonblocking/callback-first primary Request path**. Local SendRequest-style admission returns promptly, with accepted local sequence or a typed failure (such as Q-008 Queue Full); it does **not** wait for the peer's response, nor imply network delivery or application processing.
- Every admitted pending Request completes through **one** local completion path (matching response, timeout, transport failure, shutdown), without double completion or leaked tracker; exactly how the callback is registered is still API-001b.
- Option B (primary synchronous `Request(ctx,...)` returning the peer's response) is **not selected** for V1. Any future optional blocking helper requires separate design and must not degrade the async hot path.
- Performance affects **V1 readiness**: benchmark throughput, response latency percentiles, per-frame allocations/copies, GC pressure, memory and contended multi-Session scaling; protect existing wire and correctness contracts. Numerical pass/fail gates must be defined from reproducible measurement, and **no numeric target is approved**.
- Avoid unbounded goroutines, queues or per-Request completion registrations. Keep Q-003 callback ordering, Q-008/009/010 bounded admission, and Pinex wire semantics.
- **PERF-002 remains OPEN:** decide public `[]byte` ownership/lifetime, choosing between safe copying and explicitly bounded borrowed views (or a measured hybrid). Do not claim zero-copy from the async choice alone.
- **PERF-003 remains OPEN:** bounded outstanding Request/completion admission, dispatch and shutdown semantics under slow user callbacks.
- **Documents:** [Public API Design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md) and [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md).

Owner approval is a **design decision only**. No source modification, benchmarks or execution is authorized.

---

## NAM-002 — Owner-renamed GitHub repository (2026-11-16)

**Status:** RECORDED / VERIFIED — repository path changed by owner, not by this assistant  
**Previous repository:** `majiddarvishan/go-simple-net`  
**Current canonical repository:** `majiddarvishan/neta-go`  
**SSH URL:** `git@github.com:majiddarvishan/neta-go.git`  
**Current branch:** `main`

GitHub connector reads confirm existing `.codex` files are present under the new repository path. The owner changed the GitHub repository name; the assistant only updated `.codex` references. At the root of `main`, neither `go.mod` nor `README.md` was found through the file lookup; this is **not** evidence of a Go module rename. **API-003 remains OPEN** for package and module-path decisions. Pinex C++ and source code remain outside the scope of this documentation update; no tests or builds have been run.

---

## PERF-002 — Payload Ownership / Copy Cost (2026-11-16)

**Status:** CLOSED — OWNER APPROVED OPTION A: Safe Snapshot + Owned Receive  
**Approval:** Project owner explicitly selected Option A after comparison with borrowed buffers and ownership transfer.  
**Implementation:** NOT AUTHORIZED

### Approved public ownership contract

1. **Outgoing `[]byte`:** For an accepted asynchronous `SendRequest`, `SendResponse`, or no-wait Stream send, the library captures the provided payload in **independent internal storage before successful return**. The outgoing frame carries exactly the bytes supplied during the call, independent of subsequent changes to the caller's slice. The library must not rely on the caller's backing array after returning success. The caller must not concurrently modify the source slice *while the snapshot is being made*; a Go slice cannot provide a consistent concurrent snapshot of racy writes.
2. **Admission and copy cost:** Validate frame and quota first where possible; failed local admission retains no caller payload, writes no frame and leaves no pending tracker. The intended normal hot path is **one library-level payload copy into the private outgoing frame** (10-byte Pinex header plus payload), with no extra internal copy between FIFO and the single writer. TCP/kernel copies can still occur: this is **not** end-to-end zero-copy.
3. **Incoming `[]byte`:** Decode into a separately owned buffer, **preferably reading the payload directly into its final owned destination** without an intermediate payload copy. Deliver that buffer to the user callback; the application **may retain it beyond callback return**. The library will not mutate, recycle or pool the user-visible backing buffer after handoff. The payload remains opaque and binary-safe; Sequence and header status are preserved for request/response handling.
4. **Memory and concurrency:** No aliasing of mutable internal scratch with callback-visible payload. While waiting for delivery, incoming payloads remain subject to approved Q-009/Q-010 queue admission rules, with additional bounded in-progress/active accounting to be designed under PERF-003/API-001b. Memory retained by the application after handoff is outside the library's queued-frame accounting and process-RSS guarantee.
5. **Pooling:** Optional internal-only buffer pooling is a subsequent **PERF-004 OPEN** optimization, allowed only after all library users finish with each buffer; no automatic reuse of user-visible incoming payload. No public Borrowed/Ownership Transfer API is approved for V1. No unsafe/zero-copy API is implied.

### Explicitly not decided by this approval

- Outstanding pending/completion count limits, Server aggregate completion slots, incoming callback event-count limit and their numeric defaults (**PERF-003 OPEN**).
- Whether an admitted request's completion callback must run after forced close, callback dispatch and `Done`/terminal notification ordering (**API-001b OPEN**).
- Timer policy, request timeout defaults, reconnect and public method names (**API-002/API-001 OPEN**).
- Concrete size-class pooling, runtime benchmarks or numeric performance SLA (**PERF-004/PERF-001 OPEN**).

**Detailed design:** [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md).  
**Next:** PERF-003, then API-001b. No coding, tests, benchmarks or operational changes.

---

## PERF-003 — Bounded Pending / Completion Admission (2026-11-16)

**Status:** OWNER APPROVED OPTION A — **count-bounded model APPROVED**; numeric defaults and remaining lifecycle details OPEN  
**Approval:** Project owner explicitly selected Option A after comparison with reliance on existing byte queue limits (Option B).  
**Implementation:** NOT AUTHORIZED

### Approved model (policy level)

1. Asynchronous callback-based `SendRequest` uses **bounded outstanding Request/Completion count** rather than relying solely on outgoing byte queues. A request may have left the Writer's FIFO and still be pending a response or queued for delivery to a callback. Such work must remain accounted.
2. Apply **independently configurable per-Session and Server-aggregate count ceilings**. This includes Client-owned Sessions locally; the Server aggregate counts its own Sessions. Capacity is a lazy admission limit, not eager preallocation.
3. The count admission covers the **complete async lifecycle** of an accepted request: waiting for a peer response, a terminal result queued for callback delivery, and the actively executing completion callback (subject to forced-close semantics still open under API-001b). A response/timeout/close transition must not create additional unbounded completion work.
4. On reaching either count ceiling, **reject a new callback-based Request immediately**, without publishing a frame or retaining its pending/sequence state; **keep that Session connected**. The public error should be distinguishable from normal success and from Q-008's outgoing byte-based Queue Full; exact exported error spelling is still OPEN.
5. Preserve all **previously approved** Q-003 same-Session callback serialization and Session independence, Q-008 outgoing 4 MiB/Session byte cap and fail-fast behavior, Q-009 incoming overload closes only offender, and Q-010 aggregate queued-byte budget. These limits complement rather than replace one another.
6. Use the already-approved **API-004 typed Functional Options** to configure the new count ceilings independently; invalid zero/negative count limits should be rejected at configuration validation, not treated as unbounded. Avoid per-Request goroutine fan-out and unbounded callback queues in the design.

### Not approved by this selection

- The previously illustrated count values **1024 per Session** and **65536 per Server** were *examples only*. They are **NOT APPROVED defaults or SLAs**; final defaults, practical maxima and sizing methodology still require assessment/approval.
- A **separate incoming callback event-count cap**, although technically recommended to protect the number of small/empty events, was **not explicit in the owner's selected Option A wording**; the precise cap and overload policy extension remain a design proposal. Existing Q-009/Q-010 incoming **byte limits** and Session-only close reaction stay binding.
- Forced-close behavior, exact completion callback delivery/credits release for a stuck user callback, `OnClose` vs `Done` and terminal event order remain **API-001b OPEN**. Internal once-only settlement is not a promise of unconditional callback execution.
- Start of Request deadline (on admission vs completed TCP Write), defaults, Connect/Bind/Reconnect/Stop policy remain **API-002 OPEN**.
- Specific completion scheduler / timer implementation, any optional internal pooling, exported API names, numerical performance thresholds, builds/benchmarks/tests remain OPEN.

### Next design-only review

Finalize API-001b's forced-close/callback guarantees and then the remaining PERF-003 numerical/event-count details. Keep the contract and open questions reflected in [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md) and [Handoff](HANDOFF.md).

**Option B (byte FIFO bounds only) is NOT SELECTED. No code, tests, benchmarks or operational changes are authorized.**

---

## Q-003-R1 — Configurable Parallel Application Callback Dispatch (2026-11-16)

**Status:** OWNER APPROVED — replaces the unconditional same-Session callback serialization clause in Q-003  
**Owner selection:** **Option B** — parallel application callbacks with configurable maximum concurrency; setting **1** reproduces the serial processing mode for application messages.  
**Implementation:** NOT AUTHORIZED.

### Binding revision

1. The original Q-003 **unconditional serial and non-overlapping application callback rule is revoked**. Deliver received Application messages via bounded Session-local dispatch, with a **positive independently configurable max number of concurrent application callbacks per Session**. Actual execution may overlap when the configured value is greater than one.
2. **Concurrency = 1:** only one application message handler at a time for that Session; subsequent application messages wait, as in old Q-003. **Concurrency > 1:** multiple application handlers for the same Session may run at once; **completion order is not guaranteed**. Preserve FIFO **queue admission/dispatch selection order**, not sequential completion, for parallel mode. Do not infer delivery-order completion.
3. The per-Session limit must be enforced without one unbounded goroutine per received message. Incoming waiting backlogs remain subject to **Q-009/Q-010** configurable byte bounds; if a valid Application frame cannot fit the bounded waiting queue, **close only that Session**, with no silent loss or backpressure policy change. A per-Session worker limit does not increase the 4 MiB incoming queue limit.
4. **Independent internal engine:** frame parsing, Bind/Unbind, protocol Control responses, socket Reader/Writer ownership, Enquire Link, Sequence/Pending correlation, timeout processing, once-only logical completion and Close/Done resource settlement must **not depend on the application handler returning**. These are internal **state-machine functions, not user callback invocations**, and must proceed under bounded internal resources even if an application handler blocks forever. SendResponse remains safe from a handler.
5. If **all** configured application workers are blocked, additional Application messages remain **waiting in the bounded queue**, not silently dispatched to unlimited workers; a subsequent overflow retains the Q-009/Q-010 Session-only close policy. No library can forcibly terminate an arbitrarily blocked user function. Already executing application callbacks may outlive socket closure; library-owned queued buffers, pending state and accounted resources must be reclaimed subject to the still-open API-001b terminal delivery policy.
6. Previous approvals that remain binding: goroutine-safe public Client/Server/Session methods, cross-Session isolation, single serialized TCP Writer with no frame interleaving, deterministic controlled teardown and at-most-once logical terminal state. Pinex C++ 1.5.0 **wire protocol stays unchanged**.
7. The configured `MaxConcurrentApplicationCallbacks` semantic is **approved**; the exact exported option name, **default value**, maximum validation rules, global running-worker admission and whether *user-facing* OnClose/per-request completion callbacks should share or use separate dispatch lanes are **OPEN**. Do not silently infer a default of 1 or N merely from the user's selection.

### Internal-versus-user callback separation (engineering feasibility)

- **Approved intent:** an application callback which hangs must not stop internal protocol/transport and Request/Pending timeout/close state-machine progress.
- **Design recommendation, NOT YET APPROVED as an API-001b choice:** separate callback **work classes**: (a) internal state transitions (no arbitrary user code); (b) bounded application `OnRequest` workers; (c) bounded asynchronous user notification delivery for `OnClose` and per-request completion. The latter remain **user code** and may themselves block. They cannot be guaranteed to run promptly/exactly once under arbitrary permanent stalls without additional scheduling/resource tradeoffs.
- **Open API-001b:** choose completion and lifecycle notification dispatch semantics, cancel/drop or defer unstarted callbacks on forced Close, `Done` signal and per-class ordering. The earlier forced-close Options A and B must be **reevaluated for the new parallel callback design**, not automatically approved by the user's selection of **callback concurrency Option B**.
- The number of **active** callbacks can now be N per Session; their retained payloads and stacks are separate from the Q-010 **waiting queue** 4 MiB byte budget and 256 MiB Server aggregate **queued** application bytes. Propose bounded active-callback count and memory/admission accounting before implementation; do not present these queue limits as a global RSS cap.

### Outstanding design questions

- Default concurrency (recommend a conservative value pending representative load results), independent Server-wide worker ceiling, exact public Functional Option name and count-based validation.
- Whether user-visible Request completions and OnClose execute in a separate bounded lane (and their ordering/overlap semantics).
- API-001b forced Close behavior for already-running and queued application/notification callbacks.
- PERF-003 approved bounded outstanding completion count remains binding and independent of application dispatch concurrency. Numeric pending/completion defaults still OPEN.
- No implementation, tests, benchmark, builds, Go module changes or operational actions are authorized.

**References:** [Public API design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [Resource admission](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md), [Async completion design](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [Handoff](HANDOFF.md).
