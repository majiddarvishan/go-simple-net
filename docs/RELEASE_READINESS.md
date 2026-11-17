# V1 Release Readiness

**Status: IN PROGRESS. No V1 tag or GitHub Release is authorized or published.**

## Completed gates

- [x] Architecture and public API freeze retained.
- [x] Phases 1–8 complete.
- [x] P9-00 tracker/status reconciliation complete.
- [x] P9-01 Client and P9-02 Server examples added and compiled in Go CI.
- [x] P9-03 Public API, configuration, lifecycle, errors and Quick Start documentation added.
- [x] P9-04 Pinex 1.5.0 compatibility and verification evidence documented.
- [x] Go correctness CI and Pinex golden compatibility CI pass on the final documentation/GoDoc HEAD.

## P9-05 evidence

- Final Go CI [44119107169](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169): PASS; Pinex Wire Compatibility [44119107078](https://github.com/majiddarvishan/neta-go/actions/runs/44119107078): PASS on test-hardened commit `958601ee55dd6229021e36135a017527dfa8f778`.
- `neta`: 79.7% statement coverage; `internal/wire`: 90.3%; examples: 0.0% package-local coverage (no tests in those main packages). No threshold is approved.
- Loopback smoke verified Request/Response, one-way event reception and graceful shutdown of both programs. Both example binaries built.
- Artifact: [Final Phase 9 coverage and smoke evidence](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169/artifacts/14344846005).

## P9-05 gates still to close

- [x] Paired Client/Server runtime smoke and graceful shutdown pass in CI.
- [x] Record `go test -cover ./...` output; no minimum percentage has been approved.
- [x] Owner approved Go 1.23 minimum and Linux/amd64 (Ubuntu 24.04) CI-tested support baseline on 2026-11-17; [SUPPORT.md](SUPPORT.md) records the decision. Other platforms remain unverified.
- [x] Copyright holder **Majid Darvishan** and year **2026** confirmed by the owner on 2026-11-17; Apache-2.0 `LICENSE` copyright notice updated. The owner explicitly permits commercial use under Apache-2.0; no non-commercial restriction was added.
- [x] Owner reviewed and approved the current [CHANGELOG.md](../CHANGELOG.md) Unreleased entries on 2026-11-17 as the V1 release-notes draft. Preserve Unreleased until a separately authorized release.
- [ ] Resolve branch protection and GitHub Release settings review: `main` branch metadata reports `protected=false`; repository rulesets API reports `[]`; full classic protection endpoint returns HTTP 403. No tag or release exists. Owner/admin should verify the intended protection policy and release configuration. Do not claim the branch is protected.
- [x] Owner confirmed semantic version **`v1.0.0`** and approved the current Unreleased changelog content for planned V1 on 2026-11-17. This is a release **candidate decision only**, not permission to create a tag or publish.

## P9-05 release-readiness verification matrix — 2026-11-17

| Area | Status | Evidence / limitation |
|---|---|---|
| Public contract freeze and Phase 9 docs/examples | PASS | P9-00..P9-04 complete; no production behavior change in approval commits |
| Go correctness, race, examples and smoke | PASS | [Go CI 44159858260](https://github.com/majiddarvishan/neta-go/actions/runs/44159858260) on `11d2a0a9508b59e603f038e14772d29bb1bb17ea` |
| Pinex wire compatibility | PASS | [Pinex CI 44159858233](https://github.com/majiddarvishan/neta-go/actions/runs/44159858233) on the same commit |
| Go/platform minimum | OWNER APPROVED | Go 1.23; Ubuntu 24.04 / Linux amd64 verified, other targets not claimed |
| LICENSE and commercial use | OWNER APPROVED | Apache-2.0; Copyright 2026 Majid Darvishan |
| Changelog and version proposal | OWNER APPROVED | Current Unreleased entries and intended `v1.0.0` approved, publication not authorized |
| Main branch protection | NOT CONFIGURED IN VISIBLE METADATA / ADMIN DETAILS NOT READABLE | Branch API: `protected=false`; rulesets: `[]`; classic-protection endpoint HTTP 403; requires owner/admin review |
| GitHub Release configuration | NOT VERIFIED | Tags and Releases each: 0; release settings are not exposed by the available integration |
| V1 tag and GitHub Release | NOT RUN / NOT AUTHORIZED | P9-06 requires separate explicit owner approval |

**P9-05 remains IN PROGRESS**, pending resolution or explicit owner acceptance of the branch-protection/release-configuration review. Successful CI and approval of `v1.0.0` are not release authorization.

## Separate P9-06 release gate

- [ ] All P9-05 gates accepted and recorded in [TASKS.md](../TASKS.md).
- [ ] Final main HEAD passes Go correctness, example smoke, coverage capture and Pinex compatibility CI.
- [ ] Owner explicitly approves creating the agreed V1 tag and GitHub Release.

**Do not create a tag or release from this checklist alone.** The general Phase 9 authorization does not authorize P9-06.
