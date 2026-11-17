package neta

import "time"

// ClientOption configures a Client before it is constructed. Its target is
// intentionally unexported so callers can only use the library's typed options.
type ClientOption func(*clientConfig)

// ServerOption configures a Server before it is constructed.
type ServerOption func(*serverConfig)

// SessionOption configures immutable defaults inherited by Sessions of one owner.
type SessionOption func(*sessionConfig)

// WithClientAddress sets the remote TCP address for the Client.
func WithClientAddress(address string) ClientOption {
	return func(c *clientConfig) { c.address = address }
}

// WithClientSystemID sets the System ID sent in the Client Bind Request. The empty ID is wire-valid.
func WithClientSystemID(systemID string) ClientOption {
	return func(c *clientConfig) { c.systemID = systemID }
}

// WithClientBindType sets the Pinex Bind Type sent by the Client.
func WithClientBindType(bindType uint8) ClientOption {
	return func(c *clientConfig) { c.bindType = bindType }
}

// WithConnectTimeout sets the absolute timeout for each Client Dial attempt.
func WithConnectTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) { c.connectTimeout = timeout }
}

// WithBindResponseTimeout sets the absolute deadline for the Client Bind write and matching response.
func WithBindResponseTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) { c.bindResponseTimeout = timeout }
}

// WithAutoReconnect enables or disables the Client reconnect supervisor. It is disabled by default.
func WithAutoReconnect(enabled bool) ClientOption {
	return func(c *clientConfig) { c.autoReconnect = enabled }
}

// WithClientSessionOptions applies immutable Session defaults to every Session generation created by this Client.
func WithClientSessionOptions(options ...SessionOption) ClientOption {
	copied := append([]SessionOption(nil), options...)
	return func(c *clientConfig) {
		for i, option := range copied {
			if option == nil {
				c.optionErrors = append(c.optionErrors, optionIndex("Client Session", i))
				continue
			}
			option(&c.session)
		}
	}
}

// WithClientOnBound registers the Client OnBound callback at construction time.
func WithClientOnBound(handler OnBoundHandler) ClientOption {
	return func(c *clientConfig) { c.onBound = handler }
}

// WithClientOnClose registers the Client OnClose callback at construction time.
func WithClientOnClose(handler OnCloseHandler) ClientOption {
	return func(c *clientConfig) { c.onClose = handler }
}

// WithClientApplicationWorkers sets the Client-owned Application worker ceiling.
func WithClientApplicationWorkers(limit int) ClientOption {
	return func(c *clientConfig) { c.workers.applicationWorkers = limit }
}

// WithClientCompletionWorkers sets the Client-owned Completion worker ceiling.
func WithClientCompletionWorkers(limit int) ClientOption {
	return func(c *clientConfig) { c.workers.completionWorkers = limit }
}

// WithClientOnCloseWorkers sets the Client-owned OnClose worker ceiling.
func WithClientOnCloseWorkers(limit int) ClientOption {
	return func(c *clientConfig) { c.workers.onCloseWorkers = limit }
}

// WithClientApplicationWorkerIdleGrace sets the idle retirement grace for this Client Application worker class.
func WithClientApplicationWorkerIdleGrace(grace time.Duration) ClientOption {
	return func(c *clientConfig) { c.workers.applicationIdleGrace = grace }
}

// WithClientCompletionWorkerIdleGrace sets the idle retirement grace for this Client Completion worker class.
func WithClientCompletionWorkerIdleGrace(grace time.Duration) ClientOption {
	return func(c *clientConfig) { c.workers.completionIdleGrace = grace }
}

// WithClientOnCloseWorkerIdleGrace sets the idle retirement grace for this Client OnClose worker class.
func WithClientOnCloseWorkerIdleGrace(grace time.Duration) ClientOption {
	return func(c *clientConfig) { c.workers.onCloseIdleGrace = grace }
}

// WithClientOnCloseEventCredits sets the Client-owned queued-plus-running OnClose event credit ceiling.
func WithClientOnCloseEventCredits(limit int) ClientOption {
	return func(c *clientConfig) { c.workers.onCloseEventCredits = limit }
}

// WithClientOutstandingRequestCredits sets the Client-owned aggregate outstanding Request credit ceiling.
func WithClientOutstandingRequestCredits(limit int) ClientOption {
	return func(c *clientConfig) { c.workers.aggregateOutstandingRequests = limit }
}

// WithClientRetainedApplicationBytes sets the Client-owned retained Application payload byte ceiling.
func WithClientRetainedApplicationBytes(limit uint64) ClientOption {
	return func(c *clientConfig) { c.workers.aggregateRetainedApplication = limit }
}

// WithClientRetainedCompletionBytes sets the Client-owned retained Completion payload byte ceiling.
func WithClientRetainedCompletionBytes(limit uint64) ClientOption {
	return func(c *clientConfig) { c.workers.aggregateRetainedCompletion = limit }
}

