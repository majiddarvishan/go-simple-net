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

---

## API-001b-N1 — Independent Bounded User Notification Dispatch (2026-11-16)

**Status:** CLOSED — OWNER APPROVED STRUCTURAL SEPARATION  
**Owner confirmation:** Explicit “yes” to delivering `OnClose` and Request Completion callbacks through a separate **bounded** callback path, independent of the Application `OnRequest` worker path.  
**Parent contract:** API-001b remains **PARTIALLY OPEN** for exact completion/close ordering, capacity defaults, and forced-close policy.  
**Implementation:** NOT AUTHORIZED.

### Approved contract

1. Separate three execution domains:
   - **Internal protocol/lifecycle engine:** TCP Reader/Writer, frame decode, Bind/Unbind and Enquire Link control handling, request/response correlation, timeout, once-only logical settlement, connection Close and internal resource/accounting cleanup. **Must not invoke arbitrary application/user callback inline or wait for one to return.**
   - **Application dispatch:** `OnRequest` user handlers; bounded same-Session parallelism under **Q-003-R1** with independently configurable positive concurrency (1 = serial application handlers), bounded inbound queue, existing Q-009/Q-010 overload outcomes.
   - **User notification dispatch:** `OnClose` and per-Request Completion user callbacks use a **separate bounded lane** so blocking `OnRequest` callbacks do not occupy the notification lane or prevent internal lifecycle settlement.
2. The notification lane must **not** run user callbacks on a protocol-engine goroutine, own an unbounded queue, or create an unbounded goroutine per event. It requires explicit queue/admission, worker/concurrency, memory and shutdown bounds consistent with **PERF-003**. Multiple Sessions remain independent as far as bounded resource scheduling permits.
3. A blocked application callback must not by itself block `OnClose`/completion **dispatch** or internal protocol/transport cleanup. This does **not** guarantee that a notification callback itself returns; it is still arbitrary user code and cannot be forcibly terminated by Go.
4. Internal completion outcome is settled once, irrespective of if/when a user notification executes. Preserve PERF-003's approved per-Session and Server-aggregate count-bounded pending/completion admission. No duplicate completion or extra completion slot allocation merely because a response arrived.
5. Preserve Pinex 1.5.0 wire protocol, frame-safe single Writer, Q-008 Queue Full behavior, Q-009 incoming Session-only close-on-overflow, Q-010 bounded byte admission, PERF-002 owned payload contract, and API-004 typed Functional Options.

### Explicitly OPEN at N1 approval time (historical snapshot; later decisions supersede relevant items)

- Whether `OnClose` and per-Request Completion share one notification worker pool vs have sublanes/partitions, and their exact scheduling/ordering/fairness across Sessions. **One bounded notification domain independent of the application domain is the approved minimum.**
- Notification worker-count/default/capacity configuration and aggregate safeguards; **no numeric default was approved**. Previously suggested 1024 per-Session / 65536 Server PERF-003 completion credits are still examples, not chosen values.
- Delivery policy on forced close: **API-001b Option A** cancel not-yet-started callbacks for bounded teardown vs **Option B** retain finite terminal callbacks for conditional later delivery. Neither option was selected by this approval.
- Whether transport-level `Done` and observable terminal cause are exposed, `OnClose` ordering/at-most-once invocation details, independent notification-lane congestion policy, and timing of credit release if a user notification blocks.
- Q-003-R1 application callback concurrency *default*, global active-callback ceiling; API-002 request deadline origin and other timeouts; API-003 Go module/package name.

**Next:** finish API-001b forced-close notification guarantees and numeric bounds, not implementation. No code, tests, benchmarks, builds, dependency/module changes or operational operations.

**References:** [API/performance detailed draft](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md), [Public API](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md), [Handoff](HANDOFF.md).

## API-001b-R2 — Forced Close / Notification Capacity Review (2026-11-16)

**Historical review status (subsequently superseded for the forced-close choice by API-001b-FC-A):** DESIGN ANALYSIS ONLY at that time. Owner-approved API-001b-N1 (independent bounded Notification lane), Q-003-R1 (configurable parallel Application handlers), PERF-003 A count-admission *model*, PERF-002 A, API-001a A and API-004 A remain unchanged. Verified `main@893b96629b69b0c67b5aaf456b3e0f178e9fc811` before this documentation-only review.

**Engineering recommendation, not approval:** prefer **API-001b forced-close Option A** (cancel not-yet-started Completion user notifications and Application work) for bounded teardown; running user callbacks cannot be stopped. Use separate per-Session Request completion credits and globally capped active workers/close-event notification capacity so reconnect churn cannot multiply unaccounted blocked goroutines. Consider a `Done`-like signal for internal transport settlement with best-effort `OnClose` after it; this ordering and all callback delivery guarantees remain **OPEN**.

**Important:** forced-close A has **NOT** been selected by the owner. Previously proposed completion limits 1024/Session and 65536/Server are **NOT approved defaults**. Notification worker counts, close-event budgets, Q-003-R1 callback concurrency default, incoming event-count cap, API-002 deadlines/Stop/reconnect and API-003 module/package path are **OPEN**.

**Design details and explicit question:** [API-001b Forced Close / Notification Review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md). No Go/C++ code, module changes, tests, benchmarks or operations performed.

## API-001b-FC-A — Forced Close Option A (2026-11-16)

