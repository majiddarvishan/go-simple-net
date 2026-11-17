# Phase 4 — Resource Admission & Accounting Implementation Report

**Status:** COMPLETE / CI PASS  
**Source verification HEAD:** `2935402366603267ba60c181a057d26cf0e7e9a6`  
**Date:** 2026-11-17  
**Authorization:** Explicit owner instruction in this conversation; Phase 4 only.

## Implemented scope

- Added an owner-scoped resource ledger created independently by each `NewClient` and `NewServer`. Session construction supports injection of an existing owner ledger so Phase 6 can share one Client/Server budget across all Sessions and reconnect generations.
- Added checked, mutex-protected reserve/release counters for owner Request credits, retained Application bytes, retained Completion bytes, OnClose event credits and Server aggregate queued Application bytes/frames.
- Server aggregate queue admission is one combined budget across incoming and outgoing Application Frames. Client owners do not acquire the Server-only aggregate queue ceiling.
- Outgoing `SendRequest`, `SendNoWait` and `SendResponse` reserve local queue/count and retained-Application capacity plus owner/Server capacity before allocating/copying the Frame. Rejected capacity does not consume a Sequence or publish Pending state.
- Request credits are reserved before payload copy and remain held through response wait, queued Completion and active Completion callback; they release exactly once on callback return/cancellation or forced terminal cleanup.
- Incoming Stream Request Headers reserve full declared Frame bytes/count and retained-Application bytes before Body allocation. Incoming overload closes only the offending Session. Queue-only bytes/count release at callback start; retained bytes release after callback return/cancellation.
- Matched Stream Responses reserve retained Completion bytes before Body allocation. If capacity is unavailable, the Body is drained with a fixed 4 KiB scratch buffer under the original absolute Frame Assembly deadline; a still-live Request receives a normal Completion with no payload and `ErrCapacity`, while the Session remains open.
- Unknown, late and pre-Writer-claim Responses are drained without allocating a retained Body.
- OnClose is best-effort: it reserves one owner event credit after Done, skips permanently if the owner budget is full, and releases the credit after callback return/cancellation.
- Added idempotent release paths for partial reads, Completion drain timeout, Forced Close A, queued callback cancellation, callbacks that outlive Session Done and Writer-owned frames.

## Verification

- `session_test.go`: 46 focused Session/transport/resource-accounting tests.
- `resource_ledger_test.go`: 8 owner-ledger tests.
- Go Actions [44062568415](https://github.com/majiddarvishan/neta-go/actions/runs/44062568415): PASS — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility [44062568425](https://github.com/majiddarvishan/neta-go/actions/runs/44062568425): PASS — compile pinned C++ 1.5.0 production serializer fixture, compare golden Frames and run Go wire compatibility tests.
- The compatibility workflow is a golden-frame check, **not** live Go↔Pinex TCP interoperability.

## Explicitly not implemented in Phase 4

- Phase 5 physical callback Worker Scheduler and fair per-Session worker dispatch.
- Phase 6 Client Dial/Server Listen/accept, Bind registry, reconnect and Session-generation orchestration.
- Phase 7 live TCP interoperability/fault-injection campaign.
- Phase 8 performance benchmark/pooling work.
- Phase 9 examples, release and deployment.

No public API, wire behavior, defaults, resource ceilings or frozen owner decisions were changed.
