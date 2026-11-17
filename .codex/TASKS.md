# neta-go — Canonical Implementation Task Tracker

**Repository:** `github.com/majiddarvishan/neta-go`  
**Branch:** `main`  
**Tracker owner:** Project owner  
**Last updated:** 2026-11-17  
**Baseline HEAD when this tracker was introduced:** `c98f731fef89519d62d95248982538f25a36db82`  
**Current phase:** Phase 4 — Resource Admission & Accounting (**AUTHORIZED / IN PROGRESS**)

This is the single, canonical, implementation-facing task list for the repository. Update the checkboxes in the same commit that completes a task or its acceptance evidence. Keep design decisions in `DECISION_LOG.md`; this file tracks implementation and verification.

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
| 4 | Resource admission and accounting | IN PROGRESS | **Authorized 2026-11-17** |
| 5 | Bounded callback Worker Scheduler | NOT STARTED | Not yet authorized |
| 6 | Client/Server orchestration and reconnect | NOT STARTED | Not yet authorized |
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

<badge color="warning">AUTHORIZED / IN PROGRESS</badge>

**Owner authorization:** 2026-11-17, this conversation. Scope is limited to the already-approved WORKER/Q-008/Q-009/Q-010/PERF-002/PERF-003 contracts. Do not change public API, defaults, wire behavior or frozen overload policies.

### Existing Phase 3 prerequisites

- [x] Per-Session outgoing Application FIFO byte and frame limits; the active Writer-owned frame remains charged until full Write or discard.
- [x] Separate bounded per-Session Control-PDU byte reserve.
- [x] Per-Session pending Request population is bounded and held through normal Completion callback return/cancellation.

### Implementation tasks

- [ ] **P4-01 — Reservation primitives:** implement race-safe, checked, idempotent reserve/release primitives for byte and count resources; reject overflow/underflow and make release exactly-once.
- [ ] **P4-02 — Owner ledgers:** implement independent Client/Server-owned counters for outstanding Request credits, retained Application bytes, retained Completion bytes and OnClose event credits. Client reconnect generations must share one owner ledger; independent owners must remain isolated.
- [ ] **P4-03 — Server aggregate queue ledger:** enforce combined incoming+outgoing Application queued-frame bytes (256 MiB default) and frame count (65,536 default) per Server; count a frame in exactly one aggregate queue bucket at a time.
- [ ] **P4-04 — Outgoing Application admission:** reserve per-Session queue bytes/count, retained Application bytes and applicable owner/Server budgets before frame allocation/copy or Request publication. Fail fast with the existing Queue Full/Capacity distinctions; do not consume a Sequence, publish Pending state or transmit rejected work.
- [ ] **P4-05 — Incoming Application admission:** after Header validation, reserve declared full-frame bytes/count and retained Application bytes before Body allocation/read. On local or Server aggregate overload, close only the offending Session; release reservations on read failure, callback cancellation and callback return.
- [ ] **P4-06 — Completion admission:** reserve matched Response retained bytes before Body allocation. On Completion-byte capacity exhaustion, drain exactly the declared Body through a fixed small scratch buffer under the original absolute Frame Assembly deadline; if drain succeeds and this Request still wins, deliver its normal Completion with no payload and `ErrCapacity`, keeping the Session open.
- [ ] **P4-07 — Request credits:** enforce per-Session and owner Request-credit ceilings; keep credit through response wait, terminal settlement, queued Completion and active Completion callback; release exactly once on callback return/cancellation or terminal path.
- [ ] **P4-08 — OnClose event credits:** make one nonblocking owner-scoped reservation after Done; permanently skip OnClose on budget exhaustion; release admitted event credit only after callback return/cancellation.
- [ ] **P4-09 — Lifecycle transfer/release:** prove queue-to-active transitions release queue-only charges while retained payload charges follow ownership; cover Writer completion, close, timeout, partial read, Forced Close A, callback cancellation and callbacks that outlive Session Done.
- [ ] **P4-10 — Admission verification:** exact-boundary and one-over tests, simultaneous multi-Session owner-budget tests, rollback and duplicate-release tests, scratch-drain tests, callback-retention tests and race-detector coverage.
- [ ] **P4-11 — CI acceptance:** gofmt, go vet ./..., go test ./... and go test -race ./... pass on the final Phase 4 source HEAD; update this tracker with commit and run links.

