# Owner-approved pre-qualification integration tag: v1.0.0-rc.1

**Owner instruction: 2026-11-20.** Merge the completed work into `main`, assign a version and a Git tag so independent applications can adopt a pinned neta-go revision while the heavy Phase Q test plan continues.

**Chosen release identity:** `v1.0.0-rc.1` (**release candidate**, not general availability) for the **exact protected-main squash-merge commit** of the PR adding this file, the VERSION declaration and release notes. Tag MUST point to the reviewed, green CI commit, not an earlier source anchor or an unreviewed branch head. `go get github.com/majiddarvishan/neta-go@v1.0.0-rc.1` is the intended consumer command **after the actual Git tag exists**.

**Authorized:** merging the pre-release documentation and version metadata into `main`, creating **one immutable Git annotated tag** `v1.0.0-rc.1` for the final exact-main SHA after required postmerge Go and C++ compatibility check success, and using it for parallel integration.

**Not authorized by this request:** `v1.0.0` final/GA tag or GitHub Release, claiming complete QA-004/QA-005 qualification, claiming memory retention correctness or performance limits, changing production or protocol contracts, automatic enabling of mandatory Performance Gate, running heavy Phase Q benchmarks/fuzz/soaks, silently rewriting immutable Source-V1 anchor, force-moving/overwriting any tag, or merging stale/unreviewed branches.

**Acceptance and custody checks:** unique tag name and absence of tag collision; protected `main` exact SHA, `VERSION` and release-note identity match; postmerge checks `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0` SUCCESS on the **same SHA**; final tag must resolve to that SHA. A failed permission or CI check is not an authorization to bypass gates. Document any inability to publish and leave a precise manual fallback; **do not claim the tag exists unless independently verified through GitHub refs**.

**Scope accounting:** Phase D 16/16 and 14 runnable example commands remain already merged; no open PRs were observed at the preflight on 2026-11-20. Remaining **20 normalized top-level work packages** (10 qualification, 7 decisions, 3 release) are tracked in [TASKS](TASKS.md) and [Phase Q backlog](NETA_GO_PHASE_Q_EXECUTION_BACKLOG_2026-11-20.md). This integration candidate does not close final GA release work packages `R-01`, `R-02`, or `P9-06`.

**Test program:** [Phase Q parallel execution plan](NETA_GO_PHASE_Q_PARALLEL_EXECUTION_PLAN_2026-11-20.md); explicit test-run and source/tool edit authorizations remain separate.
