# Project Handoff — Go Simple Net / Pinex Compatibility

**Updated:** 2026-11-16  
**Repository:** majiddarvishan/go-simple-net  
**Branch:** main  
**Implementation status:** NOT STARTED — NOT AUTHORIZED  
**Current work:** protocol analysis and task planning only

## 1. Repository state

- Default branch: main
- Latest documentation commit before this handoff update: dbf731f097222d7ebce25ea95889472c028eba24
- Only documentation under .codex has been added/updated for this planning step.
- No Go application source, tests, go.mod, CI, runtime configuration or dependencies were added or changed.

## 2. Source inputs

### Uploaded Go sample

- Archive: ss7.tgz
- SHA-256: 25b3e0cdfed10c170cf1a4aa02945bfe765bfa1378ef979ed5546883dadd8aa3
- 24 Go files total; 14 non-generated package files and generated Protobuf files.
- Treat it as a behavioral/features reference, not as the authoritative current Pinex wire specification.

### C++ Pinex compatibility target

- Repository: github.com/majiddarvishan/pinex
- Branch: develop
- Version: 1.5.0
- High-level API: messaging_client / messaging_server
- Exact develop HEAD SHA must be pinned immediately before implementation and cross-language testing.
- Main is not the compatibility target for this work.

## 3. New planning documents

- PINEX_GO_PORT_TASKS_2026-11-16.md — task breakdown, dependencies, acceptance criteria, open decisions.
- PINEX_WIRE_PROTOCOL_2026-11-16.md — current Pinex 1.5.0 wire contract and differences from the uploaded Go sample.
- NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md — design-only draft of public API, config, session lifecycle, concurrency, idle keepalive, request tracking and backpressure; Q-006 through Q-010 policy approvals are reflected.
- NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md — detailed API-004 draft: Functional Options vs optional/pointer Config, explicit-zero keepalive, independent overrides, validation, immutable per-instance snapshot. Owner choice still OPEN.
- NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md — Q-008 outgoing 4 MiB/Fail-Fast, Q-009 B incoming overflow close, and Q-010 A configurable inbound/global/session/pre-bind/control defaults are approved. API-004 design remains open.
- DECISION_LOG.md — authoritative approved decisions Q-001 through Q-010 (as applicable), plus NAM-001; distinguishes remaining open API details.
- SERVER_REVIEW_2026-11-16.md — original static review of the uploaded server sample.

## 4. Confirmed protocol facts

- 10-byte header: total length BE u32, command ID u8, command status u8, sequence BE u32.
- Command IDs: Bind 0x01/0x81, Stream 0x02/0x82, Unbind 0x03/0x83, Enquire Link 0x04/0x84.
- Status: rok 0x00, rfail 0xFF.
- Bind body: bind_type u8 followed by null-terminated system_id; bind_type 0=uni_direction, 1=bi_direction.
- Bind Response uses the same body shape and carries server system_id.
- Stream payload is raw remaining frame bytes, with no extra length prefix or mandatory Protobuf envelope.
- Enquire Link response and Unbind response reuse the request sequence number.
- Sequence is per Session, increments, and skips zero.
- Pinex uses a 10-second Unbind protocol deadline.
- Current Pinex pre-bind defaults: 30-second handshake deadline, 128 pending sessions, optional per-IP quota disabled by default.
- Current Pinex Client defaults documented on develop: 30-second connect timeout, 30-second Bind response timeout, 5-second reconnect delay when auto-reconnect is enabled.
- Pinex default behavior responds to Enquire Link and uses OS TCP Keepalive; it does not send periodic Enquire Link by default.

## 5. Important incompatibilities found in uploaded Go sample

1. Go Bind Request/Response bodies omit the Bind Type byte required by Pinex 1.5.0.
2. Go ConnectionReader drops command_status from the Header.
3. Go ClientProtocolHandler.handleBindResponse incorrectly reads the first Body byte as Status; Pinex status is Header offset 5 and Bind Body begins with bind_type.
4. submit_req/submit_resp names differ from Pinex stream_req/stream_resp, but their numeric wire values match (0x02/0x82).
5. The sample uses random Sequence values and automatic deduplication; Pinex uses per-session increasing nonzero sequence values and does not define Sequence-based deduplication as a protocol feature.

