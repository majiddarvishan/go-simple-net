# QA-005 MEM-004 M1/D1 — source-grounded ownership-scope audit
Date: 2026-11-19
Status: DESIGN / COVERAGE AUDIT; OWNER APPROVED SCOPE OPTION B ON 2026-11-19 — NOT A QUALIFIED BYTE METRIC
Branch: feature/qa004-qa005-qa007-implementation-2026-11-18
Immutable Source-V1: 9b7b2318f73d173a99d22489042f84f8ce58b745
Latest verified owned-buffer adapter: QA005-OWNED-BUFFER-OVERLAY-V9-UNQUALIFIED

## Purpose and hard boundary

The owner approved MEM-001 Option B: Hybrid Ownership Accounting. The V9
test-only adapter verifies F1–F5 buffer backing responsibility and per-Session
counter consistency; it **does not** have proof of actual neta-owned M1/D1
metadata bytes. The owner subsequently approved incremental **test-only** M1/D1 attribution under Option B; this audit is NOT a full coverage declaration, an absolute SLA or permission to enable a gate.

**Do not** use policy reservations, sizeof(map/slice header), `len(map)`,
`cap(slice)`, callback-credit ceilings or Go process heap/RSS as a substitute
for attributable live allocations. Retained-reference ownership and logical
quota are different concepts. A neta reference to a user-created graph does
not make the entire graph neta-owned for accounting.

## Source ownership inventory and alias/lifecycle risks

| Scope | Source origin | Native retention/transfer | What must be proven before byte attribution |
| --- | --- | --- | --- |
| M1a Session fixed object | `session_internal.go`, `types.go` | Client/Server Session creation; Server `sessions` map; callbacks may continue after `Done` | Exact owner identity and allocation lifetime, not just map presence or Session.Done; avoid counting same object from Server map and Client slice |
| M1b pending/request and charge objects | `Session.pending`, `pendingRequest`, `applicationReadCharge`, `completionReadCharge`, `onCloseEventCharge` | Pending settlement and callback completion may occur after forced close; charge transfer/release is independent of actual allocator size | Object allocation and final owner transitions, incl. canceled admission and callback after Done; no double count for references in closures and maps |
| M1c queue backing arrays and outboundFrame metadata | `appQueue`, `controlQueue`, `barrierQueue`, `outboundFrame` | Queue -> active writer, compaction, cancellation/Close; Go slices may retain backing capacity after logical removal | Backing-array identity, live capacity and sharing/reallocation, distinct from F1/F4 `item.bytes` backing; an empty Queue does not prove zero retained array capacity |
| M1d dispatcher/worker Tasks and closure environments | `callbackTask`, `sessionCallbackDispatcher` | Bounded owner callback worker queues; active callback can outlive Session.Done; captures may alias user objects | Neta-created Task/queue storage only, exclude external handler closure/referenced graph; avoid goroutine stacks and runtime scheduler memory |
| M1e Server identity maps and ownerResourceLedger | `server_lifecycle.go`, `types.go`, `resource_ledger.go` | `sessions`, `preBind`, `pendingIDs`, `boundIDs`, `sessionIDs`; reservations count policy bytes | Map-bucket/entry allocation provenance and alias cardinality; resource ledger's retained/queued bytes are quotas not physical memory |
| D1a diagnostic record and text | `diagnostics.go`: `PanicRecord`, `panicRecentEntry` | `recordCallbackPanic` stores `Stack`, `ValueType`, `ValueText`, `Callback` and structural records | Net-owned backing text allocations, lifetime, shared strings between First/OnClose/Recent, removal/eviction under `panicMu` |
| D1b raw Panic references | `firstRaw`, `onCloseRaw`, `panicRecentEntry.raw` | Original recovered `any` values can be retained; same object may be reachable from several slots and from user | No arbitrary graph traversal or multiplying the same raw object by slot count; treat opaque/foreign object graph as exclusion until a separately approved object-identity rule exists |
| D1c caller Diagnostics snapshot | `Session.Diagnostics()` | Allocates independent copied snapshot that is handed to user | Neta-owned duration until transfer only; post-return references are application-owned, never included in long-lived Session attribution |

Important source-specific exception: in `diagnostics.go`,
`callbackPanicByteBudget` controls recent *record-storage* bytes only.
`first` and `onClose` records and their `firstRaw` / `onCloseRaw`
references remain independent of that budget. `recentBytes` measures
selected text lengths, not underlying allocation size and not raw graph size.
The `PanicRecord` text values can be shared across First/OnClose/Recent.
`Diagnostics()` copies rendered records to an independent caller snapshot.

