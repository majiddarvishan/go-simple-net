# neta-go QA-004 — Pending Request Deadline Scan Measurement

**Date:** 2026-11-18  
**Source:** PR [#16](https://github.com/majiddarvishan/neta-go/pull/16), benchmark commit `72fd61e5809f7b5cbea7737cefb8f31cfd770811`.  
**CI measurement:** [Go run 44209837908](https://github.com/majiddarvishan/neta-go/actions/runs/44209837908), Ubuntu 24.04, Go 1.23.x, PASS.

## Measurement method

The new `BenchmarkPendingDeadlineScan` uses a synthetic Bound Session and exercises the production `Session.nextDeadlineLocked()` under `Session.mu`. It fills the map with non-expired Pending Requests, disables outgoing Enquire Link probes, and repeats the scan until the benchmark timer stops. Each population is run 3 times at `GOMAXPROCS=1` and `GOMAXPROCS=4` with `-benchtime=200ms -benchmem`.

The benchmark isolates the scan and mutex, **not** full TCP Request/Response throughput, callback execution, timeout settlement, enqueue paths, or real-world production latency. These measurements are runner-dependent engineering evidence and **not SLA guarantees**.

## Measurements

| Pending Requests | GOMAXPROCS=1, ns/scan (3 reps) | GOMAXPROCS=4, ns/scan (3 reps) | Allocations |
|---:|---|---|---|
| 0 | 8.745 / 8.775 / 8.805 | 8.923 / 8.774 / 8.830 | 0 |
| 1 | 47.02 / 46.95 / 48.77 | 47.34 / 47.58 / 47.13 | 0 |
| 32 | 494.9 / 495.3 / 486.9 | 490.1 / 491.5 / 482.8 | 0 |
| 128 | 1960 / 1995 / 1948 | 1971 / 1990 / 1937 | 0 |
| 512 | 8114 / 8100 / 7973 | 8034 / 8032 / 7988 | 0 |

The scan demonstrates the expected **O(N) growth** from a map iteration. A 512-Pending scan spends approximately 8 µs under the Session lock per wake; this could matter when a *single Session* has hundreds of outstanding requests at high wake rates.

## Context from previous end-to-end profiling

[Performance suite 44202869096](https://github.com/majiddarvishan/neta-go/actions/runs/44202869096) reported about **1.25% cumulative CPU** under `Session.nextDeadlineLocked` and **13.25% cumulative CPU** in the entire `Session.controlLoop` for its small-payload profile. This is a **different workload** and not a measurement of the 512-Pending stress case. A substantial part of the control loop's cost may be outside the deadline scan.

## Decision and pending work

**Do not change production scheduling yet.** There is measurable linear growth but no confirmed production workload distribution or service-level target. Replacing the map scan with a heap or deadline index entails more moving parts on Request admit, timeout, response, cancellation and Session teardown; such an optimization should be evidence-driven.

- [x] Add repeatable microbenchmark to `benchmark_test.go` and measured PR CI step.
- [x] Record baseline across 0/1/32/128/512 in-flight requests, 3 repeats, CPU=1 and 4, and no allocations.
- [ ] Measure actual server/client throughput, p95/p99, CPU, mutex contention with **hundreds of outstanding Requests per Session**.
- [ ] Decide if the measured hot-path cost exceeds production-relevant targets; if so, implement and compare a deadline-index alternative with race/close/timer tests.
- [ ] Verify post-merge protected-main CI and compare results on a matched runner before closing QA-004.

**Release boundary:** QA-004 remains open as an investigated performance risk, not a confirmed regression. No new API, timing semantics, wire change, tag or Release is authorized by this benchmark.
