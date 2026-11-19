# SMPP Server internal multi-ME neta-go example

This is a **new internal transport example**, NOT an SMPP protocol server or production SMSC implementation. It combines `server-config` (strict optional JSON/CLI and Functional Options) with `server-concurrency` (bounded workers, queues, multiple simultaneous bound peers). The SMPP Server process **accepts TCP neta Sessions from multiple Message Engine processes** and records them in a concurrency-safe, volatile registry.

## Demonstration topology

```text
SMPP Server process (internal neta Server)
  listener :19090
    Bind "me-east" --> logical ME "me-east-01" --> exact route "east"
    Bind "me-west" --> logical ME "me-west-01" --> exact route "west"
  OnBound / OnClose --> volatile ME -> Session registry
  SMPP internal router --> route key -> selected ME Session -> SendRequest
                         (NO automatic retry, replay or alternate ME)
```

The JSON mapping is intentionally **static and exact** for this demonstration; no phone-number/MSISDN routing policy is invented by the network library. The owning SMSC application provides the route key or performs its real internal routing before calling a handler equivalent to `registry.dispatch(routeKey, protobufBytes, completion)`. The Bind ID is a **short connection label** (V2 max 20 bytes; V1 max 19), not the durable ME node ID nor authenticated process identity.

## Run two ME connections

From the repository root:

```bash
go run ./examples/server-smpp-multi-me -config examples/config/server-smpp-multi-me.json
```

Open separate terminals and keep both *original* neta Client examples running:

```bash
go run ./examples/client -addr 127.0.0.1:19090 -system-id me-east
```

```bash
go run ./examples/client -addr 127.0.0.1:19090 -system-id me-west
```

In the Server terminal:

```text
list
send east sample-dlr-command
send west sample-other-command
send unknown should-fail
```

`list` displays configured ME identities and current Bound state. `send` maps an exact route to a single current bound Session, reserves a global in-flight credit, and calls `Session.SendRequest` once. Unknown routes, offline MEs, admission pressure or invalid Bind identities fail explicitly; **no retry or reroute** is performed. The body in this console demo is plain text; **production SMSC uses the approved versioned Protobuf payload** without adding another TCP length prefix.

**Critical current limitation:** the public `neta-go` Client API has no `WithClientOnRequestHandler`. A Server-originated Request can be locally admitted and put on the wire, but the demo ME Client has no application handler to receive it or send a matching Response. Expect `ErrRequestTimeout` in the Server's outbound Completion, **not a business success**. Do **not** use this example to claim working two-way SMSC dispatch or D-108 end-to-end proof. A narrowly reviewed additive Client inbound-handler API, real ME application HELLO/role/incarnation validation, and a paired end-to-end test are separate prerequisites.

## Config and behavior

- [Sample JSON](../config/server-smpp-multi-me.json): two ME mappings, exact route keys, all Server worker/queue/session budgets and timeout options.
- `-config` is optional: built-in defaults allow two test ME IDs. Unknown JSON fields and trailing JSON values fail; file size limit is 1 MiB.
- Explicit CLI flags override JSON, which overrides defaults. Supported flags: `-listen`, `-max-bound`, `-app-workers`, `-app-concurrency`, `-queue-frames`, `-queue-bytes`, `-max-inflight`, `-request-timeout`.
- All Session and owner options use the published `neta-go` Functional Options. Examples are immutable at startup; they do not hot-reload route maps.
- Every configured ME's Bind label must be unique and must use a bidirectional Bind. A duplicate live SystemID is rejected by `neta-go`; the registry's pointer-identity cleanup prevents a late old OnClose from removing a newer Session.
- The registry only stores *live Session pointers*; disconnected sessions are not replayed, messages are not queued persistently, and an accepted Request's Completion must not be mistaken for a durable business result.
- Bound is **not application READY** in this demo. Production must perform the approved HELLO/HELLO_ACK checks for full node ID, role, expected counterpart, incarnation/fencing and Protobuf major version **before** allowing business commands. Bind allowlists alone are not authentication.

## Test locally

```bash
go test ./examples/server-smpp-multi-me -count=1
go test -race ./examples/server-smpp-multi-me -count=1
```

Focused tests validate JSON/CLI precedence, duplicate/unknown IDs, route configuration, two real concurrently Bound Clients, exact target selection, bounded outbound admission, expected unhandled-request timeout and disconnected-ME rejection. They do **not** replace SMSC Protobuf/integration/qualification tests.

## SMSC contract notes

D-107 and D-108 specify the ME as the connecting `neta-go` Client and the SMPP process as the accepting `neta-go` Server. One bidirectional logical Session per configured ME/SMPP process pair should carry original SUBMIT toward ME and DLR dispatch in the opposite direction. Keep distinct immutable SMSC business correlation IDs in Protobuf, never transport Sequence. The Message Engine controls durable WAL acceptance and physical-operation authority; this example neither persists those decisions nor implements an SMPP ESME listener.

This is an *additive example only*. It does not change `neta-go` production API, protocol, benchmark source anchors, qualification state or Performance Gate.
