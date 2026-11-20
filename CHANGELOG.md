# Changelog

Notable neta-go changes are recorded here.

## Unreleased

Changes after the first integration RC will be recorded here. The pre-release tag is an integration snapshot and does not replace full Phase Q qualification.

## v1.0.0-rc.1 — 2026-11-20 (integration release candidate)

**Status:** usable pinned API/integration snapshot for parallel applications; **not** a qualified production/performance GA release. Phase Q validation, canonical retained memory, numerical baseline and absolute SLA remain open.

### Added
- Callback-oriented TCP Client and Server API in the root `neta` package.
- Fixed 10-byte Frame Header, bounded 1 MiB total Frame codec and raw Stream payloads.
- Session lifecycle, Request/Response correlation, bounded callback scheduling and resource accounting.
- Client and Server examples and API, configuration, protocol and lifecycle documentation.
- `WithClientOnRequestHandler` for incoming Requests on bidirectional Client Sessions; response with the original Sequence via `SendResponse`. A nil handler retains the prior ignore behavior; non-nil is incompatible with one-way Client Bind.
- General-purpose `server-peer-router` and `client-peer-router` examples: bounded multi-peer routing with strict configuration and two-way Request/Response.
- Immutable per-Session `ProtocolVersion` selection (`V1` or `V2`), default V2, via `WithProtocolVersion` and `Session.ProtocolVersion()`.
- V2 Bind Request/Response bodies without trailing NUL and up to 20 SystemID bytes. V1 retains its NUL-terminated Bind and 19-byte maximum. Header and Stream formats remain unchanged.

### Documentation and tooling
- Purpose-based test, script, Makefile and benchmark workflow names.
- A neta-go wire protocol reference replaces the former external-compatibility guide.

### Release status
- **Owner authorized one Git tag `v1.0.0-rc.1` for parallel integration**, gated on exact protected-main CI after merge ([approval record](.codex/PRERELEASE_TAG_APPROVAL_2026-11-20.md)); an actual tag is only valid once verified in GitHub refs.
- No **final** `v1.0.0` tag or GitHub Release is approved or created by this RC decision.
- Q-ENV-01, QA-004/005, MEM-004/005/006 and exact-SHA final qualification remain OPEN; Performance Gate DISABLED. See [parallel test plan](.codex/NETA_GO_PHASE_Q_PARALLEL_EXECUTION_PLAN_2026-11-20.md).
