# v1.0.0 official release owner authorization — 2026-11-20

**Explicit owner instruction:** delete excess merged branches; publish **the same code** with the **official `v1.0.0` version** rather than keeping only the `v1.0.0-rc.1` candidate.

## Authorized now

1. Base the release on current protected `main` `433e4bf3678f4a9ccf57b33e051a68c0eb51015b` and **make no Go source, benchmark, test, CI qualification, wire-protocol or public API changes**. Update version and release-documentation metadata only.
2. Merge an owner-approved release-preparation PR after required Go and C++ Wire checks PASS. For `v1.0.0`, use the **exact final protected-main postmerge commit** so `VERSION`, Changelog and Git tag agree.
3. Once the same commit's two postmerge checks succeed, create **one annotated immutable `v1.0.0` Git tag** and **non-prerelease GitHub Release** using approved [Release notes](../docs/RELEASE_V1.0.0_NOTES.md). Fail closed on tag collisions, unstable main, failed CI, publication permission errors or mismatched target. Do not force-move/recreate tags.
4. Preserve the prior `v1.0.0-rc.1` tag and its original commit. The promotion changes **documentation/version metadata** only, so runtime Go files and existing V1/V2 wire contracts match the RC.
5. After GA publication is independently verified, remove obsolete merged PR branches (the previously inspected docs/release branches plus the GA publication/documentation branches). Delete only refs with a **merged PR targeting main and exact unchanged PR head SHA**; skip unknown, active, protected, diverged or unmatched refs, never touch `main` or any tag.

## Mandatory disclosure, despite official version

The user now authorizes `v1.0.0` publication **before** Phase Q readiness. This overrides the *publication hold* from older owner policies, **NOT** technical acceptance of missing data. The release must clearly disclose that Q-ENV-01, QA-004/005, MEM-004/005/006 and Q-COR independent tests remain pending/INCONCLUSIVE; numeric baseline and absolute SLA not approved, performance gate DISABLED. Never mark those tasks [x] just because the release is published. After GA publication `P9-06` release publication can close; Q qualification still remains open. Do not make an ungrounded claim of load/performance suitability.

**Existing integration RC:** `v1.0.0-rc.1` annotated tag target `0bd50760a9200194f9ae0088d26751d0f6ee00c7`, verified and not to be modified. Exact GA SHA unknown until release-preparation PR squash merge and must be verified from GitHub, not guessed.

**Clean-up boundary:** at preflight 2026-11-20, 11 non-main branches exist; all are preserved source heads of merged PR #28–#38, and no open PRs exist. All have squash-merged changes in protected main. Avoid deleting unmerged local/unknown Git refs or any future actively edited branch. Release work branch must itself merge before deleting.
