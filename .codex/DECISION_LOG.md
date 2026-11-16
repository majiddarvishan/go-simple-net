# Decision Log — Go Simple Net / Pinex Compatibility

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

**Status:** APPROVED — owner confirmed all five proposed rules  
**Applies to:** GO-PINEX-002, GO-PINEX-004, GO-PINEX-007, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-013  
**Implementation:** NOT AUTHORIZED

### Confirmed contract

1. **Goroutine-safe public APIs:** Go Client, Server and Session methods must support concurrent calls from multiple goroutines.
2. **Per-session callback ordering:** Callbacks for a given Session must execute serially, in message-receipt order. There must not be simultaneous callbacks for the same Session.
3. **Session independence:** A slow or blocked callback for one Session must not stall callbacks or network handling for unrelated Sessions. The owner approved independence, not unlimited background goroutines.
4. **Concurrent sends:** Multiple goroutines may send over a single Session concurrently; outgoing Frame bytes must not interleave or corrupt. A session-local serialized writer must preserve the integrity of each frame.
5. **Controlled shutdown:** On disconnect or shutdown, callback completion/cancellation and outstanding operations must be handled deterministically without duplicate terminal notifications, goroutine leaks, hangs or corrupted state.

### Engineering constraints for later API design

- The precise callback signatures, queue capacities, backpressure strategy, shutdown deadline and pending-request error taxonomy remain to be designed within GO-PINEX-002/004/007/010 and must not be silently assumed from this policy.
- Keep blocking user callback execution off unrelated Session readers/writers. Do not hold global/session transport locks across user callbacks.
- Race tests, concurrent-send frame-integrity tests, per-session order tests, cross-session isolation tests, and shutdown/churn tests are part of the planned acceptance work.
- These are requirements to implement later; **no application code is approved**.

Q-003 is **CLOSED**.

---

## NAM-001 — Go library naming (2026-11-16)

**Status:** CLOSED — APPROVED: `neta-go`  
**Approved by:** Project owner  
**Implementation / repository rename:** NOT AUTHORIZED

### Final owner decision

- The selected name for the **native Go TCP Messaging Client/Server** is **`neta-go`** (NETA Go).
- The name **NETA** had been proposed as a short form of *Network Easy Transport API*; the owner explicitly selected the name `neta-go` (not a separately approved expansion).
- It must remain interoperable with the C++ Pinex 1.5.0 wire protocol. Choosing a Go library name does not rename or change C++ Pinex.
- This settles the **library branding** only. Proposed code package name `neta` and future repository/module name `neta-go` are implementation/migration proposals, not performed operations.
- Current GitHub repository remains `majiddarvishan/go-simple-net` and existing branch remains `main`.
- Do not change code, module paths, imports, README branding outside documentation, repository settings, or C++ Pinex naming without a separate instruction to execute those changes.
- Naming collision, module-path availability, and trademark review have not been performed; verify as needed before public release.

### Next planning work

Continue Q-004 and the outstanding configuration/API details. No application implementation is authorized.

---

## Q-004 — Optional features scope for neta-go V1 (2026-11-16)

**Status:** PROPOSED — awaiting explicit owner approval  
**Applies to:** GO-PINEX-002, GO-PINEX-008, GO-PINEX-009, GO-PINEX-010, GO-PINEX-011, GO-PINEX-014  
**Implementation:** NOT AUTHORIZED

### Background from the existing project task plan

The uploaded Go sample includes several application-oriented features that are not part of Pinex C++ 1.5.0's wire contract. Go V1 must remain a general-purpose, interoperable TCP messaging library rather than inherit SS7-specific or sample-only policy.

### Proposed V1 Core (retain)

- Shared Pinex-compatible codec, TCP client/server, session and Bind/Unbind state machine.
- Opaque binary payload, stream request/response, sequence correlation and request deadlines.
- Concurrent-safe public API, per-session ordered callbacks, frame-safe concurrent writes and independent sessions per approved Q-003.
- Bounded queues, backpressure and connection admission for predictable resource behavior; FIFO by default.
- Configurable client reconnect, transport timeouts, TCP keepalive and approved idle-only Enquire Link (Q-002), including `0` to disable automatic sending.
- Controlled closure, terminal event notifications, and direct Go↔C++ Pinex interop tests.

### Proposed V1 exclusions / future backlog

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

### Proposed acceptance checks

- V1 can perform complete Bind, Stream, Enquire Link and Unbind interoperability against C++ Pinex with only the Go Core.
- No optional application feature is needed to get a usable client/server.
- No automatic sequence deduplication; repeated valid sequence-like payload content is not dropped.
- Overloaded sessions use defined bounded behavior and do not cause unbounded memory growth.
- Optional application policies can be layered outside the public API without wire changes.

### Approval requested

Owner to choose **A — approve the lean V1 Core and defer the optional features above**, or **B — identify specific optional capabilities that must ship in V1**.

This entry is **not** an approved decision until the owner replies. No source or build modifications are authorized.
