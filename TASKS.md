# neta-go — Canonical Implementation Task Tracker

**Repository:** `github.com/majiddarvishan/neta-go`  
**Branch:** `main`  
**Tracker owner:** Project owner  
**Last updated:** 2026-11-17  
**Original tracker baseline HEAD:** `c98f731fef89519d62d95248982538f25a36db82`  
**Last verified source HEAD:** `1484f26437cb509a56b377e0efe68ea32b175c3d`  
**Current phase:** Phase 6 — Client and Server orchestration (**COMPLETE / CI PASS**, owner approval 2026-11-17). Phase 7+ remain gated.

This root-level file is the single, canonical, implementation-facing task list for the repository. It is intentionally placed beside the source tree so it is visible immediately on the GitHub repository page. Update the checkboxes in the same commit that completes a task or its acceptance evidence. Keep design decisions in `DECISION_LOG.md`; this file tracks implementation and verification.

## Status legend

- `[x]` Implemented and accepted with evidence recorded below.
- `[ ]` Not complete. A task may be in progress only when its phase heading says so.
- A completed design decision does **not** mean its implementation task is complete.
- CI links refer to GitHub Actions runs for the exact source/documentation HEAD shown.

## Overall status

| Phase | Scope | Status | Authorization |
|---|---|---|---|
| 0 | Architecture and contract freeze | COMPLETE | Complete |
| 1 | Module and immutable configuration | COMPLETE / CI PASS | Complete |
| 2 | Pinex-compatible Wire Codec | COMPLETE / CI PASS | Complete |
| 3 | Session transport and lifecycle | COMPLETE / CI PASS | Complete |
| 4 | Resource admission and accounting | COMPLETE / CI PASS | Authorized 2026-11-17 |
| 5 | Bounded callback Worker Scheduler | COMPLETE / CI PASS | Owner approved 2026-11-17 |
| 6 | Client/Server orchestration and reconnect | COMPLETE / CI PASS | Owner approved 2026-11-17 |
| 7 | Live interoperability and fault-injection verification | NOT STARTED | Not yet authorized |
| 8 | Performance baseline and evidence-driven optimization | NOT STARTED | Not yet authorized |
| 9 | Examples, documentation and V1 release readiness | NOT STARTED | Not yet authorized |

---

## Phase 0 — Architecture and contract freeze

<badge>COMPLETE</badge>

