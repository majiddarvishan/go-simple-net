# Configuration Reference

All configuration uses typed Functional Options:

- `ClientOption` configures a Client.
- `ServerOption` configures a Server.
- `SessionOption` configures immutable defaults inherited by Sessions created by a Client or Server.

Options are applied before final validation. The resulting configuration is immutable for the lifetime of that owner; there is no live hot reload. A zero-value/nil option is invalid. A nil option slice is equivalent to no additional Session overrides.

## Client options

| Option | Default | Validation / behavior |
|---|---:|---|
| `WithClientAddress` | Required | Valid TCP host:port with a non-empty remote host |
| `WithClientSystemID` | Required | Non-empty, no embedded NUL. **V1: 19 bytes maximum** (plus a required terminator); **V2: 20 bytes maximum** (no terminator). Measured in bytes, not Unicode characters. |
| `WithClientBindType` | `BindTypeBiDirectional` | `BindTypeUniDirectional` or `BindTypeBiDirectional` |
| `WithConnectTimeout` | 30s | Positive duration; absolute per-Dial-attempt deadline |
| `WithBindResponseTimeout` | 30s | Positive duration; absolute Bind handshake deadline |
| `WithAutoReconnect` | false | Explicit opt-in |
| `WithClientOnBoundHandler` | nil | Optional Bound-generation callback |
| `WithClientOnCloseHandler` | nil | Optional best-effort OnClose callback |
| `WithClientSessionOptions` | Session defaults | Applies copied Session options to every Client generation |
| `WithClientApplicationWorkers` | GOMAXPROCS-derived | 1..512 active Application workers per Client |
| `WithClientCompletionWorkers` | GOMAXPROCS-derived | 1..256 active Completion workers per Client |
| `WithClientOnCloseWorkers` | GOMAXPROCS-derived | 1..128 active OnClose workers per Client |
| `WithClientApplicationWorkerIdleGrace` | 30s | 1ms..10m |
| `WithClientCompletionWorkerIdleGrace` | 30s | 1ms..10m |
| `WithClientOnCloseWorkerIdleGrace` | 30s | 1ms..10m |
| `WithClientOnCloseEventCredits` | 1024 | 1..4096 queued + running OnClose events |
| `WithClientMaxOutstandingRequests` | 32768 | 1..131072 owner-wide Request credits |
| `WithClientMaxRetainedApplicationBytes` | 512 MiB | 10 bytes..2 GiB |
| `WithClientMaxRetainedCompletionBytes` | 256 MiB | 10 bytes..1 GiB |

## Server options

| Option | Default | Validation / behavior |
|---|---:|---|
| `WithServerListenAddress` | Required | Valid TCP host:port; an empty host is permitted for wildcard listening |
| `WithBindValidator` | nil | Optional Bind admission callback; nil accepts otherwise valid unique System IDs |
| `WithServerOnBoundHandler` | nil | Optional Bound callback |
| `WithServerOnRequestHandler` | nil | Optional inbound Application Request callback |
| `WithServerOnCloseHandler` | nil | Optional best-effort OnClose callback |
| `WithServerSessionOptions` | Session defaults | Applies copied Session options to every accepted Session |
| `WithServerPreBindHandshakeTimeout` | 30s | Positive absolute accept-to-Bind deadline |
| `WithServerMaxBoundSessions` | 1024 | Must be positive; no separate upper hard maximum is imposed by the current validator |
| `WithServerMaxPendingPreBindSessions` | 128 | Must be positive; no separate upper hard maximum is imposed by the current validator |
| `WithServerMaxAggregateQueuedApplicationBytes` | 256 MiB | Positive Server-wide queued Application byte budget |
| `WithServerMaxAggregateQueuedApplicationFrames` | 65536 | 1..262144 aggregate queued Application frames |
| `WithServerApplicationWorkers` | GOMAXPROCS-derived | 1..512 active Application workers per Server |
| `WithServerCompletionWorkers` | GOMAXPROCS-derived | 1..256 active Completion workers per Server |
| `WithServerOnCloseWorkers` | GOMAXPROCS-derived | 1..128 active OnClose workers per Server |
| `WithServerApplicationWorkerIdleGrace` | 30s | 1ms..10m |
| `WithServerCompletionWorkerIdleGrace` | 30s | 1ms..10m |
| `WithServerOnCloseWorkerIdleGrace` | 30s | 1ms..10m |
| `WithServerOnCloseEventCredits` | 1024 | 1..4096 queued + running OnClose events |
| `WithServerMaxOutstandingRequests` | 32768 | 1..131072 owner-wide Request credits |
| `WithServerMaxRetainedApplicationBytes` | 512 MiB | 10 bytes..2 GiB |
| `WithServerMaxRetainedCompletionBytes` | 256 MiB | 10 bytes..1 GiB |

