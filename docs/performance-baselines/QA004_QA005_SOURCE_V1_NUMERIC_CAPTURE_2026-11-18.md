# QA004-QA005 SOURCE-V1 — owner-local numeric capture (draft, not approved)

**Owner-uploaded archive (UTC 2026-11-18):** `source-v1-all-20261118T215013Z-2501287.tar.gz`.
**Archive SHA-256:** `fb1581c95ead3fa55c626646370637a4d1dd4b5cfd6b736fbdaa1c368427d036`.
**Frozen code source:** `QA004-QA005-SOURCE-V1` at exact SHA `9b7b2318f73d173a99d22489042f84f8ce58b745`; this is an owner-approved **source anchor**, not a numerical baseline approval.
**Review date:** 2026-11-19. **Capture overall exit status:** `FAIL` due to worktree cleanup failure **after** all benchmark runs. Benchmark measurements passed their individual run and matrix checks. Do not relabel the original full run as PASS.

## Evidence integrity

| Mode | Independent run logs | Selected scenarios / run | Rows | SHA-256 of ten ordered raw logs |
|---|---:|---:|---:|---|
| Core | 10 PASS | 8 | 80 | `da855ee0d78e058c2bfad0453c9ab63a490bb4dfad8c85a46de7a20df66e8ae0` |
| Full | 10 PASS | 24 | 240 | `2de902beb825ef82b02ea7374cf83eb4a5720772a73b5f9b274801c8e69126c1` |
| QA-004 | 10 PASS | 5 | 50 | `6197b8cc8182347e0067dd5c7831a5892ab971f369888bec9f574c84c2baac0c` |
| **Total** | **30 PASS** | — | **370** | — |

All three digests were independently recalculated from the supplied raw tar contents and matched both `statistics.json` and `DRAFT-NOT-APPROVED.json` per mode. Each row passed the exact scenario checks: 320 Core/Full benchmark rows had zero capacity rejections and zero owner-ledger post-workload retained/queued/request/OnClose counters; 50 QA-004 rows each had exactly 1,200 real internal terminal-settlement samples (**60,000** observations). The source SHA and benchmark fixture hashes matched the previously selected source anchor. Core and Full were captured separately; the eight Core cases appear again within Full and are *not* the same run.

Go toolchain `go1.27.0 linux/amd64`, Intel Core i5-12400, 12 logical CPUs, approximately 31 GiB physical RAM, fixed benchmark `-cpu=4`; Core/Full used 1s per benchmark, QA-004 used `1200x`. Core/Full harness digest `6a3adf62a50508f5c5aef8ac6e062176523d20e9fae95c11e225bf1e1e937172`; QA-004 harness digest `3ca4f929f3bd73004159d607ff02e60011dd2ac83ef190a3f932de3086f5d7bf`.

**Machine condition before → after:** CPU governor `powersave`; 1-minute load average ~1.70 → 1.66; CPU PSI some avg10 3.01% → 2.84%, I/O PSI some avg10 0.53% → 1.10%, `pswpin` +27, `pswpout` **+5,495**, `pgmajfault` +415. 7.4 GiB swap occupied at both points. These are host-wide counter deltas, not necessarily caused by the benchmark processes, and cannot independently prove swap thrashing; nonzero swap-out and rising I/O pressure do counsel caution for latency reference approval.

## Draft numeric dispersion (not SLA thresholds)

| Mode | Median of scenario P99 run-CVs | Worst scenario P99 run-CV | Scenarios with P99 CV above 10% |
|---|---:|---:|---:|
| Core (8) | 5.81% | 14.40% | 2 |
| Full (24) | 3.75% | 40.59% | 4 |
| QA-004 (5) | 2.50% | 4.11% | 0 |

QA-005 most unstable scenario: `Core/RR-65536B-S1-C4-near` when run inside Full, **P99 CV 40.59%** (median P99 **3.215 ms**); the same named scenario in a separate Core capture had CV 11.49% (median ~3.175 ms). `Core/NoWait-65536B-S1-C4-near` had 31.26% P99 CV within Full. This is a repeatability warning, **not evidence of a candidate regression**.

QA-004 synthetic Pending population median detection P99s: 32 → 226.035 µs, 128 → 228.317 µs, 512 → 351.829 µs, 2048 → 795.803 µs and 8192 → 1482.310 µs. Populations 2048/8192 are synthetic timer-scan fixtures, not admitted outstanding requests in one Session.

## Machine-readable advisory summaries

- [QA-005 traffic medians and P99 run dispersion](qa005-source-v1-draft-metrics-2026-11-18.csv): **32 rows** (8 Core + 24 Full). Source `statistics.json` values are rounded to the displayed integral nanosecond / operation-per-second units; the original archive holds full precision and all 10 raw logs per mode.
- [QA-004 deadline median P99 and dispersion](qa004-source-v1-draft-p99-2026-11-18.csv): five synthetic populations, 10 independent runs each, exact medians as captured.
- **The original owner-uploaded `.tar.gz` is the canonical raw log evidence and must be retained separately.** This repository records its SHA-256 and summarized data; the compressed binary archive is not added to GitHub.

## Cleanup error and approval boundary

The original collector output reported `capture_status=FAIL` and `cleanup_failure=/tmp/neta-numeric-ref.5S0ymlDB`. This means the script's `git worktree remove` returned nonzero; **the archive did not capture the command's stderr**. The owner subsequently ran `git worktree list` and `git -C /tmp/neta-numeric-ref.5S0ymlDB/source status --short --ignored`: this identified **only `?? scripts/__pycache__/`** in the detached source checkout. The Python `importlib` analysis of `qa005_compare.py` generates a bytecode cache there by default, and its untracked cache prevents normal `git worktree remove`. This independently explains the prior cleanup failure; the owner has not yet reported cleanup completion. It does not negate the independently validated benchmark rows or hashes. The collector's later fix (`505f3ca3a54d2890e3251e2319cf13b241421589`) logs Git's actual cleanup error and separates `measurement_status` from `worktree_cleanup_status`; it has not been used for this capture. Do not run destructive forced worktree removal without inspecting any remaining files.

**Numeric baseline: NOT APPROVED. Hardware: NOT APPROVED. Attributable retained-library memory: NOT MEASURED. Absolute SLAs: UNSET. Performance Gate: DISABLED.** Ten independent one-source captures are not the ten alternating baseline/candidate pairs required for a future code-regression qualification; the source-only Option A approval does not authorize a numeric baseline manifest or a release. QA-004/005 remain OPEN and QA-007 remains CLOSED.

**Follow-up script fix:** commit `68c861233a280cac3929c0d3db85e9d10dbe94cf` adds `PYTHONDONTWRITEBYTECODE=1` for all Python children and explicitly runs the comparator import with `python3 -B`. This avoids generating new bytecode cache in a fresh detached source checkout. The previous cleanup error remains accurately recorded as `FAIL`; no data or numeric baseline was changed and no expensive rerun was performed. The fix has not yet been verified in a new local capture. Existing leftover `scripts/__pycache__/` should be inspected and removed only from that known temporary worktree, then `git worktree remove` without force.
