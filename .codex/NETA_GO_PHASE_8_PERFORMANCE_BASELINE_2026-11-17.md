# neta-go Phase 8 — Performance Baseline and Evidence-Driven Optimization

**Status:** IN PROGRESS — OWNER AUTHORIZED 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Starting documentation HEAD:** 7f9fe62e4ffa85966b5f028365086707ec08fed2  
**Starting source HEAD:** 209c24a7dbc5a5aa3db95ec63705915406acacb6  
**Owner authorization:** «ادامه بده» in response to the explicit Phase 8 approval question.

## 1. Scope

P8-01 through P8-05 are authorized. This phase covers benchmark infrastructure, reproducible performance measurement, profiling, evidence-driven optimization candidates and regression gates.

Phase 9 examples, release readiness and deployment are not authorized.

## 2. Frozen constraints

- Do not change public API, Pinex 1.5.0 Wire Protocol, default values, timeout/reconnect policy, worker ceilings, queue/resource limits, callback ordering or frozen architecture.
- PERF-002 A remains binding: outgoing async admission takes one safe snapshot; incoming payload is owned and user-retainable; never pool/recycle inbound payload after handoff.
- Preserve bounded admission, all owner/session resource ledgers, Request credits, single serialized Writer and class-isolated fair callback scheduling.
- No numeric performance SLA is approved. First establish a reproducible baseline.
- Race instrumentation is a correctness gate, not a performance result.

## 3. Source and design review

Starting source HEAD: 209c24a7dbc5a5aa3db95ec63705915406acacb6

Starting docs HEAD: 7f9fe62e4ffa85966b5f028365086707ec08fed2

Reviewed:
- NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md
- NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md
- Worker Capacity & Resource Accounting design
- Architecture Freeze and Decision Register
- current Client, Server, Session transport, wire codec, resource ledger and callback scheduler implementation
- current Go and Pinex golden CI workflows

AGENT_CONTEXT.md is absent under .codex on main; root TASKS.md is the canonical implementation tracker.

## 4. Required workload matrix

| Dimension | Planned cases |
|---|---|
| Payload | 64 B, 256 B, 1 KiB, 4 KiB, near 1 MiB total frame |
| Sessions | 1, 8, 32 initially; extend only within configured limits |
| Producers | 1, 4, 16, 64 where admission budgets permit |
| Callback concurrency | 1, 2, 4, 8, 16 where owner limits permit |
| Operations | wire encode/decode, Request/Response, SendNoWait, saturation/Queue Full |
| Transport | Go↔Go loopback baseline; real Pinex C++ peer remains the compatibility gate |
| Stability | bounded reconnect/close and mixed-payload soak |

## 5. Metrics

- Throughput (completed ops/s and MiB/s)
- End-to-end Request/Response p50, p95 and p99
- Local submission/admission latency where separately measurable
- allocs/op and B/op
- CPU profile and process CPU utilization
- Heap allocation/live heap, GC count and pause data
- Goroutine count and callback worker snapshots
- RSS / peak RSS
- Mutex and block profiles
- per-Session fairness and queue/capacity behavior

The benchmark report must record Go version, GOOS/GOARCH, CPU model and available CPUs, GOMAXPROCS, runner/cgroup limits, exact source SHA, payload and concurrency profile, command line and artifact links.

## 6. Task status

| Task | Status |
|---|---|
| P8-01 — Reproducible baseline | IN PROGRESS |
| P8-02 — Metrics | NOT STARTED |
| P8-03 — Profiles | NOT STARTED |
| P8-04 — PERF-004 evidence review | NOT STARTED |
| P8-05 — Regression gate | NOT STARTED |

## 7. Initial implementation boundary

The first implementation is test/benchmark/CI/documentation-only. No production source or API changes are planned for baseline collection. Candidate optimizations will be selected only after benchmark and profile evidence exists. If no safe, material benefit is established, record NO CHANGE rather than adding complexity.

