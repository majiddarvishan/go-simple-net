# neta-go release readiness

**Status:** The `v1.0.0` candidate is **not published**. Engineering evidence from earlier commits is recorded below. Publication requires current-commit verification and explicit owner authorization.

## Verified engineering gates

- [x] Public Client/Server/Session API and resource-admission behavior implemented.
- [x] Client and Server examples compiled and exercised together: Request/Response, one-way delivery and graceful shutdown.
- [x] Go correctness, vet, race, coverage and example smoke CI passed on protected `main` commit `0fadd380d558028a85333fa044eeb4c93ae5c11f`: [Go run 44202868979](https://github.com/majiddarvishan/neta-go/actions/runs/44202868979). Additional QA-002/QA-003 regression tests passed in PR #15: [Go run 44208242381](https://github.com/majiddarvishan/neta-go/actions/runs/44208242381).
- [x] Wire golden-vector verification passed on the same protected-main commit: [wire CI run 44202868919](https://github.com/majiddarvishan/neta-go/actions/runs/44202868919). PR #15: [wire run 44208242378](https://github.com/majiddarvishan/neta-go/actions/runs/44208242378).
- [x] V2 default, NUL-free V2 Bind, V2 20-byte SystemID limit, V1 encoding and unchanged 10-byte Header are covered by tests.
- [x] `main` is protected and both mandatory CI checks were enforced on previously merged Pull Requests.
- [x] Minimum Go toolchain 1.23 and CI-tested Ubuntu 24.04 / Linux amd64 documented in [SUPPORT.md](SUPPORT.md).
- [x] Apache-2.0 license, Copyright 2026 Majid Darvishan, is committed.

The separate C++ golden-vector workflow is an internal wire regression check. Its check name remains unchanged because branch protection requires that exact context; the library does not require a C++ dependency.

## Pending publication gates

- [x] Repository naming/documentation cleanup merged in PRs #10–#13 and verified on protected `main` (`0fadd380d558028a85333fa044eeb4c93ae5c11f`).
- [ ] Complete [independent QA audit](../.codex/NETA_GO_SENIOR_CODE_AUDIT_2026-11-18.md) remediation and requalification: QA-001 real C++ E2E is still unverified, QA-002/QA-003 fixes are in PR #15, and QA-004..QA-009 await disposition.
- [ ] Obtain **fresh owner approval** for the revised [Unreleased changelog](../CHANGELOG.md), including V1/V2 behavior. An earlier approval does not cover these changes.
- [ ] Independently inspect strict/up-to-date and administrator-bypass branch-protection settings (full settings are not readable through the connected integration).
- [ ] Confirm GitHub Release creation permissions and publishing settings under the intended publisher account.
- [ ] Recheck the intended tag target SHA and all required CI immediately before publication.
- [ ] Obtain **separate, explicit owner permission** to create the `v1.0.0` tag and GitHub Release.

Keep the changelog under `Unreleased` and do not create a tag or Release without the final permission.

Historical engineering decisions and test evidence remain in [`.codex/`](../.codex/README.md); they are not part of the public API contract.
