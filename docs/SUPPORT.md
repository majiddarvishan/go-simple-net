# Go Toolchain and Platform Support

**Status: proposed release baseline; owner approval is still required.**

## Verified evidence

| Dimension | Verified evidence |
|---|---|
| Module Go directive | `go 1.23` in `go.mod` |
| Go CI selector | `1.23.x`, resolving to Go 1.23.12 in the 2026-11-17 CI run |
| Go CI OS/architecture | GitHub Actions `ubuntu-24.04`, Linux/amd64 |
| Performance baseline | Ubuntu 24.04 / Linux amd64, Go 1.23.12 |
| Owner local toolchain | Go 1.27.0; owner-reported `make verify` PASS on 2026-11-17 |
| Owner local OS/architecture | Not captured in the committed verification artifact |
| External dependencies | No third-party Go modules are declared; no `go.sum` is present |

## Proposed V1 support statement

- **Minimum Go toolchain:** Go 1.23, consistent with the module directive and current CI selector.
- **Continuously tested platform:** Linux/amd64 on Ubuntu 24.04.
- **Other GOOS/GOARCH combinations:** not yet verified by the repository CI matrix. Do not claim support until they are built and tested.

This is an evidence-based proposal, not a finalized support promise. The owner must approve the Go floor and platform support matrix during P9-05 before V1 publication.
