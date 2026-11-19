# Multi-peer TCP routing example

A generic `neta-go` Server that combines strict JSON/CLI configuration, bounded callback workers and queues, and a synchronized registry of multiple concurrently Bound Client Sessions.

## Overview

The Server accepts any of the explicitly configured short Bind identities, stores each live Session under a logical peer ID, and selects the destination via an exact application-provided route key. A route is not a load balancer: an unknown, disconnected or saturated target fails explicitly. There is **no automatic fallback, replay, retry, persistence or discovery**.

```text
client-peer-router -system-id peer-east   ──bind──┐
                                                  ├── server-peer-router
client-peer-router -system-id peer-west   ──bind──┘     |
                                                     routing table
                                                     east -> peer-east-01
                                                     west -> peer-west-01
```

## Start the Server and two Clients

Run from the repository root in three terminals:

```bash
go run ./examples/server-peer-router -config examples/config/server-peer-router.json
go run ./examples/client-peer-router -addr 127.0.0.1:19090 -system-id peer-east
go run ./examples/client-peer-router -addr 127.0.0.1:19090 -system-id peer-west
```

Keep each process running in its own terminal. In the Server terminal:

```text
list
send east hello-east
send west hello-west
send missing should-fail
```

`list` reports configured identities and Bound status. `send` uses the configured route to find exactly one current Session, reserves an in-flight dispatch credit, and calls `SendRequest` once. The Client example uses `WithClientOnRequestHandler`, responds with `SendResponse`, and the Server prints its asynchronous Completion (status and echoed payload). Client requests to the Server are also handled, demonstrating **bidirectional Request/Response on a single Session**.

**Semantics:** Successful `SendRequest` or `SendResponse` means local admission only. A successful Completion means the peer produced a matching protocol Response, *not* that an external business transaction was committed.

## Configuration

See [the example JSON](../config/server-peer-router.json). It has two explicitly configured peers and `routes` from arbitrary text keys to their logical IDs. All capacity and timeout fields are bounded. The `-config` option is optional; strict JSON rejects unknown keys and extra JSON values. Explicit CLI options override JSON, which overrides built-in defaults. Use `-h` for available flags.

A Bind label is a short, unique transport identity (V1 max 19 bytes; V2 max 20). The demo validator rejects unknown labels and unidirectional binds. **It is not authentication.** Real applications must enforce their own identity, authorization, peer lifecycle and application protocol. The routing table is immutable until restart; there is no hot reload.

The Registry removes a peer on OnClose only if that callback still refers to the registered Session, so late old callbacks cannot invalidate a newer generation. Connection loss does not trigger a business replay. Resource limits apply to connection admission, queued frames, workers and in-flight outgoing Requests.

## Tests

```bash
go test ./examples/server-peer-router ./examples/client-peer-router -count=1
go test -race ./examples/server-peer-router ./examples/client-peer-router -count=1
```

Tests cover strict config/CLI precedence, invalid identities/routes, multiple real concurrent TCP peers, bidirectional response delivery, bounded admission, and offline routing refusal. This is an independent, general-purpose demonstration and not a production routing framework.
