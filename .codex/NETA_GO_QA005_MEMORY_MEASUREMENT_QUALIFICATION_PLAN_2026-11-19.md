# QA-005-MEM-003 — Paired attributable-memory measurement and source-V1 compatibility plan
Date: 2026-11-19
Status: **V9 DIAGNOSTIC TOOLING AND PINNED-SOURCE ADAPTER COMPATIBILITY VERIFIED; MEM-004/005/006 QUALIFICATION INCONCLUSIVE**
Owner decision: QA-005-MEM-001 Option B (Hybrid Ownership Accounting).
Owned category definitions: .codex/NETA_GO_QA005_ATTRIBUTABLE_MEMORY_OWNERSHIP_DESIGN_2026-11-19.md
Immutable source anchor: QA004-QA005-SOURCE-V1 / 9b7b2318f73d173a99d22489042f84f8ce58b745

## 1. Actual runner/comparator preflight (audited)
- `scripts/qa005_qualify.sh` lines 40–67 reject modified tracked source, untracked Go source, missing fixtures and differing `benchmark_test.go` / `qa005_qualification_benchmark_test.go`; QA-004 checks its separate detection fixture. The script creates 10 alternated pairs, runs `go test -bench ... -cpu=4`, and emits per-side environment metadata with `harness_sha256` (lines 68–120).
- `scripts/qa005_compare.py` reads `retained-attributed-B` from benchmark logs (line 136), refuses missing, zero, invalid or noncomparable measurements, requires matching environment plus owner-approved evidence, and implements 95%-confidence Bonferroni-adjusted paired decisions for the already-approved 15% retained metric. Baseline post-run counters/heap-live are deliberately not a substitute.
- `scripts/qa005_baseline_capture.sh` pins a detached worktree to source-V1, records 10 independent *one-source* samples per mode and produces an **unapproved** draft. Do not rerun the prior 370 rows merely to design memory instrumentation.
- Earlier `source-v1-all-20261118T215013Z-2501287.tar.gz` measurements are immutable *non-memory* evidence. They contain **no** `retained-attributed-B` values. Archival SHA256 remains `fb1581c95ead3fa55c626646370637a4d1dd4b5cfd6b736fbdaa1c368427d036`. The original cleanup FAIL remains historical fact.
- The pinned SHA and original tracked benchmark fixture bytes must not be overwritten or silently redefined. There is no justified comparison with historical pre-fixture `main`.

## 2. Instrumentation compatibility: proposed design, not a bypass
**Hard requirement:** baseline and candidate must execute a semantically identical **versioned measurement adapter** and fixture, while their actual production sources remain their recorded exact SHA. No empty missing-value substitution, code drift or silent patch to baseline.

Proposed implementation route to prototype first (NOT verified): `go test -overlay` with a generated, read-only overlay mapping existing Go test files to a versioned measurement-enabled test fixture in an external temporary directory. A single adapter version and same effective fixture bytes must be applied to **both** detached checkouts. If internals require differing integration, prove equivalent semantics through a contract test; otherwise this method is INCONCLUSIVE. Do not assume `-overlay` can arbitrarily inject a brand-new package or source file; verify the actual toolchain behavior first.

**Runner design contract before any execution:**
1. Verify repository identity, exact baseline SHA, candidate SHA, clean tracked trees, zero untracked Go sources, expected scenario names, Go version, benchmark duration, `-cpu=4`, OS/arch/hardware/host and unchanged source fixture digests.
2. Generate the *same* effective instrumented benchmark fixture on each side without changing tracked source trees. Store an adapter-version identifier, SHA-256 of every effective overlaid file, generated overlay JSON SHA-256 and actual compiler command/flags. Reject if baseline/candidate measurement contracts or semantics differ.
3. Record two distinct fingerprints: **source fixture** digest (the original `harness_sha256`, unchanged) and **effective measurement fixture** digest (including overlays). Refuse a comparison if either differs unexpectedly; never make a changed effective test fixture look unchanged by hashing only tracked source.
4. Make the wrapper run `qa005_qualify.sh`'s 10 alternating pairs only after the extra overlay/provenance preflight is implemented and validated. A replacement wrapper must preserve existing matrix, provenance and error checks; no use of `--approved-manifest` until a separate reviewed numerical baseline manifest exists.
5. Generate cache/temp files outside the detached worktrees, set `PYTHONDONTWRITEBYTECODE=1`, and use ordinary non-force worktree removal with diagnostics on failure. Do not edit the pinned source checkout, `git stash` owner changes or delete unknown files.

