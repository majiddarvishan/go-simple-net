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
- [ ] Owner approves the proposed Go 1.23 minimum and Linux/amd64 tested-platform statement in [SUPPORT.md](SUPPORT.md).
- [ ] Confirm the intended copyright holder and year. The Apache-2.0 `LICENSE` currently retains the template placeholder `[yyyy] [name]`; do not guess these values.
- [ ] Review the Unreleased entries in [CHANGELOG.md](../CHANGELOG.md).
- [ ] Verify main branch protection/ruleset and release settings. The GitHub integration could not read branch protection (HTTP 403); its state is unknown, not confirmed disabled.
- [ ] Confirm the intended semantic version and final release notes. `v1.0.0` is a candidate only, not an approved tag.

## Separate P9-06 release gate

- [ ] All P9-05 gates accepted and recorded in [TASKS.md](../TASKS.md).
- [ ] Final main HEAD passes Go correctness, example smoke, coverage capture and Pinex compatibility CI.
- [ ] Owner explicitly approves creating the agreed V1 tag and GitHub Release.

**Do not create a tag or release from this checklist alone.** The general Phase 9 authorization does not authorize P9-06.
