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

// WithProtocolVersion selects the immutable, local version for each Session.
// V2 is the default and sends NUL-free Bind bodies. V1 retains the
// NUL-terminated Bind representation. No peer negotiation is performed.
func WithProtocolVersion(version ProtocolVersion) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.protocolVersion = version })
}

// WithClientAddress sets the remote TCP host:port endpoint.
func WithClientAddress(address string) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.address = address }}
}

// WithClientSystemID sets the required Client Bind identity.
// V1 allows at most 19 bytes plus a trailing NUL for neta-cpp compatibility.
// V2 allows 20 bytes with no trailing NUL. Embedded NUL is always invalid.
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

// WithClientOnBoundHandler sets the callback invoked when a Client Session generation becomes Bound.
func WithClientOnBoundHandler(handler OnBoundHandler) ClientOption {
	return ClientOption{apply: func(c *clientConfig) { c.onBound = handler }}
}

// WithClientOnCloseHandler sets the best-effort callback invoked after a Client Session is Done. The event may be skipped if the Client's OnClose event capacity is exhausted.
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

// WithClientApplicationWorkers sets the Client-wide active Application callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..512.
func WithClientApplicationWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.applicationWorkers = limit })
}

// WithClientCompletionWorkers sets the Client-wide active Completion callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..256.
func WithClientCompletionWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.completionWorkers = limit })
}

// WithClientOnCloseWorkers sets the Client-wide active OnClose callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..128.
func WithClientOnCloseWorkers(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseWorkers = limit })
}

// WithClientApplicationWorkerIdleGrace sets the idle retirement grace for Client Application workers. The default is 30s; the valid range is 1ms..10m.
func WithClientApplicationWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.applicationWorkerIdleGrace = grace })
}

// WithClientCompletionWorkerIdleGrace sets the idle retirement grace for Client Completion workers. The default is 30s; the valid range is 1ms..10m.
func WithClientCompletionWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.completionWorkerIdleGrace = grace })
}

// WithClientOnCloseWorkerIdleGrace sets the idle retirement grace for Client OnClose workers. The default is 30s; the valid range is 1ms..10m.
func WithClientOnCloseWorkerIdleGrace(grace time.Duration) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseWorkerIdleGrace = grace })
}

// WithClientOnCloseEventCredits sets the Client-wide budget for queued and running OnClose events. The default is 1024; the valid range is 1..4096.
func WithClientOnCloseEventCredits(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.onCloseEventCredits = limit })
}

// WithClientMaxOutstandingRequests sets the Client-wide outstanding Request credit ceiling shared by all Session generations. The default is 32768; the valid range is 1..131072.
func WithClientMaxOutstandingRequests(limit uint32) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxOutstandingRequests = limit })
}

// WithClientMaxRetainedApplicationBytes sets the Client-wide retained Application payload byte budget. The default is 512 MiB; the valid range is 10 bytes..2 GiB.
func WithClientMaxRetainedApplicationBytes(limit uint64) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxRetainedApplicationBytes = limit })
}

// WithClientMaxRetainedCompletionBytes sets the Client-wide retained Completion payload byte budget. The default is 256 MiB; the valid range is 10 bytes..1 GiB.
func WithClientMaxRetainedCompletionBytes(limit uint64) ClientOption {
	return clientOwnerOption(func(c *ownerConfig) { c.maxRetainedCompletionBytes = limit })
}

// WithServerListenAddress sets the TCP listen host:port endpoint.
func WithServerListenAddress(address string) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.listenAddress = address }}
}

// WithBindValidator sets the optional Server Bind admission callback. A nil validator accepts otherwise valid Binds with unique System IDs.
func WithBindValidator(validator BindValidator) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.bindValidator = validator }}
}

// WithServerOnBoundHandler sets the callback invoked when an accepted Server Session becomes Bound.
func WithServerOnBoundHandler(handler OnBoundHandler) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.onBound = handler }}
}

