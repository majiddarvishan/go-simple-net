# neta-go V1 — Architecture Freeze Report and Implementation Roadmap

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Status:** ARCHITECTURE FROZEN — OWNER DECISIONS COMPLETE  
**Implementation authorization:** PHASE 1 + PHASE 2 + PHASE 3 COMPLETE / PHASE 4+ NOT AUTHORIZED  
**Compatibility baseline:** Pinex C++ 1.5.0, audited develop HEAD f7bd1eb779baff0d10e624f20522df345e6ee747

## 1. Executive result

All owner decisions required for the V1 architecture freeze are closed. The owner approved Option A for API-002-04 through API-002-08, API-001 final public surface, API-003 module/package architecture and PROTOCOL-001 Maximum Frame Size adoption on 2026-11-17.

The architecture is frozen at the design/contract level. Phases 1, 2 and 3 are COMPLETE / CI PASS. Phase 4+ remain gated by separate owner authorization.

## 2. Architecture decision closure

| Area | Final state | Binding result |
|---|---|---|
| Core protocol and Q-001..Q-010 | CLOSED | Pinex-compatible 10-byte Header and existing command/status/sequence semantics; lean V1 core; approved queue, admission, idle-probe and session policies |
| API-001a | CLOSED / A | Nonblocking callback-first SendRequest; local admission result immediately, exactly-once later Completion |
| PERF-002 | CLOSED / A | Snapshot outgoing payload before successful return; incoming owned retainable payload |
| PERF-003 | CLOSED / A | Per-Session and owner Request credits; exactly-once settlement and bounded Completion lifecycle |
| API-001b | CLOSED | Forced Close A; N-ORD2; DONE-A; C-ORD-B; three independent bounded callback lanes and exact callback/resource lifetimes |
| WORKER-001..009C | CLOSED | Owner-scoped isolated Application/Completion/OnClose workers, fairness, idle retirement, approved defaults, maxima, ranges and independent ledgers |
| API-004 | CLOSED / A | Typed independent Functional Options; apply all then validate; immutable effective config before network/Goroutine creation |
| API-002-01..08 | CLOSED / A | Request, Frame Assembly, Connect, Bind, Unbind and Writer deadlines; bounded reconnect; terminal Graceful-then-Forced Stop |
| API-001 | CLOSED / A | Minimal root-package Client/Server/Session API, callback-first completion, typed options/errors, immutable terminal cause and Done contract |
| API-003 | CLOSED / A | Module github.com/majiddarvishan/neta-go, root package neta, one public package plus internal/wire |
| PROTOCOL-001 | CLOSED / A | Fixed 1,048,576-byte total Frame ceiling in both directions, including 10-byte Header |

## 3. Binding lifecycle and timeout matrix

| Contract | Default | Start / scope |
|---|---:|---|
| Server Pre-Bind handshake | 30s | Absolute from TCP accept |
| Client Connect | 30s | Absolute per Dial; includes DNS and TCP |
| Client Bind response | 30s | Absolute from TCP establishment before Bind Write; covers Bind Write and matching response |
| Request | 30s | Starts immediately after successful local admission; includes queue, Writer and response |
| Bound Frame assembly | 30s | Absolute from first byte of each Frame Header through full Body |
| Enquire Link idle | 30s | Only after real Session inactivity; explicit zero disables automatic outgoing probes |
| Enquire Link response | 10s | Starts after full probe Write; matching sequence required |
| Graceful Unbind | 10s | Absolute from Unbinding transition; includes FIFO drain, Unbind Write and response |
| Generic Writer stall | 30s | Absolute per Frame from Writer ownership; no progress reset |

Timeouts are independent. Positive durations reject explicit zero except the already-approved EnquireLinkIdleInterval=0 disable behavior. No additional hard maximum was approved for API-002 durations.

## 4. Reconnect and shutdown invariants

