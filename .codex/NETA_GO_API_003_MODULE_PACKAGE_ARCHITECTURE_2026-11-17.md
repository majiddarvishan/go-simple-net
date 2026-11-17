# neta-go V1 — API-003 Module and Package Architecture

**Date:** 2026-11-17  
**Status:** CLOSED / OWNER APPROVED / OPTION A  
**Repository:** github.com/majiddarvishan/neta-go / main  
**Implementation:** NOT AUTHORIZED. No go.mod, source, tests or dependency changes are created by this design record.

## 1. Approved module and package identity

- Go module path: github.com/majiddarvishan/neta-go.
- Root package identifier: neta.
- V1 exposes exactly one public import package: the repository root.
- Repository and module name retain neta-go; Go package identifier is neta because Go identifiers cannot contain a hyphen.
- This decision does not authorize creating go.mod or changing repository metadata.

## 2. Approved V1 source layout

    neta-go/
      go.mod
      client.go
      server.go
      session.go
      request.go
      options.go
      errors.go
      state.go
      internal/
        wire/
          frame.go
          codec.go
          command.go
          bind.go
          stream.go
          control.go
      unexported root-package runtime files:
        client_runtime.go
        server_runtime.go
        session_runtime.go
        reader.go
        writer.go
        request_tracker.go
        admission.go
        worker_scheduler.go
        lifecycle.go
      examples/
      testdata/
      internal test packages

The listing is a logical responsibility map, not a mandate to create every file at once. Files may be combined until implementation complexity justifies separation.

## 3. Public / internal boundary

- Export only Client, Server, Session, Request, Response, BindInfo, SessionState, typed Functional Option types, documented callback types and stable error classifications.
- Keep protocol codecs, Frame/PDU representations, Reader/Writer loops, state-machine internals, pending trackers, worker schedulers, resource ledgers and transport adapters private.
- Do not create public client, server, session or protocol subpackages in V1.
- internal/wire is the only initially required internal package. It is pure Go, imports only the standard library, and must not import root package neta. Root neta may import internal/wire.
- Runtime mechanisms stay as unexported files in root package initially. Split additional internal packages only when code ownership or dependency boundaries provide a demonstrated benefit.

## 4. Dependency direction and constraints

- internal/wire depends only on Go standard library primitives and has no dependency on runtime, Client, Server or Session.
- Root package neta may depend on internal/wire and standard library.
- No cyclic dependency is permitted.
- No mandatory Protobuf, C++ bridge, third-party event loop, metrics exporter or business framework dependency is part of V1.
- Prefer standard library TCP, context, synchronization and time primitives. Any future external dependency requires separate engineering justification and owner approval if it changes the public/runtime contract.

## 5. Rationale and future extraction

One root public package minimizes import friction and avoids prematurely exposing implementation boundaries that would become compatibility commitments. The internal boundary prevents downstream applications from depending on codec and scheduler internals. Starting with one pure-Go wire package and unexported runtime files minimizes adapter/interface overhead on hot paths while retaining later refactoring freedom.

A new internal package may be extracted only when it has a stable dependency boundary, materially improves isolated testing without hot-path abstraction overhead, or is independently reusable with a clear owner. A public subpackage requires a new API decision and is outside the V1 freeze.

**Decision:** API-003 is CLOSED / OWNER APPROVED / OPTION A. Implementation remains NOT AUTHORIZED.