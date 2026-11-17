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
