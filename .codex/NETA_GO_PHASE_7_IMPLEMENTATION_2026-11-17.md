# Phase 7 — Live Interoperability and Fault-Injection Implementation Report

**Status:** IN PROGRESS  
**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Owner authorization:** Explicitly granted in this conversation on 2026-11-17  
**Implementation source HEAD:** be2511ba9701313526fdc03e27e8e51583f4af21

## Scope and safety boundary

The owner authorized P7-01..P7-08 only. No frozen public API, Wire Protocol, timeout/reconnect policy, resource ceiling, default or architecture decision may be changed without a newly justified owner decision.

Phase 8 performance/optimization and Phase 9 examples/release/deployment remain unauthorized.

## P7-01 — Pinex source re-pin

**PASS.**

- Repository: majiddarvishan/pinex
- Branch: develop
- Version: 1.5.0
- Exact HEAD: f7bd1eb779baff0d10e624f20522df345e6ee747
- The current branch HEAD is identical to the frozen source Snapshot. No source drift or Wire Contract change was found.

## Verification harness added

### Real Pinex C++ peer

test/interop/pinex_peer.cpp is a small test-only process using the actual Pinex public high-level APIs:

- pa::pinex::messaging_server for the Pinex C++ Server role.
- pa::pinex::messaging_client for the Pinex C++ Client role.
- The peer exchanges one request/response and performs graceful Unbind.

No Pinex private implementation source or header tree is copied into the public neta-go repository.

### Go interoperability tests

phase7_interop_test.go adds two real loopback TCP scenarios:

1. Go Client → Pinex C++ Server; validates Bind identity, Request/Response payload and graceful Stop.
2. Pinex C++ Client → Go Server; validates Bind, Request/Response payload and graceful Unbind.

These tests are opt-in through NETA_PINEX_PEER_BIN, so ordinary Go-only CI does not pretend to perform cross-language verification.

### Transport faults

phase7_transport_test.go adds real-TCP coverage for:

- fragmented Bind Response delivery;
- two Stream Responses arriving out of order and matching the correct Request Sequence;
- slow-trickle Bind Response exceeding the absolute Frame Assembly deadline;
- abrupt TCP RST after a successful Bind.

Existing Client tests additionally cover silent Bind timeout and Bind rejection without retry.

**Remaining P7-04 gaps:** graceful EOF-specific coverage and privileged kernel-level SYN blackhole/network-namespace injection have not yet been run. The existing silent accepted-socket timeout is an application-stage blackhole, not proof of a firewall-level SYN drop.

### Wire fuzzing

internal/wire/wire_fuzz_test.go adds five fuzz targets:

- FuzzParseHeader
- FuzzDecodeFrame
- FuzzDecodeBindRequest
- FuzzDecodeBindResponse
- FuzzDecodeStreamRequest

Ordinary go test exercises their seed corpora. The bounded fuzz campaigns remain pending the dedicated workflow.

### Reconnect soak

phase7_soak_test.go adds an environment-gated 3-Client loopback reconnect soak. It measures Bound generations, checks configured callback worker ceilings and verifies owner Request/retained-byte/event credits return to zero after Stop.

The dedicated workflow configures a 30-second soak. It has not yet been executed.

## Dedicated cross-repository CI

Pinex test-only branch: test/neta-go-phase7-interop

Workflow:

https://github.com/majiddarvishan/pinex/actions/workflows/neta-go-interop.yml

The workflow:

1. verifies Pinex develop is still the exact pinned SHA;
2. compiles the C++ peer against the actual Pinex public headers and installed Boost/fmt dependencies;
3. checks out neta-go main;
4. runs formatting, vet, all tests under the Go race detector, and the 30-second soak;
5. runs five bounded 10-second fuzz campaigns.

The workflow and branch are isolated from Pinex develop; no production Pinex source branch was modified.

Current run:

https://github.com/majiddarvishan/pinex/actions/runs/44080043667

## Verification evidence so far

At source HEAD be2511ba9701313526fdc03e27e8e51583f4af21:

- Go CI https://github.com/majiddarvishan/neta-go/actions/runs/44079860986: PASS — gofmt, go vet ./..., go test ./... and go test -race ./....
- Pinex Golden Compatibility https://github.com/majiddarvishan/neta-go/actions/runs/44079861215: PASS. This is still a golden-frame check, not live socket proof.

The initial test commit had a Go vet failure because the RST test called SetLinger through the net.Conn interface. The test was corrected to use *net.TCPConn; the final Go CI run above passes.

## Task status

| Task | Status | Evidence / remaining |
|---|---|---|
| P7-01 | PASS | Exact Pinex develop SHA re-pinned; no drift |
| P7-02 | PENDING | Bidirectional real-socket harness added; dedicated workflow running |
| P7-03 | PENDING | Five fuzz targets added; campaigns pending |
| P7-04 | IN PROGRESS | Fragmentation, reordering, slow-trickle and RST tests pass; graceful EOF and privileged SYN blackhole remain |
| P7-05 | PASS | Existing Phase 4 admission matrix passes in Go race CI |
| P7-06 | PASS | Existing Worker Scheduler fault/race matrix passes in Go race CI |
| P7-07 | PENDING | 30-second reconnect soak added; execution pending |
| P7-08 | PENDING | Final acceptance waits for P7-02, P7-03, P7-04 and P7-07 |

## Next actions

1. Inspect the dedicated Pinex workflow result and fix any actual compile/interoperability/test failure.
2. Complete the fuzz campaigns and soak with the exact Pinex baseline.
3. Add the remaining graceful EOF transport case; attempt privileged network-namespace SYN blackhole only if the runner supports it without changing the host network.
4. Re-run both directions and the full verification matrix; update TASKS.md, this report and HANDOFF.md with only proven evidence.
5. Close P7-08 only after every required acceptance criterion is satisfied.

No Phase 8 or Phase 9 work is included.