## 8. Results

Pending initial benchmark harness and dedicated CI run.


## 9. Repeatable local and CI automation — 2026-11-17

The repository now exposes the same entry points locally and in GitHub Actions:

- `make verify`: gofmt, go vet, uncached unit tests and uncached race tests.
- `make perf-smoke`: short benchmark smoke test.
- `make soak`: mixed-payload memory soak; 300 seconds by default, configurable from 300 to 600 seconds.
- `make phase8`: benchmark matrix, 1–512 Session scaling, five-minute soak, small/large CPU/heap/mutex/block profiles and pprof summaries.

Implementation details:
- `scripts/ci-check.sh` and `scripts/phase8-benchmark.sh` are child-process scripts. They do not modify the caller's interactive shell.
- Each invocation creates a unique run directory below `artifacts/ci/` or `artifacts/phase8/`, captures `run.log`, environment/source metadata and `status.txt`.
- Performance profiles and resource timing are preserved. The root `artifacts/` directory is ignored by Git.
- GitHub Actions calls `make verify` and `make phase8`, rather than maintaining separate inline command sequences.
- CI uses Go 1.23.x as configured by the project workflows; local execution records its actual toolchain version.

**Validation:** implementation committed; GitHub Actions validation and artifact review pending. P8-01 remains IN PROGRESS. No production code, public API, protocol, defaults, worker limits or resource limits were changed.


## 10. Correctness automation verification — 2026-11-17

The new shared correctness entry point has now been exercised both locally by the owner and in GitHub Actions.

**Owner's local run**
- Command: `make verify` after a safe `git stash push -u`, `git fetch origin` and `git pull --ff-only origin main`.
- Local repository synchronized to `708d5776e389dd04f0b60b347fa8cd15d42d40bc`; worktree reported clean and aligned with `origin/main`.
- gofmt: PASS.
- go vet: PASS.
- uncached unit tests: PASS for root and `internal/wire`.
- uncached race tests: PASS for root and `internal/wire`.
- Local output directory: `artifacts/ci/20261117T143242Z-708d5776e389-verify-3664856`.
- The pre-existing local changes were preserved in `stash@{0}`; they have not been applied or dropped.