## Session options

| Option | Default | Validation / behavior |
|---|---:|---|
| `WithProtocolVersion` | `ProtocolVersionV2` | Only `ProtocolVersionV1` / `ProtocolVersionV2`. V1 uses NUL-terminated Bind; V2 omits the NUL in Bind Request/Response. A new Go V2 Server accepts both and replies in kind; a V1-only Server rejects V2. Clients must explicitly choose V1 for V1-only peers. No automatic fallback. |
| `WithCallbackPanicRingCapacity` | 8 | 1..32 recent panic records; lazily allocated on first panic |
| `WithCallbackPanicByteBudget` | 256 KiB | 64 KiB..2 MiB recent record-text budget; First/OnClose and foreign graphs excluded |
| `WithRequestTimeout` | 30s | Positive; starts after successful local Request FIFO admission |
| `WithFrameAssemblyTimeout` | 30s | Positive absolute inbound Frame assembly deadline |
| `WithUnbindTimeout` | 10s | Positive absolute Graceful Unbind deadline |
| `WithWriteTimeout` | 30s | Positive absolute per-Frame Writer deadline |
| `WithEnquireLinkIdleInterval` | 30s | Zero disables outgoing idle probes; negative is invalid |
| `WithEnquireLinkResponseTimeout` | 10s | Positive matching-response deadline |
| `WithMaxQueuedOutgoingBytes` | 4 MiB | Positive per-Session outgoing Application FIFO byte budget |
| `WithMaxQueuedIncomingBytes` | 4 MiB | Positive per-Session incoming Application backlog byte budget |
| `WithMaxQueuedControlBytes` | 4 KiB | Positive per-Session reserved Control-PDU byte budget |
| `WithApplicationCallbackConcurrency` | 1 | 1..32 concurrent Application callbacks per Session |
| `WithCompletionCallbackConcurrency` | 1 | 1..32 concurrent Completion callbacks per Session |
| `WithMaxIncomingApplicationFrames` | 256 | 1..1024 incoming Application frames per Session |
| `WithMaxOutgoingApplicationFrames` | 256 | 1..1024 outgoing Application frames per Session |
| `WithMaxOutstandingRequestsPerSession` | 128 | 1..512 outstanding Request credits per Session |
| `WithMaxRetainedApplicationBytesPerSession` | 16 MiB | 10 bytes..64 MiB |
| `WithMaxRetainedCompletionBytesPerSession` | 16 MiB | 10 bytes..64 MiB |

Queue byte budgets and aggregate queued-byte budgets must be positive; the current validator does not impose a separate numeric upper hard maximum on them. Timeout values must be positive except that `EnquireLinkIdleInterval=0` explicitly disables outgoing probes. The current timeout validators do not impose an additional upper hard maximum.

## GOMAXPROCS-derived worker defaults

Let `P = max(1, runtime.GOMAXPROCS(0))`.

- If `P < 32`: Application workers = `max(4, 4*P)`; Completion workers = `max(2, 2*P)`; OnClose workers = `max(2, P)`.
- If `P >= 32`: defaults are capped at 128 Application, 64 Completion and 32 OnClose workers.

These are owner-wide active-worker ceilings, not per-Session goroutine counts. Workers are lazy and idle workers retire after the configured grace. Hard maxima are 512/256/128 respectively.

## Independent resource limits

The limits are separate budgets, not one combined process-memory cap:

- Outgoing and incoming Application queue bytes/frames are queue-admission budgets.
- Control bytes are reserved separately from Application traffic.
- Per-Session and owner-wide outstanding Request credits are independent.
- Per-Session and owner-wide retained Application and Completion byte budgets are independent.
- The Server aggregate queued Application budget covers queued frames across its Sessions.
- Worker ceilings bound active callbacks by owner and callback class.

There are intentionally no product constraints requiring an owner budget to equal a particular multiple of a per-Session budget. Actual process RSS also includes Go runtime, stacks, transport buffers, application allocations and other memory outside these library ledgers.

## Fixed protocol limit

The maximum neta-go Frame is **1,048,576 bytes total**, including the 10-byte Header. The maximum Body is 1,048,566 bytes. This limit is fixed and cannot be raised by configuration. Queue and retained-byte limits are separate from the protocol Frame maximum.

## Runnable settings demonstrations

See [runnable Client/Server examples](../examples/README.md) for strict optional JSON, CLI precedence, defaults, error cases and scenario-specific Functional Options.
