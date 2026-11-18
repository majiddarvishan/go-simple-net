# Lifecycle, Callbacks, Payload Ownership and Errors

> **QA-007 implementation branch (2026-11-18):** Source changes on the feature branch implement owner-approved retained Completion and admission-reserved OnClose. Full correctness, race and qualification evidence is still pending final validation; this is not a release statement.

## Construction and startup

`NewClient` and `NewServer` apply Functional Options, validate the final effective values and snapshot immutable configuration. They do not perform network I/O or start goroutines.

- `Client.Connect(ctx)` performs TCP Dial and then the Bind handshake. It returns when Bound or when the attempt fails. The caller Context bounds the attempt; cancelling it after successful Connect does not close the Session.
- `Server.Start()` opens the listener and returns when the Accept loop is ready. `Server.Addr()` exposes the last bound listener address. A Server cannot be restarted after Stop.
- `Server.AcceptDone()` is a channel closed when the listener's Accept loop terminates (normally after `Stop`, or abnormally on a fatal `Accept` error). It is nil before `Start`. After the channel closes, `Server.AcceptErr()` returns the wrapped fatal error, or nil for a normal Stop. A terminal Accept failure stops **new** connections without forcibly closing existing bound Sessions; call `Server.Stop(ctx)` to clean up and create a fresh Server to restart. Calling `Start` on a failed Server returns the saved error.

A Server may install a BindValidator. It receives the peer SystemID, BindType and actual RemoteAddr. When no validator is installed, otherwise valid Binds with unique System IDs are accepted. Duplicate System IDs and Bound-session capacity overflow are rejected without evicting existing Sessions.

A BindValidator is an application-supplied synchronous function. A Pre-Bind handshake timeout closes the Session, but Go cannot forcibly cancel a validator function that is already blocked. The Server therefore caps the number of running validators at `MaxPendingPreBindSessions`; each execution keeps its permit until the function returns, even if its Session has timed out. Additional handshakes waiting for a permit are abandoned when their Session closes. A blocked validator may outlive `Server.Stop`; **applications must place their own timeouts on external work inside BindValidator**. This prevents unbounded validator goroutine accumulation without changing the existing callback signature.

## Session state

The public states are:

`new → binding → bound → unbinding → closing → closed`

A Session accepts Application traffic only while Bound. During Unbinding, new Application admission is closed while already admitted outgoing Application Frames drain in FIFO order.

## Request and Response semantics

### Request/Response

`SendRequest(payload, completion)`:

1. Validates Session state and resource capacity.
2. Reserves queue and Request credits.
3. Snapshots the outgoing payload.
4. Assigns a nonzero Sequence and admits the Frame to the local FIFO.
5. Starts RequestTimeout after successful local admission.
6. Invokes the Completion callback with the matching Response or terminal error.

A nil CompletionHandler is rejected. A successful return means **local admission only**; it does not mean the peer received, accepted or processed the Request. An accepted Request is never replayed by automatic reconnect. **Current implementation (superseded target policy):** a queued Completion callback may be cancelled if Forced Close wins before that callback starts; a callback that has started is allowed to return. **New approved target QA-007.26:** retain every accepted Request's pending/queued Completion across *any* Forced Close; exactly one internal settlement and at most one callback start. Request credit remains held until its Completion responsibility actually ends.

### One-way traffic

`SendNoWait` admits a Stream Request without pending Response state or a Completion callback. The peer may receive it, but the sender does not track a matching Response. Successful return still means local admission only.

### Responding

A Server OnRequest handler receives the original Sequence. It replies with `SendResponse(sequence, payload, status)`; the response uses that Sequence. The response payload is snapshotted before successful admission returns.

## Callback execution

User callbacks are dispatched asynchronously; they never execute inline in the transport Reader or Writer.

Each Client and Server owns three isolated bounded callback lanes:

- Application: Server OnRequest and Bound notifications.
- Completion: per-Request Completion callbacks.
- OnClose: terminal notifications after Session.Done.

Worker ceilings are owner-wide and independent by callback class. Per-Session Application and Completion concurrency are separate configurable intersections with those owner ceilings. Both default to one. Application callbacks for one Session are FIFO-started; when concurrency exceeds one, callbacks may overlap and finish out of order. Completion callbacks preserve callback-start FIFO order, not necessarily completion-return order.

**Current implementation (superseded target policy):** OnClose is best-effort and at-most-once; it is eligible only after Done and may be permanently skipped when the owner event-credit budget is exhausted. **New approved target QA-007.30:** reserve one OnClose credit before admitting a Session with an OnClose handler; reject new eligible Session admission immediately at capacity; transfer the same credit to the post-Done at-most-once notification and release after return.

A callback should not block indefinitely. A blocked callback retains its worker permit and the applicable callback/request/payload accounting until it returns or is cancelled under the defined lifecycle policy.

## Payload ownership

- Outgoing payload bytes are snapshotted before a successful SendRequest, SendNoWait or SendResponse admission returns. The caller may reuse or mutate its original slice after that return.
- Incoming Request and Response payloads are owned snapshots. Application code may retain them after the callback returns.
- Do not assume the library recycles or pools user-visible inbound payload memory.
- Queue budgets, retained-payload budgets and owner budgets are distinct library accounting limits; none is a total process RSS limit.

