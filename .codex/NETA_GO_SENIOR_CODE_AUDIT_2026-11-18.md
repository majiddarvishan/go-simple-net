# neta-go — Independent Senior Engineering Audit

**Date:** 2026-11-18  
**Reviewed source:** protected `main` at `0fadd380d558028a85333fa044eeb4c93ae5c11f`  
**Method:** source-level inspection of framing/Binds, Client/Server lifecycle, Session control/transport, resource ledgers, callback scheduler, tests, CI and benchmark instrumentation; comparison with available GitHub CI. **No new runtime test, long-running load or adversarial reproduction was executed as part of this static audit.**  
**Outcome:** GO CI PASS and functional foundations strong, **but conditional release readiness / remediation required**. Do not interpret a successful CI run as proof of bug-free production behavior.

## CI evidence at the exact audited source

| Check | GitHub result |
|---|---|
| Go: gofmt, vet, unit, race, paired-example smoke and benchmark smoke | [44202868979](https://github.com/majiddarvishan/neta-go/actions/runs/44202868979) — PASS |
| Golden wire compatibility (pinned historical C++ snapshot) | [44202868919](https://github.com/majiddarvishan/neta-go/actions/runs/44202868919) — PASS |
| Full performance suite, scaling, bounded 300s memory soak and profiles | [44202869096](https://github.com/majiddarvishan/neta-go/actions/runs/44202869096) — PASS |

The Go check invokes `scripts/ci-check.sh verify`, which runs uncached `go test` and `go test -race`. The C++ check only rebuilds and compares golden wire fixtures and exercises the Go wire package. **Neither check sets `NETA_CPP_PEER_BIN`.** Consequently the real C++ peer tests are skipped in normal CI, and the performance workflow skips C++-peer benchmarks. The latest full suite PASS is not a numeric regression threshold.

## Findings, ranked by severity

### QA-001 — P1 — V2 default breaks the current V1-only C++ test client's intended live interop path

**Code evidence:** `interoperability_test.go:120-136` constructs a Go Client for the pinned C++ peer **without** `WithClientSessionOptions(WithProtocolVersion(ProtocolVersionV1))`. `interoperability_benchmark_test.go:129-144` does the same. `config.go:127-145` defaults to V2; `internal/wire/pdu.go:35-68` distinguishes V1 NUL-terminated Bind from V2 NUL-free Bind. The golden source snapshot is the legacy V1 C++ serializer (`testdata/neta-cpp-1.5.0/source/SNAPSHOT.md`). `interoperability_test.go:15-20` skips without `NETA_CPP_PEER_BIN`, which no current Go CI workflow configures.

**Impact:** Normal CI can be green while the Go Client ↔ pinned legacy C++ Server test/benchmark is incompatible at Bind. The failure is **predicted from code**, not presented as a newly executed failing test.

**Required remediation:** Set V1 explicitly in legacy peer Client tests/benchmarks; provision a real C++ peer for **both-direction TCP interop** in an authorized CI path; verify the asserted legacy protocol version, successful Bind/Request/Response/Unbind and V2 behavior separately. Do not change the public default V2.

### QA-002 — P1 — Unbounded lifetime of user BindValidator goroutines after socket timeout

**Code evidence:** `server_lifecycle.go:142-145` starts one `handleBind` goroutine per admitted pre-Bind Session, and `:158-171` calls a synchronous, contextless `BindValidator`. `session_control.go:33-36` terminates the connection after the configured deadline, while `server_lifecycle.go:216-225` releases its pending socket counter. No path can interrupt a blocked Go function inside `BindValidator`.

**Impact:** If a validator permanently blocks (e.g., downstream auth outage), expired Sessions free the admission counters while their `handleBind` goroutines remain. New connections can create more blocked validators beyond `MaxPendingPreBindSessions` over time; `Server.Stop` closes Sessions but cannot join those validator goroutines.

**Required remediation:** Define explicit validator execution/timeout policy and **bound validator goroutines by lifetime of the validator itself**, not just its socket. Offer a context-aware validator API if appropriate without silently breaking current callers. Adversarial test: blocking validator, repeated expired binds beyond the pending-socket cap, and Stop; verify steady bounded goroutine count.

### QA-003 — P2 — Unnecessary pre-auth allocation for malformed oversized Bind bodies

**Code evidence:** `session_transport.go:68-80` accepts any Frame Length up to the 1 MiB generic limit; `:123-132` allocates `make([]byte, bodyLength)` before Bind validation in `:236-257`. Both Bind bodies have an effective **21-byte** maximum (1-byte BindType, SystemID up to 20 bytes for V2; V1 includes its NUL).

**Impact:** An unauthenticated connection can provoke near-1-MiB allocation/read for a Bind that the decoder will necessarily reject. Existing pre-Bind socket quotas limit concurrency but do not eliminate avoidable memory/CPU pressure.

**Required remediation:** At Header dispatch, reject Bind Request/Response body lengths beyond the version-independent 21-byte upper bound *before allocating*. Keep existing full-Frame bounds for Stream data. Add malformed-length socket tests and allocation checks.

### QA-004 — P2 performance risk — Repeated O(N) pending-request scans on hot path

**Code evidence:** `session_control.go:47-59` iterates over **all** Pending requests for expiry; `:169-173` iterates over the same map again to find the next deadline. `session.go:99`, `:159`, `:353`, `:382` and transport activity signal the control loop frequently. Per-Session outstanding limit defaults to 128 and permits up to 512 (`config.go:38,54,143`).

**Impact:** CPU/control-lock work per wake grows with outstanding Request count even when no deadline expires. This can degrade throughput and p95/p99 latency under high in-flight concurrency. **Performance regression is not measured yet**; this is an algorithmic hotspot requiring profiling.

**Required remediation:** Add isolated microbenchmarks for pending populations 1/32/128/512, measure control lock contention and p95/p99 throughput; optimize with deadline indexing/heap or a suitable timer architecture only if evidence supports it.

### QA-005 — P2 verification gap — No automated numeric performance regression acceptance

**Code evidence:** `scripts/performance.sh:148-191` records matrix/scale/soak/profiles and succeeds when commands exit successfully; it does not compare measurements against a versioned tolerance or workload SLA. ` .codex/TASKS.md ` notes numeric thresholds deliberately unset at P8-05.

**Impact:** A meaningful performance regression can occur while `Performance Benchmarks` remains green.

**Required remediation:** Owner chooses representative message sizes, session count and production-relevant SLAs; establish stable baseline and threshold-based CI gating with noise controls. Do **not** claim an arbitrary percent is a validated SLA.

### QA-006 — P2 resilience — Fatal Accept failure is not surfaced

**Code evidence:** `server_lifecycle.go:76-92` retries temporary Accept errors but simply returns on other errors. The server remains `started` with no distinct fatal-accept notification, and `Server.Start()` called again is a no-op (`:24-29`).

**Impact:** A rare permanent listener error may silently leave a Server that no longer accepts new connections. This is inferred from the code path and needs a fault-injecting Listener test.

**Required remediation:** Establish terminal listener failure reporting and controlled stop/close policy; add deterministic injected-Accept-error test.

### QA-007 — P2 contract decision — User callback panic propagates through worker goroutine

**Code evidence:** `worker_scheduler.go:367-398` executes `task.run()` without a recovery boundary. A panic in an application callback can terminate the hosting process under normal Go semantics.

**Impact:** This is **not automatically a library bug**; choose and document whether a callback panic is deliberately fatal or should be isolated/reported. Do not silently swallow panics or invent a policy.

**Required remediation:** Decide callback-panic contract; document it and test the selected behavior.

### QA-008 — P3 maintainability — Stale status and redundant handshake work

**Code evidence:** `docs/RELEASE_READINESS.md:20-22` still treats completed repository naming cleanup as pending and mainly cites an old SHA; the latest post-merge CI evidence is absent. `session_internal.go:258-264` encodes a preliminary Bind frame, then `:275-282` encodes again using the actual sequence. This double encoding is harmless but unnecessary.

**Impact:** Misleading release status and minor extra allocation in Bind setup.

**Required remediation:** Update release readiness evidence while preserving **Unreleased / P9-06 NOT AUTHORIZED**, remove obsolete checklist claims, remove extra Bind serialization after ensuring validation and error behavior remain unchanged.

### QA-009 — P2 verification gap — Fuzz mutation and real C++ TCP checks are not routine PR gates

**Code evidence:** `internal/wire/wire_fuzz_test.go` contains fuzz targets, but `scripts/ci-check.sh:90-100` only uses standard and race tests (seed corpus, not a mutation campaign); `interoperability_test.go:17-20` skips TCP C++ tests without the external peer binary. The golden workflow checks serializer bytes, not the whole live C++ Session path.

**Impact:** Parser-fuzz and genuine cross-language lifecycle regressions can escape routine PR gates.

**Required remediation:** Add bounded scheduled or opt-in fuzz campaigns and a permission-appropriate C++ end-to-end CI job. Record actual `V1` peer version and test outcomes, and keep golden fixtures as an independent faster gate.

## Engineering strengths and boundaries

- Three separated worker pools for Application, Completion and OnClose, with per-Session concurrency and owner ceilings; FIFO and race/churn tests exist.
- Session-owned reader/writer/control goroutines, explicit deadlines, bounded queues/request credits and ownership-aware release paths.
- V1/V2 fixed Session selection, explicit V2 NUL-free Bind and 20-byte SystemID, with legacy V1 wire preserved.
- Resource counters are synchronized and use checked uint64 arithmetic.
- Automated Go formatting, vet, uncached unit/race, sample application smoke, 300-second soak and captured CPU/heap/mutex/block profiles.
- No external Go modules in `go.mod`, with a narrow pinned C++ serialization snapshot.
- **Security boundary:** Bind SystemID is not cryptographic authentication; no TLS is provided by the library itself. Apply transport/network protections and a real validator for untrusted environments. This is an existing documented limitation, **not** presented as a surprise security exploit.

## Release and remediation guidance

**Audit status: OPEN — DO NOT CLAIM BUG-FREE OR PRODUCTION-READY ON CI PASS ALONE.** Resolve or explicitly mitigate QA-001, QA-002 and QA-003 before release signoff. Treat QA-004 as measured optimization, QA-005/QA-009 as qualification gaps, and QA-006/QA-007/QA-008 as resilience/design/cleanup. Priority levels are engineering triage recommendations, not owner approvals.

For every fix: add a focused failing regression test first where feasible; run `make verify`, `make smoke`, `make perf-smoke` and both protected PR checks; after merge verify the exact new main SHA. For performance changes, run `make benchmark` and compare profiles on matched hardware. Do not change frozen protocol or public default without explicit architecture approval. Do not create a tag or GitHub Release without separate owner authorization.

## Remediation update — PR #15, 2026-11-18

The original findings above describe `main` at `0fadd380...`. They are retained as historical audit evidence, not silently rewritten as if fixes had already been present.

- **QA-001 partial:** Go Client legacy peer test and benchmark now share explicit V1 configuration. Added a non-skipped Go configuration test. The true private C++ Client/Server E2E job is **not** supplied by regular CI, which only validates pinned golden bytes. Leave QA-001 open.
- **QA-002 remediated:** bounded Server validator lifetime permits held until user callbacks return; waiting handshakes exit on Session close; late callback cannot re-reserve an identity after `sessionDone`. User-blocked functions cannot be forcibly cancelled, so application-side timeouts remain mandatory.
- **QA-003 remediated:** reject Bind Body lengths greater than 21 before allocating/reading. Header-only malicious-client and malicious-server tests included.
- The initial Go CI failure was gofmt-only, corrected in follow-up. [Go PR run 44208242381](https://github.com/majiddarvishan/neta-go/actions/runs/44208242381) and [wire PR run 44208242378](https://github.com/majiddarvishan/neta-go/actions/runs/44208242378) PASS on the formatted implementation. The additional shared legacy-peer configuration test is committed on PR #15 and its final CI must also pass before merge.
- No change to default V2, the fixed 10-byte Header, serialized payloads, or the public BindValidator signature. Release/tag remains separately unauthorized.
