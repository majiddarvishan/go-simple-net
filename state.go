package neta

// SessionState is the externally observable lifecycle state of a Session.
type SessionState uint8

const (
	SessionStateNew SessionState = iota
	SessionStateBinding
	SessionStateBound
	SessionStateUnbinding
	SessionStateClosing
	SessionStateClosed
)

func (s SessionState) String() string {
	switch s {
	case SessionStateNew:
		return "new"
	case SessionStateBinding:
		return "binding"
	case SessionStateBound:
		return "bound"
	case SessionStateUnbinding:
		return "unbinding"
	case SessionStateClosing:
		return "closing"
	case SessionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}