- [x] Freeze the Pinex C++ 1.5.0 compatibility target and record source commit `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- [x] Freeze the 10-byte Header, command/status/sequence layout and 1 MiB total Frame maximum.
- [x] Close API-001 public surface and root-package boundary.
- [x] Close API-002 timeout, reconnect and shutdown contracts.
- [x] Close API-003 module/package architecture.
- [x] Close API-004 Functional Options and immutable configuration semantics.
- [x] Close Q-008/Q-009/Q-010 queue and overload policies.
- [x] Close PERF-002 payload ownership and PERF-003 Request-credit model.
- [x] Close WORKER-001..009C ownership, worker topology, capacity defaults, limits and accounting policy.
- [x] Record final decisions and implementation boundaries in `DECISION_LOG.md`.

**Evidence:** `.codex/DECISION_LOG.md`, `.codex/NETA_GO_ARCHITECTURE_FREEZE_DECISION_REGISTER_2026-11-17.md`, and the linked design records.

## Phase 1 — Module and immutable configuration

<badge color="success">COMPLETE / CI PASS</badge>

- [x] Create `go.mod` with module path `github.com/majiddarvishan/neta-go` and root package `neta`.
- [x] Implement typed `ClientOption`, `ServerOption` and `SessionOption`.
- [x] Implement complete defaults and immutable effective Client/Server/Session configuration snapshots.
- [x] Apply all options before final validation; reject nil/invalid options deterministically.
- [x] Implement API-004 and WORKER-009C ranges, hard maxima, cross-field validation and checked arithmetic.
- [x] Add typed public error categories and Session state declarations.
- [x] Add configuration/default/range/precedence tests.
- [x] Pass formatting, vet, unit and race checks.

**Evidence:** Phase 1 implementation recorded in `IMPLEMENTATION_AUTHORIZATION_2026-11-17.md`; CI run [44040206052](https://github.com/majiddarvishan/neta-go/actions/runs/44040206052).

## Phase 2 — Pinex-compatible Wire Codec

<badge color="success">COMPLETE / CI PASS</badge>

- [x] Implement pure-Go `internal/wire` Header and Frame codec.
- [x] Implement Bind, Stream, Unbind and Enquire Link codecs with Pinex-compatible command/status/sequence semantics.
- [x] Enforce the 1 MiB total Frame maximum inbound and outbound, before Body allocation.
- [x] Add deterministic golden fixtures generated from the pinned production C++ serializer source snapshot.
- [x] Compile the C++ fixture generator and compare Go codec output against the golden fixtures in CI.
- [x] Match Pinex's omitted Bind Response Body default-value behavior.
- [x] Pass gofmt, vet, unit and race tests plus C++ golden compatibility workflow.

**Evidence:** `NETA_GO_PHASE_2_WIRE_IMPLEMENTATION_2026-11-17.md`; Go CI [44044273742](https://github.com/majiddarvishan/neta-go/actions/runs/44044273742), C++ golden CI [44044273818](https://github.com/majiddarvishan/neta-go/actions/runs/44044273818).

## Phase 3 — Session transport and lifecycle

<badge color="success">COMPLETE / CI PASS</badge>

- [x] Implement one Reader and one serialized Writer per Session.
- [x] Implement client Bind and server Pre-Bind Session-side handshake primitives and state transitions.
- [x] Implement absolute Bind/Pre-Bind, Frame Assembly and per-Frame Writer deadlines.
- [x] Implement idle-only Enquire Link, matching response tracking and incoming Enquire Link replies.
- [x] Implement per-Session nonzero Sequence allocation and Request correlation.
- [x] Implement Request timeout, late-response suppression and exactly-once internal settlement.
- [x] Implement bounded per-Session outgoing Application FIFO byte/frame admission and separate Control-PDU reserve.
- [x] Implement Graceful Unbind, Forced Close A, immutable terminal cause and Done after internal teardown.
- [x] Ensure no user callback runs inline in Reader or Writer; retain private dispatcher boundary for Phase 5.
- [x] Harden queued Enquire Link cancellation after outbound activity.
- [x] Ignore Responses that arrive before Writer claims their Request.
- [x] Preserve the first post-Bind Frame Assembly deadline across the server Bind transition.
- [x] Start RequestTimeout after successful local FIFO admission.
- [x] Add focused fragmentation, partial-write, timeout, correlation, close and race tests.
- [x] Pass gofmt, vet, unit and race CI.

**Evidence:** `NETA_GO_PHASE_3_SESSION_TRANSPORT_LIFECYCLE_2026-11-17.md`; source verification HEAD `cc0d25e9252b48f38dcddeeb53eaa6e7fe1cbf4c`; Go CI [44057820360](https://github.com/majiddarvishan/neta-go/actions/runs/44057820360), C++ golden compatibility CI [44057820405](https://github.com/majiddarvishan/neta-go/actions/runs/44057820405). The C++ workflow is golden-frame compatibility, **not** live TCP interoperability.

## Phase 4 — Resource Admission and Accounting

<badge color="success">COMPLETE / CI PASS</badge>

**Owner authorization:** 2026-11-17, this conversation. Scope is limited to the already-approved WORKER/Q-008/Q-009/Q-010/PERF-002/PERF-003 contracts. Do not change public API, defaults, wire behavior or frozen overload policies.

### Existing Phase 3 prerequisites

- [x] Per-Session outgoing Application FIFO byte and frame limits; the active Writer-owned frame remains charged until full Write or discard.
- [x] Separate bounded per-Session Control-PDU byte reserve.
- [x] Per-Session pending Request population is bounded and held through normal Completion callback return/cancellation.

### Implementation tasks

- [x] **P4-01 — Reservation primitives:** implement race-safe, checked reserve/release primitives for owner byte and count resources; reject overflow/underflow. Session lifecycle idempotency is verified in the integration tasks below.
- [x] **P4-02 — Owner ledgers:** implement independent Client/Server-owned counters for outstanding Request credits, retained Application bytes, retained Completion bytes and OnClose event credits. NewClient/NewServer each create one independent owner ledger; Phase 6 will inject the same Client ledger into every reconnect generation.
- [x] **P4-03 — Server aggregate queue ledger:** one combined incoming+outgoing Application queued-frame byte (256 MiB default) and frame-count (65,536 default) budget per Server.
- [x] **P4-04 — Outgoing Application admission:** per-Session queue/count, retained Application and owner/Server budgets reserved before Frame allocation/copy or Request publication; Queue Full and Capacity remain distinct; rejected work consumes no Sequence/Pending state.
- [x] **P4-05 — Incoming Application admission:** full declared Frame bytes/count and retained Application capacity reserved after Header validation but before Body allocation; offender-only close on overload; partial-read/cancel/return release paths covered.
- [x] **P4-06 — Completion admission:** matched Response retained bytes reserved before Body allocation; capacity exhaustion uses fixed 4 KiB scratch drain under the original absolute Frame Assembly deadline, then no-payload `ErrCapacity` Completion if the Request still wins.
- [x] **P4-07 — Request credits:** per-Session and owner Request ceilings; credits held through response wait and Completion callback, released once on return/cancel/terminal cleanup.
- [x] **P4-08 — OnClose event credits:** one nonblocking owner reservation after Done; permanent skip at capacity; release after callback return/cancellation.
- [x] **P4-09 — Lifecycle transfer/release:** queue-only charges release at callback start; retained payloads follow callback ownership after Done; Writer completion, partial I/O, timeout, Forced Close A and callback cancellation release paths tested.
- [x] **P4-10 — Admission verification:** exact/over boundaries, concurrent owner admission, multi-Session aggregate limits, scratch-drain success/timeout, partial reads, cancellation and callback-retention tests.
- [x] **P4-11 — CI acceptance:** gofmt, go vet ./..., go test ./... and go test -race ./... all PASS on Phase 4 source HEAD.

**Phase 4 exclusions:** no Client Dial/Server Listen or reconnect orchestration (Phase 6), no live Pinex TCP interoperability campaign (Phase 7), no benchmark/pooling work (Phase 8), and no release/deployment (Phase 9). Phase 5 is separately authorized below.

## Phase 5 — Bounded callback Worker Scheduler

<badge color="success">COMPLETE / CI PASS</badge>

- [x] **P5-01 — Three isolated pools:** each Client/Server owner now creates separate lazy Application, Completion and OnClose pools with independent queue, lock, ready list and wake-up state.
- [x] **P5-02 — Active permits:** owner/class worker ceilings and per-Session concurrency intersections are enforced; the same owner dispatcher is reusable by every Client generation and every Server Session.
- [x] **P5-03 — Fair scheduling:** one reusable ready-Session node per class; FIFO per-Session tasks and round-robin one-callback-per-eligible-Session turns.
- [x] **P5-04 — Idle retirement:** per-class configurable grace, 30s default, reusable worker timer, zero warm-idle floor, worker reuse before expiry and no retirement of running callbacks.
- [x] **P5-05 — Forced Close A:** callback start is linearized with Forced Close by Session.dispatchMu; queued Application/Completion work is cancelled once, while started callbacks retain permits and payload/Request charges through return.
- [x] **P5-06 — OnClose policy:** queued only after Done, best-effort/at-most-once, existing owner event credit covers queued + running callbacks, and saturation permanently skips the event.
- [x] **P5-07 — Isolation/liveness tests:** blocked Application does not consume Completion/OnClose workers; blocked OnClose does not consume Application/Completion workers; real Session Reader/Writer and Done progress while callbacks block; no goroutine-per-event.
- [x] **P5-08 — Race and churn tests:** concurrent enqueue/retirement and enqueue/Forced Close tests, retired-generation permit retention, queued+running OnClose credit saturation, fairness tests and Go race detector pass.
- [x] **P5-09 — CI acceptance:** gofmt, go vet ./..., go test ./... and go test -race ./... pass; Pinex C++ 1.5.0 golden compatibility workflow passes.

### Phase 5 completion evidence — 2026-11-17

- Implementation report: [.codex/NETA_GO_PHASE_5_WORKER_SCHEDULER_IMPLEMENTATION_2026-11-17.md](.codex/NETA_GO_PHASE_5_WORKER_SCHEDULER_IMPLEMENTATION_2026-11-17.md).
- Source verification HEAD: `c72136693f85e61239bfca42bd04b40e66eddc27`.
- 16 Worker Scheduler tests; existing 46 Session tests and 8 owner-ledger tests remain green.
- Go CI [44066876353](https://github.com/majiddarvishan/neta-go/actions/runs/44066876353) PASS; Pinex Wire Compatibility [44066876291](https://github.com/majiddarvishan/neta-go/actions/runs/44066876291) PASS.
- Phase 6 was authorized on 2026-11-17; implementation is in progress. See the Phase 6 plan and authorization addendum.

## Phase 6 — Client and Server orchestration

<badge color="success">COMPLETE / CI PASS — 2026-11-17</badge>

**Authorization:** Owner replied «ادامه بده» after the explicit Phase 6 authorization question. Scope is limited to P6-01..P6-11. Phase 7+ remain gated.

**Client Bind input decision CLOSED:** Owner approved Option B. WithClientSystemID(string) is mandatory; WithClientBindType(BindType) is optional with bi-directional default and uni-directional support. Client.Connect(ctx) signature remains unchanged. See DECISION_LOG.md §56.

- [x] **P6-01 — Client Connect:** Dial with absolute ConnectTimeout, Bind with absolute BindResponseTimeout and Context cancellation semantics.
- [x] **P6-02 — Client lifecycle:** CurrentSession exposes only the active Bound generation; terminal/idempotent Stop cancels Dial/Bind/reconnect, gracefully Unbinds or forces the current Session, and waits for internal Done; generation publication and stale-generation fencing are implemented.
- [x] **P6-03 — Reconnect supervisor:** opt-in AutoReconnect=false by default; approved bounded exponential Full Jitter policy; no Request replay/resend; share owner resource/worker ledgers across generations.
- [x] **P6-04 — Server listener:** Start/Addr/Accept lifecycle and listener-close ordering.
- [x] **P6-05 — Pre-Bind admission:** enforce pending socket count before Session allocation, absolute accept-to-Bind deadline and early refusal.
- [x] **P6-06 — Bound Session registry:** enforce MaxBoundSessions without eviction, reject excess Bind with Pinex rfail where feasible.
- [x] **P6-07 — Identity admission:** BindValidator and duplicate System ID rejection with atomic registry insertion/removal.
- [x] **P6-08 — Server Stop:** terminal/idempotent Stop; close listener and Pre-Bind Sessions, concurrently Graceful-Unbind Bound Sessions, force remaining Sessions on caller Context expiry.
- [x] **P6-09 — Owner integration:** Server Sessions share the Server resource ledger and worker owner; every Client generation reuses the Client resource ledger and callback dispatcher.
- [x] **P6-10 — Lifecycle tests:** Server lifecycle/admission, Client Stop/CurrentSession, Connect/Bind, timeout/rejection, reconnect generation replacement and uni-directional requester-only tests are implemented.
- [x] **P6-11 — CI acceptance:** gofmt, go vet ./..., go test ./..., go test -race ./... and pinned Pinex C++ 1.5.0 Golden Compatibility all PASS at source HEAD 1484f26437cb509a56b377e0efe68ea32b175c3d.

### Phase 6 Server implementation evidence — 2026-11-17

- Report: [Phase 6 Server implementation report](.codex/NETA_GO_PHASE_6_SERVER_IMPLEMENTATION_2026-11-17.md).
- Source verification HEAD: `8eead509c6817722d6b7c6f5e5a17e1734749911`.
- P6-04 through P6-08 complete; P6-02 is partially complete (Client CurrentSession/Stop); P6-09 and P6-10 are partially complete across Server and Client lifecycle slices.
- 11 focused Server lifecycle/admission tests.
- Go CI [44070940615](https://github.com/majiddarvishan/neta-go/actions/runs/44070940615) PASS; Pinex Wire Compatibility [44070940706](https://github.com/majiddarvishan/neta-go/actions/runs/44070940706) PASS.
- At this historical checkpoint, Client Connect/Bind remained blocked by the missing Bind identity input. The owner later approved Option B; Client Connect/Bind and reconnect are now complete. See the Client implementation evidence below.

## Phase 7 — Live interoperability and fault-injection verification

<badge color="secondary">NOT STARTED — NOT YET AUTHORIZED</badge>

- [ ] **P7-01 — Re-pin Pinex:** verify Pinex C++ develop version/HEAD and reconcile any source change from the pinned 1.5.0 baseline.
- [ ] **P7-02 — Bidirectional TCP interoperability:** Go Client ↔ Pinex C++ Server and Pinex C++ Client ↔ Go Server using real sockets.
- [ ] **P7-03 — Wire fuzzing:** fuzz Header length, command/status/sequence, Bind body and malformed/oversized Frames.
- [ ] **P7-04 — Transport faults:** partial reads/writes, slow-trickle, blackhole, EOF, reset, delayed and reordered responses, Bind rejection and timeout races.
- [ ] **P7-05 — Admission faults:** queue/byte/count/Request/retained-memory exhaustion and scratch-drain timeout; verify offender-only close and no leaked charges.
- [ ] **P7-06 — Callback faults:** blocked callbacks, Forced Close races, OnClose saturation and old-generation callback retention.
- [ ] **P7-07 — Long-running soak:** verify bounded goroutine/heap/ledger levels under Session churn, saturation and reconnect.
- [ ] **P7-08 — Compatibility acceptance:** all golden vectors and both real-socket directions pass.

## Phase 8 — Performance baseline and evidence-driven optimization

<badge color="secondary">NOT STARTED — NOT YET AUTHORIZED</badge>

- [ ] **P8-01 — Reproducible baseline:** benchmark payload sizes, one/many Sessions, concurrent producers, callback concurrency, saturation and reconnect churn.
- [ ] **P8-02 — Record metrics:** throughput, p50/p95/p99 latency, allocs/op, bytes/op, CPU, heap/GC, goroutines and contention.
- [ ] **P8-03 — Profile hot paths:** identify allocation, copy, mutex and scheduler costs before optimization.
- [ ] **P8-04 — PERF-004 decisions:** evaluate pooling/batching only where measured benefit justifies complexity and ownership remains safe.
- [ ] **P8-05 — Regression gate:** rerun correctness, race and interoperability after every accepted optimization; set numeric targets only from workload evidence.

## Phase 9 — Documentation, examples and V1 release readiness

<badge color="secondary">NOT STARTED — NOT YET AUTHORIZED</badge>

- [ ] **P9-01 — Client example:** connect, Bind, SendRequest/SendNoWait, Completion and graceful Stop.
- [ ] **P9-02 — Server example:** Start, BindValidator, OnRequest, SendResponse and graceful Stop.
- [ ] **P9-03 — Contract docs:** callback concurrency/order, payload ownership, admission errors, resource defaults, timeout/reconnect and shutdown semantics.
- [ ] **P9-04 — Compatibility docs:** Pinex version matrix, golden evidence and real TCP interoperability results.
- [ ] **P9-05 — Release review:** API stability, examples, supported Go/platform baseline, versioning/changelog and owner release approval.
- [ ] **P9-06 — Tag V1:** only after all verification gates and explicit owner release approval.

---

## Verification and update log

| Date | HEAD / Commit | Scope | Evidence |
|---|---|---|---|
| 2026-11-17 | `cc0d25e9252b48f38dcddeeb53eaa6e7fe1cbf4c` | Phase 3 source | Go [44057820360](https://github.com/majiddarvishan/neta-go/actions/runs/44057820360) PASS; C++ golden [44057820405](https://github.com/majiddarvishan/neta-go/actions/runs/44057820405) PASS |
| 2026-11-17 | `c98f731fef89519d62d95248982538f25a36db82` | Documentation/handoff | Go [44058276200](https://github.com/majiddarvishan/neta-go/actions/runs/44058276200) PASS; C++ golden [44058274150](https://github.com/majiddarvishan/neta-go/actions/runs/44058274150) PASS |
| 2026-11-17 | `8eead509c6817722d6b7c6f5e5a17e1734749911` | Phase 6 Server lifecycle/admission | Go [44070940615](https://github.com/majiddarvishan/neta-go/actions/runs/44070940615) PASS; Pinex Wire Compatibility [44070940706](https://github.com/majiddarvishan/neta-go/actions/runs/44070940706) PASS |
| 2026-11-17 | `a05a735aa0383d2588b41b1a2258282d734ff72b` | Phase 6 Client CurrentSession/Stop lifecycle | Go [44071959633](https://github.com/majiddarvishan/neta-go/actions/runs/44071959633) PASS; Pinex Wire Compatibility [44071959704](https://github.com/majiddarvishan/neta-go/actions/runs/44071959704) PASS |
| 2026-11-17 | `1484f26437cb509a56b377e0efe68ea32b175c3d` | Phase 6 Client Connect/Bind, reconnect and final acceptance | Go [44077921877](https://github.com/majiddarvishan/neta-go/actions/runs/44077921877) PASS; Pinex Wire Compatibility [44077921981](https://github.com/majiddarvishan/neta-go/actions/runs/44077921981) PASS |

## Canonical references

- [Project handoff](.codex/HANDOFF.md)
- [Architecture freeze and implementation roadmap](.codex/NETA_GO_ARCHITECTURE_FREEZE_AND_IMPLEMENTATION_ROADMAP_2026-11-17.md)
- [Implementation authorization](.codex/IMPLEMENTATION_AUTHORIZATION_2026-11-17.md)
- [Resource admission design](.codex/NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md)
- [Worker capacity and resource accounting design](.codex/NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md)
- [Performance contract](.codex/NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md)
- [Phase 6 Server implementation report](.codex/NETA_GO_PHASE_6_SERVER_IMPLEMENTATION_2026-11-17.md)
- [Phase 6 Client lifecycle report](.codex/NETA_GO_PHASE_6_CLIENT_STOP_IMPLEMENTATION_2026-11-17.md)
- [Decision log](.codex/DECISION_LOG.md)


### Phase 4 progress evidence — 2026-11-17

- P4-01 and P4-02 implemented in `resource_ledger.go`; Client/Server constructors now create independent owner ledgers.
- Tests cover exact capacity, over-capacity rejection, release/underflow protection, independent Application/Completion budgets, shared Server aggregate queue counters, Client-vs-Server aggregate behavior, OnClose event credits, owner isolation and concurrent Request admission.
- Implementation commit: `9f2514115b601c7cdc2efb91fe98daa628b1c7b0`; formatting-only follow-up: `94760d4d0cb2a319b7c49665b0cfcffc4b7b45db`.
- Go CI [44060075443](https://github.com/majiddarvishan/neta-go/actions/runs/44060075443) PASS; Pinex Wire Compatibility [44060075468](https://github.com/majiddarvishan/neta-go/actions/runs/44060075468) PASS.
- Current source baseline for the next Phase 4 slice: `94760d4d0cb2a319b7c49665b0cfcffc4b7b45db`.


### Phase 4 completion evidence — 2026-11-17

- Implementation report: [NETA_GO_PHASE_4_RESOURCE_ADMISSION_IMPLEMENTATION_2026-11-17.md](NETA_GO_PHASE_4_RESOURCE_ADMISSION_IMPLEMENTATION_2026-11-17.md).
- Source verification HEAD: `2935402366603267ba60c181a057d26cf0e7e9a6`.
- 46 Session tests + 8 owner-ledger tests.
- Go CI [44062568415](https://github.com/majiddarvishan/neta-go/actions/runs/44062568415) PASS; Pinex Wire Compatibility [44062568425](https://github.com/majiddarvishan/neta-go/actions/runs/44062568425) PASS.
- At the time Phase 4 closed, Phase 5 had not yet been authorized. Phase 5 was subsequently authorized and is now COMPLETE / CI PASS; see the Phase 5 completion evidence below.


### Phase 5 implementation authorization — 2026-11-17

The owner explicitly approved Phase 5 by replying «آره» after the request to begin the bounded callback Worker Scheduler.

**Authorized scope:** implement the already-frozen WORKER-001..009C contracts only:
- Three independent owner-scoped Application, Completion and OnClose pools; lazy creation; no shared scheduling/lock/wakeup domain across classes.
- Enforce owner active-worker ceilings and per-Session concurrency intersections; preserve owner accounting across Session generations.
- Per-class FIFO round-robin scheduling with one reusable ready-Session node and one callback per eligible Session turn.
- Independently configurable 30s idle retirement grace, zero warm-idle floor, worker reuse and enqueue/retirement linearization.
- Forced Close A cancellation/start linearization; queued Application/Completion cancellation and exact resource release; started callbacks retain permits/credits until return.
- OnClose only after Done, best-effort/at-most-once, bounded queued+running event credits and permanent skip at saturation.
- Isolation, fairness, lifecycle, race and churn tests plus gofmt, vet, unit and race CI.

**Not authorized by this approval:** Phase 6 Client/Server networking and reconnect, Phase 7 live Pinex TCP interoperability, Phase 8 benchmark/pooling optimization, Phase 9 examples/release/deployment. No frozen decision, default, public API or wire contract is reopened.


### Phase 5 authorization closure

Phase 5 was explicitly authorized by the owner on 2026-11-17 and is now COMPLETE / CI PASS. This closure does not authorize Phase 6 or later phases.


### Phase 6 Client lifecycle evidence — 2026-11-17

- Report: [Client CurrentSession/Stop lifecycle](.codex/NETA_GO_PHASE_6_CLIENT_STOP_IMPLEMENTATION_2026-11-17.md).
- Source verification HEAD: `a05a735aa0383d2588b41b1a2258282d734ff72b`.
- 4 focused Client lifecycle tests.
- Go CI [44071959633](https://github.com/majiddarvishan/neta-go/actions/runs/44071959633) PASS; Pinex Wire Compatibility [44071959704](https://github.com/majiddarvishan/neta-go/actions/runs/44071959704) PASS.
- At this earlier Client lifecycle checkpoint, Connect/Bind still awaited the owner Bind identity decision; Option B was subsequently approved and implemented. Final evidence is recorded in the completion section below.


### Phase 6 Client Connect/Bind and reconnect implementation — 2026-11-17

- Owner decision: Option B, recorded in .codex/DECISION_LOG.md §56.
- Added mandatory WithClientSystemID(string) and optional WithClientBindType(BindType); default is bi-directional.
- Added Client.Connect(ctx) with absolute Dial and Bind deadlines, caller Context precedence and no post-success Context coupling.
- Added generation fencing and a single reconnect supervisor using the approved 1s/30s Full-Jitter exponential windows; retries only Dial failures/timeouts and unexpected post-Bound transport failures. Bind rejection/timeout are non-retryable. No Request replay.
- Every generation reuses the Client-owned resource ledger and callback dispatcher.
- Uni-directional Client behavior is requester-only and compatible with Pinex 1.5.0.
- Added Client Bind validation, connect/stop, timeout, rejection, reconnect-generation, uni-directional behavior and Bind-vs-Context race tests. Final Go and Pinex CI evidence is recorded below.


### Phase 6 Client Connect/Bind and reconnect completion evidence — 2026-11-17

- Report: [.codex/NETA_GO_PHASE_6_CLIENT_CONNECT_IMPLEMENTATION_2026-11-17.md](.codex/NETA_GO_PHASE_6_CLIENT_CONNECT_IMPLEMENTATION_2026-11-17.md).
- Owner decision: Option B, DECISION_LOG.md §56.
- Final verified source HEAD: 1484f26437cb509a56b377e0efe68ea32b175c3d.
- Go CI [44077921877](https://github.com/majiddarvishan/neta-go/actions/runs/44077921877) PASS: gofmt, vet, unit tests and race tests.
- Pinex Wire Compatibility [44077921981](https://github.com/majiddarvishan/neta-go/actions/runs/44077921981) PASS: pinned C++ 1.5.0 golden fixtures.
- Phase 6 P6-01..P6-11 are COMPLETE. Phase 7 live TCP interoperability, Phase 8 performance and Phase 9 release remain NOT AUTHORIZED.
