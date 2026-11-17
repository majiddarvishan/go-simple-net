# Pinned Pinex C++ 1.5.0 compatibility source snapshot

This directory contains verbatim copies of the production headers needed to compile
`golden.cpp` against Pinex's actual PDU serializers.

- Upstream repository: `majiddarvishan/pinex`
- Upstream branch at audit time: `develop`
- Pinned commit: `f7bd1eb779baff0d10e624f20522df345e6ee747`
- Upstream project version: `1.5.0`
- Snapshot date: 2026-11-17

The source snapshot is intentionally narrow. It includes the actual upstream
`serialization.hpp`, PDU definitions, and the common headers they include.
The compatibility CI compiles `golden.cpp` against this snapshot. It does not
fetch the upstream repository because that repository is private and GitHub-hosted
CI for neta-go has no cross-repository read credential.

The frame header writer in `golden.cpp` mirrors the production
`session::serialize_header` implementation in upstream
`include/pa/pinex/net/session.hpp` at the pinned commit. Control PDUs are emitted
by Pinex's `session::send_command` using that header writer and have no Body.
The PDU Body bytes are emitted and round-tripped by the verbatim production
PDU serializers copied below.

Upstream Git blob SHA-1 values for each copied file:

| Upstream path | Blob SHA |
|---|---|
| `include/pa/pinex/common.hpp` | `d8078c27c90b39ea9a8f35c10a1fa2c45a3b804b` |
| `include/pa/pinex/common/bind_type.hpp` | `024bbd509a0ab0a2173dcf60b26777f91990645e` |
| `include/pa/pinex/common/command_id.hpp` | `06f69151072de8c204afea53b280a88a70c38596` |
| `include/pa/pinex/common/command_status.hpp` | `32dc0ba1b3aa48cf9fc7fa23ec67ce7e22fce591` |
| `include/pa/pinex/common/serialization.hpp` | `9e492054f78136ce828df6f5c81c17a34d233efb` |
| `include/pa/pinex/common/logging.hpp` | `e04bd16f3bc686d661df1e7836195a596dcf6b1a` |
| `include/pa/pinex/common/thread_affinity_guard.hpp` | `36f7bdf28de63a3b967d47a5fb5029e3d873a13f` |
| `include/pa/pinex/pdu.hpp` | `9a77c27b87bd74e2011f3529b1fe0ae89afbc374` |
| `include/pa/pinex/pdu/bind_request.hpp` | `52723103c03b1f79842f00a6d5640fab6d753d65` |
| `include/pa/pinex/pdu/bind_response.hpp` | `8976335347ebb813560ea803d006832a2fb3085b` |
| `include/pa/pinex/pdu/stream_request.hpp` | `eb17ec2f18148fa931cc92551edc36ba84e051ee` |
| `include/pa/pinex/pdu/stream_response.hpp` | `f97c2710d849568a04b2e882e656aa8ae6a280e0` |

Do not update this snapshot silently. A Pinex source upgrade requires a deliberate
re-pin, source review, regenerated fixtures, and CI verification.
