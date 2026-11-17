# neta-go — Approved main branch protection; operator application

**Date:** 2026-11-17  
**Repository:** `majiddarvishan/neta-go`  
**Branch:** `main`  
**Policy decision:** OWNER APPROVED  
**GitHub enforcement:** NOT YET APPLIED / NOT VERIFIED  
**Last observed protected flag:** `false` at repository HEAD `b3fad24938dd689cd270b6954a3a98e37f630614`

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

The connected GitHub integration exposes repository reads and Git data writes but **does not expose branch-protection mutation**. Even the classic protection read endpoint returned `403 Resource not accessible by integration`; therefore this document is NOT evidence that protection was applied.

A repository administrator with an authorized `gh` CLI session and branch administration permission may execute the following **once**. The command will update branch protection and should be treated as a repository configuration change. It supplies only the two owner-approved required checks; it does not create tags or releases.

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

Expected: `strict=true`, both exact required check strings, `enforce_admins=true`. Additionally confirm that `main` branch metadata reports `protected=true` and a new PR actually shows both checks as required. If GitHub returns an error (including plan/permission restrictions), record it instead of marking the task complete.

GitHub Release configuration still needs an owner/admin review. No change to release settings is authorized by this branch protection decision.

## 5. Scope and release boundary

- Approved **policy** does not mean **applied configuration**. Do not check off the implementation/verification gate until GitHub reports the actual rule.
- `P9-05`: in progress, pending protection application/verification and release-settings review.
- `P9-06`: NOT AUTHORIZED. Neither this decision nor the one-time command creates a V1 tag, GitHub Release or publication permission.
- No production Go source, public API, wire protocol, configuration defaults, test suite, workflow or local `stash@{0}` is changed by documenting this operation.