## Timeout and reconnect behavior

| Operation | Default | Semantics |
|---|---:|---|
| ConnectTimeout | 30s | Absolute per-Dial-attempt deadline |
| BindResponseTimeout | 30s | Absolute Bind handshake deadline |
| PreBindHandshakeTimeout | 30s | Absolute accept-to-Bind deadline on Server |
| RequestTimeout | 30s | Starts after successful local Request admission |
| FrameAssemblyTimeout | 30s | Absolute per-inbound-Frame deadline; progress does not reset it |
| WriteTimeout | 30s | Absolute per-Frame deadline after Writer ownership |
| UnbindTimeout | 10s | Absolute Graceful Unbind deadline |
| EnquireLinkIdleInterval | 30s | Idle-only outgoing probe interval; zero disables outgoing probes |
| EnquireLinkResponseTimeout | 10s | Deadline for the matching Enquire Link Response |

All mandatory timeout durations must be positive. EnquireLinkIdleInterval is the only duration where zero has a defined disable meaning.

AutoReconnect is false by default. When enabled, the Client uses one bounded exponential Full-Jitter supervisor with a 1s base window and 30s cap; the retry window resets after 30s continuously Bound. Only transient Dial failures/timeouts and unexpected post-Bound transport loss are retryable. Bind rejection and Bind timeout are not retryable. Accepted Requests are never replayed.

## Graceful and forced shutdown

- `Session.Unbind(ctx)` closes new Application admission, drains admitted Application Frames and waits for the matching Unbind Response or deadline.
- `Session.Close()` initiates immediate Forced Close and returns without waiting.
- `Client.Stop(ctx)` and `Server.Stop(ctx)` are terminal and idempotent. They attempt graceful Unbind and force remaining Sessions when the caller Context expires.
- `Session.Done()` closes after internal transport/state teardown and terminal-cause publication. It does **not** wait for arbitrary user callbacks.
- `Session.Err()` exposes the immutable terminal cause after Done; a clean graceful Unbind may have a nil cause.

## Errors

Use `errors.Is` for stable categories:

| Error | Meaning |
|---|---|
| `ErrInvalidConfig` | Effective configuration is invalid |
| `ErrNotBound` | Operation requires a Bound Session |
| `ErrClosing` | Session is closing and no longer accepts Application work |
| `ErrClosed` | Client, Server or Session is terminal |
| `ErrQueueFull` | Outgoing Application FIFO is full |
| `ErrCapacity` | A configured resource/admission budget is exhausted |
| `ErrFrameTooLarge` | Frame exceeds the fixed protocol maximum |
| `ErrRequestTimeout` | Matching Request Response deadline expired |
| `ErrConnectTimeout` | Dial attempt timed out |
| `ErrBindTimeout` | Bind handshake timed out |
| `ErrWriteTimeout` | Frame write deadline expired |
| `ErrUnbindTimeout` | Graceful Unbind deadline expired |
| `ErrBindRejected` | Peer rejected Bind |
| `ErrProtocol` | Malformed or directionally invalid protocol operation |

Use `errors.As` for `*ConfigError` to inspect `Field()`, `Value()` and `Constraint()`, or for `*OperationError` to inspect `Operation()`. Both preserve the underlying error chain.

## Planned callback panic contract — design approved, not yet implemented

The final owner-approved QA-007 design is documented in [QA-004 / QA-005 / QA-007 approved contracts](../.codex/NETA_GO_QA_004_005_007_APPROVED_CONTRACTS_2026-11-18.md), including QA-007.14–30 and explicit supersessions of Forced Close/OnClose legacy policies.

**Approved target, not yet implemented:**

- Recover all user callback panics (including OnClose), close only the affected Session and preserve the first Closing cause as immutable `Session.Err()`. Panics from already-started callbacks are still diagnosed after Done.
- Expose typed `Session.Diagnostics()` containing `CallbackPanics` (First, Recent, OnClose, Dropped) with independent coherent snapshots; lazy count/byte-bounded recent records, complete retained stacks, safely bounded 1 KiB value text and internally retained raw panic values.
- Linearize Application callback Start Claim with Close; cancel unstarted Application callbacks; never forcibly kill callbacks already started.
- Preserve accepted Request Completion on **all** Forced Close causes using pre-reserved Request credits, and continue other Completions when one of them panics.
- Reserve owner OnClose event capacity on admission of each OnClose-eligible Session; when the owner credit budget is exhausted, reject new eligible admission rather than silently discard post-Done OnClose. OnClose uses its independent bounded pool and can overlap Completion.
- `Done` remains independent of arbitrary user callback return. Permanent user callback blocking can prevent eventual execution; do not promise finite-time callback delivery.

**Not current runtime/API behavior:** `ErrCallbackPanic`, `Session.Diagnostics()`, mandatory Completion preservation and admission-reserved OnClose are owner-approved design changes, not verified implemented. They need a separately authorized source/test phase and updated release documentation.
