# neta-go release readiness

**Status:** The `v1.0.0` candidate is **not published**. Engineering evidence is linked below with its exact tested commits. Publication requires fresh verification on the actual tag-target SHA and separate explicit owner authorization.

## Verified engineering gates

- [x] Public Client/Server/Session API and resource-admission behavior implemented.
- [x] Client and Server examples compiled and exercised together: Request/Response, one-way delivery and graceful shutdown.
- [x] Go correctness, vet, race, coverage and example smoke CI passed on protected `main` commit `1f2fb281f26e9502b457a98f6fb4e5fc9270cc45`: [Go run 44261035349](https://github.com/majiddarvishan/neta-go/actions/runs/44261035349).
- [x] Wire golden-vector verification passed on the same protected-main commit: [wire CI run 44261035440](https://github.com/majiddarvishan/neta-go/actions/runs/44261035440).
- [x] V2 default, NUL-free V2 Bind, V2 20-byte SystemID limit, V1 encoding and unchanged 10-byte Header are covered by tests.
- [x] `main` is protected and both mandatory CI checks were enforced on previously merged Pull Requests.
- [x] Minimum Go toolchain 1.23 and CI-tested Ubuntu 24.04 / Linux amd64 documented in [SUPPORT.md](SUPPORT.md).
- [x] Apache-2.0 license, Copyright 2026 Majid Darvishan, is committed.

The separate C++ golden-vector workflow is an internal wire regression check. Its check name remains unchanged because branch protection requires that exact context; the library does not require a C++ dependency.

## Pending publication gates

- [x] Repository naming/documentation cleanup merged in PRs #10–#13 (historical CI in the PRs); current protected-main Go/Wire CI is linked above.
- [x] Verify real two-direction TCP interoperability against pinned C++ 1.5.0 via a **private** workflow (normal and `-race`): [run 44260379077](https://github.com/majiddarvishan/pinex/actions/runs/44260379077) on neta-go `0824a70127594df72f78d1048cc360e33f57bd5b`. Details in the [QA-001 evidence](../.codex/NETA_GO_QA001_REAL_TCP_VERIFICATION_2026-11-18.md). This is **not** a required status gate on public neta-go PRs.
- [x] Validate QA-006 terminal Accept error handling on protected `main`: [Go CI 44259964992](https://github.com/majiddarvishan/neta-go/actions/runs/44259964992) and [Wire CI 44259965112](https://github.com/majiddarvishan/neta-go/actions/runs/44259965112) PASS.
- [x] Complete QA-008 cleanup (one Bind encoding per client handshake, input-error connection ownership preserved, current release evidence refreshed); verify the exact protected-main CI after PR merge.
- [ ] Complete the [independent QA audit](../.codex/NETA_GO_SENIOR_CODE_AUDIT_2026-11-18.md) disposition: QA-001/002/003/006/008 are closed; QA-004 remains measured without an owner SLA decision and QA-005/007/009 remain open.
- [ ] Obtain **fresh owner approval** for the revised [Unreleased changelog](../CHANGELOG.md), including V1/V2 behavior. An earlier approval does not cover these changes.
- [ ] Independently inspect strict/up-to-date and administrator-bypass branch-protection settings (full settings are not readable through the connected integration).
- [ ] Confirm GitHub Release creation permissions and publishing settings under the intended publisher account.
- [ ] Recheck the intended tag target SHA and all required CI immediately before publication.
- [ ] Obtain **separate, explicit owner permission** to create the `v1.0.0` tag and GitHub Release.

Keep the changelog under `Unreleased` and do not create a tag or Release without the final permission.

Historical engineering decisions and test evidence remain in [`.codex/`](../.codex/README.md); they are not part of the public API contract.