**Fallback if the overlay cannot be proven compatible:** mark memory qualification INCONCLUSIVE and request a reviewed adapter/harness design. Do not mutate source-V1 or turn source-only approval into a new pinned baseline implicitly.

## 3. Measurement windows, sample integrity and peak semantics
- For each QA-005 case measure only the active `b.ResetTimer()` -> `b.StopTimer()` traffic window; set up the benchmark and Session/Client population before observation, stop the sampler after producers have completed and before destructive cleanup. Report setup/teardown separately if measuring lifetime overhead.
- Produce a per-run monotonic series of `{since_start_ns, live_owned_B, category_bytes, owner_count, accounting_health}`. Observe in the benchmark test process, not via host RSS. No new continuous sampling in the library.
- Under B, prefer an exact per-owner *tracked charge* high-watermark updated with its already-held ledger mutex when the overhead test proves acceptable. This is a **policy-charge peak**; do not claim that the reserved charge equals actual backing allocations or that different owners' peaks were simultaneous.
- For attributable owned allocations, sample known live backing storage and separately defined metadata in a benchmark-only observer; hold each lock briefly and never across user callback, socket I/O or another owner lock. Record per-sample acquisition duration, lag/maximum gap and aggregation timestamps. The owner+dispatcher aggregate may be temporally skewed under concurrent updates. Report `sampled_global_peak_B` as observed, never exact event-global peak.
- Capture sample P50/P95/P99 **across time**, using an explicit equal-time or time-weighted distribution and separately declaring which; record mean/time-weighted mean, sample count, max gap and sampling interval. These percentiles are not operation latency P99s.
- Proposed candidate `retained-attributed-B` for eventual QA-005 comparison is the observed **in-window sampled peak of covered owned categories**; it is a lower bound on an instantaneous peak, not process heap and not a policy quota. Do not emit this canonical comparator field before the coverage, max-gap and A/A stability conditions are accepted. Especially at low traffic, a sampler may repeatedly observe zero; zero must remain INCONCLUSIVE under existing comparator until a separately approved meaningful absolute-delta policy exists.
- Provide **upper-bound diagnostic** `sum_owner_charge_HWM_B` only as an explicitly labelled upper bound on *charged* application/completion peaks, never as the observed global attributable heap peak. A precise global event peak would require stronger coordination and its hot-path overhead is not implicitly authorized.
- A measurement may include physically retained allocations not yet reclaimed by GC after references are dropped. Define 'owned' by live library responsibility, not whether the Go runtime has collected the object. Exclude post-handoff application-retained slices and external graph sizes; add a coverage annotation instead of imputing zero.

## 4. Fail-closed validity checks before any memory PASS
- Every selected 8 Core / 24 Full scenario emits an attributable value **only if** its owner/category coverage satisfies the reviewed scope; report `MEMORY_UNQUALIFIED` separately where it does not.
- Owner and Session charge totals reconcile where expected; any `resourceAccountingFailed`, underflow, duplicate allocation identity, unbalanced responsibility, missing sample window, drop/gap beyond approved bounds, sampler crash, unregistered owner or deadlocked snapshot invalidates the case.
- Every admitted operation delivered, capacity rejection zero, no nonzero terminal ledger usage. Keep separate `post-*` counters for existing checks.
- Memory adapter is identical and provenance verifiable; baseline/candidate environment metadata exactly comparable; no postprocessing that changes archived raw logs.
- Failure to observe a short-lived peak is not automatically a lack of retained memory. Use controlled deterministic slow-writer/blocked callback fixtures to validate sampler visibility and exact charge transitions, but run these separately from unchanged QA-005 traffic fixtures.
- Panic raw arbitrary reference graphs, user closure captures and Go runtime/kernel allocations remain out of direct core measure. If they are present in a scenario and affect the approved coverage, label INCONCLUSIVE; never count arbitrary process-wide heap as library-owned bytes.
- Do not claim that an observed A/A relative PASS, or a nonzero process heap sample, qualifies the owner machine. CPU governor, swapping, PSI and P99 spread remain explicit environmental risks.

