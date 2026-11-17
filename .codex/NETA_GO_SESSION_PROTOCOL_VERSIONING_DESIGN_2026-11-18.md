# neta-go — Future Session-Level Protocol Versioning

**Decision:** PROTOCOL-VERSION-001 — OWNER APPROVED (2026-11-18)  
**Scope:** Architecture decision and future design backlog ONLY  
**Implementation:** NOT AUTHORIZED; no production code, tests, CI, wire fixtures or defaults changed  
**V1 / Pinex 1.5.0 contract:** FROZEN  
**Release:** `v1.0.0` remains the already approved intended V1 release; P9-06 remains separately unauthorized

## 1. Motivation and inspected facts

The owner noticed a trailing zero byte after `SystemID` in Bind Request and Bind Response and asked whether it could be removed while remaining backward-compatible.

Current on-wire Pinex 1.5.0 Bind body:

```text
[BindType:1][SystemID:0..19 bytes][NUL terminator:1]
```

Current header remains **10 bytes**: `Length:u32be`, `Command:u8`, `Status:u8`, `Sequence:u32be`; Length includes the header.

Facts verified from the pinned C++ source and the current Go wire implementation:
- Pinex C++ `c_octet_str<20>` searches for the mandatory NUL terminator and throws if absent; the string is limited to 19 bytes.
- `internal/wire/pdu.go` currently emits that NUL and rejects non-NUL-terminated Bind bodies.
- A Bind Response with **entirely omitted body** is accepted by the legacy C++ serializer's `can_be_omitted` rule and by Go, producing default bi-directional / empty-ID values. This does not carry an arbitrary `SystemID`.
- Removing NUL from newly emitted Bind frames and sending them to old Pinex 1.5.0 peers would **break** interoperability. Being able to decode both shapes in a future Go library alone does not make new senders compatible with old receivers.
- A V2 marker cannot simply be inserted into the fixed 10-byte V1 header: existing peers will interpret shifted header fields incorrectly.

## 2. Approved invariant — select once per Session

**PROTOCOL-VERSION-001 (owner-approved):** The effective wire-protocol version is selected once per TCP Session, remains **immutable for the entire Session**, and is not repeated as an extra byte in every Frame.

- **V1 remains the default and is always legacy Pinex 1.5.0 compatible.** Do not change its header, Bind encoder, fixed maximum frame length, command/status/Sequence meaning or legacy response omission semantics.
- A newer version (working name `V2`) may only be activated when **both peers are positively known to support it**. The version is a Session-scoped attribute; never silently switch Frame/Bind formats mid-Session.
- Retain the 10-byte V1 header for legacy Sessions. No new mandatory header field is authorized. The exact V2 layout, if ever needed, is a **separate future design decision**; no assumption that the new protocol must have an 11-byte header.
- No per-Frame version overhead is introduced by this architectural decision.
- Whether optional removal of the Bind SystemID NUL is desirable in a future mutually agreed V2 is **OPEN**, not already approved for implementation.

## 3. Backward-compatibility matrix

| Client | Server | Safe permitted behavior |
|---|---|---|
| Legacy Pinex 1.5.0 | Legacy Pinex 1.5.0 | V1 only, unchanged |
| New | Legacy | V1 only, including NUL-terminated Bind |
| Legacy | New | V1 only, including NUL-terminated Bind |
| New | New | V1 by default; V2 only with **positive prior agreement** |

**Important:** A peer's ability to receive a V1 Bind does **not** by itself prove that it supports V2. Do not assume that unknown commands, reserved Command/Status/Sequence values, an extra header byte, a malformed Bind, or an appended Bind field can serve as a backward-compatible negotiation probe.

## 4. Negotiation / selection mechanism — deliberately not yet frozen

Only the **Session-scoped invariant** is approved today. Selection mechanism remains a separate architectural question:

- **Candidate A: explicit out-of-band version selection**, such as a configured peer/endpoint protocol version. Both ends must be configured consistently, with V1 the default. This is the simplest backward-compatible starting point and requires no speculative legacy-unsafe bytes.
- **Candidate B: in-band capability negotiation**, but only if an independently specified, legacy-safe handshake or discovery mechanism is proven on old and new peers. A V1-compatible Bind alone does not provide a general extensible negotiation field.
- **Candidate C: dedicated versioned endpoint**, if operationally desirable. Old endpoints always speak V1; a separately configured endpoint may speak an explicitly defined new version.

No automatic upgrade, downgrade, fallback reconnect, negotiation packet, new public Functional Option, new command value, or extra connection attempt is authorized by this decision. In particular, do not infer support from a failed Bind or automatically retry using another version: this needs a standalone state-machine, security and side-effect review.

## 5. Deferred design/acceptance tasks (NOT AUTHORIZED)

- [ ] **PV-02:** Select a safe Session version-selection/negotiation mechanism and specify who is authoritative, when version becomes fixed and how mismatch fails.
- [ ] **PV-03:** Specify optional V2 Bind encoding (including whether NUL can be removed), any V2 Header behavior and precise legacy downgrade/error semantics; the NUL removal is not yet approved.
- [ ] **PV-04:** Design API/configuration defaults and capability visibility without breaking the frozen public V1 API.
- [ ] **PV-05:** Plan/execute interoperability and security tests only after separately authorized implementation: old→new, new→old, new↔new V1/V2, unsupported/mismatch, malformed Bind, reconnect/Session generation and mixed-version fleet.

## 6. Preservation / release boundary

This is a **future-protocol design record**, not a V1 change request. It does not reopen the V1 architecture freeze and does not delay or authorize the planned `v1.0.0` release. All existing Pinex 1.5.0 golden vectors and live interoperability evidence stay authoritative.

**No production code, test, workflow, generated file, or wire fixture is modified.** No release/tag is created. Owner's local `stash@{0}` is not touched.
