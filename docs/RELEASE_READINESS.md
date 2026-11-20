# neta-go release readiness

**Status (2026-11-20): an integration release-candidate TAG `v1.0.0-rc.1` is explicitly owner-authorized, subject to exact protected-main postmerge CI and tag-ref verification; final `v1.0.0` GA and GitHub Release are NOT approved.** Development is merged to protected `main`, but QA-004/QA-005 reference qualification and final-release authorization remain open. See [scoped RC approval](../.codex/PRERELEASE_TAG_APPROVAL_2026-11-20.md) and [parallel test plan](../.codex/NETA_GO_PHASE_Q_PARALLEL_EXECUTION_PLAN_2026-11-20.md). This page summarizes the *current* release gates; dated engineering/owner evidence remains in [`.codex/`](../.codex/README.md).

**Reviewed protected `main` SHA (2026-11-20):** `bd8421cf3cabc5cb72263ed964434a48bf54ef22`, [PR #27](https://github.com/majiddarvishan/neta-go/pull/27) merge commit following [#24](https://github.com/majiddarvishan/neta-go/pull/24), [#25](https://github.com/majiddarvishan/neta-go/pull/25) and [#26](https://github.com/majiddarvishan/neta-go/pull/26). This is a GitHub evidence snapshot, **not** an approved release tag target. Re-evaluate SHA, checks and qualification before publication.

## Pre-qualification integration RC (separate from final GA)

Owner instruction 2026-11-20 permits publishing **one immutable annotated Git tag** `v1.0.0-rc.1` from the final release-preparation PR squash-merge SHA, after **both Go and pinned C++ wire checks PASS on that same SHA**; this is a distribution checkpoint for a Go module used by parallel applications, **not** retrospective approval of Phase Q, retained-attributed memory or absolute SLAs. The [single-purpose tag workflow](../.github/workflows/publish-integration-rc-tag.yml) rejects a colliding tag or advanced `main`, waits for exact-SHA CI and cannot force-retag. If workflow/tag creation fails or lacks permission, report the blocker, do not mislabel a branch as a tag; a manual exact-SHA `git tag` and `git push` remains the fallback under owner control.

Use `go get github.com/majiddarvishan/neta-go@v1.0.0-rc.1` only **after** the tag ref is independently verified. The version declaration in `VERSION` and the `v1.0.0-rc.1` Changelog section are not proof that the Git tag exists.

The old R-01/R-02/P9-06 gates below refer to **final GA `v1.0.0`** and remain open. All qualification cases still require their own accepted evidence. Current normalized distinct open packages: **20** (10 Qualification + 7 other Owner Decisions + 3 final Release), not the historical 22.

## Readiness at a glance

| Scope | Current disposition | Evidence / remaining boundary |
|---|---|---|
| Development-first scope | **COMPLETE — 16/16** | 6 DEV + 10 EX tasks, historically 12 executable commands; [Development closeout](../.codex/NETA_GO_DEV_READY_FOR_QUALIFICATION_2026-11-19.md) |
| Post-Development API and examples | **MERGED** | [PR #27](https://github.com/majiddarvishan/neta-go/pull/27) added `WithClientOnRequestHandler` (bi-bind only; nil compatibility), V1/V2 TCP tests and generic `server-peer-router` + `client-peer-router`; **14 runnable commands total**. The historical 12-command smoke did not cover the extra two. | 
| QA-007 callback/close semantics | **IMPLEMENTED AND MERGED** | Completion preservation, recovered callback panic diagnostics, pre-admission OnClose credits; [Lifecycle](LIFECYCLE.md), [approved contracts](../.codex/NETA_GO_QA_004_005_007_APPROVED_CONTRACTS_2026-11-18.md) |
| Main-branch correctness CI | **PASS at reviewed SHA** | Go and C++ wire workflows linked below |
| Hosted performance workflow | **PASS, ADVISORY ONLY** | No approved reference host/baseline or acceptance threshold; **not** a QA-004/QA-005 qualification PASS |
| QA-004 pending-timeout P99 | **OPEN / NOT QUALIFIED** | Stable reference environment, historical comparator and owner-approved absolute SLA still required |
| QA-005 performance acceptance | **OPEN / NOT QUALIFIED** | 8 Core / 24 Full qualified workload comparison, ten matched baseline/candidate pairs, accepted metrics and absolute thresholds pending |
| MEM-004 / MEM-005 / MEM-006 | **OPEN / PARTIAL_INCONCLUSIVE** | Full attributable retained memory, observer overhead/temporal accuracy and qualified long soak not accepted |
| Mandatory performance gate | **DISABLED** | No authorization or approved numerical limits to activate it |
| Integration RC tag / final GA | **RC TAG OWNER-AUTHORIZED (publication to be verified); final GA NOT AUTHORIZED** | Exact-SHA CI + immutable `v1.0.0-rc.1` only; original final-release gates still open |

A merged feature, a green CI run or a completed Development checklist is **not** a release approval.

## Verified integration and CI evidence

- [PR #24](https://github.com/majiddarvishan/neta-go/pull/24) merged the 2026-11-18 feature to protected `main` with the two required checks passing, along with the bounded V1/V2 parser-mutation workflow. The merge commit's tree matches the complete Feature source tree; the temporary integration commit did not change files.
- **On the current protected `main` SHA `bd8421cf3cabc5cb72263ed964434a48bf54ef22`:** required [Go 44630477642](https://github.com/majiddarvishan/neta-go/actions/runs/44630477642) **SUCCESS** and [C++ wire 44630477596](https://github.com/majiddarvishan/neta-go/actions/runs/44630477596) **SUCCESS**. Branch-protection reports exactly these required contexts: `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0`. These results verify correctness CI, not numerical qualification.
- **Historical merged PR #24 SHA `fb57668e182dc102f94044393b3d8f2a33aeddae`:** [Go CI 44547311083](https://github.com/majiddarvishan/neta-go/actions/runs/44547311083), [C++ wire 44547311177](https://github.com/majiddarvishan/neta-go/actions/runs/44547311177) and hosted advisory [Performance 44547311004](https://github.com/majiddarvishan/neta-go/actions/runs/44547311004) all PASS **at that earlier SHA**. No qualified reference-host approval.
- **On the reviewed PR head `920d37ce7a746ef9878c0be9f13c8714d0c289ff`:** [Go 44547010827](https://github.com/majiddarvishan/neta-go/actions/runs/44547010827), [C++ wire 44547010760](https://github.com/majiddarvishan/neta-go/actions/runs/44547010760) and [bounded parser mutation 44547010745](https://github.com/majiddarvishan/neta-go/actions/runs/44547010745) **PASS**.
- **Examples:** historical bounded [12-command smoke 44542311954](https://github.com/majiddarvishan/neta-go/actions/runs/44542311954) **PASS** for the original Phase D suite. PR #27 brought the count to **14** and added focused real TCP tests for Client-originated/Server-originated Request/Response; the older 12-command smoke is **not** represented as validating both new routers. [Example guide](../examples/README.md).
- The C++ workflow checks wire compatibility; it does not make C++ a runtime dependency of the Go library. [Supported Go/platform baseline](SUPPORT.md) remains Go 1.23+ and CI-tested Ubuntu 24.04 / Linux amd64.

Earlier green feature/`main` runs and completed QA-001/002/003/006/008/009 remain **historical evidence on their exact SHAs**, not a new release preflight. The [canonical reconciled task tracker](../.codex/TASKS.md) records **20 distinct open work packages** (10 Qualification, 7 Owner Decisions, 3 final Release); [Phase-Q execution plan](../.codex/NETA_GO_PHASE_Q_EXECUTION_BACKLOG_2026-11-20.md) records dependencies, acceptance criteria and hardware requirements. [Performance qualification protocol](PERFORMANCE_QUALIFICATION.md) preserves numerical methodology and historical evidence.

## Open engineering qualification gates

- [ ] **QA-004:** approve a stable reference environment and executable historical/candidate comparison; run/validate the specified pending-deadline terminal-settlement P99 matrix and separately approve an **absolute** P99 SLA. Existing A/A and hosted results are advisory, not cross-version qualification.
- [ ] **QA-005:** approve the reference machine, equivalent benchmark source and immutable numeric baseline; collect ten alternating matched baseline/candidate pairs for the approved 8 Core / 24 Full scenarios, including 95% statistical checks and any separately approved absolute limits.
- [ ] **MEM-004:** finish the owner-approved Hybrid-B owned-byte attribution model with category coverage, alias/lifetime consistency, and defensible concurrent observation/sampling; existing F1–F5/M1/D1 adapters are only partially attributable. Do not emit a canonical `retained-attributed-B` from an incomplete snapshot.
- [ ] **MEM-005 / MEM-006:** qualify the observer's overhead and temporal validity, and perform/accept the required 30-minute memory-stability soak on the approved reference machine and reviewed source. Earlier short or different-SHA runs do not close these gates.
- [ ] Obtain separate owner decisions on the reference environment, numeric baseline, absolute limits and whether/when any **mandatory** Performance Gate may be enabled. Until then it stays **DISABLED**.

Do not change the frozen QA004-QA005 Source-V1 anchor (`9b7b2318f73d173a99d22489042f84f8ce58b745`), historical numerical evidence or benchmark acceptance policy as a side effect of documentation cleanup.

## Release-specific approval and publication gates

- [ ] Obtain **fresh owner approval for final GA** of the current [`Unreleased` changelog](../CHANGELOG.md), including default V2, NUL-free Bind, the V2 20-byte/V1 19-byte SystemID contract, and post-approval feature/diagnostics/example changes. The distinct `v1.0.0-rc.1` integration snapshot is authorized, not a substitute for final GA notes acceptance.
- [ ] Independently verify protected-main required checks and strict/up-to-date / administrator-bypass policy through an authorized administrator. The connected GitHub integration cannot read all classic branch-protection settings; do not claim full administrator-level verification from `protected=true` alone.
- [ ] Verify the intended publisher's tag and GitHub Release permissions and the absence/conflict of an existing `v1.0.0` tag at the **time of publication**. The GitHub Releases listing contained **no published release** at this review; tag existence was not independently rechecked.
- [ ] For **final `v1.0.0` GA only**, recheck the exact proposed GA tag-target SHA, required CI and approved qualification evidence, then obtain **separate explicit owner permission** to publish. Do not create the GA tag or GitHub Release until these conditions hold. The RC tag's narrower permission and preflight are separate.

**This document by itself creates no tag or GitHub Release.** The separately authorized RC tag workflow is gated on exact-main checks and its result must be independently verified. No final `v1.0.0` tag/Release is authorized; keep future changes under `Unreleased`. The prior Phase 9 checklist [P9-06](../.codex/TASKS.md) remains open.

## Documentation map

- Current API and usage: [API](API.md), [Configuration](CONFIGURATION.md), [Lifecycle](LIFECYCLE.md), [Protocol](PROTOCOL.md), [Examples](../examples/README.md).
- Measurement methodology, exclusions and dated numeric evidence: [Performance qualification](PERFORMANCE_QUALIFICATION.md).
- Historical implementation decisions and task evidence: [`.codex/`](../.codex/README.md). Historical files can say “not merged” or “PR unauthorized” because they describe a **past checkpoint**; this page records the 2026-11-19 merged state.