**Status: CLOSED / OWNER APPROVED for the forced-close disposition ONLY.** Parent **API-001b remains PARTIALLY OPEN** for Notification ordering, worker/queue ceilings, `Done`, exact OnClose scheduling and API names.

**Owner's explicit choice:** "برای API-001b Forced Close گزینه A را تأیید می‌کنم." This is a new decision subsequent to the historical preapproval comparison `API-001b-R2`. This **does NOT** mean Q-003-R1's former "Option B" or PERF-003 Option A has changed.

### Binding V1 rules

1. On **Forced Close**, atomically cease admission and settle each outstanding Request's *internal terminal result* exactly once (response, timeout, failure or Close race has a unique winner). No later peer response may revive it.
2. **Cancel all not-yet-started queued Application user callbacks and per-Request Completion user notifications**, including already-settled results that have been queued but not yet invoked. Release their library-owned queued payloads/metadata and completion credits; do not retain a deferred backlog of terminal callbacks after forced Close.
3. **Already-running user callbacks cannot be killed.** They may outlive the Session; library-owned capacity/active-worker credits that account for their execution must remain charged until return. A new Session after reconnect cannot bypass finite aggregate execution bounds.
4. The internal Reader/Writer, timers, pending registry, socket closure, resource release and once-only state settlement must **never wait for arbitrary Application or Notification callback return**. This preserves owner-approved Q-003-R1 and API-001b-N1 lane separation.
5. **Accepted Request != guaranteed user Completion invocation.** During ordinary progress, admitted Requests should receive at most one user Completion invocation, but Forced Close **may cancel an unstarted callback**. Logical settlement is exactly once internally; callback invocation is at most once, not unconditionally exactly once.
6. `OnClose` is a **best-effort, at-most-once user notification** under the previously disclosed Option A tradeoff, not an unconditional liveness guarantee when Notification work is congested or permanently blocked. Its exact ordering, capacity-admission, callback overlap and `Done` relationship remain separate **OPEN** choices.
7. Callback start versus Forced Close must have a single race resolution: if Close wins before dispatch start, cancel; if callback-start wins, it is already-running and cannot be stopped. Exact dispatch handoff mechanism is an engineering design detail, not authorization to write code.
8. Preserve approved PERF-003 A per-Session/Server completion-credit admission, Q-008/009/010 bounded queue outcomes, immutable ownership PERF-002 A, API-001a A and API-004 A. No new numeric default is approved.

**Not decided by this approval (historical snapshot):** graceful Stop/drain contract, taxonomy of conditions classified as Forced Close under API-002, `Done` publication, `OnClose` versus Completion execution ordering, single versus partitioned Notification worker domains, Application/Notification worker counts and aggregate budgets, OnClose event capacity, callback concurrency default, PERF-003 numeric credits/event count, API-002 request deadlines and reconnect, API-003 module/package path. Subsequent N-ORD2 and DONE-A entries resolve the Notification sublane and Done-publication questions; worker limits, saturation, graceful Stop and remaining API details stay open.

**Follow-on proposals (not approved):** [Forced-close review and Notification ordering/capacity analysis](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md) §11; [Handoff](HANDOFF.md) §24. **Documentation only; no code, tests, benchmarks, module or operational change.**

## API-001b-NORD2 — Independent Completion / OnClose Notification Sublanes (2026-11-16)

**Status: CLOSED / OWNER APPROVED — Notification class separation and cross-class overlap policy.** Parent **API-001b remains PARTIALLY OPEN** for worker/event capacities and defaults, payload-byte bounds and remaining API-002/API-003 details. Within-Completion concurrency is separately CLOSED by API-001b-CORD-B; Session Done semantics are CLOSED by API-001b-DONE-A.

**Owner's explicit selection:** N-ORD2 — Completion and OnClose execute in distinct, independently bounded Notification sublanes; OnClose may run concurrently with an already-running Completion callback from the same Session. N-ORD1 same-Session serialization across Completion and OnClose is **NOT SELECTED**.

### Binding V1 rules

1. Keep two distinct user Notification work classes:
   - **Completion sublane:** per-Request terminal result callbacks.
   - **OnClose sublane:** Session lifecycle close callback.
   Both remain separate from the bounded Application OnRequest dispatcher and from the internal protocol/lifecycle engine.
2. Completion and OnClose have **independent bounded execution capacity**. Completion callbacks cannot consume the OnClose lane's reserved active-worker capacity or block its dispatch. Separate queues and worker pools are the recommended realization; a common scheduler is acceptable only if it enforces independent per-class queue/event admission and reserved active-worker capacity. A single unpartitioned pool is not N-ORD2.
3. OnClose **may overlap** a Completion callback that already started for that same Session. No cross-class callback start/return ordering is promised. OnClose does not wait for Completion callbacks to return.
4. OnClose remains **best-effort and at-most-once**, not an unconditional delivery guarantee. It is bounded independently of Request completion credits. Forced Close A continues to cancel not-yet-started per-Request Completion callbacks; it does not kill already-running Completion or OnClose code.
5. Internal Request settlement, transport teardown, timers, registry cleanup and resource release remain independent of all three user-code classes. No goroutine-per-event, unbounded queue or waiting for arbitrary user callback return is permitted.
6. Preserve API-001b-FC-A, API-001b-N1, Q-003-R1, PERF-003 A count-admission model, PERF-002 A payload ownership, Q-008/Q-009/Q-010 outcomes and API-004 A.

