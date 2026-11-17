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