## 5. Minimum implementation and test work (not yet authorized as complete)
### MEM-004A — Adapter feasibility and provenance, no long benchmarks
- Prototype `go test -overlay` with the pinned source and a same-source A/A *compilation/unit* check; confirm overlay is actually compiled, both fixture fingerprints are captured, and checked-out trees remain clean. This is a feasibility experiment, not performance evidence.
- Verify comparator rejects mismatched measurement adapters, missing/zero/NaN values, empty observation windows, invalid category coverage and unapproved manifests. Existing comparator remains unchanged until test-only adapter preflight exists.

### MEM-004B — Ownership and concurrency proof
- Unit fixtures: outbound queue -> active write, two inbound callback lifetimes, matched Response Completion, errored Body read, late Response, pending timeout, canceled Admission, Forced Close and active callback after Done, OnClose transfer and panic, and lazy diagnostics ring with First/Recent sharing.
- For each fixture, show no double count on transfer and no negative/fake zero on actual retained ownership. Exercise -race and bounded worker pools. Do not invent new release behavior.

### MEM-005 — Performance qualification of instrumentation
- Same-source overlay ON/OFF A/A for Core 8 first, fixed 10 matched pairs (or a short feasibility-only set clearly labelled, not as acceptance); record performance/CPU/op/P99/allocs and sample acquisition time. Full 24 and 30m stability **only when useful and authorized** after preliminary evidence.
- For any measurable distortion, reduce observation scope/frequency or mark INCONCLUSIVE; never secretly subtract an estimated instrumentation cost from a measured metric.
- Only after the exact memory field definition, exclusions/coverage, sample-gap limits and reference environment are separately reviewed may the 15% tolerance be applied as a *qualification* input. Absolute SLA and mandatory gate require additional owner approval.

## 6. Evidence artifact schema sketch (not active)
```json
{
  "memory_schema_version": "QA005-ATTRIBUTION-V1-PROPOSED",
  "source_anchor_sha": "9b7b2318f73d173a99d22489042f84f8ce58b745",
  "source_fixture_sha256": "from exact source",
  "effective_fixture_sha256": "from effective compiled fixture",
  "measurement_adapter_sha256": "exact adapter bytes",
  "owner_approved_memory_method": false,
  "performance_gate": "DISABLED",
  "scenarios": [{
    "name": "exact QA005 case",
    "window_start_ns": 0,
    "window_end_ns": 0,
    "sampled_global_peak_B": null,
    "retained_attributed_B": null,
    "covered_categories": [],
    "excluded_categories": [],
    "sample_count": 0,
    "max_sample_gap_ns": null,
    "accounting_reconciled": false,
    "measurement_valid": false
  }]
}
```
The specimen deliberately has no numeric memory reading, no asserted measurement and no approval. Real evidence must contain matched run-pair identities, env records, ordered raw-log digests, per-case sample series and schema version. Never mark any null as zero or PASS.

## 7. Decision boundary and status
MEM-003 design documented. MEM-004 final **definition and coverage for the comparator memory metric**, snapshot feasibility, overlay behavior and quantitative sampling bounds still require proof/review. MEM-005 production changes and test/benchmark evidence NOT EXECUTED. Source SHA and old archive unchanged; no new numerical baseline, no absolute SLA, no performance gate; QA-004/005 OPEN and QA-007 CLOSED.

