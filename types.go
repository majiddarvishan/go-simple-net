package neta

import "net"

// Request is an inbound Application Request delivered to a Server handler.
// Payload is an owned snapshot and may be retained after the handler returns.
type Request struct {
	Sequence uint32
	Payload  []byte
}

// Response is the matching response delivered to a Request Completion handler.
// Payload ownership follows the same owned-snapshot contract as Request.Payload.
type Response struct {
	Sequence uint32
	Status   uint8
	Payload  []byte
}

// BindInfo contains the peer identity and transport address presented to a
// Server BindValidator.
type BindInfo struct {
	SystemID   string
	BindType   uint8
	RemoteAddr net.Addr
}

type CompletionHandler func(Response, error)
type OnBoundHandler func(*Session)
type OnRequestHandler func(*Session, Request)
type OnCloseHandler func(*Session, error)
type BindValidator func(BindInfo) error

// Client is a configured messaging client. Network lifecycle methods are
// implemented in the Client/Server orchestration phase.
type Client struct {
	config    clientConfig
	resources *ownerResourceLedger
}

// Server is a configured messaging server. Network lifecycle methods are
// implemented in the Client/Server orchestration phase.
type Server struct {
	config    serverConfig
	resources *ownerResourceLedger
}
