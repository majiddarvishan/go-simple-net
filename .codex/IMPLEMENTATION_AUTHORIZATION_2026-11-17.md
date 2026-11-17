# neta-go — Implementation Authorization Record

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Architecture:** Frozen; all fundamental owner decisions closed.

## Owner authorization

The owner explicitly authorized Phase 1 in this conversation: «آره، اجازه داری».

## Authorized scope — Phase 1 only

- Create/maintain the approved module path github.com/majiddarvishan/neta-go and root package neta.
- Implement typed ClientOption, ServerOption, and SessionOption.
- Implement immutable effective configuration and approved defaults.
- Apply every Functional Option before final validation.
- Validate all API-004 and WORKER-009C ranges, zero-value exception, independent owner/Session budgets, and cross-field invariants.
- Protect arithmetic helpers against overflow/underflow.
- Provide the public config errors/state declarations required by frozen API-001.
- Add and run focused Phase 1 tests and CI.

## Baseline and current evidence

- Main HEAD at authorization: f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- Existing Phase 1 source/module commit: 8c1132392e9c6e919f72ad16ce8f285d989188d9.
- Subsequent config hardening and CI-green source baseline: f3f525e5f36f65235a4c8e023cfafb4bfd6b5737.
- Additional Server-side WORKER-009C boundary tests: 18bec96bb13cbdfa5a7819301b23d61e3fb3db75.
- CI for the baseline before the additional tests: PASS (gofmt, go vet, go test, go test -race).
- CI for the additional tests: pending at record creation.

## Authorization boundary

This authorization does NOT authorize Phase 2 or later: Wire Protocol, transport/runtime, lifecycle engine, worker scheduler, reconnect, Pinex interoperability, benchmarks, release, or deployment. Those phases require a separate explicit owner approval.

Earlier blanket statements that no implementation/tests are authorized are superseded only for the Phase 1 scope above. No architecture decisions are reopened.


## Phase 2 Authorization — Wire Protocol (2026-11-17)

The owner explicitly approved beginning Phase 2 by replying «ادامه بده» after being asked whether Phase 2 (Wire Protocol) was authorized. This approval is scoped to protocol implementation only.

**Authorized scope:** pure-Go `internal/wire` package; Pinex 1.5.0 10-byte frame header and Big Endian fields; frame encode/decode and size validation; Bind Request/Response with bind_type and C-octet system_id; Stream Request/Response raw body; Unbind and Enquire Link control PDUs; sequence fields/semantics; unit and golden-vector tests; documentation and CI.

**Compatibility reference:** Pinex C++ develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Re-pin/re-audit if the develop branch moves before interoperability testing.

**Hard protocol maximum:** 1,048,576 bytes total including the 10-byte header; maximum Body 1,048,566 bytes. The limit is fixed, not configurable, and applies inbound and outbound.

**Not authorized:** TCP transport/read-write loops, Client/Server runtime integration, lifecycle/shutdown/reconnect, worker scheduler, resource-admission runtime, cross-language socket test harness requiring C++ execution, benchmarks, release or deployment. Those belong to later phases and require separate owner approval.

No frozen architecture decision is reopened. Phase 1 remains complete with CI PASS. Phase 2 implementation must not import root package `neta` from `internal/wire`; the package uses only the Go standard library.
