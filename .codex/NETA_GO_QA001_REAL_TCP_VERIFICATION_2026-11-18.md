# QA-001 — Genuine C++ V1 TCP Interoperability Evidence

**Date:** 2026-11-18  
**Finding:** Legacy pinned neta-cpp 1.5.0 speaks V1 Bind (NUL terminated) while public Go Clients default to V2 (NUL free). Normal Go CI had previously skipped the C++-peer process tests without `NETA_CPP_PEER_BIN`. Explicit V1 selection was added in neta-go PR #15.

## Verification matrix

| Dimension | Provenance |
|---|---|
| Go repository | `majiddarvishan/neta-go`, exact protected-main `0824a70127594df72f78d1048cc360e33f57bd5b` |
| Private C++ source | `majiddarvishan/pinex` at pinned revision `f7bd1eb779baff0d10e624f20522df345e6ee747` (release line 1.5.0) |
| Private test workflow | `pinex/.github/workflows/neta-go-live-interop.yml` merged in [PR #29](https://github.com/majiddarvishan/pinex/pull/29), develop HEAD `34b37905e6a38d21847fa95322121615b38b9cd6` |
| Post-merge CI evidence | [neta-go real TCP, run 44260379077](https://github.com/majiddarvishan/pinex/actions/runs/44260379077): **SUCCESS** |
| PR CI evidence | [private PR run 44259636306](https://github.com/majiddarvishan/pinex/actions/runs/44259636306): **SUCCESS** |
| Platform / toolchain | GitHub Actions Ubuntu 24.04, Go 1.23.x, g++ C++20 with public Boost and fmt development packages |

The private workflow checks out the original **private** C++ repository and the **public** Go repository separately. It compiles the checked-in Go interoperability C++ peer `neta-go/test/interop/neta_cpp_peer.cpp` against the original pinned private C++ header tree. It does **not** copy private C++ sources into the public Go repository or require the public Go Action to read a private repository.

## Executed non-skipped live-TCP tests

1. `TestNetaCPPServerWithGoClient` — Go Client explicitly selects **V1**, Binds to the actual C++ Server, performs Request/Response, then closes gracefully. **PASS**, ordinary and `go test -race`.
2. `TestGoServerWithNetaCPPClient` — actual C++ Client Binds and sends to Go Server, gets the Go response and closes gracefully. **PASS**, ordinary and `go test -race`.

The post-merge job's recorded source SHAs match the exact versions above. This meets the **two-direction real-peer E2E** acceptance criterion from QA-001; previously the public golden checks alone did not.

## Continuous verification and residual boundaries

The private workflow runs on `pinex/develop` pushes and PRs, may be started manually and is scheduled weekly against the latest public `neta-go/main`. It is not a required status check on each public neta-go PR; this remaining automation gap belongs to **QA-009**, not QA-001. The separate public `neta-cpp Wire Compatibility` workflow still validates deterministic pinned wire vectors without the private source.

Tests run over local TCP on a CI runner. They establish encoding/session functional compatibility, not production throughput, packet-loss resilience or representative network performance. The C++ peer's source remains private; only authorized maintainers can view its full CI logs. This is not a release signoff; P9-06 remains unauthorized.
