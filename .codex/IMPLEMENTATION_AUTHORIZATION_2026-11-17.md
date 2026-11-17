# neta-go — Implementation Authorization Record

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Architecture:** Frozen; all fundamental owner decisions closed.  
**Current implementation status:** Phases 1–5 COMPLETE / CI PASS; Phases 6–9 outside current authorization.

## Owner authorization

The owner explicitly authorized Phase 1 in this conversation: «آره، اجازه داری».

## Authorized scope — Phase 1 only

- Create/maintain the approved module path github.com/majiddarvishan/neta-go and root package neta.
- Implement typed ClientOption, ServerOption, and SessionOption.
- Implement immutable effective configuration and approved defaults.
- Apply every Functional Option before final validation.
- Validate all API-004 and WORKER-009C ranges, zero-value exception, independent owner/Session budgets, and cross-field invariants.
- Protect arithmetic helpers against overflow/underflow.
- Provide the public config errors/state declarations required by frozen API-001.
- Add and run focused Phase 1 tests and CI.

## Baseline and current evidence

- Main HEAD at authorization: f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- Existing Phase 1 source/module commit: 8c1132392e9c6e919f72ad16ce8f285d989188d9.
- Subsequent config hardening and CI-green source baseline: f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- Additional Server-side WORKER-009C boundary tests: 18bec96bb13cbdfa5a7819301b23d61e3fb3db75.
- CI for the baseline before the additional tests: PASS (gofmt, go vet, go test, go test -race).
- CI for the additional tests: pending at record creation.

## Authorization boundary

This authorization does NOT authorize Phase 2 or later: Wire Protocol, transport/runtime, lifecycle engine, worker scheduler, reconnect, Pinex interoperability, benchmarks, release, or deployment. Those phases require a separate explicit owner approval.

Earlier blanket statements that no implementation/tests are authorized are superseded only for the Phase 1 scope above. No architecture decisions are reopened.


## Phase 2 Authorization — Wire Protocol (2026-11-17)

The owner explicitly approved beginning Phase 2 by replying «ادامه بده» after being asked whether Phase 2 (Wire Protocol) was authorized. This approval is scoped to protocol implementation only.

**Authorized scope:** pure-Go `internal/wire` package; Pinex 1.5.0 10-byte frame header and Big Endian fields; frame encode/decode and size validation; Bind Request/Response with bind_type and C-octet system_id; Stream Request/Response raw body; Unbind and Enquire Link control PDUs; sequence fields/semantics; unit and golden-vector tests; documentation and CI.

**Compatibility reference:** Pinex C++ develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Re-pin/re-audit if the develop branch moves before interoperability testing.

**Hard protocol maximum:** 1,048,576 bytes total including the 10-byte header; maximum Body 1,048,566 bytes. The limit is fixed, not configurable, and applies inbound and outbound.

**Not authorized:** TCP transport/read-write loops, Client/Server runtime integration, lifecycle/shutdown/reconnect, worker scheduler, resource-admission runtime, cross-language socket test harness requiring C++ execution, benchmarks, release or deployment. Those belong to later phases and require separate owner approval.

No frozen architecture decision is reopened. Phase 1 remains complete with CI PASS. Phase 2 implementation must not import root package `neta` from `internal/wire`; the package uses only the Go standard library.


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

## Phase 2 implementation completion evidence — 2026-11-17

The owner explicitly authorized Phase 2 Wire Protocol in this conversation by replying «ادامه بده». That authorization remains limited to the pure-Go `internal/wire` codec, source-derived C++ golden fixture validation, tests, docs and CI.

**Phase 2 status:** COMPLETE.

