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
