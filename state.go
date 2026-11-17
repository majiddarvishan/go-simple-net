package neta

// SessionState describes the externally observable lifecycle state of a Session.
type SessionState uint8

const (
	SessionStateUnbound   SessionState = iota // TCP connected but Bind is not complete.
	SessionStateBinding                       // Bind handshake is in progress.
	SessionStateBound                         // Bind succeeded; Application traffic is admitted.
	SessionStateUnbinding                     // Graceful Unbind is in progress.
	SessionStateClosed                        // Internal Session teardown is complete.
)

func (s SessionState) String() string {
	switch s {
	case SessionStateUnbound:
		return "unbound"
	case SessionStateBinding:
		return "binding"
	case SessionStateBound:
		return "bound"
	case SessionStateUnbinding:
		return "unbinding"
	case SessionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

const (
	BindTypeUniDirection uint8 = 0 // Client may send Requests but does not dispatch peer Requests.
	BindTypeBiDirection  uint8 = 1 // Both peers may send and receive Application Requests.
)