- AutoReconnect is opt-in and defaults false.
- Retry only transient Dial failure/timeout and unexpected transport loss after Bound. Bind rejection and Bind timeout are non-retryable.
- One reconnect supervisor per Client; exponential Full Jitter, 1s base window, 30s cap; reset after 30s continuously Bound.
- No Request replay or resend across Session generations. Old-generation Requests settle once.
- Client/Server Stop is terminal and idempotent. Client disables reconnect and cancels Dial/Bind; Server closes Listener and Pre-Bind Sessions first, then begins concurrent Graceful Unbind for all Bound Sessions.
- Caller Context expiry escalates remaining Sessions to Forced Close, waits for internal Done and returns the Context error.
- Done means internal teardown and immutable terminal cause publication, not user callback completion. Started callbacks retain worker and payload/request credits until actual return; queued callbacks are cancelled under Forced Close A. OnClose is best-effort after Done and may run after Stop returns.

## 5. Public API and package freeze

Public surface:
- NewClient(opts ...ClientOption) returns (*Client, error).
- NewServer(opts ...ServerOption) returns (*Server, error).
- Client.Connect(ctx), Client.CurrentSession(), Client.Stop(ctx).
- Server.Start(), Server.Addr(), Server.Stop(ctx).
- Session.SendRequest, Session.SendNoWait, Session.SendResponse, Session.Unbind(ctx), Session.Close(), Session.Done(), Session.Err(), Session.State(), Session.SystemID(), Session.RemoteAddr().

Only root package neta is public. Protocol, scheduler, worker, tracker, ledger and transport internals remain private. Typed Functional Options are independent and immutable after construction/start; no live setters, generic map or hot reload.

## 6. Protocol compatibility freeze

- Pinex C++ 1.5.0 source accepts **1,048,576 bytes total Frame**, Header included.
- Header is 10 bytes; maximum Body is **1,048,566 bytes**.
- Go rejects oversized inbound Frames after Header validation and before Body allocation; close the offending Session/Pre-Bind transport.
- Go rejects oversized outbound Frames before Sequence/Pending/Queue admission with ErrFrameTooLarge.
- This is a fixed Wire contract, not a configurable resource quota. Queue and retained-byte limits remain independent.
- No Wire layout or command/status/sequence semantics change.

Golden-frame vectors and actual Go↔C++ socket-level verification remain implementation validation tasks. Re-pin Pinex develop HEAD immediately before implementation/interoperability testing.

## 7. Items not blocking Architecture Freeze

These are evidence/implementation tasks, not open owner architecture decisions:
1. Create golden-frame fixtures and compare all command/status/sequence/length encodings.
2. Verify real Go Client ↔ Pinex C++ Server and Pinex C++ Client ↔ Go Server over TCP.
3. Add unit, fuzz, race, lifecycle, admission, fault-injection and soak tests.
4. Establish a reproducible performance baseline before setting numeric throughput/latency/allocation pass/fail targets.
5. Determine pooling/allocation policy (PERF-004) from measurements; do not promise zero-copy or introduce pooling without evidence.
6. Validate docs/examples and release/versioning after implementation is complete.

No numeric Performance SLA is invented at Architecture Freeze. Throughput, p50/p95/p99 latency, allocations/op, bytes/op, heap/GC, goroutine count, CPU and contention are first-class measurements.

## 8. Implementation roadmap — gated, production-grade

**Gate 0 — Owner authorization and baseline pin (PASS for Phase 1 only)**
- Owner explicitly authorized Phase 1 on 2026-11-17.
- Verified starting main HEAD: f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- Re-check Pinex develop HEAD and version; re-audit any change from f7bd1eb779baff0d10e624f20522df345e6ee747.
- Record Go toolchain and supported platform baseline; do not add dependencies by default.

**Phase 1 — Module skeleton and immutable configuration (COMPLETE / CI PASS, 2026-11-17)**
- [x] Create go.mod with the approved module path and root package neta.
- [x] Define ClientOption, ServerOption, SessionOption, complete defaults and immutable effective config snapshots.
- Implement validation after all options and before any Socket/Goroutine creation; enforce all WORKER-009C ranges, cross-field rules and overflow checks.
- [x] Add public typed error categories and state enums. Keep API surface exactly within API-001.