### Explicitly OPEN — not approved by N-ORD2

- **Resolved by API-001b-CORD-B:** configurable positive same-Session Completion concurrency; 1 is serial, values greater than 1 permit overlap. FIFO start selection follows serialized per-Session terminal-result settlement/enqueue order, not SendRequest admission order. Callback return order is not guaranteed when concurrency is greater than 1.
- Exact Completion and OnClose worker counts, defaults, maxima, per-Session and aggregate quotas, queue/event capacities, fairness policy and validation.
- Terminal-cause accessor and Close return behavior remain open; Done visibility and OnClose eligibility ordering are resolved by API-001b-DONE-A.
- OnClose event-budget saturation policy (recommended: skip the event rather than block cleanup or allocate unbounded work).
- Completion payload-byte accounting in addition to PERF-003 count credits; active callback and user-retained payload memory are not covered by Q-010 waiting-queue byte caps.
- Per-Server / per-Client versus process-wide aggregate active-worker ceiling; graceful Stop/drain and the API-002 definition of Forced Close remain open.
- Numeric PERF-003 per-Session/Server count defaults, incoming callback event-count cap, Q-003-R1 concurrency default, API-002 timeout/reconnect/Stop and API-003 module/package.

**Follow-on worker-capacity proposal:** [Forced Close / Notification / Done / Worker Capacity review](NETA_GO_API_001B_FORCED_CLOSE_NOTIFICATION_REVIEW_2026-11-16.md) §11.7. Done semantics are owner-approved by API-001b-DONE-A and within-Completion concurrency by API-001b-CORD-B; worker counts/defaults, queue/event capacities, payload-byte bounds and saturation policy remain proposals/open.

**Implementation:** NOT AUTHORIZED. Documentation only; no Go/C++ code, go.mod, tests, benchmarks, builds, CI or operational changes.

## API-001b-DONE-A — Session Done Signal After Internal Teardown (2026-11-16)

**Status: CLOSED / OWNER APPROVED.** The owner explicitly selected **Option A** for Session Done semantics.

### Binding V1 contract

1. The public Session lifecycle signal is a read-only channel: **Done() <-chan struct{}**. It is closed exactly once when the Session's **internal teardown is complete**.
2. Before closing Done, the internal lifecycle must:
   - close Session admission and reject new sends/responses;
   - settle each pending Request's internal terminal result exactly once;
   - on a Forced Close path, cancel not-yet-started Application and Completion callbacks under API-001b-FC-A, releasing their queued library-owned payload/metadata and credits;
   - close/detach the transport and stop internal Reader/Writer/control loops;
   - stop Session timers, remove pending/registry state and release internal queue/accounting reservations not retained by already-running callbacks;
   - publish the immutable terminal cause/state.
3. **Done never waits for arbitrary user callback return.** Already-running Application, Completion and OnClose callbacks may outlive Done and retain their active-worker permits until they return. A running Completion also retains its PERF-003 Request completion credit until return.
4. An admitted OnClose event becomes **eligible to start only after Done has closed**. Under owner-approved N-ORD2 it may then overlap a Completion callback already running for the same Session. Done does not guarantee OnClose admission, start or return; OnClose remains best-effort/at-most-once under its separately bounded lane.
5. Done reports internal Session/transport lifecycle termination only. It does not mean every Completion ran, any callback returned, OnClose ran, or all user-retained payload memory was released.
6. This rule applies to the Session terminal transition regardless of which internal close path wins; Client/Server Stop aggregation and graceful-drain semantics remain API-002 work.

### Explicitly still OPEN

- Exported terminal-cause accessor name/signature; the terminal cause itself must already be immutable before Done closes.
- Whether Close() returns before, at, or after Done closes; Done is the observable internal-teardown signal and is not a user-callback join.
- OnClose event admission/saturation policy and numeric event/worker capacities.
- Completion concurrency policy is closed by API-001b-CORD-B; numeric defaults, Worker/event capacities and aggregate-scope details remain open.
- Client/Server Stop and drain behavior under API-002.

**Implementation:** NOT AUTHORIZED. Documentation only; no Go/C++ code, go.mod, tests, benchmarks, builds, CI or operational changes.

## API-001b-CORD-B — Configurable Same-Session Completion Concurrency (2026-11-16)

**Status: CLOSED / OWNER APPROVED.** The owner selected **C-ORD-B** for within-Completion concurrency.

### Binding V1 contract

1. Completion callback concurrency is configurable as a **positive per-Session limit**.
2. A value of **1** gives serial Completion callback execution for that Session. Values greater than 1 permit multiple Completion callbacks from the same Session to overlap.
3. The Session's Completion dispatcher preserves FIFO callback **start selection** by the order in which the internal engine serializes terminal-result settlement/enqueue for that Session. This is **not** SendRequest admission order: network responses may arrive and settle out of request order.
4. With concurrency greater than 1, callback return/completion order is **not guaranteed** to match FIFO start order. Applications sharing mutable state between Completion callbacks must synchronize that state themselves.
5. The configured per-Session limit is always intersected with a finite aggregate active Completion-worker ceiling for the owning Client/Server resource domain. It must never create an unbounded goroutine per Request or bypass the aggregate ceiling.
6. Completion lifecycle admission continues to consume the existing PERF-003 Request credit from before SendRequest acceptance through pending, queued and running Completion callback until return or cancellation. A separate active Completion-worker permit is held only while the user callback is physically running.
7. N-ORD2 remains binding: Completion and OnClose use independent bounded sublanes; OnClose may overlap a running Completion after Done has closed. Forced Close A cancels not-yet-started Completion callbacks on a Forced Close path; already-running callbacks continue and retain their active worker and Request credit until return.

