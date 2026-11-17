# neta-go Phase 3 — Session Transport & Lifecycle Implementation

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Authorization:** OWNER APPROVED — Phase 3 only  
**Baseline:** ebf269dc9ba1a8f5d88922cf1e6ab29c99782a05  
**Compatibility:** Pinex C++ 1.5.0, develop f7bd1eb779baff0d10e624f20522df345e6ee747

## 1. Owner authorization

The owner explicitly approved Phase 3 in chat pg_07 by replying «اره هست و مجوز داری» to the direct question asking permission to begin Phase 3.

## 2. Objective

Implement the private Session transport and lifecycle engine against the frozen API-001, API-002, API-003, resource and Pinex wire contracts. The engine is the internal foundation used later by Client/Server orchestration; Phase 3 does not make Client.Connect or Server.Start operational.

## 3. Authorized work

1. Exactly one Reader loop and one serialized Writer loop per Session.
2. Session-side Bind/Pre-Bind handshake primitives and state transitions. The Server owner/accept loop remains responsible for consuming a Bind request event and deciding admission.
3. Absolute Pre-Bind/Bind response deadlines, absolute per-Frame assembly deadline from the first Header byte, and absolute per-Frame Writer deadline from Writer ownership.
4. Idle-only Enquire Link with the approved 30s default, explicit zero disable, one outstanding probe, and matching-sequence 10s response deadline beginning after full probe Write.
5. Session-scoped nonzero Sequence allocation and Request correlation. RequestTimeout is 30s by default and begins immediately after successful local admission; timeout/response/close has one terminal-settlement winner. A late response never resurrects a settled Request.
6. Graceful Unbind with atomic send-admission closure, FIFO drain, Unbind Request after previously admitted Application frames, and the approved 10s absolute deadline.
7. Forced Close A, immutable terminal cause, Session.Done after internal teardown, Session.Err, and exact cleanup of timers, queues, pending state and transport loops.
8. Focused deterministic tests for fragmented I/O, slow-trickle timeout, partial writes, queue cancellation, response correlation, timeout races, Unbind, Bind and close.

## 4. Frozen implementation constraints

- Preserve the 10-byte Pinex Header and 1 MiB total Frame maximum.
- Use Go standard library only; no third-party event loop or network dependency.
- One Reader and one Writer per Session; concurrent API calls cannot interleave Frame bytes.
- No user callback runs inline in Reader or Writer.
- Sending snapshots caller payload before successful admission returns. Receiving provides an owned buffer that may be retained.
- No transparent Request replay/resend, automatic sequence deduplication, or protocol change.
- No goroutine per Request, frame, callback or timer. Use one Session control/deadline loop for bounded timer management.
- Keep public surface exactly as frozen in API-001 §21.

## 5. Strict phase boundary

| Later phase | Excluded from this authorization |
|---|---|
| Phase 4 — Admission & Resource Accounting | Full independent queue byte/count, retained Application/Completion byte and owner aggregate ledgers; the minimal per-Session FIFO bound already frozen may be used to keep the Writer bounded |
| Phase 5 — Worker Scheduler | Three isolated lazy bounded Application, Completion and OnClose worker pools, fairness and idle retirement; Phase 3 uses only a private dispatch interface |
| Phase 6 — Client/Server Orchestration | Client Dial/Connect, Server Listen/Start/accept manager, reconnect supervisor, generation fencing, owner registry, duplicate System ID management and full BindValidator orchestration |
| Phase 7 — Verification campaign | Live Go↔Pinex TCP interoperability in both directions, full fault-injection and soak campaign |
| Phase 8 — Performance | Benchmark baseline, profiling and PERF-004 pooling/batching decisions |
| Phase 9 — Release | Examples/documentation release readiness, version tag and deployment |

No phase beyond Phase 3 is authorized by this record.

## 6. Exit criteria

- Session has one Reader, one serialized Writer and one bounded internal deadline/control loop.
- Valid Pinex Frames are read/written under fragmentation and short-write conditions.
- Header is validated before Body allocation; Frame assembly deadline is absolute and not reset by progress.
- Bind/Pre-Bind handshake transitions are deterministic and bounded by the approved deadlines.
- Request correlation, timeout, late response and close races settle internally exactly once.
- Graceful Unbind preserves outbound FIFO and completes or escalates at its absolute deadline.
- Forced Close A never waits for user callbacks; Done closes after internal teardown and terminal cause publication.
- gofmt, go vet ./..., go test ./..., and go test -race ./... pass in CI.

## 7. Implementation status — COMPLETE / CI PASS

**Phase 3 source verification HEAD:** `cc0d25e9252b48f38dcddeeb53eaa6e7fe1cbf4c`.

### Implemented

- Exactly one Reader loop, one serialized Writer loop and one Session control/deadline loop.
- Client Bind and server Pre-Bind handshake primitives, state transitions and absolute deadlines.
- Absolute Frame Assembly deadline from the first Header byte, including the server BindResponse-to-Bound transition race.
- Absolute per-Frame Writer deadline measured from Writer ownership.
- Idle-only Enquire Link, zero-disable behavior, incoming probe response, one outstanding probe and matching-sequence response deadline.
- Session-scoped nonzero Sequence allocation, bounded Request tracking, Request deadline from successful local admission, response correlation, late-response suppression and exactly-once terminal settlement.
- Graceful Unbind admission closure/FIFO barrier/deadline; Forced Close A, immutable terminal cause, Done publication and internal cleanup.
- Private callback-dispatch interface; no user callback runs inline in Reader or Writer.

### Verification evidence

- `session_test.go`: 33 focused Session/transport/lifecycle tests.
- Go GitHub Actions run `44057820360`: PASS — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility run `44057820405`: PASS — pinned C++ 1.5.0 production-serializer fixture build, golden comparison and Go wire tests.
- No third-party runtime/network dependency was added.
- Live Go↔Pinex TCP interoperability is intentionally deferred to Phase 7.

### Remaining phase boundaries

Phase 3 is complete. Phase 4 full Resource Admission/owner ledgers, Phase 5 bounded callback Worker Scheduler, Phase 6 Client/Server orchestration/reconnect, Phase 7 live interoperability/fault-injection, Phase 8 performance and Phase 9 release remain outside this authorization and must not be started without the corresponding approval.