## Current evidence vs missing proof

- Existing F1–F5 allocation slices are charged by `cap([]byte)`, not
  physical allocator buckets or a runtime heap graph.
- V7 rejects Server Session admission churn even between samples; V9 checks
  observed category sum == the Session total, under the Session lock.
- `qa005OwnedPeakUpper` is a sum of independent per-Session event HWMs and
  **not** a simultaneous global retained peak. Sampling at 5 ms can return
  zero over a successful 1x workload; sample density is unqualified.
- All above M1 and D1 groups remain excluded from verified observed bytes,
  even if their **logical counts** are zero at the end of a run.
- A normal Core benchmark with no observed Panic does not establish that all
  M1 metadata allocations were absent. A fully empty live queue snapshot
  cannot prove its backing array was never allocated.
- Source-V1's original Go source, fixture, archived 370 rows and numeric
  baseline are untouched. The 3-mode MEM-005 25x smoke on a hosted runner
  only establishes feasibility, not overhead pass/fail.

## Bounded follow-up without owner-policy changes

1. Build test-only lifetime/alias fixtures for pending/queue/Task/diagnostic
   ownership (including Forced Close, active callback after Done and raw panic
   sharing). Record identities and release events, **not unapproved bytes**.
2. Define which neta-allocated structural objects can be counted without
   overlapping existing F1–F5 slices; separate logical object counts from
   byte estimates.
3. If coverage cannot be demonstrated for a requested case, retain
   `PARTIAL_INCONCLUSIVE` and do not emit `retained-attributed-B`.
4. After an owner-approved scope, investigate narrowly attributed metadata
   bytes with explicit ownership rules and A/A instrumentation-overhead
   evidence. No global high-frequency atomic counters.
5. Go 1.27 stable reference machine, 10 matched pairs, full matrix,
   approved maximum sampling gap, 30m soak and absolute SLAs remain separate.

## Scope decision reserved for owner

These are **proposals only**, not decisions:

- A — Keep M1/D1 excluded. Least overhead and robust partial diagnostic,
  but cannot claim full retained-attributed library memory.
- **B — APPROVED**: Incrementally instrument only provably neta-allocated M1 structures
  and D1 rendered record storage with alias-dedup and ownership transitions;
  keep foreign raw graphs / Go runtime storage excluded and coverage
  INCONCLUSIVE until the final metric contract is approved.
- C — Attempt full runtime/foreign object-graph attribution. Highest
  complexity and perturbation; likely conflicts with low overhead and
  source-only comparison unless independently authorized.

**Owner selected Option B (2026-11-19)**: incrementally prove neta-created
structural M1 and internally rendered D1 ownership, dedup aliases and
validate lifetimes via versioned test-only overlays; external raw graphs,
runtime overhead and unsupported byte estimates stay excluded. See
DECISION_LOG §77. Incomplete coverage remains PARTIAL_INCONCLUSIVE.

## Provenance

