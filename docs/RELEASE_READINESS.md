# V1 Release Readiness

**Status: P9-05 qualified engineering review COMPLETE; P9-06 NOT AUTHORIZED. No V1 tag or GitHub Release is published.**

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
- [x] Owner approved main protection requiring two exact checks: `test` (Go) and `Verify Go wire codec against pinned Pinex C++ 1.5.0` (Pinex). Both jobs are present, pass on [Go run 44161319800](https://github.com/majiddarvishan/neta-go/actions/runs/44161319800) / [Pinex run 44161319863](https://github.com/majiddarvishan/neta-go/actions/runs/44161319863), and are triggered on pull requests.
- [x] **Applied and verified** `main` branch protection: GitHub branch metadata reports `protected=true`, `required_status_checks.enforcement_level=everyone`, and the two exact mandatory contexts `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0`. Both checks PASS on [Go 44163815612](https://github.com/majiddarvishan/neta-go/actions/runs/44163815612) / [Pinex 44163815661](https://github.com/majiddarvishan/neta-go/actions/runs/44163815661) at `b992beac7495934ce43111bd3a692ec272c8195d`.
- [x] Verify real PR enforcement: [PR #2](https://github.com/majiddarvishan/neta-go/pull/2) initially `blocked` while both checks were pending, then `clean` after success and successfully squash-merged as `b5839a42b0de99f1ce9abf278e1fba3ff66d6617`. Go [44165799994](https://github.com/majiddarvishan/neta-go/actions/runs/44165799994) and Pinex [44165800103](https://github.com/majiddarvishan/neta-go/actions/runs/44165800103) post-merge CI PASS on this same HEAD.
- [x] Review detailed protected-branch controls with qualified visibility: owner reported completing the requested browser settings; GitHub reports protection active and mandatory checks at `enforcement_level=everyone`, including a real blocked-then-merged PR. The **exact strict and bypass flags are NOT independently verified** due to classic-protection API HTTP 403. Carry admin settings verification forward as a mandatory P9-06 prepublication gate.
- [x] Review GitHub Releases without publishing: repository is public, authenticated connection's repo metadata reports admin permissions, Releases and tags list endpoints are accessible and empty, and no existing `v1.0.0` tag conflicts. **Create/publish permission is NOT tested**; this must be checked at P9-06 after explicit owner authorization, not now.
- [x] Owner confirmed semantic version **`v1.0.0`** and approved the current Unreleased changelog content for planned V1 on 2026-11-17. This is a release **candidate decision only**, not permission to create a tag or publish.

## P9-05 release-readiness verification matrix — 2026-11-17

| Area | Status | Evidence / limitation |
|---|---|---|
| Public contract freeze and Phase 9 docs/examples | PASS | P9-00..P9-04 complete; no production behavior change in approval commits |
| Go correctness, race, examples and smoke | PASS | Post-merge [Go CI 44165799994](https://github.com/majiddarvishan/neta-go/actions/runs/44165799994) on `b5839a42b0de99f1ce9abf278e1fba3ff66d6617` |
| Pinex wire compatibility | PASS | Post-merge [Pinex CI 44165800103](https://github.com/majiddarvishan/neta-go/actions/runs/44165800103) on the same commit |
| Go/platform minimum | OWNER APPROVED | Go 1.23; Ubuntu 24.04 / Linux amd64 verified, other targets not claimed |
| LICENSE and commercial use | OWNER APPROVED | Apache-2.0; Copyright 2026 Majid Darvishan |
| Changelog and version proposal | OWNER APPROVED | Current Unreleased entries and intended `v1.0.0` approved, publication not authorized |
| Main branch protection | ACTIVE / REAL PR ENFORCEMENT PASS; DETAILS PARTIAL | GitHub: `protected=true`, `enforcement_level=everyone`, contexts `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0`. [PR #2](https://github.com/majiddarvishan/neta-go/pull/2) blocked pending checks and merged after PASS. Strict-up-to-date and admin-bypass controls not directly exposed (HTTP 403). [Guide](../.codex/NETA_GO_MAIN_BRANCH_PROTECTION_APPLICATION_2026-11-17.md) |
| GitHub Release configuration | REVIEW COMPLETE / PUBLISH ABILITY NOT TESTED | Public repository, owner-account repo admin metadata, release and tag listings accessible and empty; actual release creation untested until P9-06 authorization, with mandatory prepublication permission/setting check |
| V1 tag and GitHub Release | NOT RUN / NOT AUTHORIZED | P9-06 requires separate explicit owner approval |

**P9-05 is COMPLETE as a qualified engineering release-readiness review.** Code correctness, compatibility, examples, post-merge CI, protected-branch/PR enforcement, support, copyright/license and version/changelog approvals are PASS. The exact `strict`/admin bypass toggles and actual GitHub Release creation permission are **not independently verified**; they are mandatory P9-06 prepublication checks, not silently treated as PASS. This closure is NOT permission to publish.

## PV-01 change after the previous P9-05 engineering review

- [ ] Requalify the new Go Session version setting and default V2 against the exact protected main HEAD, including Go/race, examples and Pinex CI.
- [ ] Owner review the added Unreleased CHANGELOG entry. The previous approved notes did not contain this new API/default.
- [x] Clearly distinguish local V2 configuration from an actual new V2 wire format: both selections still use the **same 10-byte Header and NUL-terminated Bind Body**. There is no automatic negotiation and no peer capability assertion.

## Separate P9-06 release gate

- [x] P9-05 engineering review completed with transparent release-time verification limits, recorded in [TASKS.md](../TASKS.md).
- [ ] Independently confirm strict branch up-to-date and administrator bypass controls through GitHub admin settings or an authorized branch-protection read; owner UI completion is recorded, but detailed API remains HTTP 403 to this connector.
- [ ] Verify release publication permissions and create-release settings under the authorized publishing account without publishing prematurely.
- [ ] Re-read the exact intended tag target HEAD from protected `main` and require final Go correctness, example smoke, coverage capture and Pinex compatibility CI PASS on that same HEAD.
- [ ] Owner **separately and explicitly** approves creating the intended `v1.0.0` tag and GitHub Release after reviewing the final release target SHA.

**Do not create a tag or release from this checklist alone.** The general Phase 9 authorization does not authorize P9-06.
