# QA-005-MEM-004B — Actual backing-buffer ownership overlay (2026-11-19)

**Status: IMPLEMENTED AS TEST-ONLY DIAGNOSTIC; NOT FULLY ATTRIBUTED MEMORY; awaiting actual Go overlay CI.**

Option B remains: no production source edits or new hot-path bookkeeping in normal builds. New external source overlay `scripts/qa005_owned_buffers_overlay.py` injects Session-mutex-protected accounting only into the benchmark's **temporary compile view** of four existing Go files. Original pinned QA004-QA005-SOURCE-V1 `9b7b2318f73d173a99d22489042f84f8ce58b745` is untouched; the script rejects output inside the checkout and hashes every original/effective overlaid source.

- F1: one outgoing accepted Stream frame backing slice, counted on enqueue, released on frame-charge release (writer or close), not on writer claim.
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

