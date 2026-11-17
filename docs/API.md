# Public API Reference

Import path:

```go
import "github.com/majiddarvishan/neta-go"
```

The public API is intentionally contained in the root package `neta`. Wire codecs, transport loops, worker schedulers and resource ledgers are private implementation details.

## Construction

```go
func NewClient(options ...ClientOption) (*Client, error)
func NewServer(options ...ServerOption) (*Server, error)
```

Constructors apply all supplied Functional Options, validate the effective configuration and retain an immutable snapshot. They do not dial, listen, open sockets or start goroutines.

A Client requires both `WithClientAddress` and `WithClientSystemID`. A Server requires `WithServerListenAddress`.

## Client

```go
func (c *Client) Connect(ctx context.Context) error
func (c *Client) CurrentSession() (*Session, bool)
func (c *Client) Stop(ctx context.Context) error
```

- `Connect` performs TCP Dial followed by the neta-go Bind handshake and returns only after the Client is Bound or the attempt fails. Cancellation of the supplied Context after successful return does not close the Session.
- `CurrentSession` returns only the active Bound generation. It returns `(nil, false)` while disconnected, connecting, unbinding or stopped.
- `Stop` is terminal and idempotent. It cancels Connect/reconnect, gracefully Unbinds the current Bound Session and waits for internal teardown. It does not join arbitrary user callbacks.
- Automatic reconnect is disabled by default. When enabled, the Client uses the approved bounded Full-Jitter policy and never replays or resends an accepted Request.

## Server

```go
func (s *Server) Start() error
func (s *Server) Addr() net.Addr
func (s *Server) Stop(ctx context.Context) error
```

- `Start` opens the configured TCP listener and returns after the Accept loop is ready. A Server is terminal after Stop and cannot be restarted.
- `Addr` returns the actual bound address after Start, including the selected port when listening on port zero.
- `Stop` is terminal and idempotent. It closes the listener, closes Pre-Bind Sessions, gracefully Unbinds Bound Sessions and forces remaining Sessions if the caller Context expires. It waits for internal teardown, not user callbacks.

`WithBindValidator` is optional. If omitted, otherwise valid Binds with unique System IDs are accepted. Duplicate System IDs are rejected; an existing Session is never evicted to admit a duplicate.

## Session

```go
func (s *Session) SendRequest(payload []byte, completion CompletionHandler) (uint32, error)
func (s *Session) SendNoWait(payload []byte) (uint32, error)
func (s *Session) SendResponse(sequence uint32, payload []byte, status uint8) error
func (s *Session) Unbind(ctx context.Context) error
func (s *Session) Close()
func (s *Session) Done() <-chan struct{}
func (s *Session) Err() error
func (s *Session) State() SessionState
func (s *Session) ProtocolVersion() ProtocolVersion
func (s *Session) SystemID() string
func (s *Session) RemoteAddr() net.Addr
```

### Sending

- `SendRequest` snapshots the supplied payload and attempts nonblocking local admission. On success it returns the assigned nonzero Sequence. The Completion callback receives the matching Response or terminal error.
- `SendNoWait` admits a one-way Stream Request without pending Response state or a Completion callback. Its successful return is local admission only.
- `SendResponse` sends a Stream Response using the original inbound Request Sequence. The payload is snapshotted before successful return.
- `SendRequest` requires a non-nil CompletionHandler. A successful admission does not mean the peer received or processed the Request.

### Closing and observation

- `Unbind` performs Graceful Unbind: it closes new Application admission, drains already admitted Application Frames in FIFO order and waits for the matching Unbind Response or a bounded close.
- `Close` initiates immediate Forced Close and returns without waiting for teardown.
- `Done` closes after transport loops, timers and internal Session state have been torn down and the terminal cause is immutable. It is not a user-callback join.
- `Err` returns the terminal cause and is immutable after Done. A clean graceful Unbind may have a nil cause.
- `State`, `ProtocolVersion`, `SystemID` and `RemoteAddr` expose Session state, selected wire version and peer identity/address.

## Public data and callback types

- `BindType`: `BindTypeUniDirectional` or `BindTypeBiDirectional`. A uni-directional Client is requester-only: it sends Stream Requests, ignores incoming peer Requests and rejects `SendResponse` with `ErrProtocol`.
- `Request`: inbound Sequence and owned Payload delivered to a Server OnRequest handler.
- `Response`: matching Sequence, raw Status and owned Payload delivered to a Completion handler.
- `BindInfo`: peer SystemID, raw BindType and RemoteAddr supplied to BindValidator.
- `CompletionHandler`: `func(Response, error)`.
- `OnBoundHandler`: `func(*Session)`.
- `OnRequestHandler`: `func(*Session, Request)`.
- `OnCloseHandler`: `func(*Session, error)`.
- `BindValidator`: `func(BindInfo) error`.

See [Configuration](CONFIGURATION.md) for every Functional Option and its default/range, and [Lifecycle](LIFECYCLE.md) for callback, ownership, timeout and shutdown semantics.