### Explicitly still OPEN

- Completion concurrency numeric default, minimum/maximum public configuration bounds and exact exported option name.
- Active Completion-worker counts and aggregate ceilings; whether a process-wide ceiling is needed in addition to per-owning-Client/Server limits.
- Completion event/queue capacities, OnClose event saturation disposition and payload-byte/frame-size accounting.
- Terminal-cause accessor name/signature and Close() return timing; API-002 Stop/drain/reconnect semantics; API-003 module/package path.

**Implementation:** NOT AUTHORIZED. Documentation only; no Go/C++ code, go.mod, tests, benchmarks, builds, CI or operational changes.


---

## WORKER-001 — Owner-Scoped Worker Capacity (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “برای WORKER-001 گزینه A را تأیید می‌کنم.”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Every Client instance owns an independent callback-resource/worker-capacity domain.
2. Every Server instance owns an independent callback-resource/worker-capacity domain.
3. All Sessions owned by one Server share that Server instance's aggregate callback-worker budgets.
4. All reconnect generations of one Client share that Client instance's aggregate callback-worker budgets.
5. A callback already started in a retired Session generation remains charged to its owning Client/Server active-worker capacity until the callback returns. Reconnect must not reset or bypass that charge.
6. Independent Client and Server instances do not consume each other's worker permits. V1 does not require a hidden process-global callback pool or process-global active-worker ceiling.
7. This decision fixes the **ownership scope only**. It does not select physical worker-pool layout, per-class aggregate-cap policy, numeric worker counts/defaults, queue/event limits, payload-byte bounds, public option names or OnClose saturation behavior.

### Rationale

Instance-scoped capacity preserves isolation between unrelated Client/Server instances, avoids hidden cross-instance contention and keeps the hot path free from a mandatory process-global worker admission point. Retired callbacks remain charged to the same owner, so reconnect churn cannot manufacture additional capacity.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md).  
**Next at the time (resolved by the following WORKER-002 entry):** explicit owner-wide active-worker ceilings were subsequently approved. No numeric defaults were implied.

---

## WORKER-002 — Independent Owner-Wide Active Worker Ceilings (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “برای WORKER-002 گزینه A را تأیید می‌کنم.”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

Each Client instance and each Server instance MUST have three independently configurable aggregate active-worker ceilings, one per user callback class:

1. **Application:** active `OnRequest` callbacks.
2. **Completion:** active per-Request Completion callbacks.
3. **OnClose:** active Session `OnClose` callbacks.

Rules:

- A Session's configured concurrency is intersected with the remaining active capacity of its corresponding owner/class ceiling. Neither setting can be bypassed by the other.
- Capacity is class-isolated: Application, Completion and OnClose cannot borrow, consume or steal one another's active-worker permits.
- The ceiling counts callbacks physically executing, not queued events. Each class's waiting/event population remains bounded separately under its own admission contract.
- Owner scope follows WORKER-001: all Sessions of one Server share its class ceilings; all reconnect generations of one Client share its class ceilings. Started callbacks from retired Session generations continue consuming the corresponding owner/class active permit until return.
- Independent Client/Server instances remain independent. No process-global active-worker ceiling is introduced by this decision.
- Physical pool layout, lazy worker lifecycle, public option names, numeric defaults/minimums/maximums and queue/event limits are not selected here.

### Rationale

Explicit class ceilings make the upper bound independent of current Session count, prevent one callback class from consuming another class's execution reserve, and keep reconnect churn from multiplying active work. They also make resource sizing and overload analysis explicit. The cost is three additional configuration dimensions per Client/Server; this does not require eagerly creating all worker goroutines.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §§3–8.  
**Next:** WORKER-003 — OnClose event-budget saturation and whether a close notification is skipped or protected by admission-time reservation. No numeric defaults are implied.

---

## WORKER-003 — OnClose Event-Budget Saturation (OPEN; RECOMMENDATION ONLY)

**Status: OPEN — no owner selection yet.**  
**Implementation:** NOT AUTHORIZED.

WORKER-002 bounds physically active OnClose callbacks but does not bound queued events waiting behind blocked workers. A separate owner-scoped event budget is required. It should count queued + running OnClose events, with at most one event per Session. Events from old Client reconnect generations remain charged to the same Client owner until callback return.

### Option A — skip the new event when the event budget is full (RECOMMENDED)

After Session `Done` closes, attempt a nonblocking reservation from the owning Client/Server OnClose event budget. If it succeeds, enqueue the event; it may start only after Done and when an OnClose active-worker permit is available. If the event budget is full, permanently skip this invocation: no retry, no overflow goroutine and no delay to Done/internal cleanup. Started callbacks hold both active-worker and event credits until return. This is consistent with the approved best-effort/at-most-once OnClose contract.

### Option B — reserve one event credit before admitting each Session

