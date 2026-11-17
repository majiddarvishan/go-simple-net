# neta-go V1 — Phase 1 Implementation Report

**Date:** 2026-11-17  
**Status:** COMMITTED / VALIDATION PASS / DRAFT PR #1 OPEN — MERGEABLE CLEAN  
**Repository:** \`github.com/majiddarvishan/neta-go\`  
**Branch:** \`feature/phase1-config-implementation-2026-11-17\`  
**Base main HEAD:** \`eb13272d851fa7d9aa6b3f30f90e65dbd760f91d\`

## 1. Environment and repository baseline

- Go toolchain: \`go1.23.2 linux/amd64\`.
- Module directive: \`go 1.23\`.
- No third-party modules; \`go list -m all\` lists only \`github.com/majiddarvishan/neta-go\`.
- The remote repository had no Go source or \`go.mod\` before this phase.
- \`.codex/AGENT_CONTEXT.md\` is absent.
- Pinex C++ compatibility branch \`develop\` reverified at \`f7bd1eb779baff0d10e624f20522df345e6ee747\`.

## 2. Files introduced

- \`go.mod\`
- \`doc.go\`
- \`api_types.go\`
- \`state.go\`
- \`errors.go\`
- \`config.go\`
- \`options.go\`
- \`constructors.go\`
- \`runtime.go\`
- \`config_test.go\`
- \`external_api_test.go\`

## 3. Configuration contract implemented

- Module path \`github.com/majiddarvishan/neta-go\`; sole public package identifier \`neta\`.
- Typed, independent \`ClientOption\`, \`ServerOption\`, \`SessionOption\` with private effective-config targets.
- Complete defaults for Client, Server, Session and owner worker/resource ceilings.
- Apply every option first; validate the completed configuration afterward.
- Constructors have no network or goroutine side effects.
- Owner worker defaults derive once from \`runtime.GOMAXPROCS(0)\` using overflow-safe capped multiplication.
- All WORKER-009C hard maxima and valid ranges enforced.
- Count/byte minima, 10-byte retained payload minimum, 1ms..10m worker idle grace and positive timeout requirements enforced.
- Only zero-valued exception is \`EnquireLinkIdleInterval=0\`.
- Owner and Session limits are validated independently; no invalid theoretical-sum cross-field restriction is imposed.
- Client and Server owner ceilings are instance-scoped, not process-global.
- Protocol-required System ID options and Client Bind Type option included. System ID is empty-valid, NUL-free and at most 19 data bytes; Bind Type accepts 0/1 and defaults to 1.
- Stable errors support \`errors.Is\`; ConfigError/OperationError support \`errors.As\`.

## 4. Validation evidence

| Check | Result |
|---|---|
| gofmt | PASS |
| go build ./... | PASS |
| go test ./... | PASS — 19 tests |
| go test -race ./... | PASS |
| go vet ./... | PASS |
| go test -cover ./... | PASS — 84.6% |
| go list -m all | Standard library only |

The external-package test confirms consumers can construct Client/Server using only exported Functional Option constructors while the effective configuration types remain private.

## 5. Deliberate scope boundary

This phase does not implement TCP operations, Session lifecycle methods, protocol codecs, 1 MiB frame enforcement, Request tracking, resource ledgers, worker schedulers, reconnect, shutdown or Go↔C++ interoperability. It does not run a listener, benchmark, deploy, add dependencies or change the Wire protocol.

## 6. API-001 completion note

The frozen API-001 option mapping did not explicitly list the System ID required by Bind Request/Bind Response, or Client Bind Type. The source-verified Pinex contract requires these fields. Phase 1 therefore adds \`WithClientSystemID\`, \`WithClientBindType\` and \`WithServerSystemID\` as protocol-required configuration options. Empty System ID remains valid to preserve the C++ default; maximum length and NUL rules follow \`c_octet_str<20>\`. This is a configuration-surface completion only; it changes no wire bytes or lifecycle decision.

## 7. Next phase

After committing and reviewing Phase 1 on the feature branch, proceed to Phase 2: pure-Go \`internal/wire\`, fixed 10-byte Header, command/status validation, Bind/Stream/Unbind/Enquire Link codecs, 1 MiB total-frame enforcement and deterministic golden vectors. Do not begin Phase 2 until Phase 1 is committed and reviewed.