**GitHub Actions on the same source SHA**
- Go correctness workflow [44103718615](https://github.com/majiddarvishan/neta-go/actions/runs/44103718615): PASS. Shared `make verify` completed all four checks and uploaded artifact `neta-go-correctness-708d5776e389dd04f0b60b347fa8cd15d42d40bc`.
- Pinex Wire Compatibility workflow [44103718477](https://github.com/majiddarvishan/neta-go/actions/runs/44103718477): PASS.
- Full Phase 8 performance workflow [44103718627](https://github.com/majiddarvishan/neta-go/actions/runs/44103718627): IN PROGRESS at the time of this update. It has completed checkout and Go setup and is executing the full suite. No performance baseline or P8-01 completion is claimed until the run succeeds and artifacts are reviewed.

The local and GitHub correctness evidence agrees. No production source, public API, wire protocol, defaults or resource limits were changed for this automation.


## 11. SendNoWait benchmark correlation hardening — 2026-11-17

During review of the first full run, the shared receive-signal channel was found to allow one concurrent producer to consume another producer's delivery signal. That did not affect production behavior, but could misattribute per-operation delivery latency and fairness samples.

The benchmark harness now correlates each one-way Request with its own server-side delivery using:
- the client TCP local address and matching server-side remote address to select the correct Session tracker;
- the Session's returned Stream Request sequence to route the delivery notification;
- a bounded-in-practice pending/completed map that also handles delivery racing ahead of waiter registration;
- a focused test covering out-of-order and early delivery notification.

The change is test-only. No production source, public API, protocol, defaults or resource limits changed. The prior Phase 8 workflow run was started before this correction and is not accepted as final SendNoWait latency evidence. A new full run will validate the corrected harness; P8-01 remains IN PROGRESS until that run and its artifacts are reviewed.


## 12. First full-run artifact review — preliminary, not accepted as final baseline

**Workflow:** [44103718627](https://github.com/majiddarvishan/neta-go/actions/runs/44103718627) — PASS  
**Source SHA:** `708d5776e389dd04f0b60b347fa8cd15d42d40bc`  
**Artifact:** [neta-go-phase8-708d5776e389.zip](https://github.com/majiddarvishan/neta-go/actions/runs/44103718627/artifacts/14338171799)  
**Artifact ID:** `14338171799`  
**Runner:** Ubuntu 24.04, Go 1.23.12, Linux/amd64, 4 vCPU, Intel Xeon 6975P-C.

The full script completed successfully in about 9m22s of job execution. All workload groups except SendNoWait have usable preliminary evidence. The SendNoWait results are excluded because concurrent producer delivery signals were not correlated to their own Session sequence; the corrected tracker is committed in `c5ebca4d3d0b7f1d36270501add0b059525df2c2` and a new full run is in progress.

### Preliminary stable evidence

- Five-minute mixed-payload soak: PASS.
  - baseline heap: 319,200 bytes; final heap: 317,944 bytes;
  - maximum HeapInuse: 4 MiB;
  - maximum RSS: 88,492 KiB;
  - cumulative allocations: 13,341,453,008 bytes; GC cycles: 4,609;
  - goroutines after cleanup: 2;
  - periodic samples showed server retained and queued Application bytes at zero.
- Queue Full admission: median 16.61 ns/op, 0 B/op, 0 allocs/op.
- 512 Sessions, 64-byte Request/Response: 134,807 ops/s, p95 5.88 ms, p99 8.58 ms, per-Session p95 max/min ratio 1.294.
- 512 Sessions, 256-byte Request/Response: 125,931 ops/s, p95 7.79 ms, p99 9.64 ms, per-Session p95 max/min ratio 1.238.
- 256-byte profile workload, one CPU: 82,947 ops/s, p50 47.6 µs, p95 57.0 µs, p99 67.7 µs, 1,712 B/op, 18 allocs/op.
- 256-byte profile workload, four CPUs: 61,187 ops/s, p50 61.6 µs, p95 102.9 µs, p99 132.6 µs, 1,734 B/op, 19 allocs/op.
- Near-1 MiB profile workload, one CPU: 1,311 ops/s, p50 762.9 µs, p95 788.5 µs, p99 804.4 µs, about 4.19 MB/op, 21 allocs/op.
- Near-1 MiB profile workload, four CPUs: 1,153 ops/s, p50 817.6 µs, p95 1.22 ms, p99 1.47 ms, about 4.19 MB/op, 21 allocs/op.

### Preliminary profile interpretation

- Small-payload allocation profile: `buildStreamFrame` 33.61% and `Session.readFrame` 29.59% of sampled alloc_space. Callback completion-task/queue allocations are secondary.
- Large-payload allocation profile: `Session.readFrame` 50.08% and `buildStreamFrame` 49.88%.
- CPU profile: `internal/runtime/syscall.Syscall6` 33.16% flat; `Session.controlLoop` 12.62% cumulative; `runtime.selectgo` 9.67% cumulative. This does not yet justify a timer/control-loop redesign.
- Mutex profile is not a strong case for redesign: total sampled delay was about 677 ms; the largest attribution was `sync.(*Mutex).Unlock` and runtime lost-contended-lock accounting.

### PERF-004 candidate assessment — pending corrected rerun

`buildStreamFrame` is the largest avoidable allocation candidate, but its allocation is also the safe caller-payload snapshot held until the single Writer finishes. Reuse would need a strictly bounded retention policy and correct recycling on successful write, failed write, close, queue cancellation and admission races. A generic `sync.Pool` would not provide the required hard retained-memory bound. Per-Session pooling would introduce a new per-Session retained-memory allowance and must be reconciled with owner resource accounting.

`Session.readFrame` allocates the owned inbound payload that may be retained by the application. Pooling/recycling that memory is prohibited by PERF-002 A. No inbound pooling is proposed.

**Current decision:** no production optimization is accepted from this preliminary run. Re-run on the corrected SendNoWait harness, compare the same workload, then decide whether a strictly bounded outbound-only reuse design provides enough measured benefit to justify its resource-accounting and lifecycle complexity. If not, record PERF-004 = NO CHANGE.


## 13. End-to-end SendNoWait correlation test — 2026-11-17

Added `phase8_delivery_test.go` to validate the benchmark's real TCP routing independently of the long benchmark matrix. Eight concurrent producers send one-way Requests over two Sessions; each Session begins with overlapping sequence values, and every delivery must reach the tracker for the correct TCP peer and sequence. The test also exercises delivery racing ahead of waiter registration.

This is test-only and runs in ordinary and race CI. It does not alter the production API or implementation.


## 14. Final corrected Phase 8 run — 2026-11-17

**Status:** PASS — corrected full benchmark and profile suite completed.  
**Workflow:** [44105272003](https://github.com/majiddarvishan/neta-go/actions/runs/44105272003)  
**Artifact:** [neta-go-phase8-c5ebca4d3d0b7f1d36270501add0b059525df2c2.zip](https://github.com/majiddarvishan/neta-go/actions/runs/44105272003/artifacts/14338552627)  
**Artifact ID:** `14338552627`  
**Benchmark/test harness SHA:** `c5ebca4d3d0b7f1d36270501add0b059525df2c2`  
**Latest repository HEAD after adding the separate routing integration test:** `8986636f1fefbd2ebcad20df8e3fe66ed6c5fbbc`  
**Runner:** Ubuntu 24.04, Go 1.23.12, Linux/amd64, Intel Xeon 6975P-C, 4 vCPU; benchmark CPU modes 1 and 4.  
**Production source:** unchanged from Phase 7 source HEAD `209c24a7dbc5a5aa3db95ec63705915406acacb6`.

The complete benchmark matrix, 1–512 Session scaling, corrected SendNoWait correlation, five-minute soak and small/large CPU/heap/mutex/block profiles all completed successfully. Artifacts contain environment metadata, logs, benchmark output, pprof files and summaries, time/RSS reports, and explicit PASS status.

### 14.1 Representative measurements

| Workload | Throughput | p50 | p95 | p99 | Allocation |
|---|---:|---:|---:|---:|---:|
| Request/Response 64 B, 1 Session/1 producer, 1 CPU | 71,778 ops/s | 13.5 µs | 14.7 µs | 23.2 µs | 960 B, 21 allocs/op |
| Request/Response 64 B, 1 Session/16 producers, 1 CPU | 99,885 ops/s | 156.9 µs | 170.6 µs | 226.3 µs | 900 B, 16 allocs/op |
| SendNoWait 64 B, 1 Session/1 producer, 1 CPU | 113,184 ops/s | 8.87 µs | 9.57 µs | 16.65 µs | 584 B, 14 allocs/op |
| SendNoWait 64 B, 1 Session/16 producers, 1 CPU | 178,617 ops/s | 87.5 µs | 98.7 µs | 121.4 µs | 554 B, 11 allocs/op |

- 512 Sessions, Request/Response 64 B: 123,864 ops/s, p95 6.57 ms, p99 9.58 ms, Session p95 max/min ratio 1.352.
- 512 Sessions, Request/Response 256 B: 116,244 ops/s, p95 8.84 ms, p99 11.97 ms, Session p95 max/min ratio 1.319.
- Queue Full admission: median 16.65 ns/op, 0 B/op and 0 allocs/op.
- Pure wire codec, 64 B: Encode 33.72 ns/op / 80 B / 1 alloc; DecodeFrame 32.83 ns/op / 64 B / 1 alloc; DecodeStreamRequest 62.73 ns/op / 128 B / 2 allocs.

These are GitHub-hosted loopback runner measurements, not production hardware guarantees or approved SLAs. The two independent full runs showed normal hosted-runner variation; no numeric pass/fail thresholds are inferred.

### 14.2 Corrected five-minute soak

- Baseline heap: 335,696 bytes; final heap: 308,088 bytes.
- Maximum HeapInuse: 4,374,528 bytes.
- Maximum RSS: 91,348 KiB.
- Total allocated during soak: 13,289,700,184 bytes; 4,549 GC cycles.
- Goroutines after cleanup: 2.
- Periodic samples kept server retained and queued Application bytes at zero.
- Result: PASS; no retained-memory growth observed in this bounded workload.

### 14.3 Profile findings

**Small-payload profile**
- Allocation space: `buildStreamFrame` 32.80%; `Session.readFrame` 28.82%; completion-task creation 7.42%; inbound dispatch 7.07%; SendRequest 5.96%; callback queue append 4.59%.
- CPU: `internal/runtime/syscall.Syscall6` 34.87% flat; `runtime.selectgo` 10.13% cumulative.
- Mutex sampled delay: 709.36 ms over the profile; no single application lock emerged as a dominant hot spot.
- Block profile is dominated by normal select/channel/condition-variable waits.

**Large-payload profile**
- Allocation space: `Session.readFrame` 50.07%; `buildStreamFrame` 49.88%.
- CPU: `Syscall6` 35.03%, `runtime.memclrNoHeapPointers` 20.14%, `runtime.memmove` 6.40%.
- Mutex sampled delay: 96.41 ms; no case for lock redesign.

### 14.4 PERF-004 final decision — NO CHANGE

No production optimization is accepted in Phase 8.

- `Session.readFrame` allocation represents the owned inbound payload. The application may retain it; PERF-002 A forbids recycling/pooling after handoff. It remains unchanged.
- `buildStreamFrame` is a significant outbound allocation site. Reuse could reduce allocation volume, but safe reuse requires hard-bounded retained memory and exact return/discard handling across successful writes, failed writes, Session close, queue cancellation and admission races.
- A generic `sync.Pool` does not provide the hard retained-memory bound required by this design. A per-Session pool introduces a new retained-memory allowance and owner resource-accounting implications.
- Profiles do not prove a material throughput or tail-latency win from such a pool. The soak is stable, no numeric SLA exists, and no owner production workload distribution has yet been supplied for threshold setting.

Batching also has no measured case that justifies changing the single-Writer/FIFO behavior. Therefore **PERF-004 = NO CHANGE** is the evidence-based result, not an unfinished implementation.

### 14.5 P8-05 gate and Phase 8 closure

- Shared Go correctness CI on latest repository HEAD [44107389341](https://github.com/majiddarvishan/neta-go/actions/runs/44107389341): PASS — gofmt, vet, uncached unit and race.
- Pinex Wire Compatibility on latest repository HEAD [44107389316](https://github.com/majiddarvishan/neta-go/actions/runs/44107389316): PASS.
- Phase 7 live bidirectional Pinex interoperability remains PASS at [44081220680](https://github.com/majiddarvishan/pinex/actions/runs/44081220680); production source was not changed in Phase 8.
- Corrected Phase 8 full benchmark/profile workflow [44105272003](https://github.com/majiddarvishan/neta-go/actions/runs/44105272003): PASS; artifact retained for future comparisons.
- Weekly scheduled runs continue to collect comparable evidence. Numeric regression thresholds are intentionally not enforced until the owner reviews production workload distributions and approves targets.

**Phase 8 P8-01..P8-05: COMPLETE.** No production source, public API, wire format, defaults, timeout/reconnect policy, worker ceiling or resource limit changed. Phase 9 remains NOT AUTHORIZED.
