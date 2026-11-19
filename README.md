# neta-go

**neta-go** is a Go library for TCP messaging with asynchronous callbacks, bounded queues, request/response correlation and configurable per-Session concurrency.

It provides Client, Server and Session APIs without requiring applications to manage framing, connection readers/writers or callback worker pools.

## Features

- TCP Client and Server with a Bind handshake, graceful Unbind and explicit Close.
- Request/Response and one-way messages using raw byte payloads.
- Nonblocking queue admission with configurable Session and owner resource budgets.
- Separate bounded Application, Completion and OnClose callback workers.
- Optional Client reconnect without automatic message replay.
- Configurable idle-only Enquire Link and operation deadlines.
- Immutable, Session-scoped V1/V2 wire selection; **V2 is the default**.

## Quick start

Requires Go **1.23+**. CI currently verifies Linux/amd64 on Ubuntu 24.04.

Start the Server:

```bash
go run ./examples/server -listen 127.0.0.1:2775
```

Then start the Client in another terminal:

```bash
go run ./examples/client -addr 127.0.0.1:2775 -system-id neta-example-client
```

The Client sends a Request, receives an asynchronous Completion and submits a one-way event. Both examples support graceful shutdown with Ctrl-C. The sample BindValidator is intentionally minimal; a production application must provide its own identity policy.

## Session protocol versions

Both versions use the same 10-byte Frame Header and Stream payload representation. Each Session fixes its version once.

| | V1 | V2 (default) |
|---|---|---|
| Bind body | BindType + SystemID + terminating NUL | BindType + SystemID, no terminator |
| Maximum SystemID | 19 bytes | 20 bytes |
| Header / Stream | Unchanged | Unchanged |

Choose V1 explicitly when connecting to a V1-only peer:

```go
client, err := neta.NewClient(
    neta.WithClientAddress("127.0.0.1:2775"),
    neta.WithClientSystemID("example"),
    neta.WithClientSessionOptions(neta.WithProtocolVersion(neta.ProtocolVersionV1)),
)
```

A default V2 Server accepts V1 or V2 Bind requests and replies using the detected version. A V1-only Server rejects V2. There is no automatic fallback or version-negotiation exchange. `Session.ProtocolVersion()` returns the effective version.

See the [wire protocol reference](docs/PROTOCOL.md) for the full Frame structure, commands, boundaries and version semantics.

The [twelve-example guide](examples/README.md) covers CLI/JSON configuration, concurrent Client/Server workloads, reconnect, explicit V1 and Diagnostics.

## Development

```bash
make verify       # format, vet, unit tests and race detector
make smoke        # coverage and paired Client/Server example smoke
make perf-smoke   # short benchmark
make benchmark    # full benchmarks, profiling and memory soak
make soak         # memory stability soak (NETA_SOAK_SECONDS=300..600)
```

Generated logs and profiles live in ignored `artifacts/` directories.

## Documentation

- [Documentation index and current status](docs/README.md)
- [Public API](docs/API.md)
- [Configuration](docs/CONFIGURATION.md)
- [Lifecycle and callbacks](docs/LIFECYCLE.md)
- [Wire protocol](docs/PROTOCOL.md)
- [Go and platform support](docs/SUPPORT.md)
- [Performance qualification methodology](docs/PERFORMANCE_QUALIFICATION.md)
- [Release readiness and pending approval gates](docs/RELEASE_READINESS.md)
- [Changelog](CHANGELOG.md)
- [Implementation tasks](.codex/TASKS.md)

## Status and license

The library is under active development; `v1.0.0` is **not** tagged or published. Until a release exists, consumers can use `go get github.com/majiddarvishan/neta-go@main`; pin an exact commit for reproducible builds.

Licensed under [Apache-2.0](LICENSE), Copyright 2026 Majid Darvishan. Commercial and proprietary use is permitted subject to the license conditions.