**Phase 2 — Wire protocol core (COMPLETE / GO CI + PINEX C++ GOLDEN CI PASS, 2026-11-17)**
- [x] Implement pure-Go internal/wire: 10-byte Header, command/status fields, Bind/Stream/Unbind/Enquire Link codecs, Sequence handling and 1 MiB total-frame ceiling.
- [x] Reject oversized outbound frames and validate inbound Header lengths before accepting a Frame.
- [x] Add deterministic golden vectors from the pinned Pinex C++ 1.5.0 production PDU serializer source snapshot; compile the C++ fixture generator and compare Go output in CI.
- [x] Match Pinex's omitted Bind Response Body default-value behavior.
- [x] Pass gofmt, go vet, go test, go test -race and the C++ golden compatibility workflow.

**Phase 3 — Session transport and lifecycle engine (OWNER AUTHORIZED 2026-11-17 / COMPLETE / CI PASS)**

- [x] One Reader and one serialized Writer per Session; no user callback inline in either loop.
- [x] Session-side Bind/Pre-Bind handshake state transitions and absolute Bind/Pre-Bind deadlines. Client Dial and Server Listen/accept orchestration remain Phase 6.
- [x] Absolute Frame Assembly and per-Frame Writer deadlines; idle-only Enquire Link and matching response timeout.
- [x] Request correlation, nonzero Sequence allocation, once-only internal settlement, RequestTimeout origin and late-response suppression.
- [x] Graceful Unbind, Forced Close A, immutable terminal cause, Done publication and exact cleanup of timers, transport loops, pending state and queues.
- [x] Focused transport/lifecycle, partial-I/O, timeout-race and race-detector tests.

**Phase 3 boundary:** use the approved per-Session bounded FIFO as a minimal transport prerequisite only. Full independent resource ledgers remain Phase 4; isolated callback worker pools remain Phase 5; Client/Server orchestration, reconnect and generation fencing remain Phase 6. No public API expansion, dependency, benchmark, release or deployment.

**Phase 4 — Admission and resource accounting**
- Implement independent ledgers for queue bytes/counts, retained Application/Completion bytes, Request credits, OnClose event credits and worker permits.
- Add per-Session and owner limits exactly as approved; owner limits remain per Client/Server, not process-wide.
- Prove no double-counting across queue-to-Writer and queue-to-active callback transitions; use checked arithmetic and overflow/underflow guards.
- Enforce incoming offender-close and outgoing fail-fast semantics.

**Phase 5 — Worker scheduler**
- Implement three isolated lazy worker classes: Application, Completion and OnClose.
- Enforce owner-wide ceilings, per-Session concurrency, fair per-Session scheduling, FIFO start order and finite idle retirement with zero warm-idle floor.
- Ensure callbacks already running after Done/reconnect keep their permits/credits until return; never spawn a goroutine per event.
- Test blocked callbacks without blocking internal Reader/Writer, Done or Stop.

**Phase 6 — Client and Server orchestration**
- Implement Client Dial/Bind/optional reconnect supervisor and Session-generation fencing.
- Implement Server Listen/accept, bounded Pre-Bind/Bound admission, Bind Validator and duplicate System ID registry.
- Implement terminal/idempotent Client.Stop and Server.Stop with concurrent Graceful Unbind and Context-driven Forced escalation.
- Verify no replay/resend across reconnect and no stale generation mutating the current Session.

**Phase 7 — Verification and fault injection**
- Unit tests for configuration/defaults/ranges, every codec and state transition.
- Fuzz parsers and Frame length/command/status handling; test malformed and oversized input without body-sized allocation.
- Race tests for concurrent Sends, callbacks, Close/Stop, reconnect and admission counters.
- Integration tests against Pinex C++ 1.5.0 in both connection directions.
- Fault injection: partial writes, blackholes, slow-trickle reads, EOF, reset, Bind rejection, timeout races, queue/credit exhaustion, callback stalls and reconnect churn.
- Soak tests for goroutine/heap stability, bounded ledgers, fairness and retired-generation callbacks.

**Phase 8 — Performance baseline and evidence-driven optimization**
- Benchmark varied payload sizes, one/many Sessions, concurrent producers, callback concurrency, queue saturation and reconnect churn.
- Record throughput, p50/p95/p99 latency, allocs/op, bytes/op, CPU, heap/GC, goroutines and contention.
- Establish reproducible baseline first; agree numeric pass/fail thresholds from workload and production requirements afterward.
- Evaluate PERF-004 pooling/batching only where benchmarks demonstrate benefit and ownership remains safe. Re-run correctness/race/interoperability after every optimization.

