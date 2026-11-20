# Official GA release verified — v1.0.0, 2026-11-20

**Official GA release verified** independently from GitHub, after successful exact-main required checks.

- Owner-authorized [publication PR #39](https://github.com/majiddarvishan/neta-go/pull/39) squash-merged at **source SHA `d8437f36ec812a596bada1398c7af9b530a18118`**.
- GitHub annotated Tag **`v1.0.0`**, object SHA `b5c5ce7b9fe6c3ea4fceec0eb98a4621b749e28b`, dereferences to **commit SHA `d8437f36ec812a596bada1398c7af9b530a18118`**.
- [Official GitHub Release](https://github.com/majiddarvishan/neta-go/releases/tag/v1.0.0) exists, `draft=false`, `prerelease=false`.
- [Publication Actions run 44764649618](https://github.com/majiddarvishan/neta-go/actions/runs/44764649618) **completed/success**, with both protected-main checks `test` ([Go run 44764649879](https://github.com/majiddarvishan/neta-go/actions/runs/44764649879)) and `Verify Go wire codec against pinned Pinex C++ 1.5.0` ([C++ run 44764648693](https://github.com/majiddarvishan/neta-go/actions/runs/44764648693)) **completed/success on the same SHA**.
- Earlier `v1.0.0-rc.1` annotated tag remains unchanged and continues to point to `0bd50760a9200194f9ae0088d26751d0f6ee00c7`.
- Identical production Go/wire/benchmark source between RC and GA was independently enforced by the release-publisher job.

**Qualification status is NOT implicitly PASS:** `Q-ENV-01`, `Q-COR-01/02/03/04`, `QA-004/005`, `MEM-004/005/006` remain open/INCONCLUSIVE. No approved absolute SLA, canonical retained-attributed memory or mandatory Performance Gate. The [owner-approved four-week parallel test plan](NETA_GO_PHASE_Q_PARALLEL_EXECUTION_PLAN_2026-11-20.md) is unchanged.

**Release task accounting:** `P9-06` (actual official tag and Release publication) **[x] CLOSED by explicit owner override**, but `R-01` and `R-02` remain unresolved release-review packages; no fictional QA acceptance is recorded. Normalized remaining top-level work packages: **19 = 10 Qualification + 7 Owner Decisions + 2 Release**.

**Obsolete branch cleanup authorization:** [Decision Log §85](DECISION_LOG.md) permits deletion of the 11 unchanged merged PR source branches #28–#38 plus the GA PR #39 branch and this verification PR's branch **only after** this marker is merged and the cleanup workflow verifies the official Release. The [cleanup workflow](../.github/workflows/cleanup-merged-release-branches.yml) must check exact merged PR SHA and protection on each allowlisted ref before DELETE, and refuse unmatched/advanced refs. This marker is **the single deliberate one-shot trigger** for that cleanup; by itself it is not proof that any branch was deleted.

Official Go module pin:

```bash
go get github.com/majiddarvishan/neta-go@v1.0.0
```