- Current verified main HEAD: `b026dadd9ad0b33118b05f49477e1412a4255978`.
- Go CI run `44044273742`: PASS — gofmt, go vet, go test, go test -race.
- C++ compatibility run `44044273818`: PASS — compiled fixture generator against pinned Pinex production PDU serializer source snapshot, regenerated and diffed golden frames, and ran Go comparisons.
- Pinex baseline: version 1.5.0, develop commit `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Empty Bind Response compatibility was fixed and covered.
- Source provenance and copied header Git blob hashes are recorded in `testdata/pinex-1.5.0/source/SNAPSHOT.md`.

The C++ fixture generator uses actual production PDU serializer templates and a header writer mirroring the audited private `session::serialize_header` function. It is not a live socket-level Go↔C++ test.

**Still not authorized:** TCP transport/read-write loops, runtime integration, Session lifecycle, worker scheduler, resource admission runtime, reconnect, live socket interoperability harness, benchmarks, release and deployment. Obtain separate owner authorization before Phase 3.


## Phase 3 Authorization — Session Transport & Lifecycle Engine (2026-11-17)

The owner explicitly authorized Phase 3 in the continuation of chat pg_07 with «اره هست و مجوز داری», approving the previously asked question about beginning Phase 3.

**Authorized scope**
- Implement one Reader and one serialized Writer per Session. User callbacks must never execute inline in either I/O loop.
- Implement private Session Bind/Pre-Bind handshake state transitions and primitives, with the approved absolute deadlines.
- Implement absolute Frame Assembly Timeout, per-Frame Writer Stall Timeout, idle-only Enquire Link and matching response timeout.
- Implement Request Sequence allocation/correlation, RequestTimeout starting immediately after successful local admission, exactly-once internal settlement and late-response suppression.
- Implement Graceful Unbind, Forced Close A, immutable terminal cause, Done publication and exact internal teardown.

**Strict boundary**
- The minimal per-Session bounded FIFO required to operate one Writer may use the already-approved configuration limits. Full independent queue/count/retained-payload/owner ledgers remain Phase 4.
- Three isolated bounded Application/Completion/OnClose worker pools remain Phase 5. Phase 3 must use a private callback-dispatch interface and must not introduce goroutine-per-callback behavior.
- Client Dial, Server Listen/accept orchestration, reconnect supervisor, Session-generation fencing, owner registries and full BindValidator/duplicate-SystemID orchestration remain Phase 6. Phase 3 implements the Session-side handshake primitive only.
- No public API expansion, wire-format change, third-party dependency, benchmark, release or deployment.

**Baseline:** main HEAD ebf269dc9ba1a8f5d88922cf1e6ab29c99782a05 at authorization. Phase 1 and Phase 2 remain COMPLETE. Pinex source baseline remains f7bd1eb779baff0d10e624f20522df345e6ee747.

**Verification:** focused transport/lifecycle and race tests, plus gofmt, vet, unit and race CI. Live Pinex TCP interoperability remains a later verification phase.


## Phase 3 implementation completion evidence — 2026-11-17

The owner-authorized Phase 3 Session Transport & Lifecycle Engine is COMPLETE at `cc0d25e9252b48f38dcddeeb53eaa6e7fe1cbf4c`.

- 33 focused Session/transport/lifecycle tests in `session_test.go`.
- Go Actions run `44057820360`: PASS — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility run `44057820405`: PASS — pinned C++ 1.5.0 serializer fixture build, golden comparison and Go wire tests.
- Scope remained limited to private Session transport/lifecycle implementation. Full Resource Admission (Phase 4), Worker Scheduler (Phase 5), Client/Server orchestration (Phase 6), live TCP interoperability (Phase 7), performance (Phase 8) and release (Phase 9) remain unauthorized.


## Phase 4 Authorization — Resource Admission & Accounting (2026-11-17)

The owner explicitly instructed that a repository-visible task tracker be added first and implementation then continue. This authorizes Phase 4 only.

**Authorized scope**
- Implement the approved local and owner-scoped byte/count ledgers for outgoing/incoming Application frames, retained Application/Completion payloads, Request credits, OnClose event credits and Server aggregate queued Application frames.
- Reserve declared inbound Frame capacity before Body allocation/read.
- Preserve the approved Q-008 fail-fast outgoing Queue Full and Q-009/Q-010 offender-only incoming close behavior.
- Implement WORKER-008 Option A Completion retained-byte exhaustion: fixed-size scratch drain under the original absolute Frame Assembly deadline, then a normal Completion with no payload and ErrCapacity if that Request still wins.
- Implement exact once-only release/ownership transfer for writes, partial reads, callback start/return/cancellation, timeout and Forced Close A.
- Add focused boundary, multi-Session, owner isolation, lifecycle and race tests; pass gofmt, vet, unit and race CI.

**Strict exclusions**
- Phase 5 physical worker pools and fair scheduler.
- Phase 6 Client/Server Dial, Listen, accept, Bind registry and reconnect orchestration.
- Phase 7 live Go↔Pinex TCP interoperability and broad fault-injection campaign.
- Phase 8 benchmarks/pooling and Phase 9 release/deployment.

No public API, wire behavior, default, resource limit, owner decision or frozen architecture contract is reopened. The canonical task-by-task checklist is `.codex/TASKS.md`.


## Phase 4 implementation completion evidence — 2026-11-17

The Phase 4 scope authorized above is COMPLETE at source HEAD `2935402366603267ba60c181a057d26cf0e7e9a6`.

- 46 focused Session/transport/resource-accounting tests and 8 owner-ledger tests.
- Go Actions [44062568415](https://github.com/majiddarvishan/neta-go/actions/runs/44062568415): PASS — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility [44062568425](https://github.com/majiddarvishan/neta-go/actions/runs/44062568425): PASS.
- Implementation report: `.codex/NETA_GO_PHASE_4_RESOURCE_ADMISSION_IMPLEMENTATION_2026-11-17.md`.
- Phase 5 Worker Scheduler, Phase 6 orchestration/reconnect, Phase 7 live TCP interoperability, Phase 8 performance and Phase 9 release remain NOT AUTHORIZED.


## Phase 5 Authorization — Bounded callback Worker Scheduler (2026-11-17)

The owner explicitly authorized Phase 5 by replying «آره» after being asked whether Phase 5 Worker Scheduler could begin.

**Authorized scope:** implement the already owner-approved WORKER-001..009C design:
- Three isolated lazy owner-scoped worker pools: Application, Completion and OnClose.
- Owner-wide class ceilings and per-Session concurrency intersections.
- FIFO round-robin ready-Session scheduling, one callback per eligible Session turn, reusable ready nodes.
- Per-class finite idle grace (30s defaults), zero warm-idle floor, worker reuse and race-safe retirement.
- Forced Close A cancellation/start linearization; queued Application/Completion cancellation and exact resource release; started callbacks retain active permits and Request/payload credits through return.
- OnClose eligible only after Done, best-effort/at-most-once, bounded queued+running event credits, permanent skip on saturation.
- Tests for class/owner isolation, fairness, concurrency, callback stalls, Done, Forced Close, worker churn/retirement and race safety; gofmt, vet, unit and race CI.

**Explicit exclusions:** Phase 6 Client/Server Dial/Listen/accept/reconnect orchestration; Phase 7 live Pinex TCP interoperability/fault injection; Phase 8 benchmarks/pooling; Phase 9 examples/release/deployment. No architecture decision, numeric default, public API or wire contract is reopened.


## Phase 5 implementation completion evidence — 2026-11-17

# Phase 5 — Bounded Callback Worker Scheduler Implementation Report

**Status:** COMPLETE / CI PASS  
**Source verification HEAD:** `c72136693f85e61239bfca42bd04b40e66eddc27`  
**Date:** 2026-11-17  
**Authorization:** Explicit owner approval recorded in this conversation and in `IMPLEMENTATION_AUTHORIZATION_2026-11-17.md`.

## Implemented scope

- Added one private `ownerCallbackDispatcher` per Client and Server instance. Each owner contains three distinct pools: Application, Completion and OnClose. Each pool has its own mutex, per-Session queues, ready-session list, idle-worker list and wake-up channels; there is no process-global pool or cross-class scheduler lock.
- Client/Server constructors create only pool metadata. Worker goroutines are created lazily when dispatchable work arrives.
- Worker goroutine count is bounded by the configured owner/class ceiling. Each worker runs one callback synchronously and is reusable only within its own class.
- Added per-Session callback concurrency intersections: Application uses `applicationConcurrency`, Completion uses `completionConcurrency`, and OnClose is single-event per Session. Owner ceilings remain independent by class.
- Implemented per-class FIFO round-robin ready-Session scheduling. Each Session/class has one reusable intrusive ready node; a worker selects one callback per eligible Session turn and requeues the Session at the tail when it remains eligible. Per-Session callback FIFO is preserved.
- Implemented finite class-specific idle retirement using a reusable worker timer. A worker becomes idle only after callback return and when no dispatchable work exists; work arriving during the grace reuses it. The approved zero warm-idle floor is preserved. Running callbacks are never retired.
- Established the callback-start / Forced Close A linearization using `Session.dispatchMu`. Workers release the pool mutex before waiting for this lock, then re-check and claim a queued task while holding the Session lock followed by the pool lock. If Forced Close wins, queued Application/Completion work is cancelled exactly once. If callback start wins, the callback retains its active slot until actual return.
- Started callbacks execute without Session or pool locks held. Their Request and payload/event charges continue to be released by the existing idempotent task lifecycle only after callback return.
- OnClose remains eligible only after Session.Done. Its existing owner event credit covers queued + running work; saturation permanently skips the event. OnClose is not cancelled by Forced Close A.
- Added owner-level dispatcher fields to Client and Server. Phase 6 will pass these same owner dispatchers to every Session generation created by the Client/Server orchestration layer; this preserves the owner budget across reconnect and across all Server Sessions.

## Verification

- Added **16 Worker Scheduler tests** covering lazy pool creation, Client/Server and class isolation, blocked callback isolation, round-robin fairness, owner ceiling across Session generations, owner permits held after Session.Done, per-Session concurrency, Forced Close cancellation/start behavior, OnClose after Done, queued + running event-credit saturation, blocked OnClose isolation, worker reuse/retirement, blocked callback non-retirement, concurrent enqueue/retirement, and concurrent enqueue/Forced Close exact-once settlement.
- Added Session integration tests proving the real Reader and Writer continue processing Enquire Link while Application or Completion callbacks are blocked, Session.Done does not wait for started callbacks, and resource credits remain held until those callbacks return.
- Existing coverage remains: 46 Session/transport tests and 8 owner resource-ledger tests.
- Go Actions [44066876353](https://github.com/majiddarvishan/neta-go/actions/runs/44066876353): **PASS** — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility [44066876291](https://github.com/majiddarvishan/neta-go/actions/runs/44066876291): **PASS** — pinned C++ 1.5.0 production serializer golden-frame verification.
- The Pinex workflow is a golden-frame compatibility check, **not** live Go↔Pinex TCP interoperability.

## Explicitly not implemented in Phase 5

- Phase 6 Client Dial/Server Listen/accept, Bind registry, reconnect supervisor, Session-generation creation and Stop orchestration. Client/Server now own dispatcher objects, but network lifecycle wiring belongs to Phase 6.
- Phase 7 live bidirectional Go↔Pinex TCP interoperability and fault-injection campaign.
- Phase 8 performance benchmark baseline and evidence-driven pooling/optimization.
- Phase 9 examples, release readiness and deployment.

No public API, wire behavior, approved defaults, resource ceilings or frozen architecture decisions were changed.



## Phase 6 authorization — Client/Server orchestration (2026-11-17)

The owner explicitly authorized Phase 6 by replying **«ادامه بده»** after being asked whether Phase 6 could start.

**Authorized scope:** P6-01 through P6-11 in the root [TASKS.md](../TASKS.md):
- Client Dial/Bind, lifecycle, CurrentSession, terminal Stop, reconnect supervisor and generation fencing, strictly following API-001/API-002.
- Server Listen/Accept, pending socket admission, absolute Pre-Bind deadline, Bound Session capacity, BindValidator, atomic duplicate-SystemID admission/removal and terminal Stop.
- Shared owner resource ledger and callback dispatcher across all Sessions/generations.
- Focused lifecycle, capacity, concurrency, reconnect and shutdown tests; gofmt, vet, unit and race CI.

**Explicit exclusions:** Phase 7 live Go↔Pinex TCP interoperability/fault-injection campaign; Phase 8 benchmark/optimization; Phase 9 examples/release/deployment.

**Pre-implementation API gap:** The frozen Client API requires a Bind Request, but the approved ClientOption/config surface has no Client SystemID or BindType field/option. No default for either is approved. Server-side Phase 6 work may proceed; do not invent the missing Client Bind identity semantics or add a public option until the owner clarifies it.


## Phase 6 Server implementation evidence — 2026-11-17

Server sub-scope P6-04..P6-08 is COMPLETE / CI PASS at source HEAD `8eead509c6817722d6b7c6f5e5a17e1734749911`.

- Server Start/Addr/Accept; bounded Pre-Bind socket admission; absolute Pre-Bind deadline.
- Bound Session capacity reservation; optional BindValidator; atomic duplicate SystemID admission; no eviction.
- Shared Server resource ledger and callback dispatcher injected into every accepted Session.
- Session pointer-identity registry promotion and cleanup; no stale Session can remove a replacement ID.
- Terminal/idempotent Server Stop with listener-first close, Pre-Bind forced close, concurrent Bound Unbind and caller-Context forced escalation.
- 11 focused Server lifecycle/admission tests.
- Go CI [44070940615](https://github.com/majiddarvishan/neta-go/actions/runs/44070940615): PASS — gofmt, vet, unit, race.
- Pinex Wire Compatibility [44070940706](https://github.com/majiddarvishan/neta-go/actions/runs/44070940706): PASS.

Phase 6 remains IN PROGRESS. Client Connect/Bind, CurrentSession, Client Stop, reconnect supervisor and generation fencing remain pending the owner decision on Client Bind SystemID/BindType input. Phase 7+ remain excluded.