V8: [Actions 44489743062](https://github.com/majiddarvishan/neta-go/actions/runs/44489743062) —
all jobs and optional stages PASS. V9:
[Actions 44490084898](https://github.com/majiddarvishan/neta-go/actions/runs/44490084898) —
all jobs, 11 Python and 12 Go -race tests, pinned source compatibility
and 3-mode unqualified overhead smoke PASS. Earlier optional failures
[44488997407](https://github.com/majiddarvishan/neta-go/actions/runs/44488997407)
and [44489465368](https://github.com/majiddarvishan/neta-go/actions/runs/44489465368)
remain historical facts; top-level green did not mean optional success.

MEM-004/005/006 remain OPEN; QA-004/005 OPEN; QA-007 CLOSED;
performance gate DISABLED. No PR/merge/tag/release.

## Option B implemented first increments — V10, V11, V12 (2026-11-19)

**D1 stack text (V10):** `diagnostics.go` constructs `stack := string(debug.Stack())` per Panic record. The identical record (same `Sequence`) can live simultaneously in First/Recent/OnClose. The benchmark-only V10 observer freezes owner identities, locks `panicMu` independently and deduplicates record aliases using the Session-local `Sequence`, verifying a consistent Stack string. It counts `len(record.Stack)` once per still-reachable record; a raw panic value such as a 256KiB external graph is never counted. A separate, clearly labeled `probe-d1-unique-stack-text-peak-B` is the peak of these sample sums only. D1 ValueType/ValueText/Callback strings and `PanicRecord` metadata are **not** covered; stack string requested length is not an allocator bucket or physical heap size. Go -race test covers raw reference identity, First/Recent alias, OnClose/Recent alias, Recent eviction while First/OnClose remain, fake Sequence mismatch, and First independent of small Recent byte budget. [CI 44510555448](https://github.com/majiddarvishan/neta-go/actions/runs/44510555448): all 3 jobs + optional V10 overlay, pinned-source parity and overhead preview PASS.

**M1 pendingRequest identity (V11):** The accepted `SendRequest` stores a newly allocated `*pendingRequest` under Session.mu; its identity is then registered exactly once in a test-only `map[*pendingRequest]struct{}`. `releasePending` unregisters after Completion return/cancel, *not* when `s.pending` is replaced in `finishClose`. Therefore an active Completion can keep the object owned after Done. The map is measurement instrumentation and its own allocation is excluded from library bytes. The separate `probe-m1-pending-object-peak-count` is **object COUNT, not bytes**, and missing/duplicate registration is invalid. Deterministic test checks map removal before Completion release, correct identity retention and fail-closed transitions. [CI 44511176165](https://github.com/majiddarvishan/neta-go/actions/runs/44511176165): all 3 jobs and optional V11 overlay/source parity/overhead preview PASS.

**M1 pendingRequest requested struct footprint (V12):** After the V11 identity/lifecycle proof, a separate V12 diagnostic multiplies that unique owned count by `uint64(unsafe.Sizeof(pendingRequest{}))` with an explicit overflow guard; `unsafe` is imported **only into the test-only effective benchmark fixture**, not production. Unit coverage verifies footprint for one pending, zero after release, and overflow rejection. `probe-m1-pending-struct-logical-peak-B` reports **Go language-level struct layout footprint** (including inline padding), *not* physical allocation class bytes, map bucket allocations, Closure objects, captured application payload, or all of M1. In the hosted Go 1x Core RR smoke the reported snapshot for one pending was **40 B**; this is not a new SLA, baseline or portable constant. [CI 44511644311](https://github.com/majiddarvishan/neta-go/actions/runs/44511644311) PASS all 3 jobs, 14 Python overlay tests, Go -race lifecycle tests, 4 Python exact-source adapter parity tests, pinned Source-V1 Go -race and unqualified three-mode overhead preview.

All versions apply the **same** versioned test-only overlay to the pinned baseline and candidate and compare original/effective fixture SHA fingerprints. V12 still reports `covered_categories=[]`, `coverage_status=PARTIAL_INCONCLUSIVE`, and never `retained-attributed-B`. F1-F5, M1 and D1 samples use different lock windows; **do not add independent historical maxima** to claim a simultaneous library retained-memory peak. Hot-path test-only bookkeeping distorts work and has no approved overhead acceptance bound. The remaining M1 metadata (Session struct, queue/map capacities, dispatcher Tasks, charge metadata) and D1 rendered strings/records are unqualified. Foreign raw panic graphs and runtime/allocator remain excluded. Reference Go 1.27, 10 paired qualification, 30m soak, sampling reliability, exact complete metric definition and mandatory gate remain unapproved.

## M1 Worker Queue V13–V16 — audited partial coverage (2026-11-19)

Owner scope B produced independent [V13–V16 evidence](NETA_GO_QA005_M1_WORKER_QUEUE_V13_V16_EVIDENCE_2026-11-19.md), all three jobs/four optional steps PASS in [V13 CI 44524558066](https://github.com/majiddarvishan/neta-go/actions/runs/44524558066), [V14 CI 44525104589](https://github.com/majiddarvishan/neta-go/actions/runs/44525104589), [V15 CI 44525522696](https://github.com/majiddarvishan/neta-go/actions/runs/44525522696), [V16 CI 44525968814](https://github.com/majiddarvishan/neta-go/actions/runs/44525968814). Observed M1: unique live `callbackSessionQueue` struct footprint, Task slice backing logical `cap`-slots, live Worker count and `callbackWorker` struct layout. V14 adds exact unmodified `worker_scheduler.go` SHA parity to guard against change to the source's prefix-slice Queue semantics; V16 validates actual blocked callback/second queued callback, cleanup and idle worker retirement. All Go structure size metrics and peaks are diagnostic only, not physical allocator bytes or an instantaneous sum. Cross-offset slice overlap is not proven beyond pinned-source semantics; Client session admission churn and other M1 categories remain unqualified. D1 other strings and raw Panic graphs remain excluded. MEM-004/005/006 OPEN, no Performance Gate.
