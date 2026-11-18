# QA-005-MEM-004B — Actual backing-buffer ownership overlay (2026-11-19)

**Status: V2 TEST-ONLY DIAGNOSTIC; real-source Go overlay CI PASS at 26aa2873, full attribution INCONCLUSIVE.**

Option B remains: no production source edits or new hot-path bookkeeping in normal builds. New external source overlay `scripts/qa005_owned_buffers_overlay.py` injects Session-mutex-protected accounting only into the benchmark's **temporary compile view** of four existing Go files. Original pinned QA004-QA005-SOURCE-V1 `9b7b2318f73d173a99d22489042f84f8ce58b745` is untouched; the script rejects output inside the checkout and hashes every original/effective overlaid source.

- F1: outgoing Stream frame backing slice now counted on allocation inside the V2 SendRequest/SendNoWait/SendResponse overlay, before Queue admission, and released once on write, Close or rejected-frame rollback. Queue/Writer transfer is non-additive.
- F2: one inbound Stream Request Body backing slice, counted immediately after `make`, released only when Application retention charge finishes (callback return or cancel).
- F3: one inbound matched Stream Response Body backing slice, counted immediately after `make`, released with Completion charge on return/cancel.
- F4: outgoing Control frame backing slice, counted on enqueue, released after write or cancel/close.
- Use `cap(buffer)` in bytes for these byte slices. Buffer capacity is a requested Go backing allocation size, **not** the runtime allocator's physical size.
- Existing Session mutex protects counters and frame/charge release flags. Bench-only observer walks Server live-session map under its mutex, unlocks, then snapshots each Session under its own mutex, plus all client Sessions. Never nest Server and Session mutexes. The global sampled sum may be skewed and miss fast peaks.
- Quota reservations, Incoming queue charges, current/external user slices, callback closure graphs, Pending/Worker metadata, Panic raw object graphs, transient unclassified Bind/Control read buffers, runtime overhead, goroutine stacks and GC are **not included**. No claim of total neta heap attribution.

The diagnostic metric is **`probe-owned-buffer-peak-B`**, with `probe-owned-buffer-samples` and earlier charge probe metrics. It is *never* `retained-attributed-B`; no mandatory acceptance decision may use it until category coverage, sample-window validity, density, overflow handling, deterministic lifecycle tests and instrumented-vs-uninstrumented overhead are validated. The generator is strict about expected source anchors; any changed code forces explicit re-review. Go unit tests and one-op smoke may be run in advisory CI only. The comparator and `qa005_qualify.sh` are unchanged.

Next: run synthetic Python tests and 1x real Go overlay under CI; inspect exact raw logs and adjust if needed; then deterministic QA-007 lifecycle/forced-close/active-callback tests via overlay, P99/CPU/allocation A/A probe, potential metadata coverage contract. Do not enable Performance Gate or issue PR/merge/tag/release.

## Verified diagnostic evidence (2026-11-19)

- [Actions 44420200998](https://github.com/majiddarvishan/neta-go/actions/runs/44420200998): all 3 jobs SUCCESS; 4 Python strict-patch tests PASS, 2 Go buffer lifecycle tests under `-race` PASS, actual 1x Core RR 64B overlay PASS.
- Sampling begins after the benchmark timer reset. Over 2 samples, observed F1–F4 peak **74 B** and independent per-Session event-HWM sum **212 B**. The higher sum is NOT the instantaneous global peak. Earlier [run 44419652396](https://github.com/majiddarvishan/neta-go/actions/runs/44419652396) observed **0 B** over 2 samples for a successful operation: the sampler can miss short-lived buffers.
- Measurement coverage remains partial and unqualified. Specifically, actual F1 outgoing bytes between frame build and Queue admission are not tracked, nor are metadata or external retained graphs. Test-only source overlays are fully separate from normal production builds; source-V1 anchor and API/wire remain fixed. `retained-attributed-B` is NEVER emitted by this probe.
- The CI owned-buffer step is `continue-on-error`; check its own step log, not only overall workflow conclusion. Recorded initial fixture compile failure on 40f8a5a3 and workflow YAML failure on 40016b5e, both fixed by dfb33283.


## MEM-004C V2 correction and verified evidence (2026-11-19)

Versioned test-only adapter: **QA005-OWNED-BUFFER-OVERLAY-V2-UNQUALIFIED**. Original session.go is now included in source/effective SHA-256 manifests. Allocation of outbound F1 happens under Session lock and is observed immediately by the diagnostic counters before copying and Queue admission. The overlay-only qa005Enqueued flag distinguishes an already-owned frame transfer from duplicate Queue admission; Go source, public API and source-V1 anchor are untouched.

- [Actions 44422549332](https://github.com/majiddarvishan/neta-go/actions/runs/44422549332) at 50abb663: overall green, optional owned-buffer 1x smoke **FAIL** because already-owned frames were flagged as duplicates.
- [Actions 44423060164](https://github.com/majiddarvishan/neta-go/actions/runs/44423060164) at ae9f78b6: overall green, optional step **FAIL** during Python import because a new Go test was outside the generated fixture string.
- [Actions 44423378544](https://github.com/majiddarvishan/neta-go/actions/runs/44423378544) at 26aa2873: all jobs and the optional owned step PASS (5 Python; 3 Go -race; real 1x Core RR). Sampled peak **0 B**, Session event-HWM upper **212 B**, samples **2**. These figures are diagnostic and incomparable with exact global attributable retained memory.

**Still open:** concurrent builds/rejected Send under Forced Close; active callbacks after Done; OnClose/Panic external references; pending/worker metadata; F4 pre-admission and unclassified inbound allocations; Session removal from dynamic registry; sample gaps and exact peak semantics; 10-pair identical effective adapter; instrumentation ON/OFF overhead. A missing short peak must not become an approved zero. MEM-004 remains OPEN; MEM-005/006 OPEN; retained-attributed-B is never emitted.

## MEM-004D — Frozen identity set and callback persistence (2026-11-19)

At exact SHA `58e655a79fe61084179b82ed1b42882bc02cc8a2`, [Actions 44424670741](https://github.com/majiddarvishan/neta-go/actions/runs/44424670741) passed all three jobs plus the individually verified optional owned-buffer overlay stage: five Python tests, seven Go lifecycle tests under `-race`, actual 1x Request/Response smoke. The original Go source and source-V1 anchor are unchanged. Generator adapter version is **QA005-OWNED-BUFFER-OVERLAY-V3-UNQUALIFIED**; its effective fixture SHA differs from V2, and any future paired comparison must use matching adapter bytes on baseline and candidate.

V3 records Session identity pointers at the benchmark traffic-window start, rather than rebuilding the current Server Session set for each sample. A Server Session removed from `server.sessions` after Close remains in the frozen list while active Application/Completion callbacks can still own F2/F3. Any newly present but unregistered Server Session invalidates the sample and final upper diagnostic; no fabricated zero. `TestQA005OwnedOverlayFrozenRegistry` checks removal, attribution, new-owner rejection and final release. The observer still serially snapshots individual Sessions, so global values are not atomic or precise event peaks.

The latest 1x diagnostic showed **0 B** sampled maximum, **212 B** sum of independent per-Session high-watermarks, **2 samples**. This is not a verified absolute memory quantity. The remaining important exclusions are unclassified inbound and control-build transient allocations, pending/worker/queue metadata and diagnostic strings/foreign raw graph references, as well as benchmark-only sampling skew and missing gap/overhead qualification. Mark **INCONCLUSIVE**; never emit `retained-attributed-B` from this adapter.
