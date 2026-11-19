package neta

import (
	"context"
	"net"
	"sync"
	"time"
)

// BindType declares which direction a Client may use after Bind.
type BindType uint8

const (
	// BindTypeUniDirectional makes the Client requester-only.
	BindTypeUniDirectional BindType = iota
	// BindTypeBiDirectional permits Requests and Responses in both directions.
	BindTypeBiDirectional
)

// ProtocolVersion identifies the locally configured protocol generation for a
// Session. V1 uses NUL-terminated Bind; V2 omits that terminator.
// The value does not represent a capability negotiation on the wire.
type ProtocolVersion uint8

const (
	// ProtocolVersionV1 selects the legacy-compatible protocol generation.
	ProtocolVersionV1 ProtocolVersion = 1
	// ProtocolVersionV2 is the default and uses a NUL-free Bind body.
	ProtocolVersionV2 ProtocolVersion = 2
)

// Request is an inbound Application Request delivered to a Client or Server handler.
// Payload is an owned snapshot and may be retained after the handler returns.
type Request struct {
	// Sequence is the peer-assigned Stream Request sequence.
	Sequence uint32
	// Payload is an owned snapshot and may be retained after the handler returns.
	Payload []byte
}

// Response is the matching response delivered to a Request Completion handler.
// Payload ownership follows the same owned-snapshot contract as Request.Payload.
type Response struct {
	// Sequence is the sequence of the matching Stream Request.
	Sequence uint32
	// Status is the raw neta-cpp command status returned by the peer.
	Status uint8
	// Payload is an owned snapshot and may be retained after the Completion callback returns.
	Payload []byte
}

// BindInfo contains the peer identity and transport address presented to a
// Server BindValidator.
type BindInfo struct {
	// SystemID is the identity presented in the peer's Bind Request.
	SystemID string
	// BindType is the raw neta-cpp Bind type: 0 for uni-directional and 1 for bi-directional.
	BindType uint8
	// RemoteAddr is the peer's TCP address.
	RemoteAddr net.Addr
}

// CompletionHandler receives the terminal result of one accepted SendRequest.
// Every admitted Request retains exactly one terminal Completion responsibility
// across all forms of Close; callbacks can outlive Session.Done.
type CompletionHandler func(Response, error)

// OnBoundHandler receives a Session after its Bind has succeeded.
type OnBoundHandler func(*Session)

// OnRequestHandler receives an inbound Application Request. Calls for one Session
// are FIFO-started and may overlap when Application callback concurrency exceeds one.
type OnRequestHandler func(*Session, Request)

// OnCloseHandler receives the terminal cause after Session.Done. Delivery is
// backed by a credit reserved before Session admission and starts after Done.
// A blocked OnClose can hold this owner credit indefinitely.
type OnCloseHandler func(*Session, error)

// BindValidator validates the peer identity and transport details before the
// Server accepts a Bind. A nil validator accepts otherwise valid unique identities.
// It runs on a bounded handshake goroutine. A blocked user validator cannot
// be forcibly cancelled by a Session timeout or Server.Stop; its own external
// operations must be bounded by the application's policy.
type BindValidator func(BindInfo) error

// Client owns a configured TCP connection lifecycle and its Session generations.
type Client struct {
	config    clientConfig
	resources *ownerResourceLedger
	callbacks *ownerCallbackDispatcher

	mu               sync.Mutex
	stopped          bool
	stopDone         chan struct{}
	stopErr          error
	current          *Session
	generation       uint64
	boundGeneration  uint64
	boundAt          time.Time
	connectCancel    context.CancelFunc
	connectDone      chan struct{}
	connectGate      chan struct{}
	reconnectCancel  context.CancelFunc
	reconnectDone    chan struct{}
	reconnectWake    chan clientReconnectEvent
	reconnectStarted bool
}

// Server owns a TCP listener, accepted Sessions and their shared resource budgets.
type Server struct {
	config    serverConfig
	resources *ownerResourceLedger
	callbacks *ownerCallbackDispatcher

	startMu sync.Mutex
	mu      sync.Mutex

	listener   net.Listener
	addr       net.Addr
	started    bool
	stopping   bool
	acceptDone chan struct{}
	acceptErr  error
	stopDone   chan struct{}
	stopErr    error

	// A permit remains held until a user BindValidator actually returns,
	// even if the corresponding Session times out and frees its socket.
	validatorSlots chan struct{}

	sessions       map[*Session]struct{}
	preBind        map[*Session]struct{}
	pendingSockets uint32
	pendingIDs     map[string]*Session
	boundIDs       map[string]*Session
	sessionIDs     map[*Session]string
}
