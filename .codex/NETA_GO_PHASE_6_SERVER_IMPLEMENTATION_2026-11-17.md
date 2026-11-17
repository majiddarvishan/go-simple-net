# Phase 6 — Server Lifecycle and Admission Implementation Report

**Status:** SERVER SUB-SCOPE COMPLETE / CI PASS; PHASE 6 OVERALL IN PROGRESS  
**Date:** 2026-11-17  
**Repository:** `github.com/majiddarvishan/neta-go`  
**Branch:** `main`  
**Source verification HEAD:** `8eead509c6817722d6b7c6f5e5a17e1734749911`  
**Authorization:** Phase 6 owner authorization recorded in `IMPLEMENTATION_AUTHORIZATION_2026-11-17.md`.

## Implemented

- `Server.Start()` opens the configured TCP listener and returns after the Accept loop is ready; repeated Start is idempotent, and Start after terminal Stop returns `ErrClosed`.
- `Server.Addr()` exposes the actual listener address after successful Start.
- Accept loop reserves the Pre-Bind socket slot before allocating a Session. Excess sockets are closed immediately; default cap remains 128.
- Server Sessions inherit the Server's shared resource ledger and callback dispatcher. Each Session is registered before its Reader/Writer/Control goroutines start.
- One bounded Bind supervisor per Pre-Bind Session runs the optional BindValidator with actual remote address and Bind type.
- Unique SystemID and MaxBoundSessions admission use atomic reservations across pending Bind responses and already-Bound Sessions. Duplicate IDs and capacity overflow receive Pinex-compatible failed Bind Response; existing Sessions are never evicted.
- Successful reservations become Bound registry entries only after the Bind Response is fully written.
- Registry cleanup is idempotent and keyed by Session pointer identity. An old Session cannot remove a replacement Session using the same SystemID; a reservation is also released if close wins before Session.SystemID is published.
- Server Stop is terminal/idempotent: close Listener first, wait Accept exit, close Pre-Bind Sessions, concurrently Graceful-Unbind Bound Sessions, and Force Close remaining Sessions if caller Context expires. Stop waits for internal Done only, not user callbacks.
- Added internal Session lifecycle hooks for Bound commit and pre-Done registry/resource cleanup. No public API or wire format was changed.

## Regression tests

`server_lifecycle_test.go` contains 11 focused tests covering:
1. Start/Addr, successful Bind and graceful Server Stop/Unbind.
2. Duplicate SystemID rejection without evicting the existing Session.
3. MaxBoundSessions rejection without eviction.
4. Pre-Bind socket cap enforced before Session allocation.
5. Stop closes Pre-Bind sockets and listener.
6. Context expiry forces unresponsive Bound Sessions.
7. Concurrent duplicate Bind attempts have exactly one winner.
8. Reserved identity commits to Bound registry only at successful Bound transition.
9. Pending identity is released if close wins before Session identity publication.
10. BindValidator receives SystemID, BindType and actual RemoteAddr; rejected Bind does not reserve the ID.
11. Old Session cleanup cannot remove a replacement Session's SystemID registry entry.

## CI evidence

- Go Actions [44070940615](https://github.com/majiddarvishan/neta-go/actions/runs/44070940615): **PASS** — gofmt, go vet ./..., go test ./..., go test -race ./....
- Pinex Wire Compatibility [44070940706](https://github.com/majiddarvishan/neta-go/actions/runs/44070940706): **PASS** — pinned C++ 1.5.0 golden-frame compatibility.

These are unit/race and golden-frame checks. No live Go↔Pinex TCP interoperability was performed; that remains Phase 7.

## Remaining Phase 6 work

- Client Connect/Dial/Bind and CurrentSession.
- Client terminal Stop, Session generation fencing and owner-ledger reuse across reconnect generations.
- Single reconnect supervisor with approved Full Jitter policy and no Request replay.
- Client/reconnect lifecycle and race tests; final Phase 6 CI acceptance.

**Blocking API contract gap:** The frozen Client.Connect API performs Bind, but ClientOption/clientConfig has no SystemID or BindType input and no default is approved. Do not infer either or add a public option until the owner decides the Client Bind configuration surface.
