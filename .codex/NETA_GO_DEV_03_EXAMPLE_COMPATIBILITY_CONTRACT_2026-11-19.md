# DEV-03 — frozen basic Client/Server example compatibility contract
Date: 2026-11-19
Status: DEVELOPMENT CONTRACT FROZEN; lightweight cross-example smoke planned in DEV-04
Owner authority: DECISION_LOG §78; 12 runnable programs (2 retained + 10 new).
Reference feature production/source HEAD: `9ae13593bf24f27f1b0d31864fecb91b3ae6f847`.

## Existing entrypoints: do not rewrite or rename

| Original entrypoint | Required CLI | Default | Observed logging strings consumed by smoke |
| --- | --- | --- | --- |
| `go run ./examples/server` | `-listen` | `127.0.0.1:2775` | `listening on <addr>;`, `one-way event sequence=`, `Server stopped` |
| `go run ./examples/client` | `-addr`, `-system-id` | `127.0.0.1:2775`, `neta-example-client` | `Completion sequence=`, `one-way sequence=`, `Request sequence=` |

- Server uses demonstration-only nonempty BindValidator; recognizes one-way `event:` prefix and sends `echo: ` Request Responses.
- Client uses `SendRequest` plus Completion and `SendNoWait`. Successful calls are local queue admissions, never delivery ACK.
- Both register Ctrl-C / SIGINT signal contexts and use bounded graceful `Client.Stop`/`Server.Stop`; they intentionally continue running after the initial Request/Response until interrupted.
- Fixed V2 default remains unchanged, V1 remains explicit. Neither program gains a new required `-config` flag.
- `scripts/example-smoke.sh` currently captures coverage, builds two binary packages, binds Server to ephemeral `127.0.0.1:0`, reads readiness address from the logged `listening on ...;`, starts Client, checks both two-way and one-way events, signals SIGINT and validates `Server stopped`. Keep its success/failure and artifact contract; DEV-04 must not repurpose this script into heavy benchmarking.

## New layout and behavior

Separate independent `package main` directories: `examples/client-config`, `client-concurrent`, `client-reconnect`, `client-v1`, `client-diagnostics`, and Server peers `server-config`, `server-concurrency`, `server-bind-validator`, `server-v1`, `server-diagnostics`. Shared strict JSON sample fixtures go in `examples/config/`. A single small helper is acceptable only if genuinely reused; do not introduce a config framework/new exported production API.

- Only public `neta.NewClient/NewServer`, public Functional Options and Go standard library.
- Optional `-config` parser for Config examples: JSON strict unknown-field rejection, explicit CLI override precedence over JSON over library defaults, validation of durations/version/identity and limits; unspecified fields do not overwrite library defaults.
- All examples have bounded capacity and shutdown. Do not implicitly retry messages, renegotiate protocol, or report local admission as remote delivery.
- Preferred server readiness for DEV-04: printed actual `Server.Addr()` after `Start()`, port zero, deterministic parsing, bounded wait and process termination.

## Existing evidence vs fresh verification

At DEV-01 compare, the two existing `examples/*/main.go` and `scripts/example-smoke.sh` were **unchanged** vs main and unchanged by DEV-01/02/03 commits. The prior feature CI [44529571646](https://github.com/majiddarvishan/neta-go/actions/runs/44529571646) has verified **Run exact-SHA Go final verification: success** and **Build and run paired examples: success**. This is historical evidence for these unchanged entrypoints, not a new current-HEAD or new examples acceptance. No fresh tests for this documentation-only contract task.

**Acceptance:** compatibility contract recorded, historical build/smoke evidence identified, existing paths/flags/logs preserved in source. All new executable packages require their own fresh, bounded build/smoke through DEV-04 before DEV-06; any new compatibility break reopens DEV-03.
