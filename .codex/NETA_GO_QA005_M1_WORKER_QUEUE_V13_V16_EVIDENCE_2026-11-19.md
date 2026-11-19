# QA-005 MEM-004 Option B — Worker/Queue metadata evidence V13–V16
Date: 2026-11-19
Status: PARTIAL_INCONCLUSIVE / DIAGNOSTIC_ONLY
Repository: github.com/majiddarvishan/neta-go
Branch: feature/qa004-qa005-qa007-implementation-2026-11-18
Owner scope: B approved in .codex/DECISION_LOG.md §77
Immutable Source-V1: 9b7b2318f73d173a99d22489042f84f8ce58b745
Latest adapter: QA005-OWNED-BUFFER-OVERLAY-V16-UNQUALIFIED
Latest code commit: 4a133d83bbddc21ebd69ba231769039126c19033

## V13 — worker/queue lifetime, aliases and capacity

Code commit `40080abb5d753f36a5b8bc43b6731a120e3d265b`, [Actions 44524558066](https://github.com/majiddarvishan/neta-go/actions/runs/44524558066): all three jobs and all four optional stages individually PASS, including 15 Python owned-overlay tests, Go -race named lifecycle tests, pinned Source-V1 Go -race, strict adapter parity and 3-mode advisory overhead.

The test-only sampler acquires each `callbackWorkerPool.mu` one pool at a time, without nesting `Session.mu`. All Server and Client callback pools are inspected; multiple references to the same Pool are deduplicated, as are `callbackSessionQueue` identities and `callbackTask` backing **start** pointers. A backing pointer unexpectedly shared by distinct queues fails closed, as does an invalid Queue head. It separately reports:

- `probe-m1-worker-task-slots-logical-peak-B` = `cap(q.tasks) * unsafe.Sizeof(callbackTask{})`.
- `probe-m1-worker-queue-struct-logical-peak-B` = unique still-owned Queue lanes × `unsafe.Sizeof(callbackSessionQueue{})`.
- `probe-m1-worker-queue-peak-count`, `probe-m1-worker-live-peak-count` and `probe-m1-worker-queue-samples`.

These are **Go struct-layout logical sizes**, not physical heap allocation buckets. `q.head` advancing does not eliminate retained backing capacity; `q.tasks=nil` eliminates Task slice responsibility even if `q.active>0` retains its Queue struct. A synthetic -race test covers head advancement, duplicate Server/Client references, active callback, lane cleanup, attempted alias, corrupt head. Each independently sampled category peak cannot be combined to claim an instantaneous global memory peak.

## V14 — pin the worker_scheduler.go source dependency

Code `f82c86d17a961d281dba2cc950554064bbc3a91b`, [Actions 44525104589](https://github.com/majiddarvishan/neta-go/actions/runs/44525104589): three jobs and all optional stages PASS; 16 Python owned-overlay tests, 5 Python source-compatibility tests and Source-V1 Go -race. Because the V13 adapter assumes the scheduler's exact Queue semantics, `worker_scheduler.go` MUST be in both `source_file_sha256` and `effective_file_sha256` sets for baseline and candidate, even though the file is **not modified** by the overlay. The comparator rejects missing/changed SHA on either side.

In this pinned implementation the queue's slice start is not changed via reslicing to a different starting offset: it uses append, in-place compaction, prefix reslice and nil assignment. Start-pointer alias checks therefore match the reviewed source; a change to scheduler logic must fail source parity before attribution claims. Overlapping slices from different start offsets are NOT generally proven distinct beyond this exact-source contract.

## V15 — callbackWorker object layout, not goroutine memory

Code `813d7e1dc98602171abd5e7e2b0df03210c14f9b`, [Actions 44525522696](https://github.com/majiddarvishan/neta-go/actions/runs/44525522696): three jobs and all optional stages PASS, 17 Python owned-overlay tests, 5 Python adapter parity tests and pinned Go -race. Separately observes `probe-m1-worker-struct-logical-peak-B` = unique pool `workerCount * unsafe.Sizeof(callbackWorker{})`, with overflow validation. Two live workers in a 1x Core RR hosted-runner smoke corresponded to **96 B requested Go-struct layout**. Neither worker wake-channel buffers, idle timers, goroutine stacks, closure captures nor allocator size-class rounding is included. The repeated Client/Server owner reference is not an extra worker.

## V16 — actual blocked-worker lifetime proof

Code `4a133d83bbddc21ebd69ba231769039126c19033`, [Actions 44525968814](https://github.com/majiddarvishan/neta-go/actions/runs/44525968814): all three jobs and all four optional stages independently PASS; 18 anchored Python owned-overlay tests, 18 Go named lifecycle tests under `-race`, 5 Python source-compatibility tests and pinned baseline Go -race. The new real Worker Pool test blocks callback one, admits callback two into the queue, verifies nonzero Task backing, a live Queue lane and one Worker; releases callback one and observes callback two run, then confirms Queue/backing removal and Worker retirement after idle grace. This is an actual scheduler lifecycle test, not merely synthetic Queue construction.

The 1x benchmark reported two observed Worker objects (96 logical struct B), while Queue Task slot samples were zero over only two observations. **This does not prove that Queue storage was never allocated**: sampling may miss short-lived allocations. The hosted three-mode 25x overhead result remains `FEASIBILITY_ONLY_INCONCLUSIVE`, with gate `DISABLED`. No regression or overhead conclusion from single noisy samples.

## Explicit remaining coverage gaps

M1 still excludes actual Map buckets, Session-level queue metadata, dispatcher/worker timers and wake-channel buffers, closure/environment storage, goroutine stacks, runtime allocator size classes, some charged objects, incomplete Client-generation registry accounting, and other shared graphs. D1 only has a separate unique Panic Stack text diagnostic, not all diagnostic strings and records. Cross-pool samples are non-atomic and independent per-category highwaters cannot be summed. The static struct/slice lengths are not physical heap bytes. The 5ms sampler does not establish the maximum active-workload memory event without approved sampling gap and replay/overhead evidence.

No production source or source anchor was changed; historical 370 benchmark rows remain untouched. `retained-attributed-B` must not be emitted. MEM-004/MEM-005/MEM-006 and QA-004/005 are OPEN, QA-007 remains CLOSED. There is no new reference host, numeric baseline, SLA, mandatory performance gate, PR, merge, tag or release.