**Phase 9 — Documentation, examples and release readiness**
- Provide minimal Client and Server examples, graceful Stop, reconnect opt-in and callback ownership documentation.
- Review API stability, protocol compatibility matrix, resource defaults, operational guidance and release/versioning.
- Require all verification gates and owner release approval before tagging V1.

## 9. Exit criteria for V1 implementation readiness

- All API-001/API-002/API-003/API-004 contracts implemented exactly.
- No unbounded goroutine-per-message/request/callback/timer.
- No data race, duplicate terminal settlement, Sequence reuse collision, ledger underflow/overflow or unbounded allocation from untrusted Frame length.
- All golden vectors and bidirectional Pinex interoperability tests pass.
- Worker/queue/retained-byte ceilings hold under saturation and callback stalls.
- Graceful and forced shutdown complete under documented deadlines without waiting for arbitrary user callback return.
- Performance baseline is reproducible and no regression is accepted without an explicit tradeoff.
- Documentation, examples and versioning are complete.

## 10. Final authorization boundary

Architecture Freeze is complete as a documentation/decision milestone. Phases 1, 2 and 3 are COMPLETE / CI PASS. Phase 4+ remain gated by separate owner authorization.

## Implementation Authorization — Phase 1 only (2026-11-17)

The owner explicitly authorized Phase 1 implementation in this conversation («آره، اجازه داری»). This supersedes earlier blanket NOT AUTHORIZED statements only for the scope below.

**Scope:** approved module skeleton/root package, typed Functional Options, immutable effective configuration, approved defaults, apply-all-then-validate behavior, API-004/WORKER-009C validation and checked arithmetic, API-001 configuration errors/state declarations, focused tests and CI.

**Baseline at authorization:** main HEAD f3f525e5f36f65235a4c8e023cfafb4bfd6b5737. The authorization is also recorded in HANDOFF.md and IMPLEMENTATION_AUTHORIZATION_2026-11-17.md.

**Boundary at Phase 1 authorization:** Phase 2 and later work were not yet authorized at that point. Phase 2 was subsequently authorized separately and is now complete. Phase 3 transport/lifecycle and later runtime/worker/reconnect/live-interoperability/benchmark/release/deployment work remain NOT AUTHORIZED and require separate owner approval. No frozen architecture decision is reopened.


## 11. Phase 1 completion record — 2026-11-17

**Result:** Phase 1 is COMPLETE / CI PASS.

- Verified final Phase 1 commit: cb3bdbc2964c1a0444cd40684117e1987f4cf092.
- CI run: https://github.com/majiddarvishan/neta-go/actions/runs/44040206052 — PASS.
- Passed: gofmt, go vet ./..., go test ./..., go test -race ./....
- Configuration hardening includes TCP endpoint syntax, all Server-side WORKER-009C boundaries, final nested Session Option precedence, nil-option rejection, and checked arithmetic.
- Phase 1 authorization was limited to module/configuration and completed. Phase 2 was subsequently authorized and is now COMPLETE: pure-Go codec, compiled C++ production-serializer golden fixtures, Go compatibility tests and both CI workflows PASS. No TCP transport/runtime implementation has started.


## Phase 2 — Wire Protocol implementation authorization (2026-11-17)

Owner approval was explicitly granted in this conversation by replying «ادامه بده» to the question asking permission to start Phase 2. This approval supersedes the earlier Phase-2-not-authorized wording, only for the scope below.

**Scope:** pure-Go `internal/wire`; Pinex 1.5.0 frame Header and complete Frame codec; Bind Request/Response; raw Stream Request/Response; empty Unbind and Enquire Link Request/Response; per-Session nonzero Sequence generator; fixed 1 MiB total frame bound; focused golden/boundary/concurrency tests.

**Compatibility source:** Pinex develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Wire fields are BE u32 length, u8 command, u8 status, BE u32 sequence; Bind body is u8 bind_type followed by NUL-terminated system_id (maximum 19 bytes); Stream body is raw bytes.

