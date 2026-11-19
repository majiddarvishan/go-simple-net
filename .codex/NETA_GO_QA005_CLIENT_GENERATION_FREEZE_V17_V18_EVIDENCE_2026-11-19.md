# QA-005 MEM-004 — Client generation ownership freeze, V17/V18
Date: 2026-11-19
Status: TEST-ONLY DIAGNOSTIC / PARTIAL_INCONCLUSIVE
Owner direction: M1/D1 Option B, DECISION_LOG §77
Branch: feature/qa004-qa005-qa007-implementation-2026-11-18
Pinned source: QA004-QA005-SOURCE-V1, `9b7b2318f73d173a99d22489042f84f8ce58b745`
Latest tested adapter: `QA005-OWNED-BUFFER-OVERLAY-V18-UNQUALIFIED`
V17 source commit: `c8cf04679956dbcdf5908b9d480db90f8b54eec4`
V17 test-only fix: `3fe01bb2d8fd83c1b9cd94b10a31c95e50d3b071`
V18 code commit: `ef117e38da1b2e2ca73b174d33ecaa902d21ba83`

## Problem: a disappearing Client Session generation

Before V17, the frozen ownership registry comprised the Server's live
Session map (plus Server admission epoch) and initial `benchmarkEnv.sessions`
Client Session list. A Client can enter a new connection generation via
`Client.connectAttempt` and later close it without any new Client Session
appearing in the next sample or in the static initial list.

The pinned `client_connect.go` increments `Client.generation` **under
`Client.mu` before Dial**. `clientSessionBound` installs `current`;
`clientSessionDone` may clear it after Done. Attempts can fail before a
new Session is constructed. This epoch is a conservative churn detector:
rejecting an attempt with no allocated Session is safer than accepting
a potentially incomplete ownership window.

## V17 — frozen Client registry, fail-closed

The test-only `qa005OwnedSessions` now freezes, under `Client.mu`, each
distinct Client's `generation` and `current` Session pointer. It appends
non-nil Client.current to the already deduplicated Session registry. Repeated
references to the same Client, including Server/Client pool aliases, do not
add another tracked owner. It refuses to validate the window if any Client
is stopped, a Connect is in progress (`connectDone != nil`) at freeze,
or generation already equals max uint64.

Each `qa005OwnedRegistryComplete` call independently checks frozen Client
identities and generation under their own mutexes, with no nested Client,
Server or Session locks. A transient generation admitted then removed between
sampling ticks is rejected because its generation counter was incremented,
even if `current` was restored to the original pointer. A changed
`current` is rejected even when the generation value is unchanged. An
in-progress Connect must not yield a valid pre-Connect or post-Connect
snapshot using the same frozen observation window. The invalidation also
applies to `qa005OwnedPeakUpper` and all sample-dependent M1/D1 diagnostics.

Both original and effective SHA maps now **require** exact
`client_connect.go` and `client_lifecycle.go` bytes for the pinned source
and candidate. Neither file is overlaid or modified. Unit tests mutate/miss
their SHA entries to verify rejection.

### V17 CI provenance

The first [Actions 44528814317](https://github.com/majiddarvishan/neta-go/actions/runs/44528814317)
was overall green yet the optional owned-overlay stage **FAIL**: a previously
valid Python assertion assumed `worker_scheduler.go` was the last item in
the SHA dependency list. Inserting two Client sources invalidated this test
anchor. The follow-up test-only commit `3fe01bb2` updated the exact
assertion; it did not modify Go production code.

[Actions 44529067306](https://github.com/majiddarvishan/neta-go/actions/runs/44529067306)
then PASSed **all three jobs and all four optional steps individually**:
19 Python owned-overlay tests, Go named lifecycle -race tests, 6 Python
exact-source compatibility tests, pinned Source-V1 Go -race and a three-mode
25x hosted overhead feasibility smoke. Overhead result remains
`FEASIBILITY_ONLY_INCONCLUSIVE`, not an overhead acceptance.

## V18 — real connected Client lifecycle proof

The V17 synthetic Client state tests verify transient generation (new and
removed before next sample), `current` pointer swap, and in-flight Connect
freeze rejection. V18 additionally tests the production Client lifecycle:
a `newBenchmarkEnv(t, 1, 1, 1, true, false)` creates a real TCP Server and
Bound Client; the observer freezes its owner registry; the original Client
Session is Forced Closed and Done completes; `Client.Connect(ctx)` then
creates a new genuinely Bound generation. The old observer must reject
`qa005OwnedRegistryComplete`, `qa005SampleOwnedBuffers`, and
`qa005OwnedPeakUpper`, regardless of the new Session's eventual map presence.

[Actions 44529571646](https://github.com/majiddarvishan/neta-go/actions/runs/44529571646)
PASSed all three jobs and all four optional stages individually:
20 Python owned-overlay tests; Go -race includes the real reconnect test;
6 Python SHA compatibility tests; pinned Source-V1 Go -race; three 25x
hosted short benchmarks. `FEASIBILITY_ONLY_INCONCLUSIVE` for overhead.

## What this DOES NOT prove

- Frozen Client generation correctness does not discover arbitrary *earlier*
  retired Client Sessions that existed and retained user callbacks **before**
  the benchmark window froze, unless their pointers were already in the
  tracked list. Existing QA-005 benchmark creates the initial bound
  Sessions before freeze, but general Client lifecycle scenarios still need
  an earlier-generation ownership/release contract.
- Client and Server snapshots are individually mutex-protected but
  sequential, **not a single global atomic heap snapshot**.
- A frozen test registry itself retains references; that diagnostic
  instrumentation can alter garbage-collection liveness. Its own memory
  and bias are not qualified or counted as library-owned bytes.
- This detects omitted Session generations; it does not attribute all
  neta-created Session/pending/queue/map objects or all D1 strings.
- Sampling every 5ms can miss short-lived memory peaks; per-Session
  independently acquired HWMs cannot be summed to claim a synchronous
  global retained-memory maximum. No source-to-candidate absolute
  memory SLA or numerical acceptance was issued.

The immutable source anchor, original library Go sources, baseline
fixture and archived 370 rows are unchanged. `covered_categories=[]`;
`coverage_status=PARTIAL_INCONCLUSIVE`; no `retained-attributed-B` is
emitted. **MEM-004/005/006 and QA-004/005 OPEN; QA-007 CLOSED;
Performance Gate DISABLED.** No PR, merge, tag or release.