Reserve a future OnClose event credit when admitting a Session. Convert it to a queued/running event when the Session closes and release it after callback return. If no reservation is available, reject/defer new Server Session admission or Client reconnect. This gives admitted Sessions a stronger OnClose delivery guarantee but couples connection availability to OnClose callback progress; permanently blocked callbacks can exhaust reservations and stop future admission. Pre-bind, rejected Bind, reconnect and sessions without an OnClose handler need additional rules.

### Recommendation and owner decision

Recommend **Option A**: a nonblocking skip on saturation. N-ORD2 and DONE-A already make OnClose independent and best-effort; transport admission should not depend on arbitrary user callback progress.

**Owner decision requested:** choose Option A (skip OnClose on event-budget saturation) or Option B (reserve at Session admission and backpressure new Sessions/reconnects). No numeric event-budget default is part of this decision.

---

## WORKER-003 — OnClose Event-Budget Saturation (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “گزینه A” (skip new OnClose event on saturation).  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. OnClose event admission is best-effort and at-most-once per Session.
2. The owning Client/Server has a separate bounded OnClose event budget. Its usage counts **queued + currently executing** OnClose events, not merely waiting events.
3. After Session `Done` closes, if an OnClose callback is configured, the engine makes one nonblocking attempt to reserve one event credit from the owning instance.
4. If reservation succeeds, the event is queued and may execute when an OnClose active-worker permit is available. The event credit remains held from admission through callback return, including while queued.
5. If reservation fails because the event budget is full, that Session's OnClose callback is skipped permanently. No retry, overflow goroutine, hidden queue or wait is permitted.
6. A started OnClose callback holds both its WORKER-002 active-worker permit and its event credit until return. A queued event holds its event credit until it starts and returns.
7. Skipping OnClose never delays or changes Session `Done`, terminal cause, socket closure, internal cleanup, reconnect eligibility or Application/Completion capacity. It does not consume PERF-003 Request credits.
8. All reconnect generations of one Client share the same OnClose event budget under WORKER-001. A retired generation's queued/running OnClose event remains charged until callback return.
9. No numeric event-budget default, public option name or overflow diagnostic API is selected here.

### Rationale

This preserves transport and internal-engine liveness when user OnClose callbacks block forever. It follows N-ORD2/DONE-A's independent, best-effort OnClose contract and avoids coupling connection admission/reconnect to arbitrary user callback progress. The explicit queued+running event count prevents unbounded retained Session references and event metadata.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §10.  
**Next:** WORKER-004 — choose the logical execution-pool topology: three class-isolated worker pools versus a shared owner scheduler with strict per-class active reservations. Numeric capacities remain open.


## WORKER-004 — Three Class-Isolated Worker Pools (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Every Client instance and every Server instance owns three logically and physically isolated callback worker pools: Application, Completion and OnClose.
2. Each class pool has its own scheduler, ready-session/work queue, dispatch path, lock domain and wake-up mechanism. A common internal worker-loop implementation may be reused, but the pools must not share execution capacity, queues, scheduler locks or wake-up state.
3. Workers are created lazily when dispatchable work exists and no reusable idle worker in that same class is available. No worker is created per Request, frame or callback.
4. A worker executes one callback synchronously at a time and is reusable only within its own class pool. It never executes work belonging to another callback class.
5. The number of worker goroutines in a class is bounded by that class's configured WORKER-002 active-worker ceiling. An idle worker does not hold an active-callback permit, but its goroutine and stack remain part of the bounded worker footprint.
6. A blocked callback occupies only a worker in its own class. Saturation or permanent blocking in Application must not occupy Completion or OnClose workers; Completion must not occupy Application or OnClose workers; OnClose must not occupy either other class.
7. The internal protocol/transport engine remains outside all three user-callback pools and must not wait for arbitrary user callback return.
8. WORKER-002 owner-wide active ceilings, per-Session concurrency intersections, WORKER-001 owner scope and retired-generation accounting remain binding. Client reconnect does not create a new pool or reset old callback charges.
9. Independent Client/Server instances remain isolated. No mandatory process-global worker pool or process-global callback ceiling is introduced.
10. Pool lifecycle, scheduling and resource accounting are independent per class even if implementation code for worker loops or small internal primitives is shared.

### Rationale and performance consequences

Three independent pools provide the strongest protection against cross-class head-of-line blocking. A permanently blocked Application handler cannot consume the goroutines or scheduler path required for Completion or OnClose. Separate queues, locks and wakeups reduce cross-class lock contention and keep the hot dispatch path easier to reason about.

The cost is that idle workers cannot be borrowed by another class, and each owner has three pool lifecycle domains. Lazy creation is mandatory to avoid eagerly paying the sum of all configured ceilings. Idle-worker retirement is a separate WORKER-005 decision.

### Rejected alternative

Option B — one shared owner worker pool with strict per-class active reservations — is not selected for V1. Although it can reuse idle goroutines across classes, it introduces a shared dispatch/wakeup/lock path and additional cross-class scheduling/fairness complexity. That trade-off is not justified when arbitrary user callbacks may block indefinitely.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §11.  
**Next:** WORKER-005 — Worker Lifecycle and Idle Worker Retirement. Numeric worker ceilings, idle timeout defaults, queue/event limits and payload-byte limits remain open.


