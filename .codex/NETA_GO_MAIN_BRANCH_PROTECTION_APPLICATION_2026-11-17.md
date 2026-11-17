# neta-go — Approved main branch protection; operator application

**Date:** 2026-11-17  
**Repository:** `majiddarvishan/neta-go`  
**Branch:** `main`  
**Policy decision:** OWNER APPROVED  
**GitHub enforcement:** ACTIVE / REQUIRED CHECKS VERIFIED by branch API 2026-11-17; strict and admin-bypass options NOT independently verified  
**Last observed protected flag:** `true` at repository HEAD `b992beac7495934ce43111bd3a692ec272c8195d`

## 1. Exact required GitHub Actions check names

The owner approved protecting `main` and requiring both Go CI and Pinex Wire Compatibility to pass for future changes. **Required checks are the individual job/check-run names, not workflow names**:

1. `test` — job in `.github/workflows/go.yml` (workflow display name `Go`); executes `make verify` and `make phase9`.
2. `Verify Go wire codec against pinned Pinex C++ 1.5.0` — job in `.github/workflows/pinex-wire-compat.yml` (workflow display name `Pinex Wire Compatibility`).

These exact strings were confirmed from GitHub check runs on the Phase 9 commit `b3fad24938dd689cd270b6954a3a98e37f630614`. Both workflows run on `pull_request` and `push` to `main`. The Phase 8 performance workflow uses a different job name.

Pre-approval evidence: [Go run 44161319800](https://github.com/majiddarvishan/neta-go/actions/runs/44161319800) PASS; [Pinex run 44161319863](https://github.com/majiddarvishan/neta-go/actions/runs/44161319863) PASS, both on the same commit.

## 2. Protection settings to apply

- Protect branch `main`.
- Require **both** exact status checks to pass before accepting changes.
- Set status checks to **strict**: a proposed change must be current with the target branch.
- Enforce restrictions for administrators too, so required checks are not silently bypassed.
- Block force pushes and branch deletion.
- Do **not** introduce mandatory reviewer approvals, a merge queue, CODEOWNERS reviews or additional checks without owner approval.

**Workflow impact:** With required checks enforced for administrators, direct writes to `main` may be rejected until the checks have passed. Use a feature branch + Pull Request workflow for future source changes. This policy is distinct from mandatory reviewer approvals, which remain disabled.

## 3. One-time application by an authenticated repository administrator

The connected GitHub integration does not expose branch-protection mutation. The owner **already applied protection using GitHub's web UI**; repository branch metadata now independently confirms `protected=true` and both mandatory required checks. The classic protection read still returns `403 Resource not accessible by integration`, so strict/admin-bypass details cannot be independently confirmed. **Do not re-run the example PUT command** unless intentionally replacing the current existing rule.

For historical/reference use only: the owner has already enabled protection in the browser. The following example **overwrites** branch protection settings and must **not** be executed just to check their status. It is kept only as a record of the originally approved API configuration and is NOT necessary now that branch protection is active.

```bash
gh api --method PUT \
  -H "Accept: application/vnd.github+json" \
  repos/majiddarvishan/neta-go/branches/main/protection \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "test",
      "Verify Go wire codec against pinned Pinex C++ 1.5.0"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
JSON
```

**Alternative UI:** GitHub repository **Settings → Branches → Add branch protection rule**, target `main`, enable `Require status checks to pass before merging` and `Require branches to be up to date before merging`, select the two exact check names above, turn on admin enforcement (wording may vary), and save.

## 4. Verification — required before closing P9-05

An authenticated admin can inspect the full rule using:

```bash
gh api repos/majiddarvishan/neta-go/branches/main/protection \
  --jq '{strict: .required_status_checks.strict, contexts: .required_status_checks.contexts, enforce_admins: .enforce_admins.enabled}'
```

GitHub branch API now **confirms** `main.protected=true`, status-check `enforcement_level=everyone` and two exact required checks (`test`, `Verify Go wire codec against pinned Pinex C++ 1.5.0`). Both CI jobs PASS on `b992beac7495934ce43111bd3a692ec272c8195d` ([Go](https://github.com/majiddarvishan/neta-go/actions/runs/44163815612), [Pinex](https://github.com/majiddarvishan/neta-go/actions/runs/44163815661)). **End-to-end PR enforcement is now independently VERIFIED:** [PR #2](https://github.com/majiddarvishan/neta-go/pull/2) showed `mergeable_state=blocked` when required checks were in progress, then `mergeable_state=clean` after both checks passed; squash merge succeeded to `main` as `b5839a42b0de99f1ce9abf278e1fba3ff66d6617`. Go [44165799994](https://github.com/majiddarvishan/neta-go/actions/runs/44165799994) and Pinex [44165800103](https://github.com/majiddarvishan/neta-go/actions/runs/44165800103) post-merge CI both PASS on that SHA. **Still not independently verifiable from the current connector:** exact `strict=true` and administrator bypass flags; owner/admin should inspect their GitHub UI settings. `enforcement_level=everyone` is reported for required status checks.

GitHub Release configuration still needs an owner/admin review. No change to release settings is authorized by this branch protection decision.

## 5. Scope and release boundary

- Approved policy **is now active**: GitHub branch API reports `protected=true` with both required contexts. The remaining detailed protection options (strict and admin-bypass) require independent owner/admin verification.
- `P9-05`: in progress, pending only detailed strict/admin settings and GitHub Release publication settings review; real PR enforcement verified via merged PR #2.
- `P9-06`: NOT AUTHORIZED. Neither this decision nor the one-time command creates a V1 tag, GitHub Release or publication permission.
- No production Go source, public API, wire protocol, configuration defaults, test suite, workflow or local `stash@{0}` is changed by documenting this operation.
