# Changelog

Notable neta-go changes are recorded here.

## Unreleased

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
- No `v1.0.0` tag or GitHub Release has been created.
- The amended release notes and release publication require explicit owner approval.
