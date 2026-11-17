# neta-go V1 — Architecture Freeze Report and Implementation Roadmap

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Status:** ARCHITECTURE FROZEN — OWNER DECISIONS COMPLETE  
**Implementation authorization:** PHASE 1 GRANTED / PHASES 2+ NOT GRANTED  
**Compatibility baseline:** Pinex C++ 1.5.0, audited develop HEAD f7bd1eb779baff0d10e624f20522df345e6ee747

## 1. Executive result

All owner decisions required for the V1 architecture freeze are closed. The owner approved Option A for API-002-04 through API-002-08, API-001 final public surface, API-003 module/package architecture and PROTOCOL-001 Maximum Frame Size adoption on 2026-11-17.

The architecture is frozen at the design/contract level. The owner subsequently granted explicit authorization for Phase 1 only (module skeleton and immutable configuration). Phase 2+ remain unauthorized until a separate approval.

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

**Phase 2 — Wire protocol core**
- Implement pure-Go internal/wire: 10-byte Header, command/status validation, Bind/Stream/Unbind/Enquire Link codecs, Sequence handling and 1 MiB total-frame ceiling.
- Reject oversized outbound frames before sequence/pending/queue reservation.
- Validate inbound Header before Body allocation; enforce the same limit in Pre-Bind and Bound phases.
- Add deterministic golden vectors from the pinned C++ implementation.

**Phase 3 — Session transport and lifecycle engine**
- Implement one Reader and one serialized Writer per Session; no user callback inline in either loop.
- Implement Bind/Pre-Bind admission, absolute Connect/Bind/Frame Assembly/Write deadlines and Enquire Link state.
- Implement Request correlation, once-only settlement, RequestTimeout origin and late-response handling.
- Implement Graceful Unbind, Forced Close A, Done/Err publication and exact cleanup of timers, registries, queues and reservations.

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

Architecture Freeze is complete as a documentation/decision milestone. On 2026-11-17 the owner explicitly authorized Phase 1 only. Phase 2 (Wire Protocol) and all later phases remain gated by separate owner authorization. Phase 1 code and additional boundary tests are under final CI verification.

## Implementation Authorization — Phase 1 only (2026-11-17)

The owner explicitly authorized Phase 1 implementation in this conversation («آره، اجازه داری»). This supersedes earlier blanket NOT AUTHORIZED statements only for the scope below.

**Scope:** approved module skeleton/root package, typed Functional Options, immutable effective configuration, approved defaults, apply-all-then-validate behavior, API-004/WORKER-009C validation and checked arithmetic, API-001 configuration errors/state declarations, focused tests and CI.

**Baseline at authorization:** main HEAD f3f525e5f36f65235a4c8e023cfafb4bfd6b5737. The authorization is also recorded in HANDOFF.md and IMPLEMENTATION_AUTHORIZATION_2026-11-17.md.

**Boundary:** Phase 2 Wire Protocol and all later runtime/transport/worker/reconnect/interoperability/benchmark/release/deployment work remain NOT AUTHORIZED and require separate owner approval. No frozen architecture decision is reopened.


## 11. Phase 1 completion record — 2026-11-17

**Result:** Phase 1 is COMPLETE / CI PASS.

- Verified final Phase 1 commit: cb3bdbc2964c1a0444cd40684117e1987f4cf092.
- CI run: https://github.com/majiddarvishan/neta-go/actions/runs/44040206052 — PASS.
- Passed: gofmt, go vet ./..., go test ./..., go test -race ./....
- Configuration hardening includes TCP endpoint syntax, all Server-side WORKER-009C boundaries, final nested Session Option precedence, nil-option rejection, and checked arithmetic.
- Authorization is limited to Phase 1. Phase 2 Wire Protocol is in progress under explicit owner authorization; pure-Go codec and unit tests are implemented, with cross-language evidence outstanding. No wire codec or transport implementation has been started.


## Phase 2 — Wire Protocol implementation authorization (2026-11-17)

Owner approval was explicitly granted in this conversation by replying «ادامه بده» to the question asking permission to start Phase 2. This approval supersedes the earlier Phase-2-not-authorized wording, only for the scope below.

**Scope:** pure-Go `internal/wire`; Pinex 1.5.0 frame Header and complete Frame codec; Bind Request/Response; raw Stream Request/Response; empty Unbind and Enquire Link Request/Response; per-Session nonzero Sequence generator; fixed 1 MiB total frame bound; focused golden/boundary/concurrency tests.

**Compatibility source:** Pinex develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Wire fields are BE u32 length, u8 command, u8 status, BE u32 sequence; Bind body is u8 bind_type followed by NUL-terminated system_id (maximum 19 bytes); Stream body is raw bytes.

**Out of scope / not authorized:** TCP transport, read/write loops, Client/Server runtime integration, Session lifecycle, worker scheduling, resource admission runtime, reconnect, socket-level cross-language test execution, benchmarks, release and deployment. Those require later phase authorization.

Implementation must stay inside `internal/wire`, use only the standard library, and must not import root package `neta`. Root `neta` may depend on `internal/wire`.


## Phase 2 implementation progress — 2026-11-17

**Wire codec implementation:** initial pure-Go implementation committed and CI verified.

- Source commit containing current wire implementation and focused tests: `6e7b9fbb1cddc79d8199ae390a48f7de5d7f282c`.
- Current verified branch HEAD at the time of this record: `6e7b9fbb1cddc79d8199ae390a48f7de5d7f282c`.
- GitHub Actions run `44042140607`: PASS — gofmt, go vet, go test, go test -race.
- Implementation: 10-byte Header codec, complete-frame encode/decode, 1 MiB hard cap, Bind Request/Response, raw Stream Request/Response, empty Unbind/Enquire Link PDU encoders, nonzero wrapping Sequence generator.
- Tests: header golden vector, Bind golden vectors, raw Stream round-trip, control PDU shape/sequence, max/oversize frame boundaries, malformed/truncated/extra bytes, invalid Bind fields, Sequence wraparound and concurrent uniqueness.

**Remaining before calling Phase 2 fully complete:** deeper source-level compatibility review and a captured C++ Pinex 1.5.0 golden fixture / socket-level Go↔C++ interoperability proof. No C++ process was built or run in this phase. No TCP transport or runtime integration was started. Phase 3 and later remain unauthorized.
