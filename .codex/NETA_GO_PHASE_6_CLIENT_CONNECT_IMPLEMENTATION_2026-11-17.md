# Phase 6 — Client Connect/Bind and Reconnect Implementation Report

**Status:** IMPLEMENTED / CI VERIFICATION PENDING  
**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main

## Owner decision

The owner approved Option B for Client Bind identity:
- WithClientSystemID(string) is mandatory.
- WithClientBindType(BindType) is optional.
- Default Bind type is bi-directional; uni-directional is supported.
- Client.Connect(ctx) remains unchanged.

The decision is recorded in DECISION_LOG.md §56.

## Implemented

- Client configuration validates SystemID presence, non-empty value, NUL exclusion and the 19-byte Pinex wire limit before Dial.
- Public Bind type and constants expose uni-directional and bi-directional selection.
- Client.Connect(ctx) performs TCP Dial followed by Bind and returns only after Bound or an attempt failure.
- Dial and Bind Response deadlines are independent absolute deadlines. The caller Context wins when earlier; cancellation after successful Connect does not affect the Session.
- Session generation is incremented per attempt and only the active generation may publish CurrentSession.
- Every Client generation reuses the Client-owned resource ledger and callback dispatcher.
- One optional reconnect supervisor uses exponential Full Jitter (1s base window, 30s cap); the retry window resets after 30s continuously Bound.
- Retry candidates are limited to transient Dial failures/timeouts and unexpected transport failures after Bound. Bind rejection and Bind timeout are non-retryable. Accepted Requests are never replayed.
- Stop cancels in-flight Dial/Bind and the reconnect supervisor, waits for internal teardown and does not join user callbacks.
- Uni-directional Client behavior follows Pinex 1.5.0 requester-only semantics.

## Tests added

- Required/malformed SystemID and BindType validation.
- Default bi-directional and explicit uni-directional configuration.
- Connect/Bind payload identity and BindType plus graceful Unbind.
- Bind timeout does not publish a Session.
- Bind rejection does not trigger automatic retry.
- AutoReconnect publishes a fresh Session generation after unexpected post-Bound transport loss.
- Uni-directional Client drains and ignores peer Requests and rejects SendResponse.

Existing Client configuration tests now use an explicit test SystemID helper because SystemID is mandatory.

## Verification

Pending current GitHub Actions run:
- gofmt
- go vet ./...
- go test ./...
- go test -race ./...
- pinned Pinex C++ 1.5.0 golden-frame compatibility

No live TCP Go↔Pinex interoperability was run; that remains Phase 7 and is not authorized by this work.