**Phase 4 exclusions:** no Worker Scheduler implementation (Phase 5), no Client Dial/Server Listen or reconnect orchestration (Phase 6), no live Pinex TCP interoperability campaign (Phase 7), no benchmark/pooling work (Phase 8), and no release/deployment (Phase 9).

## Phase 5 — Bounded callback Worker Scheduler

<badge color="secondary">NOT STARTED — NOT YET AUTHORIZED</badge>

- [ ] **P5-01 — Three isolated pools:** independent lazy Application, Completion and OnClose worker pools per Client/Server owner; no shared execution capacity or scheduler lock domain.
- [ ] **P5-02 — Active permits:** enforce owner-wide class ceilings and per-Session concurrency intersections; old Client generations retain charges until callbacks return.
- [ ] **P5-03 — Fair scheduling:** one reusable ready-Session node per class and FIFO round-robin dispatch, one callback per eligible Session turn.
- [ ] **P5-04 — Idle retirement:** reuse workers within the approved class-specific grace; finite 30s default, zero warm-idle floor, no retirement of running callbacks.
- [ ] **P5-05 — Forced Close A:** one linearization point between callback start and cancellation; queued Application/Completion callbacks cancel and release resources; started callbacks retain permits/credits until return.
- [ ] **P5-06 — OnClose policy:** best-effort/at-most-once after Done; event-budget exhaustion permanently skips the callback.
- [ ] **P5-07 — Isolation/liveness tests:** permanently blocked callback in one class must not consume another class or block Reader/Writer/Done; no goroutine-per-event.
- [ ] **P5-08 — Race and churn tests:** worker create/wake/retire/enqueue races, fairness across Sessions, blocked callbacks and retired Client generations.
- [ ] **P5-09 — CI acceptance:** gofmt, vet, unit and race CI pass.

## Phase 6 — Client and Server orchestration

<badge color="secondary">NOT STARTED — NOT YET AUTHORIZED</badge>

- [ ] **P6-01 — Client Connect:** Dial with absolute ConnectTimeout, Bind with absolute BindResponseTimeout and Context cancellation semantics.
- [ ] **P6-02 — Client lifecycle:** CurrentSession generation, terminal/idempotent Stop, disable reconnect on Stop and stale-generation fencing.
- [ ] **P6-03 — Reconnect supervisor:** opt-in AutoReconnect=false by default; approved bounded exponential Full Jitter policy; no Request replay/resend; share owner resource/worker ledgers across generations.
- [ ] **P6-04 — Server listener:** Start/Addr/Accept lifecycle and listener-close ordering.
- [ ] **P6-05 — Pre-Bind admission:** enforce pending socket count before Session allocation, absolute accept-to-Bind deadline and early refusal.
- [ ] **P6-06 — Bound Session registry:** enforce MaxBoundSessions without eviction, reject excess Bind with Pinex rfail where feasible.
- [ ] **P6-07 — Identity admission:** BindValidator and duplicate System ID rejection with atomic registry insertion/removal.
- [ ] **P6-08 — Server Stop:** terminal/idempotent Stop; close listener and Pre-Bind Sessions, concurrently Graceful-Unbind Bound Sessions, force remaining Sessions on caller Context expiry.
- [ ] **P6-09 — Owner integration:** wire shared Client/Server resource ledgers and worker owners into every Session generation.
- [ ] **P6-10 — Lifecycle tests:** simultaneous accepts, pre-bind floods, duplicate IDs, capacity boundaries, reconnect churn, stale generation races and graceful/forced Stop.
- [ ] **P6-11 — CI acceptance:** gofmt, vet, unit and race CI pass.

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

## Canonical references

- [Project handoff](HANDOFF.md)
- [Architecture freeze and implementation roadmap](NETA_GO_ARCHITECTURE_FREEZE_AND_IMPLEMENTATION_ROADMAP_2026-11-17.md)
- [Implementation authorization](IMPLEMENTATION_AUTHORIZATION_2026-11-17.md)
- [Resource admission design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md)
- [Worker capacity and resource accounting design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md)
- [Performance contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md)
- [Decision log](DECISION_LOG.md)