// WithServerOnRequestHandler sets the inbound Application Request callback. Per-Session concurrency is configured with WithApplicationCallbackConcurrency.
func WithServerOnRequestHandler(handler OnRequestHandler) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.onRequest = handler }}
}

// WithServerOnCloseHandler sets the best-effort callback invoked after a Server Session is Done. The event may be skipped if the Server's OnClose event capacity is exhausted.
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

// WithServerPreBindHandshakeTimeout sets the absolute accept-to-Bind deadline. The default is 30s; the duration must be positive.
func WithServerPreBindHandshakeTimeout(timeout time.Duration) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.preBindHandshakeTimeout = timeout }}
}

// WithServerMaxBoundSessions sets the maximum number of simultaneously Bound Sessions. The default is 1024; the value must be positive.
func WithServerMaxBoundSessions(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxBoundSessions = limit }}
}

// WithServerMaxPendingPreBindSessions sets the maximum number of accepted Sessions awaiting Bind. The default is 128; the value must be positive.
func WithServerMaxPendingPreBindSessions(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxPendingPreBindSessions = limit }}
}

// WithServerMaxAggregateQueuedApplicationBytes sets the Server-wide queued Application frame byte budget. The default is 256 MiB; the value must be positive.
func WithServerMaxAggregateQueuedApplicationBytes(limit uint64) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxAggregateQueuedApplicationBytes = limit }}
}

// WithServerMaxAggregateQueuedApplicationFrames sets the Server-wide queued Application frame-count budget. The default is 65536; the valid range is 1..262144.
func WithServerMaxAggregateQueuedApplicationFrames(limit uint32) ServerOption {
	return ServerOption{apply: func(c *serverConfig) { c.maxAggregateQueuedApplicationFrames = limit }}
}

// WithServerApplicationWorkers sets the Server-wide active Application callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..512.
func WithServerApplicationWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.applicationWorkers = limit })
}

// WithServerCompletionWorkers sets the Server-wide active Completion callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..256.
func WithServerCompletionWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.completionWorkers = limit })
}

// WithServerOnCloseWorkers sets the Server-wide active OnClose callback worker ceiling. The default is derived from GOMAXPROCS; the valid range is 1..128.
func WithServerOnCloseWorkers(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseWorkers = limit })
}

// WithServerApplicationWorkerIdleGrace sets the idle retirement grace for Server Application workers. The default is 30s; the valid range is 1ms..10m.
func WithServerApplicationWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.applicationWorkerIdleGrace = grace })
}

// WithServerCompletionWorkerIdleGrace sets the idle retirement grace for Server Completion workers. The default is 30s; the valid range is 1ms..10m.
func WithServerCompletionWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.completionWorkerIdleGrace = grace })
}

// WithServerOnCloseWorkerIdleGrace sets the idle retirement grace for Server OnClose workers. The default is 30s; the valid range is 1ms..10m.
func WithServerOnCloseWorkerIdleGrace(grace time.Duration) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseWorkerIdleGrace = grace })
}

// WithServerOnCloseEventCredits sets the Server-wide budget for queued and running OnClose events. The default is 1024; the valid range is 1..4096.
func WithServerOnCloseEventCredits(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.onCloseEventCredits = limit })
}

// WithServerMaxOutstandingRequests sets the Server-wide outstanding Request credit ceiling shared by all Sessions. The default is 32768; the valid range is 1..131072.
func WithServerMaxOutstandingRequests(limit uint32) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxOutstandingRequests = limit })
}

// WithServerMaxRetainedApplicationBytes sets the Server-wide retained Application payload byte budget. The default is 512 MiB; the valid range is 10 bytes..2 GiB.
func WithServerMaxRetainedApplicationBytes(limit uint64) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxRetainedApplicationBytes = limit })
}

