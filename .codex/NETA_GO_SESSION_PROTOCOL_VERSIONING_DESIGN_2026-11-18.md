# neta-go — Future Session-Level Protocol Versioning

**Decision:** PROTOCOL-VERSION-001 — OWNER APPROVED (2026-11-18)  
**Scope:** Architecture decision and future design backlog ONLY  
**Implementation update:** Original decision was design-only. Subsequent owner approval (PV-01) authorizes Go-only local Session version configuration with V2 default; actual V2 wire/negotiation remains NOT AUTHORIZED.  
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

- **Originally V1 was the default; PV-01 now supersedes the Go local default to V2, without changing the V1-compatible wire format.** Do not change its header, Bind encoder, fixed maximum frame length, command/status/Sequence meaning or legacy response omission semantics.
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
| New | New | Go local setting now defaults to V2; both values currently use identical V1 wire bytes. Future wire-divergent V2 requires **positive prior agreement**. |

**Important:** A peer's ability to receive a V1 Bind does **not** by itself prove that it supports V2. Do not assume that unknown commands, reserved Command/Status/Sequence values, an extra header byte, a malformed Bind, or an appended Bind field can serve as a backward-compatible negotiation probe.

## 4. Negotiation / selection mechanism — deliberately not yet frozen

PV-01 now authorizes local Go configuration via WithProtocolVersion. Real peer capability negotiation and a divergent V2 wire mode remain separate architectural questions:

- **Candidate A: explicit out-of-band version selection**, such as a configured peer/endpoint protocol version. Both ends must be configured consistently, with V1 the default. This is the simplest backward-compatible starting point and requires no speculative legacy-unsafe bytes.
- **Candidate B: in-band capability negotiation**, but only if an independently specified, legacy-safe handshake or discovery mechanism is proven on old and new peers. A V1-compatible Bind alone does not provide a general extensible negotiation field.
- **Candidate C: dedicated versioned endpoint**, if operationally desirable. Old endpoints always speak V1; a separately configured endpoint may speak an explicitly defined new version.

PV-01 separately authorizes only the new local WithProtocolVersion Functional Option and getter. No automatic upgrade, downgrade, fallback reconnect, negotiation packet, new command value, or extra connection attempt is authorized. In particular, do not infer support from a failed Bind or automatically retry using another version: this needs a standalone state-machine, security and side-effect review.

## 5. Deferred design/acceptance tasks (NOT AUTHORIZED)

- [ ] **PV-02:** Select a safe Session version-selection/negotiation mechanism and specify who is authoritative, when version becomes fixed and how mismatch fails.
- [ ] **PV-03:** Specify optional V2 Bind encoding (including whether NUL can be removed), any V2 Header behavior and precise legacy downgrade/error semantics; the NUL removal is not yet approved.
- [x] **PV-01:** Go-only local configuration / getter implemented, default V2, without changes to on-wire encoding.\n- [ ] **PV-04:** Future negotiated peer-version reporting and new V2-wire-specific API requires separate design approval.
- [ ] **PV-05:** Plan/execute interoperability and security tests only after separately authorized implementation: old→new, new→old, new↔new V1/V2, unsupported/mismatch, malformed Bind, reconnect/Session generation and mixed-version fleet.

## 6. Preservation / release boundary

This is a **future-protocol design record**, not a V1 change request. It does not reopen the V1 architecture freeze and does not delay or authorize the planned `v1.0.0` release. All existing Pinex 1.5.0 golden vectors and live interoperability evidence stay authoritative.

**No production code, test, workflow, generated file, or wire fixture is modified.** No release/tag is created. Owner's local `stash@{0}` is not touched.


## 7. Owner-approved Go local version API — PV-01 (2026-11-18)

The owner explicitly requested adding version selection **only to Go** with **V2 as the default**. This supersedes the previously designed Go-local default of V1, but **does not authorize removing the trailing Bind NUL** or changing the Pinex wire contract.

- Public `ProtocolVersionV1=1`, `ProtocolVersionV2=2`.
- `WithProtocolVersion` is a `SessionOption` applied through existing Client/Server Session options, default V2.
- `Session.ProtocolVersion()` reports the immutable **locally configured** value, **not a peer-negotiated version**.
- Invalid values are rejected at Client/Server construction; tests cover defaults, overrides and immutability.
- Both versions currently use the same unchanged 10-byte Header, NUL-terminated Bind and Stream encoding. Pinex 1.5.0 compatibility is preserved.

**Release boundary:** The prior P9-05 release signoff predates this new Go public API/default. Requalification (PV-01-R) and amended CHANGELOG approval are required before any separately authorized P9-06 publication. No tag/Release is authorized.


## 8. Owner-approved PV-02A — V2 NUL-free Bind (2026-11-18)

The owner explicitly requested removing the NUL from V2. This supersedes the temporary PV-01 assumption that V1 and V2 use identical on-wire encoding. The change is **limited to Go Bind Request and Bind Response**; V1 remains unchanged. The 10-byte Frame Header and all Stream/Control frames are unaffected.

- V1: `BindType (1 byte) + SystemID (0..19 bytes) + NUL (1 byte)`.
- V2: `BindType (1 byte) + SystemID (0..19 bytes)` (no terminator).
- V2 Client (default) sends V2 and validates NUL-free V2 Bind Response. It **cannot connect to a legacy Pinex 1.5.0 Server** unless explicitly configured V1; no automatic fallback is attempted.
- Default V2 Go Server identifies V1 by the trailing NUL on the first Bind and V2 by its absence; the selected version is fixed for that Server Session and used for Bind Response. A configured V1-only Go Server rejects V2.
- This is **format identification**, not a general extensible negotiation handshake; no new Header field/command is added. Future V3 needs a separate agreement mechanism.
- Empty BindResp Body, when omitted, keeps the original default-body decode allowance. The previous Pinex V1 golden fixtures must continue matching **exactly**.

This behavior intentionally introduces new V2 wire bytes, so previous Pinex compatibility claims must be qualified: **V1** remains Pinex-compatible, while **V2 Bind** is not decodable by Pinex 1.5.0. Cross-version testing and release-readiness requalification are mandatory. P9-06 remains NOT AUTHORIZED.


## 9. PV-02B — V2 SystemID capacity raised to 20 bytes (owner-approved 2026-11-18)

The owner directed an increase of the V2 `SystemID` limit to **20 bytes** now that the trailing NUL is absent. This **supersedes only the V2 0..19 byte bound stated in section 8**; it does not change V1.

- V1 Bind Request/Response: `BindType (1 byte) + SystemID (0..19 bytes) + NUL (1 byte)`. Pinex 1.5.0 compatibility, max-19 validation and golden bytes preserved.
- V2 Bind Request/Response: `BindType (1 byte) + SystemID (0..20 bytes)`. Still no NUL; exactly 20 bytes permitted, 21 bytes rejected.
- Go Client validation depends on its `WithProtocolVersion` Session setting; default V2 permits 20 bytes, explicit V1 remains limited to 19.
- V2 Go Server decodes a 20-byte SystemID and retains it as Session identity; V1 server/codec still reject a 20-byte SystemID.
- No change to the fixed 10-byte Header, Stream/Control PDUs, command/status definitions, or limits on payloads and Session resources.

**Verification gate:** Go test/race/paired example smoke and pinned Pinex compatibility CI must pass on the protected PR, then after merge on latest main; update `.codex/TASKS.md` evidence accordingly. Publication still requires separate owner authorization P9-06 and renewed CHANGELOG acceptance.
