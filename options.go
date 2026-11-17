package neta

import "time"

// ClientOption configures one Client at construction time.
type ClientOption struct{ apply func(*clientConfig) }

// ServerOption configures one Server at construction time.
type ServerOption struct{ apply func(*serverConfig) }

// SessionOption configures immutable defaults inherited by Sessions owned by
// a Client or Server.
type SessionOption struct{ apply func(*sessionConfig) }

func clientOwnerOption(apply func(*ownerConfig)) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { apply(&c.owner) }}
}

func serverOwnerOption(apply func(*ownerConfig)) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { apply(&c.owner) }}
}

func sessionOption(apply func(*sessionConfig)) SessionOption {
	return SessionOption{apply: apply}
}

// WithClientAddress sets the remote TCP host:port endpoint.
func WithClientAddress(address string) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.address = address }}
}

// WithClientSystemID sets the required Pinex-compatible Bind identity.
// System IDs are encoded as a NUL-terminated field of at most 19 bytes.
func WithClientSystemID(systemID string) ClientOption {
	return ClientOption{apply: func(c *clientConfig) {
		c.systemID = systemID
		c.systemIDSet = true
	}}
}

// WithClientBindType selects the Client's Bind direction. The default is
// BindTypeBiDirectional.
func WithClientBindType(bindType BindType) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.bindType = bindType }}
}

// WithConnectTimeout sets the absolute per-Dial-attempt timeout.
func WithConnectTimeout(timeout time.Duration) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.connectTimeout = timeout }}
}

// WithBindResponseTimeout sets the absolute Client Bind handshake timeout.
func WithBindResponseTimeout(timeout time.Duration) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.bindResponseTimeout = timeout }}
}

// WithAutoReconnect enables or disables the Client reconnect supervisor.
func WithAutoReconnect(enabled bool) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.autoReconnect = enabled }}
}

func WithClientOnBoundHandler(handler OnBoundHandler) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.onBound = handler }}
}

func WithClientOnCloseHandler(handler OnCloseHandler) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.onClose = handler }}
}

// WithClientSessionOptions applies Session defaults to every Session generation
// created by this Client. Options are copied and applied before final validation.
func WithClientSessionOptions(options ...SessionOption) ClientOption {
	copied := append([]SessionOption(nil), options...)
	return ClientOption{apply: func(c *clientConfig) {
		for i, option := range copied {
			if option.apply == nil {
				if c.invalidOption == "" {
					c.invalidOption = "session_options[" + itoa(i) + "]"
				}
				continue
			}
			option.apply(&c.session)
		}
	}}
}

func WithClientApplicationWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.applicationWorkers = limit })
}
func WithClientCompletionWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.completionWorkers = limit })
}
func WithClientOnCloseWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseWorkers = limit })
}
func WithClientApplicationWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.applicationWorkerIdleGrace = grace })
}
func WithClientCompletionWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.completionWorkerIdleGrace = grace })
}
func WithClientOnCloseWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseWorkerIdleGrace = grace })
}
func WithClientOnCloseEventCredits(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseEventCredits = limit })
}
func WithClientMaxOutstandingRequests(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxOutstandingRequests = limit })
}
func WithClientMaxRetainedApplicationBytes(limit uint64) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxRetainedApplicationBytes = limit })
}
func WithClientMaxRetainedCompletionBytes(limit uint64) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxRetainedCompletionBytes = limit })
}

// WithServerListenAddress sets the TCP listen host:port endpoint.
func WithServerListenAddress(address string) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.listenAddress = address }}
}

func WithBindValidator(validator BindValidator) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.bindValidator = validator }}
}

func WithServerOnBoundHandler(handler OnBoundHandler) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.onBound = handler }}
}

func WithServerOnRequestHandler(handler OnRequestHandler) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.onRequest = handler }}
}

func WithServerOnCloseHandler(handler OnCloseHandler) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.onClose = handler }}
}

// WithServerSessionOptions applies Session defaults inherited by all Sessions
// accepted by this Server. Options are copied and applied before final validation.
func WithServerSessionOptions(options ...SessionOption) ServerOption {
	copied := append([]SessionOption(nil), options...)
	return ServerOption{apply: func(c *serverConfig) {
		for i, option := range copied {
			if option.apply == nil {
				if c.invalidOption == "" {
					c.invalidOption = "session_options[" + itoa(i) + "]"
				}
				continue
			}
			option.apply(&c.session)
		}
	}}
}

func WithServerPreBindHandshakeTimeout(timeout time.Duration) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.preBindHandshakeTimeout = timeout }}
}
func WithServerMaxBoundSessions(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxBoundSessions = limit }}
}
func WithServerMaxPendingPreBindSessions(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxPendingPreBindSessions = limit }}
}
func WithServerMaxAggregateQueuedApplicationBytes(limit uint64) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxAggregateQueuedApplicationBytes = limit }}
}
func WithServerMaxAggregateQueuedApplicationFrames(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxAggregateQueuedApplicationFrames = limit }}
}
func WithServerApplicationWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.applicationWorkers = limit })
}
func WithServerCompletionWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.completionWorkers = limit })
}
func WithServerOnCloseWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseWorkers = limit })
}
func WithServerApplicationWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.applicationWorkerIdleGrace = grace })
}
func WithServerCompletionWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.completionWorkerIdleGrace = grace })
}
func WithServerOnCloseWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseWorkerIdleGrace = grace })
}
func WithServerOnCloseEventCredits(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseEventCredits = limit })
}
func WithServerMaxOutstandingRequests(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxOutstandingRequests = limit })
}
func WithServerMaxRetainedApplicationBytes(limit uint64) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxRetainedApplicationBytes = limit })
}
func WithServerMaxRetainedCompletionBytes(limit uint64) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxRetainedCompletionBytes = limit })
}

func WithRequestTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.requestTimeout = timeout })
}
func WithFrameAssemblyTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.frameAssemblyTimeout = timeout })
}
func WithUnbindTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.unbindTimeout = timeout })
}
func WithWriteTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.writeTimeout = timeout })
}
func WithEnquireLinkIdleInterval(interval time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.enquireLinkIdleInterval = interval })
}
func WithEnquireLinkResponseTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.enquireLinkResponseTimeout = timeout })
}
func WithMaxQueuedOutgoingBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedOutgoingBytes = limit })
}
func WithMaxQueuedIncomingBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedIncomingBytes = limit })
}
func WithMaxQueuedControlBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedControlBytes = limit })
}
func WithApplicationCallbackConcurrency(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.applicationConcurrency = limit })
}
func WithCompletionCallbackConcurrency(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.completionConcurrency = limit })
}
func WithMaxIncomingApplicationFrames(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxIncomingApplicationFrames = limit })
}
func WithMaxOutgoingApplicationFrames(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxOutgoingApplicationFrames = limit })
}
func WithMaxOutstandingRequestsPerSession(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxOutstandingRequests = limit })
}
func WithMaxRetainedApplicationBytesPerSession(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxRetainedApplicationBytes = limit })
}
func WithMaxRetainedCompletionBytesPerSession(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxRetainedCompletionBytes = limit })
}

// itoa formats a small option index without importing fmt into the options path.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
