# Go Toolchain and Platform Support

**Status: approved Go/toolchain and CI-tested platform baseline (2026-11-17).**

**Latest merged-main CI (2026-11-19):** [Go 44547311083](https://github.com/majiddarvishan/neta-go/actions/runs/44547311083) PASS on SHA `fb57668e182dc102f94044393b3d8f2a33aeddae`. The original 2026-11-17 toolchain/platform evidence below remains a dated baseline; this does not expand supported GOOS/GOARCH or authorize a release.

## Verified evidence

| Dimension | Verified evidence |
|---|---|
| Module Go directive | `go 1.23` in `go.mod` |
| Go CI selector | `1.23.x`, resolving to Go 1.23.12 in the 2026-11-17 CI run |
| Go CI OS/architecture | GitHub Actions `ubuntu-24.04`, Linux/amd64; Go CI [44119107169](https://github.com/majiddarvishan/neta-go/actions/runs/44119107169) |
| Performance baseline | Ubuntu 24.04 / Linux amd64, Go 1.23.12 |
| Owner local toolchain | Go 1.27.0; owner-reported `make verify` PASS on 2026-11-17 |
| Owner local OS/architecture | Not captured in the committed verification artifact |
| External dependencies | No third-party Go modules are declared; no `go.sum` is present |

## Supported baseline

- **Minimum Go toolchain:** Go 1.23, consistent with the module directive and current CI selector.
- **Continuously tested platform:** Linux/amd64 on Ubuntu 24.04.
- **Other GOOS/GOARCH combinations:** not yet verified by the repository CI matrix. Do not claim support until they are built and tested.

**Owner decision, 2026-11-17:** the owner approved the directly presented recommendation by replying «آره»: minimum Go 1.23; continuously CI-verified Linux/amd64 on Ubuntu 24.04; Go 1.27.0 local `make verify` PASS is additional evidence, not a separate platform guarantee. Other GOOS/GOARCH combinations have not been qualified and are not claimed as officially supported.

This support statement does not authorize publication or change protocol, API or runtime behavior.
