# Repository hygiene — neta-go

**Date:** 2026-11-18  
**Base protected main:** `f67c91565c3ed8c08f454588efd97b1e3b619ad2`  
**Scope:** Test-only identifiers, file layout, scripts, CI, Makefile and user-facing documentation. No production wire/API/default/timeout/resource changes.

## Renames

| Historical name | Current maintained name |
|---|---|
| `phase7_interop_test.go` | `interoperability_test.go` |
| `phase7_soak_test.go` | `reconnect_soak_test.go` |
| `phase7_transport_test.go` | `transport_fault_test.go` |
| `phase8_benchmark_test.go` | `benchmark_test.go` |
| `phase8_delivery_test.go` | `delivery_test.go` |
| `phase8_pinex_benchmark_test.go` | `interoperability_benchmark_test.go` |
| `scripts/phase8-benchmark.sh` | `scripts/performance.sh` |
| `scripts/phase9-release-check.sh` | `scripts/example-smoke.sh` |
| `.github/workflows/phase8-performance.yml` | `.github/workflows/performance.yml` |
| `make phase8` / `make phase9` | `make benchmark` / `make smoke` |
| `NETA_PHASE8_SOAK_SECONDS` | `NETA_SOAK_SECONDS` |
| `NETA_PHASE7_SOAK_SECONDS` | `NETA_RECONNECT_SOAK_SECONDS` |
| `docs/PINEX_COMPATIBILITY.md` | removed; native `docs/PROTOCOL.md` added |

Test and benchmark helpers are also renamed to describe behavior rather than development stages. Performance artifacts now default to `artifacts/performance/`, and example smoke artifacts to `artifacts/smoke/`.

## Internal compatibility verification preserved

The `.github/workflows/neta-cpp-wire-compat.yml` job **must retain the exact required context** `Verify Go wire codec against pinned Pinex C++ 1.5.0` to keep protected-`main` enforcement. It and `testdata/neta-cpp-1.5.0/` are internal engineering fixtures, **not a user dependency or public product description**. Removing the standalone public compatibility document does not delete the tests or the historical evidence.

The earlier `.codex/` records retain historical phase naming as audit history; they are not current implementation entry points. Only maintained source/tests/scripts/workflows/public docs use the new names.

## Validation and release boundary

- [ ] Protected PR Go formatting/vet/unit/race and paired example smoke PASS.
- [ ] Protected PR pinned-wire fixture job PASS.
- [ ] Post-merge `main` Go and wire jobs PASS on the same new HEAD.
- [ ] Performance smoke selector and scheduled benchmark suite verified under new names.
- [ ] Owner reviews amended `CHANGELOG.md` before any release.
- [ ] Owner explicitly authorizes a `v1.0.0` tag and GitHub Release separately.

No release/tag is authorized by this housekeeping PR.

## Subsequent C++ harness naming (2026-11-18)

The project-facing C++ peer name is now `neta-cpp`. Maintained Go test names, C++ harness filename, fixture directory and workflow filename use `neta-cpp` / `neta_cpp` spellings. The environment variable is `NETA_CPP_PEER_BIN`.

The bundled headers originate from the pinned third-party source and preserve `pa/pinex` includes and namespace without edits; their recorded commit and SHA-1 provenance remain accurate. The historical protected-branch check context `Verify Go wire codec against pinned Pinex C++ 1.5.0` cannot be renamed safely until the GitHub administrator updates branch protection. Current workflow identity changes do not change that job context.

**Naming finalization (2026-11-18):** The canonical human-readable C++ peer name is **`neta-cpp`**. Original third-party namespaces/paths in pinned C++ headers and the existing required branch-protection status check retain their original upstream identifiers for compatibility and provenance.
