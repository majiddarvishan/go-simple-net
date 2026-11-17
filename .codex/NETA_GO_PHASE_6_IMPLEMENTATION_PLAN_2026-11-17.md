# Phase 6 — Client and Server Orchestration Implementation Plan

**Status:** AUTHORIZED / IN PROGRESS — 2026-11-17  
**Repository:** `github.com/majiddarvishan/neta-go`  
**Branch:** `main`  
**Phase 5 source baseline:** `c72136693f85e61239bfca42bd04b40e66eddc27`  
**Phase 6 Server slice source HEAD:** `8eead509c6817722d6b7c6f5e5a17e1734749911` (CI PASS)

## Authority

The owner authorized Phase 6 by saying «ادامه بده» after the explicit Phase 6 start question. Scope is P6-01..P6-11 in root `TASKS.md`, under the frozen API-001/API-002 contracts.

## Implementation order

1. Server lifecycle: Start/Addr/Accept, terminal/idempotent Stop, listener-close ordering.
2. Bounded accepted-socket/Pre-Bind admission before Session allocation; absolute accept-to-Bind deadline.
3. Bind supervisor per Pre-Bind Session, optional BindValidator, atomic reservation for unique SystemID and MaxBoundSessions; cleanup by Session pointer identity.
4. Shared Server resource ledger and callback dispatcher injected into every accepted Session.
5. Server Stop: close Listener first, close Pre-Bind Sessions, initiate Unbind concurrently for Bound Sessions, force remaining Sessions on caller Context expiry; Done does not join user callbacks.
6. Client lifecycle/Connect/Bind, shared owner resources and generation fencing.
7. Single reconnect supervisor/timer with Full Jitter (1s base, 30s cap; reset after 30s continuously Bound); retry only transient Dial failure/timeout and unexpected loss after Bound; never replay Requests.
8. Tests, CI, evidence and task tracker update.

## Frozen contracts to preserve

- Client ConnectTimeout 30s absolute per Dial, includes DNS and TCP; caller Context earlier deadline wins.
- Client BindResponseTimeout 30s absolute from TCP establishment, includes Bind writer wait/write and matching response; Bind rejection/timeout are non-retryable.
- Server Pre-Bind timeout 30s absolute from TCP accept; default pending cap 128.
- Default MaxBoundSessions 1024. New unique Bind at capacity is rejected with Pinex-compatible `rfail`; never evict an existing Session.
- BindValidator is optional. With no validator, an otherwise valid unique Bind is accepted. Duplicate SystemIDs are always rejected atomically.
- Client AutoReconnect defaults false. When enabled: transient Dial failure/timeout and unexpected transport loss after Bound are retryable; Full Jitter exponential window 1s base / 30s cap; reset after 30s continuously Bound.
- No accepted Request is replayed/resubmitted on a new Session generation.
- Client and Server resource ledgers and callback dispatchers are owner-scoped and shared by all their Sessions/generations.
- Client/Server Stop are terminal and idempotent. Server closes Listener first, closes Pre-Bind Sessions, then concurrently Unbinds Bound Sessions. Context expiry forces remaining Sessions and returns the caller Context error after internal Done.
- Done/Stop do not wait for arbitrary user callbacks; OnClose is best-effort after Done.

## Known API contract gap — Client Bind identity

The approved public API specifies `Client.Connect(ctx)` performing Bind, but the frozen ClientOption and clientConfig currently have no SystemID or BindType input. The current source has no approved default for either field. Do not infer a SystemID, BindType, or add an unapproved public option. Client Connect implementation is blocked pending owner clarification. The independent Server slice P6-04..P6-08 is now COMPLETE / CI PASS.

## Phase boundaries

Do not perform live Go↔Pinex TCP interoperability (Phase 7), benchmark/optimization (Phase 8), or release/deployment/examples (Phase 9).


## Phase 6 Server implementation result — 2026-11-17

P6-04 through P6-08 are COMPLETE / CI PASS at source HEAD `8eead509c6817722d6b7c6f5e5a17e1734749911`.

- Server Start/Addr/Accept and listener-close ordering.
- Pre-Bind admission slot reserved before Session allocation; default cap 128 and absolute accept-to-Bind timeout inherited from the frozen contract.
- MaxBoundSessions reservation, optional BindValidator, duplicate SystemID rejection and no-eviction policy.
- Pointer-identity registry promotion/cleanup, including close-before-SystemID-publication and old-generation cleanup protection.
- Server-owned resource ledger and callback dispatcher passed into every accepted Session.
- Terminal/idempotent Server Stop: close Listener first, close Pre-Bind, concurrently Unbind Bound Sessions, force on caller Context expiry, wait only for internal Done.
- 11 focused tests; Go CI [44070940615](https://github.com/majiddarvishan/neta-go/actions/runs/44070940615) PASS; Pinex Wire Compatibility [44070940706](https://github.com/majiddarvishan/neta-go/actions/runs/44070940706) PASS.

P6-01..P6-03 (Client Connect/Bind, lifecycle and reconnect), Client-side P6-09/P6-10, and full P6-11 remain pending. The single blocker before Client Bind is the missing SystemID/BindType input in the frozen ClientOption surface.


## Client CurrentSession/Stop implementation result — 2026-11-17

Client lifecycle sub-scope implemented at source HEAD `a05a735aa0383d2588b41b1a2258282d734ff72b`:
- `CurrentSession() (*Session, bool)` exposes only the current Bound generation.
- `Stop(ctx)` is terminal/idempotent, cancels active Connect/reconnect contexts when present, Gracefully Unbinds the current Bound Session, forces on Context expiry and waits for internal Done only.
- 4 focused Client lifecycle tests.
- Go CI [44071959633](https://github.com/majiddarvishan/neta-go/actions/runs/44071959633) PASS; Pinex Wire Compatibility [44071959704](https://github.com/majiddarvishan/neta-go/actions/runs/44071959704) PASS.

Client Connect/Bind and reconnect remain blocked by the missing SystemID/BindType input in the frozen ClientOption surface.
