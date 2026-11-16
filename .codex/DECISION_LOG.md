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

### Follow-up (not approved yet)

**Q-005 — Default Validator Policy:** Decide whether Go Server **requires** a configured validator and rejects Bind if absent (fail-closed), or allows Bind when no validator is configured (Pinex-like permissive default). Do not silently choose a default in the implementation.

### References

- [Port tasks](PINEX_GO_PORT_TASKS_2026-11-16.md)
- [Wire protocol contract](PINEX_WIRE_PROTOCOL_2026-11-16.md)
- [Project handoff](HANDOFF.md)
