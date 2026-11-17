# Changelog

All notable changes to neta-go will be recorded here.

## Unreleased

### Added
- Callback-oriented TCP Client and Server API in the root `neta` package.
- Pinex-compatible 10-byte Frame Header and bounded 1 MiB total Frame codec.
- Session lifecycle, Request/Response correlation, bounded callback scheduling and resource accounting.
- Client and Server examples, public API/configuration/lifecycle documentation and Pinex compatibility notes.\n- Go-only immutable Session `ProtocolVersion` selection (`V1` / `V2`), default V2, via `WithProtocolVersion` and `Session.ProtocolVersion`; the wire encoding remains unchanged.

### Compatibility
- The currently verified interoperability target is Pinex C++ 1.5.0 at source commit `f7bd1eb779baff0d10e624f20522df345e6ee747`.
- Other Pinex versions and non-Linux/amd64 platforms are not yet part of the verified release matrix.

### Release status
- No V1 tag or GitHub Release has been created.
- The V1 release remains gated on the release-readiness checklist and separate explicit owner approval.
