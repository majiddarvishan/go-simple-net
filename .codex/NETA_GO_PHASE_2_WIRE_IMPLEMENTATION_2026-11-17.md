# neta-go Phase 2 — Wire Protocol Implementation Record

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Status:** COMPLETE — GO CI PASS + PINEX C++ GOLDEN COMPATIBILITY CI PASS  
**Current implementation HEAD:** `b026dadd9ad0b33118b05f49477e1412a4255978`  
**Compatibility target:** Pinex C++ 1.5.0, develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`

## 1. Authorization and boundary

The owner explicitly approved Phase 2 by replying «ادامه بده». This authorizes only the pure-Go wire codec, its unit/golden tests, documentation and CI.

Not authorized: TCP transport/read-write loops, Client/Server runtime integration, Session lifecycle, worker scheduling, resource-admission runtime, reconnect, live socket-level Go↔C++ testing, benchmarks, release or deployment. Those remain Phase 3+ and require separate owner approval.

## 2. Package boundary

- Package: `github.com/majiddarvishan/neta-go/internal/wire`.
- Standard library only.
- No import from root package `neta`; root `neta` may depend on `internal/wire`.
- Codec functions operate on exactly one complete Frame. TCP partial-read accumulation and splitting concatenated frames belong to the future transport layer.

## 3. Wire contract

Header size is exactly 10 bytes:

| Offset | Size | Field | Encoding |
|---:|---:|---|---|
| 0 | 4 | total frame length, including Header | unsigned BE u32 |
| 4 | 1 | command ID | u8 |
| 5 | 1 | command status | u8 |
| 6 | 4 | sequence | unsigned BE u32 |

Maximum total Frame is 1,048,576 bytes including Header. Maximum Body is 1,048,566 bytes. This fixed limit applies inbound and outbound and is not configurable.

Command IDs:

| PDU | Command |
|---|---:|
| Bind Request | 0x01 |
| Stream Request | 0x02 |
| Unbind Request | 0x03 |
| Enquire Link Request | 0x04 |
| Bind Response | 0x81 |
| Stream Response | 0x82 |
| Unbind Response | 0x83 |
| Enquire Link Response | 0x84 |

Status: `rok=0x00`, `rfail=0xFF`. Other raw status bytes are preserved by the Header codec, as in Pinex's enum cast and dispatch path.

Bind Body: `bind_type` u8 (0=uni_direction, 1=bi_direction), followed by `system_id` as NUL-terminated C-octet string. Pinex uses `c_octet_str<20>`, so usable system ID length is 0..19 bytes. Bind Response echoes the selected bind type and server system ID.

Pinex marks Bind Response as omittable. An empty Bind Response Body decodes to the C++ struct defaults: `bi_direction` and empty system ID. Go now matches this behavior.

Stream Request/Response Body is the raw remaining bytes; no extra length prefix or mandatory Protobuf envelope exists.

Unbind and Enquire Link Request/Response have empty Bodies. Responses reuse the request Sequence. The per-Session Sequence starts at 1, increments, and skips zero on uint32 wraparound.

## 4. Source audit and malformed-input policy

Audited production sources include `common/serialization.hpp`, Bind/Stream PDU definitions, command/status enums and `net/session.hpp` at the pinned commit.

- Header serialization/deserialization uses the exact 10-byte layout above.
- Pinex validates inbound total length in the inclusive range [10, 1,048,576].
- The 1 MiB capacity is used before and after Bind.
- `bind_type` is the first Bind Body byte; `system_id` follows it.
- Bind Response and Stream Response permit an omitted/empty Body under their C++ `can_be_omitted` behavior.
- Stream serialization is the raw `std::string` contents.
- Sequence increments from zero and explicitly skips zero after wraparound.
- Unbind and Enquire Link are header-only commands; Pinex responds with the incoming Sequence.

Go rejects malformed Bind fields (invalid enum value, missing terminal NUL, embedded NUL or trailing bytes after the first terminator) and nonempty control Bodies. The legacy C++ parser is more permissive for some malformed inputs; Go's stricter rejection is intentional and does not alter valid-frame compatibility.

## 5. C++ golden fixture provenance

`testdata/pinex-1.5.0/` contains:

- `golden.cpp`: fixture generator that compiles against the pinned Pinex production PDU serializer.
- `golden.txt`: full-frame golden vectors consumed by Go tests.
- `source/include/pa/pinex/`: verbatim snapshot of the 12 production headers required by the generator.
- `source/SNAPSHOT.md`: upstream commit and original Git blob SHA values for each copied header.

Pinned source: `majiddarvishan/pinex` develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0.

The C++ generator uses the actual production `serialize_to` and `deserialize` PDU templates for Bind and Stream Body bytes. Its 10-byte header writer mirrors the audited `session::serialize_header` implementation in `include/pa/pinex/net/session.hpp` at that exact commit. Control frames are emitted as header-only frames, matching `session::send_command`.

The Pinex repository is private and neta-go's GitHub Actions runner has no cross-repository read credential. CI therefore compiles the verbatim pinned source snapshot checked into neta-go rather than checking out Pinex dynamically. The snapshot's original Git blob SHA values are retained to make provenance auditable.

This proves the Go codec against compiled C++ production PDU serialization and the audited header layout. It is not a live TCP/socket-level Go↔C++ integration test.

## 6. Golden vectors

The fixture set covers:

- Bind Request: `00000010010000000007016e6f646500`
- Bind Response: `0000000f81ff000000070073727600`
- Stream Request: `0000000c020001020304aabb`
- Stream Response: `0000000f82ff0000002a000102feff`
- Empty Bind Response: `0000000a81ff00000007`
- Unbind Request/Response and Enquire Link Request/Response using Sequence `0x12345678`.

The compatibility workflow recompiles the C++ generator, regenerates all vectors and diffs them against `golden.txt`; Go tests then compare all encoder output against the same vectors and decode selected C++ frames.

## 7. Implemented Go files

- `internal/wire/frame.go`: Header, CommandID, CommandStatus, Frame, ParseHeader, MarshalBinary, EncodeFrame, DecodeFrame and fixed bounds.
- `internal/wire/pdu.go`: Bind codec, raw Stream codec wrappers, empty control PDU encoders.
- `internal/wire/sequence.go`: concurrency-safe incrementing nonzero Sequence generator.
- `internal/wire/wire_test.go`: source-derived golden vectors, malformed/boundary tests, empty Bind Response compatibility, Stream raw-body tests and Sequence wrap/concurrency tests.

## 8. Verification

- Current verified main HEAD: `b026dadd9ad0b33118b05f49477e1412a4255978`.
- Go workflow run `44044273742`: PASS — gofmt, go vet, go test, go test -race.
- Pinex Wire Compatibility workflow run `44044273818`: PASS — C++ fixture generator compile, fixture regeneration/diff, Go golden compatibility tests.

## 9. Phase 2 completion and next boundary

**Phase 2 Wire Codec: COMPLETE.** No architecture decision is reopened.

Next possible phase is TCP Transport and runtime integration, but it is not authorized by the Phase 2 approval. Obtain explicit owner authorization before starting it. No TCP transport, runtime, workers, reconnect, benchmark, release or deployment implementation has been started in Phase 2.