// WithServerListenAddress sets the TCP listen address used when the Server starts.
func WithServerListenAddress(address string) ServerOption {
	return func(c *serverConfig) { c.listenAddress = address }
}

// WithServerSystemID sets the System ID returned in successful Bind Responses.
func WithServerSystemID(systemID string) ServerOption {
	return func(c *serverConfig) { c.systemID = systemID }
}

// WithServerBindValidator registers the optional Server Bind Validator. Without one, valid Binds are accepted by default.
func WithServerBindValidator(validator BindValidator) ServerOption {
	return func(c *serverConfig) { c.bindValidator = validator }
}

// WithServerOnBound registers the Server OnBound callback at construction time.
func WithServerOnBound(handler OnBoundHandler) ServerOption {
	return func(c *serverConfig) { c.onBound = handler }
}

// WithServerOnRequest registers the Server Application Request callback at construction time.
func WithServerOnRequest(handler OnRequestHandler) ServerOption {
	return func(c *serverConfig) { c.onRequest = handler }
}

// WithServerOnClose registers the Server OnClose callback at construction time.
func WithServerOnClose(handler OnCloseHandler) ServerOption {
	return func(c *serverConfig) { c.onClose = handler }
}

// WithServerSessionOptions applies immutable Session defaults inherited by Sessions accepted by this Server.
func WithServerSessionOptions(options ...SessionOption) ServerOption {
	copied := append([]SessionOption(nil), options...)
	return func(c *serverConfig) {
		for i, option := range copied {
			if option == nil {
				c.optionErrors = append(c.optionErrors, optionIndex("Server Session", i))
				continue
			}
			option(&c.session)
		}
	}
}

// WithServerApplicationWorkers sets the Server-owned Application worker ceiling.
func WithServerApplicationWorkers(limit int) ServerOption {
	return func(c *serverConfig) { c.workers.applicationWorkers = limit }
}

// WithServerCompletionWorkers sets the Server-owned Completion worker ceiling.
func WithServerCompletionWorkers(limit int) ServerOption {
	return func(c *serverConfig) { c.workers.completionWorkers = limit }
}

// WithServerOnCloseWorkers sets the Server-owned OnClose worker ceiling.
func WithServerOnCloseWorkers(limit int) ServerOption {
	return func(c *serverConfig) { c.workers.onCloseWorkers = limit }
}

// WithServerApplicationWorkerIdleGrace sets the idle retirement grace for this Server Application worker class.
func WithServerApplicationWorkerIdleGrace(grace time.Duration) ServerOption {
	return func(c *serverConfig) { c.workers.applicationIdleGrace = grace }
}

// WithServerCompletionWorkerIdleGrace sets the idle retirement grace for this Server Completion worker class.
func WithServerCompletionWorkerIdleGrace(grace time.Duration) ServerOption {
	return func(c *serverConfig) { c.workers.completionIdleGrace = grace }
}

// WithServerOnCloseWorkerIdleGrace sets the idle retirement grace for this Server OnClose worker class.
func WithServerOnCloseWorkerIdleGrace(grace time.Duration) ServerOption {
	return func(c *serverConfig) { c.workers.onCloseIdleGrace = grace }
}

// WithServerOnCloseEventCredits sets the Server-owned queued-plus-running OnClose event credit ceiling.
func WithServerOnCloseEventCredits(limit int) ServerOption {
	return func(c *serverConfig) { c.workers.onCloseEventCredits = limit }
}

// WithServerOutstandingRequestCredits sets the Server-owned aggregate outstanding Request credit ceiling.
func WithServerOutstandingRequestCredits(limit int) ServerOption {
	return func(c *serverConfig) { c.workers.aggregateOutstandingRequests = limit }
}

// WithServerRetainedApplicationBytes sets the Server-owned retained Application payload byte ceiling.
func WithServerRetainedApplicationBytes(limit uint64) ServerOption {
	return func(c *serverConfig) { c.workers.aggregateRetainedApplication = limit }
}

// WithServerRetainedCompletionBytes sets the Server-owned retained Completion payload byte ceiling.
func WithServerRetainedCompletionBytes(limit uint64) ServerOption {
	return func(c *serverConfig) { c.workers.aggregateRetainedCompletion = limit }
}

// WithServerMaxBoundSessions sets the maximum number of concurrently bound Sessions owned by this Server.
func WithServerMaxBoundSessions(limit int) ServerOption {
	return func(c *serverConfig) { c.maxBoundSessions = limit }
}

// WithServerMaxPendingPreBindSessions sets the maximum number of accepted TCP connections awaiting Bind on this Server.
func WithServerMaxPendingPreBindSessions(limit int) ServerOption {
	return func(c *serverConfig) { c.maxPendingPreBindSessions = limit }
}