Therefore, copying the sample's protocol handling verbatim would not interoperate with current Pinex.

## 6. Task status

See PINEX_GO_PORT_TASKS_2026-11-16.md for the complete backlog.

- GO-PINEX-001: protocol analyzed; owner approved Q-001 Option A on 2026-11-16; golden frames and further contract details are still open.
- GO-PINEX-002 through GO-PINEX-014: open.
- Cross-language tests have not been run.
- No implementation has been started.

## 7. Owner decisions

- **Q-001 — APPROVED (Option A, 2026-11-16):** Pinex-compatible Bind Validator callback based on System ID and actual remote address. No password/token on the wire, no change to Bind PDU layout. Accepted Bind => rok 0x00; rejected => rfail 0xFF. This is not cryptographic identity authentication. See DECISION_LOG.md.
- **Q-005 — APPROVED (Option B, 2026-11-16):** If no Bind Validator callback is supplied, allow an otherwise-valid Bind by default, matching Pinex C++. Duplicate `system_id`, malformed Bind, invalid state and admission limits are still enforced. If a callback is provided, enforce its allow/reject result. This is permissive admission, not authenticated peer identity. See DECISION_LOG.md.
- **Q-002 — APPROVED (2026-11-16):** Configurable, idle-only Enquire Link on Go Client and Server. Send automatically only after no successful frame exchange in either direction for a positive idle interval; `0` disables automatic sending, not replies to peer Enquire Link requests. Never send a second probe while one is outstanding; protocol bytes unchanged. Numeric idle default is **30s** (Q-006); matching-probe response timeout default **10s** and failure closure are approved (Q-007). See DECISION_LOG.md.
- **Q-003 — APPROVED (2026-11-16):** Goroutine-safe Client/Server/Session APIs; non-overlapping, receipt-ordered callbacks per Session; isolation across Sessions; frame-safe concurrent sends; controlled shutdown for callbacks and pending operations. Implementation details (queue capacity, deadline, API signatures) still need design. See DECISION_LOG.md.
- **Q-004 — CLOSED / APPROVED OPTION A (2026-11-16):** Lean V1 Core retains Pinex interoperability, sessions, backpressure, bounded FIFO, reconnect/timeout and idle-only Enquire Link. Defer Circuit Breaker, business Rate Limiter, Priority Queue, Prometheus exporter and SS7/Protobuf integrations; exclude automatic sequence dedup. Transport admission/backpressure remains mandatory. See DECISION_LOG.md.

## 8. Next action

Q-001, Q-002, Q-003, Q-004, Q-005, Q-006, Q-007, Q-008, Q-009, Q-010 and NAM-001 are **CLOSED** as design decisions. Q-010 Option A (2026-11-16) approved **configurable** initial capacities and global-overload rules: inbound 4 MiB per Session, outgoing 4 MiB per Session (Q-008), aggregate queued Application 256 MiB per Server, 1024 bound Sessions, 128 pre-bind, 30s pre-bind timeout, 4 KiB Control reserve per Session. These are defaults, not preallocated memory or process RSS limits.

**Next design-only work:** obtain owner choice for **API-004**: Option A (recommended typed Functional Options with internal complete defaults and immutable validated config) or B (optional/pointer Config fields), then finalize API-001/002 (Go public types, callbacks, queue accounting, close lifecycle) and API-003 (package/module path). Pin C++ Pinex develop HEAD and golden-wire test vectors before implementation. No code, tests, module/repository renaming or operational action authorized.

## 9. Approved Go library name

**NAM-001 CLOSED (2026-11-16):** The owner selected `neta-go` as the name of the native Go TCP Messaging library. NETA was originally proposed as "Network Easy Transport API" but the selected name itself, rather than the expansion, is the approved decision. Current GitHub repository is still `majiddarvishan/go-simple-net`; no repository/module/package renames have been authorized or performed. Pinex C++ stays named Pinex. No code has been written. See DECISION_LOG.md.