## 8. Initial read-only overlay prototype — built 2026-11-19 (diagnostic only)

`scripts/qa005_memory_overlay.py` constructs a Go test overlay **outside** a selected checkout, leaving tracked fixtures unchanged. It inserts sampling into the two existing traffic benchmark loops and records only **`probe-owner-charge-peak-B`** and **`probe-owner-charge-samples`** when `NETA_QA005_MEMORY_PROBE=1`. This is policy-reservation sampling, not unique backing buffer attribution; it does not emit `retained-attributed-B`, never enables a performance gate, and does not modify the approved numerical manifest.

Feasibility check run in the assistant's isolated container: `PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest -v scripts/test_qa005_memory_overlay.py` using a toy Go test module, both Python tests and toy `go test -overlay` compilation/1x benchmark passed on Go 1.23.2/linux-amd64. It is **not** evidence that the patch matches the actual source-V1 fixture under Go 1.27; this verification and the actual-byte method remain open. The generator intentionally does **not** inject into `qa005_qualify.sh`; its existing strict preflight/gate is unchanged. 

Proposed exact-source short feasibility procedure *after source checkout and toolchain are available*: run generator with `--source /path/to/exact-sha-checkout --output /tmp/unique-adapter-dir`, verify manifest original fixture SHA against the pinned source, compile with `go test -overlay /tmp/unique-adapter-dir/overlay.json -run '^$' -bench '^BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium$' -benchtime=1x -cpu=4` and optional environment `NETA_QA005_MEMORY_PROBE=1`. This is a short smoke, **NOT** the 10-pair 95% qualification or 30m soak. Only proceed with real attributed metric after separately validated ownership and overhead.

## 9. Confirmed short-source-overlay feasibility — GitHub Actions 44417224436 (2026-11-19)

