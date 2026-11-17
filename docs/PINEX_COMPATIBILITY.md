# Pinex Compatibility

## Supported compatibility target

| Component | Verified target |
|---|---|
| Pinex repository | `majiddarvishan/pinex` |
| Branch | `develop` |
| Version | **1.5.0** |
| Exact source HEAD | `f7bd1eb779baff0d10e624f20522df345e6ee747` |
| neta-go production source used for Phase 7 live interoperability | `209c24a7dbc5a5aa3db95ec63705915406acacb6` |

Only Pinex C++ 1.5.0 is currently verified. No compatibility claim is made for other Pinex versions.

## Wire contract

The fixed Frame Header is 10 bytes, with unsigned big-endian fields:

| Offset | Size | Field |
|---:|---:|---|
| 0 | 4 | Total Frame length, including Header |
| 4 | 1 | Command ID |
| 5 | 1 | Command status |
| 6 | 4 | Sequence |

The maximum total Frame is **1,048,576 bytes** including the Header; maximum Body is **1,048,566 bytes**. The limit applies inbound and outbound and is not configurable.

| PDU | Command ID |
|---|---:|
| Bind Request | 0x01 |
| Stream Request | 0x02 |
| Unbind Request | 0x03 |
| Enquire Link Request | 0x04 |
| Bind Response | 0x81 |
| Stream Response | 0x82 |
| Unbind Response | 0x83 |
| Enquire Link Response | 0x84 |

Pinex status values include `rok=0x00` and `rfail=0xFF`. The Go Header codec preserves other raw status bytes.

Bind Body is a BindType byte followed by a NUL-terminated SystemID. BindType is 0 for uni-directional and 1 for bi-directional. The Pinex `c_octet_str<20>` field permits at most 19 SystemID bytes before the terminator. Stream Body is raw application bytes; no mandatory Protobuf envelope is added. Unbind and Enquire Link are header-only. Responses reuse the Request Sequence.

An empty Bind Response Body is permitted by Pinex's `can_be_omitted` behavior; neta-go applies the C++ default values (bi-directional and empty SystemID).

## Golden-frame compatibility

The repository contains the pinned C++ production serializer snapshot and fixture generator under `testdata/pinex-1.5.0/`. The source snapshot records the upstream commit and original Git blob hashes.

The Pinex Wire Compatibility workflow:
1. Compiles the C++ fixture generator against the pinned production serializer headers.
2. Regenerates the golden vectors and diffs them against the checked-in fixture file.
3. Runs Go codec tests against those vectors.

Latest compatibility workflow after the Phase 9 documentation updates: [44116675706](https://github.com/majiddarvishan/neta-go/actions/runs/44116675706) PASS. This is a golden-frame check, not a live socket test.

## Live TCP interoperability

The dedicated cross-repository workflow [44081220680](https://github.com/majiddarvishan/pinex/actions/runs/44081220680) passed real loopback TCP in both directions using Pinex's public high-level APIs:

1. Go Client → actual Pinex C++ Server.
2. Actual Pinex C++ Client → Go Server.

The scenarios validated Bind identity, Request/Response payloads, Sequence correlation and graceful Unbind/Stop.

## Fault, fuzz and soak evidence

The Phase 7 report records:

- Five wire fuzz targets with **1,810,074 aggregate executions**, no crash or failing input.
- Fragmented reads/writes, slow-trickle absolute Bind timeout, silent accepted-socket Bind-response blackhole, graceful EOF, abrupt RST, reordered Responses, Bind rejection and timeout/cancellation races.
- Admission and callback fault matrices under the Go race detector.
- A 30-second reconnect/resource soak with three Clients; owner Request/retained-byte/event credits returned to zero after Stop.

**Explicit limitation:** privileged firewall-level SYN-drop/network-namespace injection was not run. The tested blackhole was an accepted TCP socket that withheld the Bind Response. No host firewall or production network was modified.

## Compatibility policy

- Keep the Pinex 1.5.0 source SHA and golden provenance explicit.
- Re-pin and re-run golden plus real TCP interoperability before claiming support for another Pinex version.
- Preserve the 10-byte Header, command/status/Sequence semantics, raw Stream Body and fixed 1 MiB total Frame maximum.
- Go intentionally rejects some malformed Bind/control encodings more strictly than legacy C++; valid-frame compatibility is the supported contract.
- Uni-directional Go Client behavior is requester-only, matching Pinex 1.5.0 high-level Client behavior.
