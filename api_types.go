package neta

import "net"

// Request is an incoming Stream Request. Payload ownership is transferred to the
// application and the application may retain it after the callback returns.
type Request struct {
	Sequence uint32
	Payload  []byte
}

// Response is a matching Stream Response.
type Response struct {
	Sequence uint32
	Status   uint8
	Payload  []byte
}

// CompletionHandler receives the terminal result of one accepted Request.
type CompletionHandler func(Response, error)

// OnBoundHandler is invoked when a Session completes its Bind handshake.
type OnBoundHandler func(*Session)

// OnRequestHandler handles an incoming Application Request on a Server Session.
type OnRequestHandler func(*Session, Request)

// OnCloseHandler receives the terminal cause after a Session's internal teardown.
type OnCloseHandler func(*Session, error)

// BindInfo is the validated peer identity and actual transport address supplied
// to a Server BindValidator.
type BindInfo struct {
	SystemID   string
	BindType   uint8
	RemoteAddr net.Addr
}

// BindValidator accepts or rejects a peer after a valid Bind Request.
type BindValidator func(BindInfo) error

// Client is the owner of one outbound peer connection and its reconnect generations.
// Runtime networking is introduced in the later transport/lifecycle phase.
type Client struct {
	config clientConfig
}

// Server is the owner of an inbound listener and all Sessions accepted by it.
// Runtime networking is introduced in the later transport/lifecycle phase.
type Server struct {
	config serverConfig
}

// Session is a single bound peer generation. Its runtime is introduced in the
// later transport/lifecycle phase.
type Session struct{}
