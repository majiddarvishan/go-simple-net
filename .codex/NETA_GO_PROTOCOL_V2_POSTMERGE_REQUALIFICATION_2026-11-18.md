# neta-go — Protocol V2 Post-Merge Release Requalification

**Date:** 2026-11-18  
**Status:** SOURCE CI VERIFIED; NEW OWNER CHANGELOG APPROVAL PENDING; RELEASE NOT AUTHORIZED  
**Repository:** `majiddarvishan/neta-go`  
**Protected main source HEAD:** `a0db0c5bfbfa364731f6b106f7f5b04de41d5ba2`  
**Scope:** PV-01-R, PV-02A-R, PV-02B-R, release-readiness evidence only.

## Evidence obtained from GitHub

1. [PR #8](https://github.com/majiddarvishan/neta-go/pull/8) was merged into `main` at `a0db0c5bfbfa364731f6b106f7f5b04de41d5ba2`; earlier V2 NUL-free Bind [PR #7](https://github.com/majiddarvishan/neta-go/pull/7) is its ancestor.
2. [Go workflow 44187548178](https://github.com/majiddarvishan/neta-go/actions/runs/44187548178) completed **success** on this *exact* SHA. Its `test` job ran and passed:
   - `Run shared correctness checks` (repository's shared Go correctness / vet / unit / race gate);
   - `Run coverage and paired example smoke`;
   - `Upload correctness and Phase 9 evidence`.
3. [Pinex Wire Compatibility 44187548184](https://github.com/majiddarvishan/neta-go/actions/runs/44187548184) completed **success** on the **same** SHA. Its `Verify Go wire codec against pinned Pinex C++ 1.5.0` job passed:
   - compile the C++ fixture generator from the pinned production headers;
   - compare fixtures against the pinned C++ serializer;
   - test the Go wire codec against C++ golden fixtures.
4. The `main` branch reports `protected=true` and two mandatory contexts: `test` and `Verify Go wire codec against pinned Pinex C++ 1.5.0`. This does **not** independently prove classic-protection `strict` or administrator-bypass flags.
5. At this audit, the GitHub release listing returned no published Releases. No tag/Release was created.

## Protocol contract now carried into release notes

- **PV-01:** A Go Session selects `ProtocolVersionV1` or `ProtocolVersionV2` with `WithProtocolVersion`; default is V2. Session selection is stable; this is *not* an automatic peer-negotiation mechanism.
- **PV-02A:** V2 Bind Request/Response omit the trailing NUL; V1 preserves `[BindType][SystemID][NUL]` and its pinned Pinex 1.5.0 wire behavior. V2 by default must **not** be presumed compatible with a legacy C++ Pinex server; the Go Client must explicitly select V1 for that peer.
- **PV-02B:** V2 Bind Request/Response accept up to **20 bytes** of SystemID (reject 21), while V1 still accepts up to **19 bytes** before its required NUL (reject 20). The 10-byte Header and Stream/Control wire formats remain unchanged.
- Tests committed in the merged PRs cover the relevant codec lengths, Bind Request/Response round-trips, version behavior and V2 20-byte TCP Bind; the Go and legacy C++ fixture workflows above confirm they run successfully at this source SHA. These CI runs are **not** a new live Go-V2 ↔ Pinex-C++ interoperability claim.

## Open gates, not closed by this audit

- [x] PV-01-R, PV-02A-R and PV-02B-R post-merge source CI verification against identical `main` SHA.
- [ ] **Fresh, explicit owner approval of the amended `CHANGELOG.md` Unreleased release-note content** containing all three PV changes. The owner's 2026-11-17 approval covered older contents only.
- [ ] At publication time, verify exact intended tag target SHA and final Go + Pinex CI for that SHA (including any intervening docs-only merges).
- [ ] Confirm strict/up-to-date and administrator-bypass controls independently (the protected-branch summary alone is insufficient).
- [ ] Confirm publish capability/settings under the actual authorized release account.
- [ ] **Separate, fresh owner approval for `P9-06`** to create `v1.0.0` tag/GitHub Release.

## Change policy

This requalification is documentation/tracker-only: it does not modify production code, test fixtures, Go API, wire encoding, Pinex C++, or the owner's local `stash@{0}`. The existing `CHANGELOG.md` content stays `Unreleased`. Do not create or publish any tag or GitHub Release from these evidence records.