## WORKER-005 — Finite Idle Worker Retirement (2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION B.**  
**Owner selection:** “برای WORKER-005 گزینه B را تأیید می‌کنم.”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Each Client/Server owner retains the three class-isolated WORKER-004 pools. Workers are still created lazily and reused only within their own class.
2. After a callback returns, if its class pool has no dispatchable work for that worker, the worker may enter Idle and start that class's finite idle-grace timeout.
3. If work arrives before expiry, the idle worker is reused. If the timeout expires and no dispatchable work exists, the worker atomically removes itself from the pool's worker count and exits.
4. The default warm-idle floor is **zero**: after sufficient inactivity, a class pool may return to zero idle worker goroutines. A nonzero warm floor is not part of the approved V1 default.
5. Idle timeout is independently configurable for Application, Completion and OnClose. Its numeric default and allowed range are deferred to WORKER-009.
6. A worker executing user code is not idle and must never be retired, interrupted or have its active permit released due to the idle timeout. A permanently blocked callback remains charged until it actually returns.
7. Retirement does not discard queued work or release Application queue-byte/event charges, Completion Request credits, OnClose event credits or any other work-item ownership. These are released only by their own lifecycle transitions.
8. Enqueue versus retirement must have one linearization point. If enqueue wins, an idle worker is woken/reused; if retirement wins, the pool's worker count is decremented before exit and enqueue/dispatch observes the new count and creates a replacement as needed. No lost wakeup, stranded ready Session or over-ceiling worker creation is permitted.
9. The idle timeout starts only after callback return and after the worker has established that no dispatchable work is available. It does not bound callback duration.
10. The implementation must not create a goroutine per idle timeout or a process-global reaper. Reusable timer/wait state is preferred; repeated idle cycles should avoid unnecessary timer allocation.
11. Owner Stop/Close and active-worker drain/cancellation semantics remain API-002; WORKER-005 does not decide them.

### Rationale

Finite idle retirement prevents long-lived owners from retaining their historical peak number of worker goroutines and stacks indefinitely. A zero warm-idle floor gives predictable quiescent resource usage across many independent Client/Server owners. A finite grace period still permits reuse during short gaps, trading a possible cold-start cost after long inactivity for lower idle memory and goroutine retention.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §12.  
**Next at that time:** WORKER-006 (subsequently approved as Option A; see the WORKER-006 decision below). Numeric idle timeout defaults/ranges remain open for WORKER-009.

---

## WORKER-006 — Fair Scheduling Between Sessions (OWNER APPROVED, OPTION A, 2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “برای WORKER-006 گزینه A را تأیید می‌کنم.”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Each of the three independent callback-class pools (Application, Completion and OnClose) schedules eligible Sessions using its own FIFO round-robin ready-Session queue.
2. Each Session may have at most one ready node in a given class pool. The node is reusable and represents the Session's eligibility, not an individual callback.
3. A worker selects one callback from the head Session's per-Session FIFO queue per turn. If the Session still has queued work and a free per-Session concurrency slot, its ready node moves to the tail. It does not drain its entire backlog in one turn.
4. If the Session has no queued work, remove/clear its ready state. If its per-Session concurrency limit is full, it is temporarily ineligible; when a callback returns, the Session is re-evaluated and, if work remains, re-enters at the tail.
5. Preserve FIFO callback-start selection within each Session. For Completion, C-ORD-B remains binding: order is the internal engine's serialized terminal-result settlement/enqueue order, not SendRequest admission order. When Completion concurrency is greater than one, callback return order remains unspecified.
6. Fairness is scoped to continuously eligible Sessions within one owner and one callback class. It does not guarantee equal CPU time or preempt already-running callbacks.
7. Application, Completion and OnClose ready queues, locks and wake-up state remain physically independent under WORKER-004. No cross-class scheduling or capacity borrowing.
8. No scheduler lock is held while invoking user code. Dispatch and ready-node rotation use bounded critical sections. No per-callback goroutine or per-turn heap allocation is required; ready nodes are reusable per Session/class.
9. A Session's eligibility, callback start and Forced Close cancellation must retain the existing single linearization boundary. A callback cancelled before start is never dispatched; a callback whose start wins retains its active permits until return.
10. The round-robin policy does not change WORKER-001 owner scope, WORKER-002 active ceilings, WORKER-003 OnClose event saturation, WORKER-004 pool isolation or WORKER-005 idle retirement.

### Rationale and performance consequences

One callback per eligible Session turn prevents a continuously busy Session from draining all available dispatch opportunities ahead of other ready Sessions. A single reusable ready node per Session/class bounds scheduler metadata by the Session population rather than callback volume. O(1) queue rotation is possible with a per-class FIFO ready list and a short isolated lock domain.

The policy gives dispatch fairness, not CPU-time fairness: a slow callback remains non-preemptive and occupies its own class worker until return. Weighted/quantum scheduling would add configuration and could increase tail latency for low-volume Sessions without guaranteeing CPU fairness.

### Rejected alternatives

- **Option B — weighted round-robin / configurable per-Session quantum:** not selected. It adds public policy and scheduling state; callback-count weights cannot guarantee CPU-time fairness and large quanta worsen tail latency.
- **Option C — global FIFO by enqueue time:** not selected. A high-volume Session can flood the queue and delay sparse Sessions; enqueue-volume order is not Session fairness.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §13.  
**Next:** WORKER-007 — Queue Capacity and Event Capacity. Numeric limits and safe defaults remain open for WORKER-009.