// WithServerMaxRetainedCompletionBytes sets the Server-wide retained Completion payload byte budget. The default is 256 MiB; the valid range is 10 bytes..1 GiB.
func WithServerMaxRetainedCompletionBytes(limit uint64) ServerOption {
	return serverOwnerOption(func(c *ownerConfig) { c.maxRetainedCompletionBytes = limit })
}

// WithRequestTimeout sets the absolute timeout for an admitted Request. The default is 30s; the duration must be positive.
func WithRequestTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.requestTimeout = timeout })
}

// WithFrameAssemblyTimeout sets the absolute inbound Frame assembly deadline. The default is 30s; the duration must be positive.
func WithFrameAssemblyTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.frameAssemblyTimeout = timeout })
}

// WithUnbindTimeout sets the absolute Graceful Unbind deadline. The default is 10s; the duration must be positive.
func WithUnbindTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.unbindTimeout = timeout })
}

// WithWriteTimeout sets the absolute deadline for each Frame after the Writer owns it. The default is 30s; the duration must be positive.
func WithWriteTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.writeTimeout = timeout })
}

// WithEnquireLinkIdleInterval sets the idle period before an automatic Enquire Link probe. The default is 30s; zero disables outgoing probes and negative values are invalid.
func WithEnquireLinkIdleInterval(interval time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.enquireLinkIdleInterval = interval })
}

// WithEnquireLinkResponseTimeout sets the deadline for a matching Enquire Link Response. The default is 10s; the duration must be positive.
func WithEnquireLinkResponseTimeout(timeout time.Duration) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.enquireLinkResponseTimeout = timeout })
}

// WithMaxQueuedOutgoingBytes sets the per-Session outgoing Application FIFO byte budget. The default is 4 MiB; the value must be positive.
func WithMaxQueuedOutgoingBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedOutgoingBytes = limit })
}

// WithMaxQueuedIncomingBytes sets the per-Session incoming Application backlog byte budget. The default is 4 MiB; the value must be positive.
func WithMaxQueuedIncomingBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedIncomingBytes = limit })
}

// WithMaxQueuedControlBytes sets the per-Session reserved Control-PDU byte budget. The default is 4 KiB; the value must be positive.
func WithMaxQueuedControlBytes(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxQueuedControlBytes = limit })
}

// WithApplicationCallbackConcurrency sets the maximum concurrent Application callbacks for one Session. The default is 1; the valid range is 1..32.
func WithApplicationCallbackConcurrency(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.applicationConcurrency = limit })
}

// WithCompletionCallbackConcurrency sets the maximum concurrent Completion callbacks for one Session. The default is 1; the valid range is 1..32.
func WithCompletionCallbackConcurrency(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.completionConcurrency = limit })
}

// WithMaxIncomingApplicationFrames sets the per-Session incoming Application frame-count limit. The default is 256; the valid range is 1..1024.
func WithMaxIncomingApplicationFrames(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxIncomingApplicationFrames = limit })
}

// WithMaxOutgoingApplicationFrames sets the per-Session outgoing Application frame-count limit. The default is 256; the valid range is 1..1024.
func WithMaxOutgoingApplicationFrames(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxOutgoingApplicationFrames = limit })
}

// WithMaxOutstandingRequestsPerSession sets the per-Session outstanding Request credit limit. The default is 128; the valid range is 1..512.
func WithMaxOutstandingRequestsPerSession(limit uint32) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxOutstandingRequests = limit })
}

// WithMaxRetainedApplicationBytesPerSession sets the per-Session retained Application payload byte budget. The default is 16 MiB; the valid range is 10 bytes..64 MiB.
func WithMaxRetainedApplicationBytesPerSession(limit uint64) SessionOption {
	return sessionOption(func(c *sessionConfig) { c.maxRetainedApplicationBytes = limit })
}

// WithMaxRetainedCompletionBytesPerSession sets the per-Session retained Completion payload byte budget. The default is 16 MiB; the valid range is 10 bytes..64 MiB.
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