// WithServerPreBindHandshakeTimeout sets the absolute Pre-Bind handshake deadline measured from TCP acceptance.
func WithServerPreBindHandshakeTimeout(timeout time.Duration) ServerOption {
	return func(c *serverConfig) { c.preBindHandshakeTimeout = timeout }
}

// WithServerMaxAggregateQueuedApplicationBytes sets the Server aggregate queued Application frame byte ceiling.
func WithServerMaxAggregateQueuedApplicationBytes(limit uint64) ServerOption {
	return func(c *serverConfig) { c.maxAggregateQueuedAppBytes = limit }
}

// WithServerMaxAggregateQueuedApplicationFrames sets the Server aggregate queued Application frame count ceiling.
func WithServerMaxAggregateQueuedApplicationFrames(limit int) ServerOption {
	return func(c *serverConfig) { c.maxAggregateQueuedAppFrames = limit }
}

// WithRequestTimeout sets the absolute Request deadline beginning after successful local admission.
func WithRequestTimeout(timeout time.Duration) SessionOption {
	return func(c *sessionConfig) { c.requestTimeout = timeout }
}

// WithFrameAssemblyTimeout sets the absolute inbound Frame assembly deadline beginning at its first Header byte.
func WithFrameAssemblyTimeout(timeout time.Duration) SessionOption {
	return func(c *sessionConfig) { c.frameAssemblyTimeout = timeout }
}

// WithUnbindTimeout sets the absolute Graceful Unbind deadline.
func WithUnbindTimeout(timeout time.Duration) SessionOption {
	return func(c *sessionConfig) { c.unbindTimeout = timeout }
}

// WithWriteTimeout sets the absolute per-Frame Writer stall deadline.
func WithWriteTimeout(timeout time.Duration) SessionOption {
	return func(c *sessionConfig) { c.writeTimeout = timeout }
}

// WithEnquireLinkIdleInterval sets the idle interval for automatic Enquire Link probes. Zero disables outgoing probes only.
func WithEnquireLinkIdleInterval(interval time.Duration) SessionOption {
	return func(c *sessionConfig) { c.enquireLinkIdleInterval = interval }
}

// WithEnquireLinkResponseTimeout sets the deadline for a matching Enquire Link Response.
func WithEnquireLinkResponseTimeout(timeout time.Duration) SessionOption {
	return func(c *sessionConfig) { c.enquireLinkResponseTimeout = timeout }
}

// WithMaxQueuedOutgoingBytes sets the per-Session outgoing Application FIFO byte ceiling.
func WithMaxQueuedOutgoingBytes(limit uint64) SessionOption {
	return func(c *sessionConfig) { c.maxQueuedOutgoingBytes = limit }
}

// WithMaxQueuedIncomingBytes sets the per-Session incoming Application waiting-queue byte ceiling.
func WithMaxQueuedIncomingBytes(limit uint64) SessionOption {
	return func(c *sessionConfig) { c.maxQueuedIncomingBytes = limit }
}

// WithMaxQueuedControlBytes sets the independent per-Session Control queue byte ceiling.
func WithMaxQueuedControlBytes(limit uint64) SessionOption {
	return func(c *sessionConfig) { c.maxQueuedControlBytes = limit }
}

// WithApplicationConcurrency sets the maximum concurrent Application callbacks for one Session.
func WithApplicationConcurrency(limit int) SessionOption {
	return func(c *sessionConfig) { c.applicationConcurrency = limit }
}

// WithCompletionConcurrency sets the maximum concurrent Completion callbacks for one Session.
func WithCompletionConcurrency(limit int) SessionOption {
	return func(c *sessionConfig) { c.completionConcurrency = limit }
}

// WithMaxIncomingApplicationFrames sets the per-Session incoming Application frame count ceiling.
func WithMaxIncomingApplicationFrames(limit int) SessionOption {
	return func(c *sessionConfig) { c.maxIncomingFrames = limit }
}

// WithMaxOutgoingApplicationFrames sets the per-Session outgoing Application frame count ceiling.
func WithMaxOutgoingApplicationFrames(limit int) SessionOption {
	return func(c *sessionConfig) { c.maxOutgoingFrames = limit }
}

// WithMaxOutstandingRequests sets the per-Session outstanding Request credit ceiling.
func WithMaxOutstandingRequests(limit int) SessionOption {
	return func(c *sessionConfig) { c.maxOutstandingRequests = limit }
}

// WithMaxRetainedApplicationBytes sets the per-Session retained Application payload byte ceiling.
func WithMaxRetainedApplicationBytes(limit uint64) SessionOption {
	return func(c *sessionConfig) { c.maxRetainedApplication = limit }
}

// WithMaxRetainedCompletionBytes sets the per-Session retained Completion payload byte ceiling.
func WithMaxRetainedCompletionBytes(limit uint64) SessionOption {
	return func(c *sessionConfig) { c.maxRetainedCompletion = limit }
}

func optionIndex(scope string, index int) string {
	return scope + " option at index " + itoa(index) + " is nil"
}

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