---

## WORKER-007 — Queue Capacity and Event Capacity (OWNER APPROVED, OPTION A, 2026-11-16)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “گزینه A”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Add a configurable incoming Application callback-frame count ceiling per Session, alongside the existing incoming byte ceiling. It bounds admitted inbound Stream Request events waiting to start an Application callback. A matching Stream Response destined for a Completion callback is already bounded by its PERF-003 Request credit and must not acquire a duplicate Completion event-count credit.
2. Add a configurable outgoing Application frame count ceiling per Session, alongside the existing outgoing byte ceiling. It covers every admitted Application frame, including the frame currently owned by the Writer, until the full frame is written or definitively discarded.
3. Add one configurable Server-aggregate queued Application frame count ceiling covering the combined incoming and outgoing queued Application frame populations. It is distinct from the already-approved aggregate queued-frame byte ceiling. It is a Server scope; Client reconnect generations continue to share Client-owned capacity according to WORKER-001 and the later payload accounting contract.
4. Incoming count admission is reserved before the body is allocated/read once the fixed header has established a valid Application Request frame. The count remains held while the frame is waiting to start its Application callback and is released at the callback-start linearization point. The in-progress Reader reservation must not create an unbounded or unaccounted frame.
5. If the incoming per-Session count or Server aggregate count cannot admit a valid Request frame, close only the originating Session under the already-approved Q-009/Q-010 overload policy. Do not drop, pause/resume, or defer without bound.
6. If the outgoing per-Session count or Server aggregate count is full, reject the new Send immediately with Queue Full, keep the Session connected, and create no pending/sequence state or transmitted frame for a rejected SendRequest.
7. Keep the existing byte limits independent: outgoing 4 MiB per Session (Q-008), incoming 4 MiB per Session (Q-010), and 256 MiB aggregate queued Application frame bytes per Server (Q-010). The aggregate byte budget covers incoming + outgoing queued Application frames. A frame is charged to exactly one queued owner bucket at a time.
8. Completion count remains the PERF-003 per-Session and Server-aggregate Request credit covering the complete Request lifecycle through callback return/cancellation. Do not add a second Completion event-count ceiling or reserve another credit on terminal settlement.
9. OnClose count remains the WORKER-003 owner-scoped event budget covering queued + running OnClose events. Do not add a second OnClose queue/event count.
10. Control-PDU headroom remains separate at the existing configurable 4 KiB per Session. Do not merge Control, Application or Completion populations into one cross-class event queue/budget.
11. All count limits are lazy admission ceilings, not eager preallocation. Numeric defaults, valid ranges, public option names and maxima remain deferred to WORKER-009/API-004.

### Rationale and performance consequences

Byte-only limits do not adequately bound queue-node and event metadata for tiny or empty frames. Per-Session counts isolate noisy peers; the Server aggregate count prevents the sum of many individually compliant Sessions from producing an unbounded queued-frame population. Reusing PERF-003 and WORKER-003 credits avoids duplicate Completion/OnClose accounting.

The additional hot-path cost is a bounded count reservation/release alongside byte admission. The Server aggregate count is a shared cache-line/counter and may contend under high fan-in; implementation should keep the reservation path short and avoid a long-held lock. Any sharding/token-cache optimization requires later measurements and must preserve a strict aggregate ceiling.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §14.  
**Next:** WORKER-008 — Resource Accounting and Payload Memory Ceiling.

---

## WORKER-008 — Resource Accounting and Payload Memory Ceiling (OWNER APPROVED, OPTION A, 2026-11-17)

**Status: CLOSED / OWNER APPROVED — OPTION A.**  
**Owner selection:** “A”  
**Implementation:** NOT AUTHORIZED.

### Binding V1 contract

1. Maintain two independent retained-payload byte ledgers: **Application** and **Completion**. Each has a per-Session ceiling and an aggregate per-owner ceiling. Owner scope follows WORKER-001: each Client and each Server has its own budgets; all reconnect generations of a Client share the same owner budget.
2. **Application retained bytes** cover inbound Stream Request frame storage from validated-header reservation and before body allocation, through body assembly, waiting callback queue and active OnRequest callback until callback return or cancellation. They also cover outbound private serialized Application frame storage from accepted Send admission through full Writer completion or definitive discard.
3. **Completion retained bytes** cover a matched Stream Response frame from before body allocation through queued Completion delivery and active Completion callback until callback return or Forced Close A cancellation. PERF-003 Request credits remain a separate count/lifecycle ledger; the Completion byte ledger does not create another event credit.
4. A started callback retains its payload-byte charge until actual return, even after Session Done, socket teardown or Client reconnect. An unstarted callback cancelled by Forced Close A releases its library-owned payload and its existing queue/Request credits exactly once.
5. A payload moving from an incoming queue to an active OnRequest callback releases only the waiting-queue byte/count charges at the callback-start linearization point. Its retained Application byte charge follows the same owned buffer until callback return. Moving an outgoing frame from FIFO to the single Writer releases no byte/count charge; release occurs only after full Write or definitive discard.
6. The outgoing private frame is charged by its full serialized frame length, including the 10-byte Pinex header, consistently with Q-008. Incoming Stream Request and Response reservations use the declared full frame length conservatively even if the final user-visible allocation stores only the body.
7. Reserve the declared destination byte budget and applicable queue/count admission before allocating or reading the body. A partial body read holds its full declared reservation; read failure, invalidation or close releases it exactly once. An oversized declared frame is rejected before body allocation.
8. The existing Q-008/Q-009/Q-010 queue byte/count ceilings remain independent admission sub-limits. They are not added to retained-payload ledgers to estimate physical RSS: the same frame may be subject to multiple independent ceilings, but it is one physical buffer and must not be double-counted in any single memory total.
9. After the outgoing frame is fully written, the original request payload storage is released. Pending correlation retains only required Request metadata, not a duplicate outgoing payload.
10. Unknown/late Stream Responses with no live Pending Request do not resurrect a Request or create Completion work; they are handled by the bounded protocol discard/close policy, not retained as an unowned Completion payload.
11. Control PDU storage remains in its separate Q-010 per-Session 4 KiB reserve. OnClose has no payload ledger under the current API. No maximum queue or payload capacity is eagerly preallocated.
12. The library never recycles or pools an inbound backing array after it is exposed to user code. If the application retains that slice after callback return, that retention is outside the library-enforceable byte ceiling.

