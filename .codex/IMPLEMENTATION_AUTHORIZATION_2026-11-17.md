# neta-go — Implementation Authorization Record

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Architecture:** Frozen; all fundamental owner decisions closed.  
**Current implementation status:** Phase 1 COMPLETE / CI PASS; Phase 2 Wire Codec COMPLETE / CI PASS; Phase 3 Session Transport & Lifecycle COMPLETE / CI PASS; Phase 4+ outside current authorization.

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