## 10. Enquire Link default decision

**Q-006 CLOSED, Option B (2026-11-16):** neta-go Client/Server use an idle-only, configurable Enquire Link interval with a **30-second default**. It is not a fixed 30-second periodic probe: actual incoming/outgoing messages reset idle time. Explicit `0` disables automatic requests but never disables responding to incoming Enquire Link. Probe response policy is approved in Q-007: **10-second configurable default**, matching response Sequence required; timeout closes the Session and Client reconnect is conditional on Auto Reconnect.


## 11. Enquire Link response timeout decision

**Q-007 CLOSED (2026-11-16):** Enquire Link response timeout is separately configurable, **10 seconds by default** on Go Client and Server. It begins after the full Request is written. Only matching `enquire_link_resp` (same Sequence) satisfies it; unrelated traffic cannot satisfy it. On expiry, Session closes once and pending operations terminate; Client retries only if Auto Reconnect was enabled. No Pinex wire changes or implementation were authorized.


## 12. Outgoing resource decision and ingress design

**Q-008 CLOSED / approved Option A (2026-11-16):** configurable 4 MiB outgoing Application Stream FIFO per Session, including serialized Frame headers, immediate Queue Full when full; no partial enqueue, silent drops, or orphan request trackers. Control path capacity must be separate and bounded, without violating single-writer framing or Pinex Unbind ordering/deadline.

**Q-009 CLOSED / APPROVED B (2026-11-16):** if a new incoming Application Frame cannot fit the bounded Callback queue, promptly close **that Session only** with an overload cause; no silent drop or pause/resume fallback. Other Sessions continue unaffected. Go cannot forcibly terminate a blocked user callback; avoid indefinite shutdown waits and finalize terminal callback semantics in API design.

**Q-010 CLOSED / APPROVED A (2026-11-16):** ALL defaults are configurable: incoming 4 MiB/Session, aggregate queued Application 256 MiB/Server (NOT process RSS), 1024 bound Sessions, 128 pre-bind, absolute 30s pre-bind deadline, and 4 KiB Control reserve/Session. Outgoing failed local/global admission => Queue Full, keep Session. Incoming failed local/global admission => close only offender. At bound limit reject new Bind without eviction; at pre-bind capacity refuse socket. Outgoing 4 MiB/Session from Q-008 also remains configurable. See resource design.

**API-004 OPEN:** Go Config must distinguish omitted `EnquireLinkIdleInterval` (approved default 30s) from explicit `0` (disable outgoing probes) and allow independent Q-008/Q-010 resource overrides. Suggested pattern: default config builder/factory with explicit overrides; alternative optional/pointer fields. Hot reload not approved.


## 13. Incoming overload decision

**Q-009 CLOSED — approved Option B (2026-11-16):** A new Application Stream Frame which cannot fit the bounded Session-local incoming Callback backlog triggers a prompt **close of that Session**, not Pause/Resume. No impact on other Sessions, no silent drop, no duplicate terminal event; pending state and queue charges are released. In the presence of blocked user callback code, transport shutdown must not wait indefinitely for arbitrary user logic. Exact callback delivery/close ordering awaits API design. Q-010 configurable default capacities and global admission rules are now APPROVED. No code changes authorized.

## 14. Q-010 final owner decision and configurability handoff

**Q-010 CLOSED — OPTION A WITH ALL SETTINGS CONFIGURABLE (2026-11-16).** Approved defaults and outcomes are defined in [Decision Log](DECISION_LOG.md), [Resource Design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md) and [API/Config Design](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md). An omitted setting selects the approved default; an explicit valid override is honored. Exact Go representation/validation remains API-004 OPEN, particularly `EnquireLinkIdleInterval=0` vs omission.

**No implementation is authorized.** The detailed API-004 configuration model is documented, but its Option A/B choice remains OPEN.