**Out of scope / not authorized:** TCP transport, read/write loops, Client/Server runtime integration, Session lifecycle, worker scheduling, resource admission runtime, reconnect, socket-level cross-language test execution, benchmarks, release and deployment. Those require later phase authorization.

Implementation must stay inside `internal/wire`, use only the standard library, and must not import root package `neta`. Root `neta` may depend on `internal/wire`.



## Phase 2 implementation evidence — 2026-11-17

**Phase 2 Wire Codec: COMPLETE** within the explicitly owner-authorized scope.

- Verified implementation commit and code/test baseline: `b026dadd9ad0b33118b05f49477e1412a4255978`.
- Go CI run `44044273742`: PASS — gofmt, go vet, go test, go test -race.
- C++ compatibility CI run `44044273818`: PASS — compile pinned C++ PDU serializer snapshot, regenerate/diff golden frames, run Go golden tests.
- Pinned Pinex source: version 1.5.0, commit `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Corrected Pinex's omitted/empty Bind Response Body semantics; Go now returns default `bi_direction` and empty system ID.
- Source snapshot provenance and original Git blob hashes: `testdata/pinex-1.5.0/source/SNAPSHOT.md`.

The C++ fixture generator uses actual production PDU serializer templates; its Header writer mirrors the audited private `session::serialize_header` implementation. The private Pinex repo cannot be checked out by neta-go Actions without a cross-repository credential, so the exact pinned source headers are snapshotted in this repo. This is compiled C++ golden compatibility, not live TCP/socket interoperability.

**Not authorized:** Phase 3 TCP transport/read/write loops, runtime integration, Session lifecycle, worker scheduler, resource admission runtime, reconnect, live socket interoperability, benchmarks, release and deployment. Obtain separate owner authorization before starting Phase 3.

## 12. Phase 2 Wire Codec completion — 2026-11-17

**Status:** COMPLETE within the explicit Phase 2 authorization.

- Current verified main HEAD: `b026dadd9ad0b33118b05f49477e1412a4255978`.
- Go CI run `44044273742`: PASS (gofmt, vet, unit tests, race tests).
- C++ golden compatibility run `44044273818`: PASS (compiled pinned Pinex production serializer snapshot, regenerated fixture frames, diffed fixture file and ran Go comparisons).
- Pinex 1.5.0 source baseline: `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Empty Bind Response Body behavior now matches Pinex's `can_be_omitted` default values.
- Source snapshot provenance is recorded under `testdata/pinex-1.5.0/source/SNAPSHOT.md`.

The fixture proves Go encoding/decoding against compiled C++ PDU serializer output and the source-audited 10-byte Header layout. It does not claim live TCP socket interoperability.

**Boundary:** Phase 3 TCP transport, runtime integration, Session lifecycle, workers, resource admission, reconnect, live socket interop, benchmarks, release and deployment remain NOT AUTHORIZED.


## 12. Phase 3 implementation authorization — 2026-11-17

Owner approval received in chat pg_07: «اره هست و مجوز داری».

The authorization is limited to the Phase 3 Session transport/lifecycle engine described in §8. It does not authorize Phase 4 independent resource ledgers, Phase 5 worker scheduler, Phase 6 Client/Server orchestration, Phase 7 live interoperability/fault-injection campaign, Phase 8 benchmarks, Phase 9 release/deployment, or any change to frozen architecture/API/protocol decisions.


## Phase 3 implementation evidence — 2026-11-17

- Phase 3 source verification HEAD: `cc0d25e9252b48f38dcddeeb53eaa6e7fe1cbf4c`.
- `session_test.go`: 33 focused Session/transport/lifecycle tests.
- Go CI run `44057820360`: PASS — gofmt, go vet, go test and go test -race.
- Pinex Wire Compatibility run `44057820405`: PASS — compiled pinned C++ 1.5.0 production-serializer fixture, golden comparison and Go wire compatibility tests.
- Live Go↔Pinex TCP interoperability remains Phase 7; Phase 3 does not claim socket-level cross-language proof.
- Phase 4 resource ledgers, Phase 5 worker scheduler and Phase 6 Client/Server orchestration remain gated and were not implemented.
