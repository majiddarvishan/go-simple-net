# neta-go Phase 2 — Wire Protocol Implementation Record

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Compatibility target:** Pinex C++ 1.5.0, develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`

## Authorization

The owner explicitly approved Phase 2 in this conversation by replying «ادامه بده» to the direct authorization question. Scope is limited to pure wire encoding/decoding and tests. Transport and runtime are not authorized.

## Package boundary

- Package: `github.com/majiddarvishan/neta-go/internal/wire`
- Standard library only.
- No import from root `neta`.
- Root package may import `internal/wire`.
- Frame codec is complete-frame oriented. TCP partial-read accumulation belongs to the later transport phase.

## Wire contract

Header size is exactly 10 bytes:

| Offset | Size | Field | Encoding |
|---:|---:|---|---|
| 0 | 4 | total frame length, including Header | unsigned BE u32 |
| 4 | 1 | command ID | u8 |
| 5 | 1 | command status | u8 |
| 6 | 4 | sequence | unsigned BE u32 |

Maximum total Frame is 1,048,576 bytes including Header. Maximum Body is 1,048,566 bytes. The fixed maximum is enforced in both Encode and Decode; it is not configurable.

Command IDs: Bind Request 0x01, Stream Request 0x02, Unbind Request 0x03, Enquire Link Request 0x04; Bind Response 0x81, Stream Response 0x82, Unbind Response 0x83, Enquire Link Response 0x84. Status values: rok 0x00, rfail 0xFF.

Bind Request and Response Body: bind_type u8 (0=uni_direction, 1=bi_direction), then system_id as C-octet string terminated by NUL. Pinex uses `c_octet_str<20>`, rejecting strings with length >=20, so maximum system_id is 19 bytes. Embedded NUL is invalid. Bind Response echoes the requested bind_type and uses the server system_id.

Stream Request/Response Body is the raw remaining Body, without an extra length prefix or mandatory Protobuf envelope.

Unbind and Enquire Link Request/Response carry no Body. Their response uses the same Sequence as the request. Per-Session request Sequence increments and skips zero on wraparound.

## Implemented files

- `internal/wire/frame.go`: Header, CommandID, CommandStatus, Frame, ParseHeader, MarshalBinary, EncodeFrame, DecodeFrame and fixed bounds.
- `internal/wire/pdu.go`: Bind codec, raw Stream codec wrappers, empty control PDU encoders.
- `internal/wire/sequence.go`: concurrency-safe incrementing nonzero Sequence generator.
- `internal/wire/wire_test.go`: header and Bind golden vectors, control frames, frame boundaries, malformed/truncated frames, Bind validation, wraparound and concurrent Sequence uniqueness.

## Golden vectors

- Stream Request: `0000000c020001020304aabb` (length 12, command 0x02, status 0, sequence 0x01020304, Body aa bb).
- Bind Request: `00000010010000000007016e6f646500` (sequence 7, bi-directional, system_id "node").
- Bind Response: `0000000f81ff000000070073727600` (sequence 7, failed status, uni-directional, system_id "srv").

These vectors are derived from the source-audited Pinex field layout and independently asserted by Go tests. They are not yet captured from a compiled C++ Pinex process. Socket-level Go↔C++ interoperability remains an outstanding evidence task and is explicitly outside this Phase 2 authorization.

## Verification

CI run for the initial Phase 2 source commit: pending at time of this record. Do not mark Phase 2 complete until gofmt, go vet, go test and go test -race all pass and the source/documentation review is complete.

## Explicit exclusions

No TCP listener/dialer, Reader/Writer loops, Session state machine, Client/Server runtime integration, worker pools, reconnect, benchmark, deployment or release implementation is included.
