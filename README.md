# neta-go

A high-performance, Pinex-compatible TCP messaging library written in Go.

## Project status

Phases 1–8 are complete. The Phase 8 performance baseline and evidence review closed with **PERF-004 = NO CHANGE**; no production optimization or public-contract change was accepted.

**Phase 9 — documentation, examples and V1 release readiness — is owner-authorized and in progress.** Tasks P9-00..P9-05 were complete on the earlier API baseline; a subsequent owner-authorized Go-only protocol version option (V2 default) requires fresh release-readiness requalification. Examples, correctness/compatibility CI, paired runtime smoke and protected-branch CI checks pass. The V1 release/tag task P9-06 remains separately gated on explicit owner approval and final admin settings / release permission preflight. No V1 release tag has been created or authorized.

- **[Implementation tasks and live progress](TASKS.md)**
- **[Phase 9 execution plan and acceptance criteria](.codex/NETA_GO_PHASE_9_RELEASE_READINESS_PLAN_2026-11-17.md)**
- [Client example](examples/client/main.go)
- [Server example](examples/server/main.go)
- [Documentation index](docs/README.md)
- [Approved Go and platform support baseline](docs/SUPPORT.md)
- [V1 release-readiness checklist](docs/RELEASE_READINESS.md)
- [Changelog](CHANGELOG.md)
- [Phase 8 performance baseline and evidence](.codex/NETA_GO_PHASE_8_PERFORMANCE_BASELINE_2026-11-17.md)
- [Test and performance automation](.codex/NETA_GO_TEST_AUTOMATION_2026-11-17.md)

## Quick Start

The owner-approved V1 minimum is **Go 1.23**. The continuously CI-verified platform is **Ubuntu 24.04 / Linux amd64**. Other platforms and architectures remain unverified; see [Support](docs/SUPPORT.md).

From a checkout of this repository, start the Server in one terminal:

```bash
go run ./examples/server -listen 127.0.0.1:2775
```

Then start the Client in another terminal:

```bash
go run ./examples/client -addr 127.0.0.1:2775 -system-id neta-example-client
```

The Client sends one Request and prints the echo Completion. It also sends a one-way `event:` message; the Server logs it without sending a Response. Press Ctrl-C in either process to exercise graceful shutdown.

The examples demonstrate API usage, not production authentication policy. The sample BindValidator accepts any non-empty SystemID; production applications must implement their own identity/authentication policy.

## Session protocol version (Go)

The locally configured Session version defaults to **V2**. To select V1 explicitly:

```go
client, err := neta.NewClient(
    neta.WithClientAddress("127.0.0.1:2775"),
    neta.WithClientSystemID("my-client"),
    neta.WithClientSessionOptions(neta.WithProtocolVersion(neta.ProtocolVersionV1)),
)
```

For a Server, use `WithServerSessionOptions(WithProtocolVersion(...))`.
`session.ProtocolVersion()` returns the immutable **local** version.

**V2 now omits the trailing `0x00` in Bind Request/Response**, while V1 remains byte-for-byte compatible with Pinex C++ 1.5.0. The 10-byte Frame Header and Stream format are unchanged. A Go V2 Server accepts legacy V1 Bind and answers in V1; a V1-only Server rejects V2. To reach a legacy Pinex Server, explicitly configure the Go Client for V1. No automatic fallback or version-negotiation Frame is sent. The protocol version is distinct from the library's proposed semantic release `v1.0.0`.

## Installing the development version

There is not yet a V1 release tag. To experiment with the current development branch from another Go module:

```bash
go get github.com/majiddarvishan/neta-go@main
```

This follows the moving development branch. For reproducible builds, pin an exact commit until a versioned release is published.

## Documentation

- [Public API reference](docs/API.md)
- [Configuration reference](docs/CONFIGURATION.md)
- [Lifecycle, callbacks, payload ownership and errors](docs/LIFECYCLE.md)
- [Pinex compatibility and verification evidence](docs/PINEX_COMPATIBILITY.md)

## License

**Apache License 2.0** — Copyright 2026 Majid Darvishan.

You may use neta-go in **commercial or proprietary projects**, and may modify and redistribute it, subject to the conditions of the Apache-2.0 license. In particular, redistribution must comply with applicable license, copyright and attribution notice requirements. The license does not require you to publish your proprietary application's source code merely because it uses this library.

See [LICENSE](LICENSE) for the full terms. No separate non-commercial restriction applies.