- Provenance: feature source exact SHA `1a9c9bb80538d3a44ea1dbfd4373fd2424f131e7`; all 3 jobs PASS on [run 44417224436](https://github.com/majiddarvishan/neta-go/actions/runs/44417224436). The added optional step uses Go 1.23.x, not owner Go 1.27.0.
- Synthetic adapter unit tests: **2/2 PASS**. Actual repo fixture overlay compiles; one Core RR 64B 1x smoke PASS. Source fixture SHA-256: `6a3adf62a50508f5c5aef8ac6e062176523d20e9fae95c11e225bf1e1e937172`; effective fixture SHA-256: `ec0b0f521e5d6f80122da323d433031d8c31b6c5a615388d67a975c9303eb42e`. The original source fixture digest matches the pre-existing QA005 fixture digest.
- Captured `probe-owner-charge-peak-B=74`, `probe-owner-charge-samples=2`; these are **sampled reserved Application+Completion policy charges**. No claim is made for attributable allocated/live bytes or true peak. The one-iteration sample cannot support statistical reliability or overhead evaluation. No `retained-attributed-B` emitted; comparator remains INCONCLUSIVE for memory.
- An earlier workflow YAML construction mistake produced [run 44416997076](https://github.com/majiddarvishan/neta-go/actions/runs/44416997076) FAIL with zero jobs. Fixed by rebuilding YAML from valid prior version in `1a9c9bb8`; corrected actual workflow PASS above. Failure evidence is retained.
- Outstanding: actual owned-buffer/metadata categories, deduplication and coverage, sampler time distribution/maximum gap, A/A instrumentation cost, Go 1.27 reference-host method, numerical baseline/absolute SLA/Performance Gate approval. No prior 370 results rerun for a new baseline and no 30-minute soak requested.

## 10. Real owned-buffer probe results and peak bias (2026-11-19)

At `dfb3328379d0481dabe097bbdb0527699cdf4c1b`, [Actions 44419293434](https://github.com/majiddarvishan/neta-go/actions/runs/44419293434) passed all normal jobs and the optional overlay smoke. The Python strict-anchor checks (4 tests), two generated Go lifecycle tests with `-race`, and 1x real request/response PASS. Observed **sampled F1–F4 peak=74 B, only 2 samples**. An earlier optional step at SHA 40f8a5a3 had a Go fixture compile failure despite overall workflow success because it is `continue-on-error`; this was fixed in 40016b5e. A subsequent replacement of a regex corrupted the CI YAML, making [44419161576](https://github.com/majiddarvishan/neta-go/actions/runs/44419161576) FAIL before jobs; workflow restored in dfb33283. Both failure records remain preserved; do not call them PASS.

At `9f77efcba5b87f4a908beec347c3b1734d14dc4c`, [Actions 44419652396](https://github.com/majiddarvishan/neta-go/actions/runs/44419652396) PASS shows **in-window sampled peak=0 B** over 2 observations for one successful 64 B operation after moving probe start to *after* `b.ResetTimer()`. This is strong evidence that a 5 ms sampler can miss all sub-millisecond ownership peaks. Zero is **NOT** evidence of zero real retained memory and must never qualify a baseline.

At `ad0ff9eef114b7ec44aa0ecf25d1eb678b1b097d`, [Actions 44420200998](https://github.com/majiddarvishan/neta-go/actions/runs/44420200998) **PASS all three jobs including both optional overlay steps**. The owned-buffer overlay keeps a **per-Session live-byte total and high-watermark** under its pre-existing Session mutex and resets the HWM at the beginning of measured traffic; it does NOT introduce a production hook. A 1x Core RR 64 B run reports `probe-owned-buffer-peak-B=74`, `probe-owned-buffer-samples=2`, `probe-owned-session-hwm-upper-B=212`. Per-Session maxima may occur at different times; their sum is only an **upper bound over these tracked Sessions**, not an observed concurrent global peak. The 74 B peak is merely an observed sample. Neither is approved `retained-attributed-B`. The adapter still excludes pending/worker metadata, external closures, panic-raw object graphs, runtime overhead, pre-enqueue in-flight outgoing allocations, and unclassified inbound bodies. Sampling interval/gap and overhead need independent evidence. A real ten-pair comparison or absolute SLA has NOT been performed.

**Pinned source compatibility check:** `benchmark_test.go`, `qa005_qualification_benchmark_test.go`, `session_internal.go`, `session_transport.go`, `resource_charges.go` have identical GitHub blob SHAs between immutable `QA004-QA005-SOURCE-V1` (`9b7b2318f73d173a99d22489042f84f8ce58b745`) and current feature code. The source anchor itself was not changed. This is source comparability evidence only, not owner-approved numerical baseline. No mandatory gate, PR, merge, tag or release.


## 11. MEM-005 minimal source/overlay/observer A/A feasibility (2026-11-19)

`scripts/qa005_overhead_advisory.py` is a strict, test-only parser of three same-source Go benchmark raw logs: unoverlaid `NETA_QA005_MEMORY_PROBE=0`, V7 overlaid with probe OFF, and the same V7 overlay with the 5ms sampler ON. Each runs the exact `BenchmarkQA005Qualification/Core/RR-64B-S1-C1-medium`, `-cpu=4`, `-benchtime=25x`, `-count=1`, `-benchmem` in one CI run. The parser checks adapter provenance, original/effective fixture hashes, source SHA, Go version, exact iteration and CPU identities, logs' SHA-256 and partially unqualified coverage. It reports raw metrics and illustrative ratios but has **no threshold and no approval path**. Its status is `FEASIBILITY_ONLY_INCONCLUSIVE` and Performance Gate DISABLED.

[Actions 44486575046](https://github.com/majiddarvishan/neta-go/actions/runs/44486575046) initially had a malformed YAML / zero jobs caused by text-substitution interpretation of a literal shell `$'`; fixed by rebuilding from a known-good workflow and preserving the dollar signs using a replacement callback (commit `fbadb551`). [Actions 44486737303](https://github.com/majiddarvishan/neta-go/actions/runs/44486737303) passed all three jobs and all optional stages, including 5 Python parser tests and the actual three-way 25-operation microprobe: native 37486 ns/op, instrumented with sampler OFF 41786 ns/op, with sampler ON 35932 ns/op. The apparent improvement ON vs OFF is **noise, not evidence of negative overhead**. No stable reference machine, paired 10x A/A, confidence intervals, approved sample-gap limit, Core 8/Full 24 qualification, 30m soak, numerical memory baseline or SLA is provided or inferred. Sampler-OFF still executes overlay bookkeeping; native-vs-OFF includes fixture changes and accounting. Therefore neither contrast isolates all possible runtime differences; further instrument design review is needed before an overhead verdict. MEM-005 OPEN.

## 12. MEM-004 V8/V9 evidence and pending M1/D1 contract (2026-11-19)

Same anchored read-only Go source overlay V9 is now verified in [Actions 44490084898](https://github.com/majiddarvishan/neta-go/actions/runs/44490084898) with 11 Python anchored tests, 12 Go lifecycle tests under -race, 4 Python pinned-source comparison tests, pinned Go -race and 5 Python overhead preview parser tests; all optional stages independently PASS. V8 F5 counts **unreserved inbound Bind/non-drained Body backing bytes only** from allocation to terminal read failure or handler return. V9 rejects mismatch of five observed byte counters vs the same Session total for both snapshot and HWM. Historical optional failure runs [44488997407](https://github.com/majiddarvishan/neta-go/actions/runs/44488997407) and [44489465368](https://github.com/majiddarvishan/neta-go/actions/runs/44489465368) were corrected by independent test-only commits. No production source or baseline measurement was modified. The memory comparator must still mark INCONCLUSIVE; total coverage and synchronous global peak remain missing.

For M1/D1 next, first agree on separately attributable *objects and alias owners*, not the apparent process heap. Queue/pending maps and dispatcher Task records are variable-runtime structures; counting policy credits/queue reservations is insufficient. Panic/raw diagnostic data can refer to user-owned object graphs and strings shared through First/Recent; arbitrary graph traversal is invalid. A future metadata test adapter should explicitly validate object lifetime and alias avoidance or exclude these bytes and retain INCONCLUSIVE. Do not silently add arbitrary `unsafe.Sizeof` or arbitrary runtime `Alloc` deltas to F1–F5. MEM-005 overhead confidence intervals, stable owner reference Go 1.27, sample-gap approval, scenarios and 30-minute soak remain NOT DONE.

## 13. M1/D1 Option B — first diagnostic observations, no metric qualification (2026-11-19)

Owner scope Option B approved in DECISION_LOG §77. Adapter V10 adds *sampled logical length of still-retained neta-created Panic Stack strings*, deduped by record Sequence across First/OnClose/Recent under panicMu. V11 adds a test-only Session-mutex-protected set of pendingRequest allocation identities from accepted SendRequest through Completion return/cancel, regardless of Done/map reset. V12 gives just those objects a separate requested Go struct-footprint estimate via `unsafe.Sizeof(pendingRequest{})` in the benchmark-only overlay, overflow checked. This does **not** include map allocations, closure/environment graphs, runtime allocation size classes or other M1. All remain distinct from F1-F5; do not sum non-simultaneous sampled peaks. [V10 CI 44510555448](https://github.com/majiddarvishan/neta-go/actions/runs/44510555448), [V11 CI 44511176165](https://github.com/majiddarvishan/neta-go/actions/runs/44511176165), [V12 CI 44511644311](https://github.com/majiddarvishan/neta-go/actions/runs/44511644311) passed all 3 jobs and optional owned overlay, pinned Source-V1 compatibility and short unqualified three-mode overhead. The one pending object in V12 smoke showed 40 B of logical struct footprint; zero D1 Stack bytes in a no-Panic Core benchmark is not full D1 zero. All measurement statuses remain PARTIAL_INCONCLUSIVE / FEASIBILITY_ONLY_INCONCLUSIVE. No owner reference Go 1.27 acceptance, paired 10-run qualification, full matrix validation, 30m soak, max sampling gap policy or mandatory gate.
