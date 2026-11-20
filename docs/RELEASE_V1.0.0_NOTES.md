# neta-go v1.0.0 — Official Go module release

**Release type: official v1.0.0 / General Availability version identifier (owner-approved 2026-11-20).**

The same production Go source and V1/V2 wire behavior as the previously published `v1.0.0-rc.1` integration tag is promoted to `v1.0.0`. The release-preparation change updates **version and documentation metadata only**, not production Go, benchmarks, tests or `go.mod`.

## Features

- Callback-oriented TCP Client and Server, framed Request/Response and one-way messages, bounded admission/queue/worker resources and configurable reconnect.
- V1/V2 per-Session wire version selection; **V2 default**; V2 Bind Request/Response omits trailing NUL and accepts 20-byte SystemID; V1 retains NUL and max 19-byte SystemID.
- Bidirectional Client inbound Requests via `WithClientOnRequestHandler`, and generic Client/Server peer-router examples; **14 runnable examples**.
- Required Go correctness CI, pinned C++ Wire compatibility and bounded parser mutation checks were green on the release-preparation PR. Final tag must target an exact protected-main merge SHA with both required postmerge checks green.

## Install

```bash
go get github.com/majiddarvishan/neta-go@v1.0.0
```

Module path: `github.com/majiddarvishan/neta-go`. Requires Go 1.23+. The Go module does not embed its Git tag into runtime source.

## Qualification transparency — IMPORTANT

**The owner explicitly authorized the official `v1.0.0` version and GitHub Release while qualification remains incomplete.** Do not interpret this as performance, memory or production-load SLA certification.

- **Q-ENV-01:** dedicated reference machine and three Mode admissions still **NOT ADMITTED**.
- **QA-004/QA-005:** full heavy P99/performance evaluation and approved numerical baseline/absolute SLA remain **OPEN/INCONCLUSIVE**.
- **MEM-004/MEM-005/MEM-006:** canonical `retained-attributed-B` peak, observer accuracy and long soak **NOT QUALIFIED**.
- **Q-COR-01..04:** independent full qualification (correctness/stress/fuzz/live C++ interoperability) pending despite quick/required CI PASS.
- Mandatory numeric **Performance Gate is DISABLED**. No numerical throughput/latency/retained-memory guarantees or unmeasured stability claims are made.
- **Known limitation**: this release is a fixed integration/API baseline for applications; users requiring qualified performance/memory guarantees should await Phase Q acceptance and potential subsequent patch release.

The owner-approved, non-background [parallel Phase Q plan](../.codex/NETA_GO_PHASE_Q_PARALLEL_EXECUTION_PLAN_2026-11-20.md) remains in effect. Defects discovered there should be fixed on a new commit/version; **do not move existing Git tags**. The existing `v1.0.0-rc.1` remains unchanged.

## Publication evidence

This document is committed **before** the final tag; the single-purpose publication workflow must verify exact-main Go+C++ CI success, create an immutable annotated `v1.0.0` tag and a non-prerelease GitHub Release at that same SHA, and then verify them via GitHub. A missing tag or failure means publication is incomplete regardless of `VERSION`.
