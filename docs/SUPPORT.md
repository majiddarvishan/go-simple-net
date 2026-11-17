# Go Toolchain and Platform Support

**Status: OWNER APPROVED — V1 Go/toolchain and CI-tested platform baseline (2026-11-17).**

## Verified evidence

| Dimension | Verified evidence |
|---|---|
| Module Go directive | `go 1.23` in `go.mod` |
| Go CI selector | `1.23.x`, resolving to Go 1.23.12 in the 2026-11-17 CI run |
| Go CI OS/architecture | GitHub Actions `ubuntu-24.04`, Linux/amd64; latest Phase 9 Go CI [44119107169](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169) |
| Performance baseline | Ubuntu 24.04 / Linux amd64, Go 1.23.12 |
| Owner local toolchain | Go 1.27.0; owner-reported `make verify` PASS on 2026-11-17 |
| Owner local OS/architecture | Not captured in the committed verification artifact |
| External dependencies | No third-party Go modules are declared; no `go.sum` is present |

## Owner-approved V1 support statement

- **Minimum Go toolchain:** Go 1.23, consistent with the module directive and current CI selector.
- **Continuously tested platform:** Linux/amd64 on Ubuntu 24.04.
- **Other GOOS/GOARCH combinations:** not yet verified by the repository CI matrix. Do not claim support until they are built and tested.

**Owner decision, 2026-11-17:** the owner approved the directly presented recommendation by replying «آره»: minimum Go 1.23; continuously CI-verified Linux/amd64 on Ubuntu 24.04; Go 1.27.0 local `make verify` PASS is additional evidence, not a separate platform guarantee. Other GOOS/GOARCH combinations have not been qualified and are not claimed as officially supported.

This decision closes the P9-05 support approval only, and does not authorize a V1 tag/Release, approve a version or copyright holder, or alter production API, wire format, defaults or behavior.
