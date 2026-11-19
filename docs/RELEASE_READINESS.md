# neta-go release readiness

**Status (reviewed 2026-11-20): `v1.0.0` is NOT approved for publication.** Development is merged to protected `main`, but QA-004/QA-005 reference qualification and independent release authorization remain open. This page summarizes the *current* release gates; dated engineering/owner evidence remains in [`.codex/`](../.codex/README.md).

**Reviewed `main` SHA:** `fb57668e182dc102f94044393b3d8f2a33aeddae`, merged through [PR #24](https://github.com/majiddarvishan/neta-go/pull/24) on 2026-11-19. This is an evidence snapshot, **not** an approved release tag target: re-evaluate the actual head and checks before any publication.

## Readiness at a glance

| Scope | Current disposition | Evidence / remaining boundary |
|---|---|---|
| Development-first scope | **COMPLETE — 16/16** | 6 DEV tasks + 10 new runnable examples, added to the 2 existing examples; [Development closeout](../.codex/NETA_GO_DEV_READY_FOR_QUALIFICATION_2026-11-19.md) and [example guide](../examples/README.md) |
| QA-007 callback/close semantics | **IMPLEMENTED AND MERGED** | Completion preservation, recovered callback panic diagnostics, pre-admission OnClose credits; [Lifecycle](LIFECYCLE.md), [approved contracts](../.codex/NETA_GO_QA_004_005_007_APPROVED_CONTRACTS_2026-11-18.md) |
| Main-branch correctness CI | **PASS at reviewed SHA** | Go and C++ wire workflows linked below |
| Hosted performance workflow | **PASS, ADVISORY ONLY** | No approved reference host/baseline or acceptance threshold; **not** a QA-004/QA-005 qualification PASS |
| QA-004 pending-timeout P99 | **OPEN / NOT QUALIFIED** | Stable reference environment, historical comparator and owner-approved absolute SLA still required |
| QA-005 performance acceptance | **OPEN / NOT QUALIFIED** | 8 Core / 24 Full qualified workload comparison, ten matched baseline/candidate pairs, accepted metrics and absolute thresholds pending |
| MEM-004 / MEM-005 / MEM-006 | **OPEN / PARTIAL_INCONCLUSIVE** | Full attributable retained memory, observer overhead/temporal accuracy and qualified long soak not accepted |
| Mandatory performance gate | **DISABLED** | No authorization or approved numerical limits to activate it |
| Release/tag/publication | **NOT AUTHORIZED** | Revised `Unreleased` notes and release-specific checks need fresh owner approval |

A merged feature, a green CI run or a completed Development checklist is **not** a release approval.

## Verified integration and CI evidence

- [PR #24](https://github.com/majiddarvishan/neta-go/pull/24) merged the 2026-11-18 feature to protected `main` with the two required checks passing, along with the bounded V1/V2 parser-mutation workflow. The merge commit's tree matches the complete Feature source tree; the temporary integration commit did not change files.
- **On the merged `main` SHA `fb57668e182dc102f94044393b3d8f2a33aeddae`:** [Go CI 44547311083](https://github.com/majiddarvishan/neta-go/actions/runs/44547311083) **PASS**; [Pinex C++ wire compatibility 44547311177](https://github.com/majiddarvishan/neta-go/actions/runs/44547311177) **PASS**; [Performance Benchmarks 44547311004](https://github.com/majiddarvishan/neta-go/actions/runs/44547311004) **PASS as hosted advisory CI only**.
- **On the reviewed PR head `920d37ce7a746ef9878c0be9f13c8714d0c289ff`:** [Go 44547010827](https://github.com/majiddarvishan/neta-go/actions/runs/44547010827), [C++ wire 44547010760](https://github.com/majiddarvishan/neta-go/actions/runs/44547010760) and [bounded parser mutation 44547010745](https://github.com/majiddarvishan/neta-go/actions/runs/44547010745) **PASS**.
- **Examples:** the [bounded 12-example smoke 44542311954](https://github.com/majiddarvishan/neta-go/actions/runs/44542311954) **PASS** includes configuration, negative cases, explicit V1, panic/timeout diagnostics and a real Server kill/restart and Client reconnect. The [Development closeout](../.codex/NETA_GO_DEV_READY_FOR_QUALIFICATION_2026-11-19.md) records source-tree parity with the merged Feature contents; this does **not** replace full qualification.
- The C++ workflow checks wire compatibility; it does not make C++ a runtime dependency of the Go library. [Supported Go/platform baseline](SUPPORT.md) remains Go 1.23+ and CI-tested Ubuntu 24.04 / Linux amd64.

Earlier green feature/`main` runs and completed QA-001/002/003/006/008/009 remain valid **historical evidence on their own exact SHAs**, not substitutes for a new release preflight. See [QA tasks](../.codex/TASKS.md) and [Performance qualification protocol](PERFORMANCE_QUALIFICATION.md).

## Open engineering qualification gates

- [ ] **QA-004:** approve a stable reference environment and executable historical/candidate comparison; run/validate the specified pending-deadline terminal-settlement P99 matrix and separately approve an **absolute** P99 SLA. Existing A/A and hosted results are advisory, not cross-version qualification.
- [ ] **QA-005:** approve the reference machine, equivalent benchmark source and immutable numeric baseline; collect ten alternating matched baseline/candidate pairs for the approved 8 Core / 24 Full scenarios, including 95% statistical checks and any separately approved absolute limits.
- [ ] **MEM-004:** finish the owner-approved Hybrid-B owned-byte attribution model with category coverage, alias/lifetime consistency, and defensible concurrent observation/sampling; existing F1–F5/M1/D1 adapters are only partially attributable. Do not emit a canonical `retained-attributed-B` from an incomplete snapshot.
- [ ] **MEM-005 / MEM-006:** qualify the observer's overhead and temporal validity, and perform/accept the required 30-minute memory-stability soak on the approved reference machine and reviewed source. Earlier short or different-SHA runs do not close these gates.
- [ ] Obtain separate owner decisions on the reference environment, numeric baseline, absolute limits and whether/when any **mandatory** Performance Gate may be enabled. Until then it stays **DISABLED**.

Do not change the frozen QA004-QA005 Source-V1 anchor (`9b7b2318f73d173a99d22489042f84f8ce58b745`), historical numerical evidence or benchmark acceptance policy as a side effect of documentation cleanup.

## Release-specific approval and publication gates

- [ ] Obtain **fresh owner approval** of the current [`Unreleased` changelog](../CHANGELOG.md), including default V2, NUL-free Bind, the V2 20-byte/V1 19-byte SystemID contract, and post-approval feature/diagnostics/example changes. Approval of an older draft does not approve amended notes.
- [ ] Independently verify protected-main required checks and strict/up-to-date / administrator-bypass policy through an authorized administrator. The connected GitHub integration cannot read all classic branch-protection settings; do not claim full administrator-level verification from `protected=true` alone.
- [ ] Verify the intended publisher's tag and GitHub Release permissions and the absence/conflict of an existing `v1.0.0` tag at the **time of publication**. The GitHub Releases listing contained **no published release** at this review; tag existence was not independently rechecked.
- [ ] Recheck the **exact proposed tag-target SHA**, required CI and approved qualification evidence, then obtain **separate explicit owner permission** to publish. Do not create the tag or GitHub Release until these conditions hold.

**No tag or GitHub Release is created by this document.** Keep `CHANGELOG.md` under `Unreleased` pending approval. The prior Phase 9 checklist [P9-06](../.codex/TASKS.md) remains open.

## Documentation map

- Current API and usage: [API](API.md), [Configuration](CONFIGURATION.md), [Lifecycle](LIFECYCLE.md), [Protocol](PROTOCOL.md), [Examples](../examples/README.md).
- Measurement methodology, exclusions and dated numeric evidence: [Performance qualification](PERFORMANCE_QUALIFICATION.md).
- Historical implementation decisions and task evidence: [`.codex/`](../.codex/README.md). Historical files can say “not merged” or “PR unauthorized” because they describe a **past checkpoint**; this page records the 2026-11-19 merged state.