### Rationale and performance consequences

Two class-separated ledgers provide a direct finite bound for payload memory retained by user callback work without introducing a counter for every lifecycle stage. Per-Session limits isolate a single noisy Session; aggregate Client/Server limits prevent reconnect generations or many Sessions from multiplying the owner's budget. Queue-to-active transfer is a release of queue-only admission, not a release/re-reservation of the underlying payload bytes.

The hot path adds a bounded reservation/release alongside queue/count admission. Session-local and owner counters must be updated with short race-safe operations; no owner lock may be held during user code or network I/O. Any future sharding must preserve a strict aggregate ceiling. These ledgers bound library-owned frame/payload storage, not process RSS, allocator fragmentation, Go metadata/stacks, kernel buffers or memory retained by the application.

### Explicitly unresolved; must close before Architecture Freeze

- If a valid matching Stream Response cannot reserve Completion retained bytes, the engine must not silently drop it, corrupt the TCP stream or create an unbounded overflow path. The exact policy (e.g. bounded discard plus one terminal resource error versus closing the offending Session and settling its pending requests) remains a separate owner decision.
- The compatibility reference currently records a 1 MiB maximum frame, but the exact C++ Pinex 1.5.0 maximum and Go/C++ cross-language confirmation are not yet frozen.
- Numeric retained-byte ceilings, count defaults, worker ceilings, valid ranges and public option identifiers remain WORKER-009/API-004 design work.

**Detailed design:** [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §15.  
**Next:** WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits.

---

## WORKER-009 — Defaults, Valid Ranges and Public Configuration Limits

**Status:** WORKER-009A CLOSED / OWNER APPROVED OPTION A; WORKER-009B OPEN (numeric defaults are proposals only).  
**Implementation:** NOT AUTHORIZED.

### 1. Scope and decomposition

WORKER-009 establishes zero-configuration defaults and finite validation boundaries without weakening the bounded-resource contracts.

- **WORKER-009A — CLOSED:** derive the three owner active-worker defaults from GOMAXPROCS; keep other resource quotas fixed and finite.
- **WORKER-009B1 — OPEN:** per-Session Application/Completion callback concurrency defaults and per-class idle-retirement grace defaults.
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

The following matrix is the current engineering recommendation. Only the three worker formulas in §2 are approved; every other value below remains open.

#### 3.1 WORKER-009B1 — per-Session concurrency and idle retirement

| Setting | Recommended default | Scope | Reason |
|---|---:|---|---|
| Application callback concurrency | 1 | Per Session | Preserves serial behavior by default; users opt into overlap explicitly |
| Completion callback concurrency | 1 | Per Session | Preserves FIFO callback-start order without overlap by default |
| Application worker idle grace | 30s | Application pool per owner | Reuses workers across short traffic gaps |
| Completion worker idle grace | 30s | Completion pool per owner | Avoids churn during response bursts |
| OnClose worker idle grace | 30s | OnClose pool per owner | Gives close bursts a reuse window while retaining finite retirement |

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
- The earlier 4 MiB Completion-per-Session draft is superseded by this 16 MiB recommendation. Against the still-provisional 1 MiB maximum-frame reference, 4 MiB would permit only four maximum-size Response payloads for one Session. 16 MiB is a less restrictive bounded burst allowance; the 256 MiB owner ceiling still caps aggregate Completion payload retention.
- These values are admission ceilings, not preallocations. User-retained inbound payload after callback return remains outside the library-owned retained-byte ledger.

The Pinex reference's 1 MiB maximum frame remains provisional until the exact C++ Pinex 1.5.0 baseline and cross-language behavior are confirmed. WORKER-009B does not freeze that frame maximum.

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

**Next: WORKER-009B1.** Confirm or revise the two per-Session callback-concurrency defaults (Application=1, Completion=1) and the three per-class idle grace defaults (30s each). After that, resolve B2 and B3 independently. WORKER-009C follows once numeric defaults are approved.

**Guardrail:** design/documentation only. No Go/C++ source, tests, benchmarks, builds, dependency changes, CI, deployment or operational changes are authorized.

---

