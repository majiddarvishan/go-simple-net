# neta-go TCP messaging protocol

neta-go uses a length-delimited TCP Frame with a fixed **10-byte Header** and an optional Body. All multi-byte Header integers use big-endian encoding. The maximum total Frame size is **1,048,576 bytes**, including the Header; the maximum Body size is 1,048,566 bytes.

## Frame layout

| Offset | Bytes | Field | Meaning |
|---:|---:|---|---|
| 0 | 4 | Length | Total Header + Body bytes (unsigned big-endian) |
| 4 | 1 | Command | Request/Response command identifier |
| 5 | 1 | Status | Raw status byte; `0x00` success, `0xFF` failure |
| 6 | 4 | Sequence | Unsigned big-endian nonzero request identifier |
| 10 | Variable | Body | Command-specific bytes |

| Command | Request | Response |
|---|---|---|
| Bind | `0x01` | `0x81` |
| Stream | `0x02` | `0x82` |
| Unbind | `0x03` | `0x83` |
| Enquire Link | `0x04` | `0x84` |

Responses reuse their Request Sequence. Sequences are generated per Session, increment and skip zero. Stream Bodies contain raw application payload bytes without a built-in Protobuf envelope or extra Body-length prefix. Unbind and Enquire Link have empty Bodies.

## Bind and protocol versions

A Bind body begins with a one-byte BindType (`0` = unidirectional, `1` = bidirectional). The remaining Body is the SystemID:

| Version | Bind Request / Response Body | SystemID maximum |
|---|---|---|
| V1 | `BindType:u8 + SystemID + NUL:0x00` | 19 bytes |
| V2 | `BindType:u8 + SystemID` | 20 bytes |

The size limits count encoded **bytes**, not Unicode characters. Embedded NUL is invalid. V2 omits the trailing NUL from Bind Request and Bind Response; V1 requires it. A Bind Response may omit its entire Body, in which case the decoder uses the default BindType and empty SystemID.

The Go library defaults to `ProtocolVersionV2`. `WithProtocolVersion` selects V1 or V2 through Client/Server Session options; `Session.ProtocolVersion()` reports the effective version. Selection is fixed for the Session and does not change per Frame.

A default V2 Server recognizes the first incoming Bind shape and replies using that Session's selected shape. A V1-only Server rejects a V2 Bind. A Client must explicitly choose V1 for a V1-only peer: there is **no** implicit retry, fallback, capability-negotiation Frame, Header extension or per-Frame version byte.

The 10-byte Header, Stream, Unbind and Enquire Link encoding are identical in both versions.

## Messaging behavior

- A successful `SendRequest` or `SendNoWait` means local queue admission, **not** remote delivery or processing.
- `SendRequest` tracks a matching Stream Response by Sequence and settles through a Completion callback, including timeout/terminal error paths.
- `SendNoWait` sends a Stream Request without holding an outstanding Response tracker.
- The library can send an idle-only Enquire Link Request after a configured interval; `0` disables outgoing probes. A matching response is required before a separate response deadline.
- Graceful Unbind drains admitted Application Frames and waits for the matching Unbind Response within its configured deadline.
- Incoming Frames are checked against the fixed size bound before allocating the declared Body; separate queue and retained-byte limits may reject them earlier.

## Security boundary

A Server may configure `WithBindValidator` to check the peer SystemID and remote address. The Bind handshake alone is **not cryptographic authentication**. Applications must establish their own trust and network-security requirements.

For the Go API and defaults, see [API](API.md), [Configuration](CONFIGURATION.md) and [Lifecycle](LIFECYCLE.md).
