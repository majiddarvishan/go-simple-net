# neta-go — Phase 9 Release Readiness Plan

**Date:** 2026-11-17  
**Repository:** `github.com/majiddarvishan/neta-go`  
**Branch:** `main`  
**Authorization:** OWNER AUTHORIZED for P9-00..P9-05; P9-06 release tag separately gated  
**Authorization baseline HEAD:** `d40467e44ef0535ba0a5ef83c9c5ce4a9109d9a5`  
**Production source HEAD:** `209c24a7dbc5a5aa3db95ec63705915406acacb6`

## 1. Current state

Phases 0–8 are complete. Phase 8's corrected performance run [44105272003](https://github.com/majiddarvishan/neta-go/actions/runs/44105272003) passed and PERF-004 was closed as **NO CHANGE**. The final pre-Phase-9 Go correctness run [44108072039](https://github.com/majiddarvishan/neta-go/actions/runs/44108072039) and Pinex Wire Compatibility run [44108072167](https://github.com/majiddarvishan/neta-go/actions/runs/44108072167) passed on HEAD `d40467e44ef0535ba0a5ef83c9c5ce4a9109d9a5`.

The root `TASKS.md` is the only implementation checklist. `.codex/TASKS.md` is a pointer to it, not a second checklist.

At the authorization baseline HEAD, the repository had no `examples/` or user-facing `docs/` directory, the root README was only a short project-status page, and package-level `doc.go` was minimal. These gaps are now closed by P9-01..P9-04. The module directive remains Go 1.23 and Go CI uses Go 1.23.x; the owner's local environment is Go 1.27.0. The owner approved the V1 release support baseline on 2026-11-17: minimum Go 1.23 and Ubuntu 24.04/Linux amd64 as the continuously CI-verified platform; other GOOS/GOARCH combinations remain unverified.

## 2. Authorization and non-negotiable boundaries

The owner approved the Phase 9 execution plan by replying «ادامه بده» on 2026-11-17.

Authorized: P9-00 through P9-05, as defined below.

**Not authorized:** P9-06 release tagging. Creating a V1 tag/release requires all prior gates to pass and a separate explicit owner approval immediately before the operation.

The following remain frozen:
- Root public package and exported API signatures.
- Pinex 1.5.0 wire format and 1 MiB total Frame limit.
- All approved Functional Option defaults, ranges and hard maxima.
- Payload ownership and no recycling of user-visible inbound payload.
- Callback class isolation, ordering/concurrency contracts and worker ceilings.
- Request-credit, retained-byte, queue and aggregate resource accounting.
- Timeout, reconnect, shutdown and Session terminal semantics.

Examples may add source under `examples/`; docs, tests and CI harness changes are in scope. Production package behavior/API changes and new third-party dependencies are not authorized by this plan.

## 3. Task sequence, estimates and acceptance criteria

Estimates are engineering effort, not calendar promises. The initial total is approximately **9–15 hours** including review and CI iteration. P9-00 through P9-04 are complete. P9-05 implementation checks now pass; remaining work is primarily owner-dependent release review. Estimated engineering work remaining: **about 1–2 hours**, plus owner decision/review time. P9-06 remains separately gated.

| Task | Estimate | Dependency | Acceptance |
|---|---:|---|---|
| P9-00 Tracker/status reconciliation | 0.5–1 h | None | Complete |
| P9-01 Client example | 1–2 h | P9-00 | COMPLETE — public-API example compiles and Go correctness CI passes; paired runtime smoke remains P9-05 |
| P9-02 Server example | 1–2 h | P9-00 | COMPLETE — public-API example compiles and Go correctness CI passes; paired runtime smoke remains P9-05 |
| P9-03 Public API/usage docs | 3–4.5 h | P9-01, P9-02 | COMPLETE — exported GoDoc, API/configuration/lifecycle docs and README Quick Start verified |
| P9-04 Pinex compatibility docs | 1–1.5 h | P9-00 | COMPLETE — exact compatibility version/source SHA, golden and live TCP evidence, and limitations documented |
| P9-05 Test/CI and release review | 2–3 h | P9-01..P9-04 | IN PROGRESS — correctness/Pinex/smoke/coverage PASS; support and Apache-2.0 copyright/commercial licensing APPROVED; branch protection and version/changelog remain open |
| P9-06 V1 tag/release | ~0.5 h | P9-05 + separate approval | Only perform after explicit owner release approval |

