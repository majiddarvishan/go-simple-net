# neta-go

A high-performance, Pinex-compatible TCP messaging library written in Go.

## Project status

Phases 1–8 are complete. The Phase 8 performance baseline and evidence review closed with **PERF-004 = NO CHANGE**; no production optimization or public-contract change was accepted.

**Phase 9 — documentation, examples and V1 release readiness — is owner-authorized and in progress.** The tracker reconciliation, Client/Server examples and public documentation are complete; examples compile and correctness/compatibility CI pass. Paired runtime smoke and release-readiness review remain in progress. No V1 release tag has been created or authorized.

- **[Implementation tasks and live progress](TASKS.md)**
- **[Phase 9 execution plan and acceptance criteria](.codex/NETA_GO_PHASE_9_RELEASE_READINESS_PLAN_2026-11-17.md)**
- [Client example](examples/client/main.go)
- [Server example](examples/server/main.go)
- [Documentation index](docs/README.md)
- [Phase 8 performance baseline and evidence](.codex/NETA_GO_PHASE_8_PERFORMANCE_BASELINE_2026-11-17.md)
- [Test and performance automation](.codex/NETA_GO_TEST_AUTOMATION_2026-11-17.md)

## Quick Start

The module directive is Go 1.23 and correctness CI currently uses Go 1.23.x. The final V1 GOOS/GOARCH support matrix is part of the release-readiness review.

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
