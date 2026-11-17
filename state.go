package neta

// SessionState is the externally observable lifecycle state of a Session.
type SessionState uint8

const (
	// SessionStateNew means the Session has not begun its Bind handshake.
	SessionStateNew SessionState = iota
	// SessionStateBinding means the Bind handshake is in progress.
	SessionStateBinding
	// SessionStateBound means the Session is ready for Application traffic.
	SessionStateBound
	// SessionStateUnbinding means graceful Unbind has started and new Application
	// admission is closed.
	SessionStateUnbinding
	// SessionStateClosing means terminal teardown is in progress.
	SessionStateClosing
	// SessionStateClosed means terminal teardown has completed.
	SessionStateClosed
)

// String returns the stable lower-case name of the Session state.
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