### P9-00 — Documentation and tracker reconciliation

**Status: COMPLETE.**

- Corrected the Phase 8 status and latest verified repository HEAD in the canonical tracker.
- Updated root README, current HANDOFF summary, implementation authorization and engineering-notes index.
- Refined Phase 9 tasks, acceptance criteria, dependencies and the P9-06 release boundary.
- Recorded the Phase 9 authorization in the decision log and this plan.
- No production source, tests, public API, wire protocol, defaults or resource limits changed.
- P9-00 commits: [dcb67647cd5748b6a92b54743b257c667d582450](https://github.com/majiddarvishan/neta-go/commit/dcb67647cd5748b6a92b54743b257c667d582450), [c6ac25338851edda1630dd6301a8eaf509f994c0](https://github.com/majiddarvishan/neta-go/commit/c6ac25338851edda1630dd6301a8eaf509f994c0), and final link/evidence correction [a43b0e5ca9b50f592ae402d18968b538af5bc128](https://github.com/majiddarvishan/neta-go/commit/a43b0e5ca9b50f592ae402d18968b538af5bc128).
- Go correctness CI [44114430747](https://github.com/majiddarvishan/neta-go/actions/runs/44114430747) PASS (`make verify`) and Pinex Wire Compatibility [44114430634](https://github.com/majiddarvishan/neta-go/actions/runs/44114430634) PASS on the final P9-00 commit.
- A relative Markdown link audit corrected one pre-existing broken Phase 4 report link in root `TASKS.md`; the audited links now resolve.

### P9-01 — Client example

Expected location: `examples/client/main.go` (or a small example package if separation materially improves clarity).

Must demonstrate:
- Construct Client using required endpoint/SystemID options and reasonable documented Session settings.
- Call `Connect(ctx)` and handle Bind/Dial errors.
- Observe the Bound Session using the supported callback/current-session API.
- Submit a Request with a Completion callback and handle both Response and error.
- Submit a one-way message with `SendNoWait` and explain that successful return means local admission, not peer delivery.
- Handle terminal notification and stop the Client gracefully with a bounded Context.
- Use only the public root package; no `internal/` imports and no external dependencies.

Acceptance: example compiles on the selected supported Go baseline, has comments explaining the peer/server prerequisite, and is exercised by the example verification path.

### P9-02 — Server example

Expected location: `examples/server/main.go`.

Must demonstrate:
- Construct and Start the Server; print/use the actual `Addr()`.
- Configure `BindValidator` and explain identity validation/duplicate SystemID behavior.
- Handle inbound `OnRequest` and send the matching `SendResponse(sequence, payload, status)`.
- Explain callback concurrency and payload ownership at the point of use.
- Handle OnClose and OS/process shutdown without leaking listener or Sessions.
- Stop gracefully with a bounded Context and handle forced cleanup if the Context expires.
- Use only the public root package and standard library.

Acceptance: example compiles, documents how the Client example connects, and is exercised by the example verification path.

### P9-01/P9-02 — Example implementation and build verification

**Status: COMPLETE for source implementation and compile/correctness gates.** Paired runtime smoke remains part of P9-05.

- Client: [`examples/client/main.go`](../examples/client/main.go). Uses Connect/Bind, CurrentSession, SendRequest/Completion, SendNoWait, OnBound/OnClose and deferred bounded Client.Stop.
- Server: [`examples/server/main.go`](../examples/server/main.go). Uses Start/Addr, BindValidator, OnBound/OnRequest/OnClose, SendResponse and bounded graceful Server.Stop.
- Both examples import only the public root package and Go standard library; no `internal/` imports or new dependencies.
- Commit: [04d905f75786542c9b51f48a33228b9bd372b419](https://github.com/majiddarvishan/neta-go/commit/04d905f75786542c9b51f48a33228b9bd372b419).
- Go correctness CI [44115049164](https://github.com/majiddarvishan/neta-go/actions/runs/44115049164): PASS. The workflow runs `make verify`, including gofmt, vet, uncached unit tests and race tests; `go test ./...` compiles the example packages.
- Pinex Wire Compatibility CI [44115049277](https://github.com/majiddarvishan/neta-go/actions/runs/44115049277): PASS.
- The examples are designed to work together: the Client sends a Request expecting an echo Response and a one-way `event:` payload for which the Server intentionally sends no Response.

### P9-03 — Public API and usage documentation

**Status: COMPLETE.**

- Added `docs/API.md`, `docs/CONFIGURATION.md` and `docs/LIFECYCLE.md`.
- Expanded root README with prerequisites, development installation, paired-example Quick Start and documentation links.
- Expanded package-level `doc.go` and GoDoc for all exported root-package declarations, fields, errors, methods and callback types.
- All 56 exported `With...` constructors have individual GoDoc comments; a source audit found zero undocumented exported top-level declarations.
- Internal Markdown link audit passed.
- Initial Go CI attempts caught formatting and a subsequently exposed missing private `newConfigError` helper; formatting was corrected and the original helper was restored unchanged. Final acceptance is the passing run below.
- Final Go correctness CI [44116677209](https://github.com/majiddarvishan/neta-go/actions/runs/44116677209): PASS (`make verify`, including gofmt, vet, unit and race). Final Pinex Wire Compatibility [44116675706](https://github.com/majiddarvishan/neta-go/actions/runs/44116675706): PASS on HEAD `3adca4f49e8565010344411dfdb0d83976218688`.



Required outputs:
- Complete GoDoc for exported package declarations and all exported methods/types/constants/errors/options. The initial audit found 56 exported `With...` constructors, only 9 with directly adjacent comments; verify every exported declaration rather than assuming this count is the entire public API.
- Configuration reference: every option, scope (Client/Server/Session), default, valid range/hard maximum, zero-value behavior, inheritance, and cross-field validation.
- Resource model: distinguish per-Session queue limits, retained payload limits, owner budgets, Server aggregate queued limits and process RSS; do not describe any one of these as a total process-memory cap.
- Payload ownership: outgoing snapshot timing, inbound retainable owned snapshot, callback lifetime and memory charges.
- Callback model: three isolated worker lanes, owner ceilings, per-Session concurrency, FIFO/round-robin behavior, Completion callback-start ordering, OnClose after Done and best-effort saturation semantics.
- Admission and errors: local admission vs delivery, Queue Full vs Capacity, typed errors, `errors.Is`/`errors.As`, terminal cause.
- Lifecycle: Connect/Bind, Server Start/Stop, CurrentSession, reconnect opt-in/no replay, Graceful Unbind, Forced Close, Done and Err.
- README Quick Start: prerequisites, module install, link to both examples, basic usage and links to the detailed docs.

No documentation may promise delivery merely because a send was admitted, claim that callbacks are joined by Done, or imply that AutoReconnect replays Requests.

### P9-04 — Pinex compatibility documentation

**Status: COMPLETE.**

- Added `docs/PINEX_COMPATIBILITY.md` with the verified Pinex 1.5.0 source SHA, wire contract, 1 MiB Frame limit, golden provenance, live TCP directions, fuzz/fault/soak evidence and explicit limitations.
- Latest compatibility workflow [44116675706](https://github.com/majiddarvishan/neta-go/actions/runs/44116675706): PASS.



Document:
- Pinex C++ 1.5.0 compatibility target and exact source SHA `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Frame header, command/status/sequence semantics and 1 MiB total Frame maximum including the 10-byte Header.
- Golden-fixture source provenance and the CI workflow that checks it.
- Actual bidirectional TCP interoperability: Go Client ↔ C++ Server and C++ Client ↔ Go Server.
- Fault/fuzz/soak evidence, with exact workflow links and tested scope.
- Explicit limitation: privileged firewall-level SYN-drop/network-namespace injection was not run; the accepted-socket silent Bind-response blackhole was tested instead.
- Version matrix: only Pinex 1.5.0 is currently verified. Do not claim compatibility with other versions without evidence.

### P9-05 — Test, CI and release-readiness review

**Status: IN PROGRESS — implementation and automated verification gates PASS; final owner review remains open.**

Completed:
- Added `scripts/phase9-release-check.sh` and `make phase9`. The script records coverage, builds both examples, starts the Server on an ephemeral loopback port, runs the Client, verifies Request/Response and one-way event delivery, and gracefully stops both processes.
- Updated Go CI to run `make verify` followed by `make phase9`, then upload correctness and Phase 9 artifacts.
- Initial final-gate Go CI [44118324345](https://github.com/majiddarvishan/neta-go/actions/runs/44118324345): PASS. A later race-test timing assertion was hardened in test-only commit [958601ee55dd6229021e36135a017527dfa8f778](https://github.com/majiddarvishan/neta-go/commit/958601ee55dd6229021e36135a017527dfa8f778); final Go CI [44119107169](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169): PASS on that HEAD. Pinex Wire Compatibility [44119107078](https://github.com/majiddarvishan/neta-go/actions/runs/44119107078): PASS.
- Coverage from `go test -count=1 -cover ./...`: `neta` 79.7%; `internal/wire` 90.3%; `examples/client` and `examples/server` 0.0% because they have no package-local tests. No minimum coverage threshold is approved.
- Runtime smoke PASS: Client Request/Response, SendNoWait event observed by Server, graceful Client Stop and graceful Server Stop. Final evidence artifact [14344846005](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169/artifacts/14344846005).
- Added `CHANGELOG.md`, `docs/SUPPORT.md` and `docs/RELEASE_READINESS.md`.

Remaining owner/repository gates:
- **CLOSED / OWNER APPROVED 2026-11-17:** minimum Go 1.23 and Ubuntu 24.04/Linux amd64 as CI-tested platform; owner-reported local Go 1.27.0 `make verify` PASS is separate evidence, not a guarantee for other architectures/platforms.
- **CLOSED / OWNER CONFIRMED 2026-11-17:** copyright holder `Majid Darvishan`, year `2026`; original Apache License 2.0 retained, with commercial use permitted under its terms and its copyright notice updated.
- Review the Unreleased changelog and proposed semantic version `v1.0.0` (not approved).
- Verify branch protection/ruleset and release settings. The GitHub integration returned HTTP 403; state is unknown.
- Produce final PASS/FAIL/NOT RUN checklist. No release tag is part of this task.

### P9-06 — V1 release tag

Do not create a tag or GitHub Release until:
1. P9-01..P9-05 are accepted and recorded in `TASKS.md`.
2. The final main HEAD passes the required correctness and compatibility CI.
3. The final release version/tag and release notes are agreed.
4. The owner gives separate explicit approval for the tag/release operation.

## 4. Dependency order

```text
P9-00
  ├── P9-01 Client example ──┐
  ├── P9-02 Server example ──┼── P9-03 Public API/docs/Quick Start ──┐
  └── P9-04 Compatibility docs ──────────────────────────────────────┤
                                                                     └── P9-05 Release review
                                                                            │
                                                                            └── P9-06 (separate owner approval)
```

P9-01, P9-02 and P9-04 were completed independently after P9-00. P9-03 followed the examples so Quick Start could refer to verified artifacts. P9-05 is now the remaining authorized work and depends on all docs/examples/evidence. P9-06 is always separately gated.

## 5. Open release questions (do not guess)

- Supported minimum Go version and supported GOOS/GOARCH matrix.
- Exact V1 semantic version/tag naming and changelog/release-note format.
- Whether the owner wants a numeric test coverage target. Until approved, report coverage without a threshold.
- Branch protection/ruleset state; previous API inspection was denied with HTTP 403, so verify through repository settings or an authorized path.
- Any production workload SLA/benchmark thresholds remain outside Phase 9 and are not inferred from hosted-runner measurements.

## 6. Stash review status

The owner reported a local `stash@{0}` containing historical workflow changes. GitHub does not expose a developer's local stash, so the exact patch cannot be inspected remotely. Do not apply or drop it. In the owner clone, the following are read-only:

```bash
git stash show --stat stash@{0}
git stash show --patch stash@{0} -- \
  .github/workflows/go.yml \
  .github/workflows/phase8-performance.yml
```

A comparison with an older committed workflow version is not a substitute for reviewing the actual stash patch.

## 7. Source authority

- Root `TASKS.md`: canonical task status and acceptance evidence.
- `.codex/DECISION_LOG.md`: owner decisions and authorization boundaries.
- `.codex/HANDOFF.md`: current continuation state and important project constraints.
- This file: Phase 9 plan, estimates, dependencies and release gates.
- Phase 8 evidence: [NETA_GO_PHASE_8_PERFORMANCE_BASELINE_2026-11-17.md](NETA_GO_PHASE_8_PERFORMANCE_BASELINE_2026-11-17.md).
- CI automation: [NETA_GO_TEST_AUTOMATION_2026-11-17.md](NETA_GO_TEST_AUTOMATION_2026-11-17.md).
