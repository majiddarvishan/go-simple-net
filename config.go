package neta

import (
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	bytesPerKiB uint64 = 1 << 10
	bytesPerMiB uint64 = 1 << 20
	bytesPerGiB uint64 = 1 << 30

	maxApplicationWorkers       = 512
	maxCompletionWorkers        = 256
	maxOnCloseWorkers           = 128
	maxSessionCallbackWorkers   = 32
	maxOnCloseEventCredits      = 4096
	maxApplicationFramesSession = 1024
	maxAggregateFramesServer    = 262144
	maxRequestsSession          = 512
	maxRequestsOwner            = 131072
	maxRetainedApplicationSess  = 64 * bytesPerMiB
	maxRetainedApplicationOwner = 2 * bytesPerGiB
	maxRetainedCompletionSess   = 64 * bytesPerMiB
	maxRetainedCompletionOwner  = 1 * bytesPerGiB

	minimumRetainedBytes uint64 = 10
	maxSystemIDBytes            = 19 // Pinex c_octet_str<20>, excluding the NUL terminator.
)

type workerConfig struct {
	applicationWorkers int
	completionWorkers  int
	onCloseWorkers     int

	applicationIdleGrace time.Duration
	completionIdleGrace  time.Duration
	onCloseIdleGrace     time.Duration

	onCloseEventCredits          int
	aggregateOutstandingRequests int
	aggregateRetainedApplication uint64
	aggregateRetainedCompletion  uint64
}

type sessionConfig struct {
	requestTimeout             time.Duration
	frameAssemblyTimeout       time.Duration
	unbindTimeout              time.Duration
	writeTimeout               time.Duration
	enquireLinkIdleInterval    time.Duration
	enquireLinkResponseTimeout time.Duration

	maxQueuedOutgoingBytes uint64
	maxQueuedIncomingBytes uint64
	maxQueuedControlBytes  uint64

	applicationConcurrency int
	completionConcurrency  int
	maxIncomingFrames      int
	maxOutgoingFrames      int
	maxOutstandingRequests int
	maxRetainedApplication uint64
	maxRetainedCompletion  uint64
}

type clientConfig struct {
	address             string
	systemID            string
	bindType            uint8
	connectTimeout      time.Duration
	bindResponseTimeout time.Duration
	autoReconnect       bool

	workers workerConfig
	session sessionConfig

	onBound OnBoundHandler
	onClose OnCloseHandler

	optionErrors []string
}

type serverConfig struct {
	listenAddress string
	systemID      string
	bindValidator BindValidator

	maxBoundSessions            int
	maxPendingPreBindSessions   int
	preBindHandshakeTimeout     time.Duration
	maxAggregateQueuedAppBytes  uint64
	maxAggregateQueuedAppFrames int

	workers workerConfig
	session sessionConfig

	onBound   OnBoundHandler
	onRequest OnRequestHandler
	onClose   OnCloseHandler

	optionErrors []string
}

func defaultWorkerConfig() workerConfig {
	p := maxInt(1, runtimeGOMAXPROCS())
	return workerConfig{
		applicationWorkers:           scaledWorkerDefault(p, 4, 4, 128),
		completionWorkers:            scaledWorkerDefault(p, 2, 2, 64),
		onCloseWorkers:               scaledWorkerDefault(p, 1, 2, 32),
		applicationIdleGrace:         30 * time.Second,
		completionIdleGrace:          30 * time.Second,
		onCloseIdleGrace:             30 * time.Second,
		onCloseEventCredits:          1024,
		aggregateOutstandingRequests: 32768,
		aggregateRetainedApplication: 512 * bytesPerMiB,
		aggregateRetainedCompletion:  256 * bytesPerMiB,
	}
}

func defaultSessionConfig() sessionConfig {
	return sessionConfig{
		requestTimeout:             30 * time.Second,
		frameAssemblyTimeout:       30 * time.Second,
		unbindTimeout:              10 * time.Second,
		writeTimeout:               30 * time.Second,
		enquireLinkIdleInterval:    30 * time.Second,
		enquireLinkResponseTimeout: 10 * time.Second,
		maxQueuedOutgoingBytes:     4 * bytesPerMiB,
		maxQueuedIncomingBytes:     4 * bytesPerMiB,
		maxQueuedControlBytes:      4 * bytesPerKiB,
		applicationConcurrency:     1,
		completionConcurrency:      1,
		maxIncomingFrames:          256,
		maxOutgoingFrames:          256,
		maxOutstandingRequests:     128,
		maxRetainedApplication:     16 * bytesPerMiB,
		maxRetainedCompletion:      16 * bytesPerMiB,
	}
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		bindType:            BindTypeBiDirection,
		connectTimeout:      30 * time.Second,
		bindResponseTimeout: 30 * time.Second,
		workers:             defaultWorkerConfig(),
		session:             defaultSessionConfig(),
	}
}

func defaultServerConfig() serverConfig {
	return serverConfig{
		maxBoundSessions:            1024,
		maxPendingPreBindSessions:   128,
		preBindHandshakeTimeout:     30 * time.Second,
		maxAggregateQueuedAppBytes:  256 * bytesPerMiB,
		maxAggregateQueuedAppFrames: 65536,
		workers:                     defaultWorkerConfig(),
		session:                     defaultSessionConfig(),
	}
}

func (c clientConfig) validate() error {
	if len(c.optionErrors) != 0 {
		return invalidConfig("options", strings.Join(c.optionErrors, "; "), "nil Functional Option")
	}
	if err := validateTCPAddress("ClientAddress", c.address, false); err != nil {
		return err
	}
	if err := validateSystemID("ClientSystemID", c.systemID); err != nil {
		return err
	}
	if c.bindType != BindTypeUniDirection && c.bindType != BindTypeBiDirection {
		return invalidConfig("BindType", c.bindType, "must be 0 (uni-directional) or 1 (bi-directional)")
	}
	if err := validatePositiveDuration("ConnectTimeout", c.connectTimeout); err != nil {
		return err
	}
	if err := validatePositiveDuration("BindResponseTimeout", c.bindResponseTimeout); err != nil {
		return err
	}
	if err := c.workers.validate(); err != nil {
		return err
	}
	return c.session.validate()
}

func (c serverConfig) validate() error {
	if len(c.optionErrors) != 0 {
		return invalidConfig("options", strings.Join(c.optionErrors, "; "), "nil Functional Option")
	}
	if err := validateTCPAddress("ListenAddress", c.listenAddress, true); err != nil {
		return err
	}
	if err := validateSystemID("ServerSystemID", c.systemID); err != nil {
		return err
	}
	if c.maxBoundSessions < 1 {
		return invalidConfig("MaxBoundSessions", c.maxBoundSessions, "must be at least 1")
	}
	if c.maxPendingPreBindSessions < 1 {
		return invalidConfig("MaxPendingPreBindSessions", c.maxPendingPreBindSessions, "must be at least 1")
	}
	if err := validatePositiveDuration("PreBindHandshakeTimeout", c.preBindHandshakeTimeout); err != nil {
		return err
	}
	if c.maxAggregateQueuedAppBytes == 0 {
		return invalidConfig("MaxAggregateQueuedApplicationBytes", c.maxAggregateQueuedAppBytes, "must be at least 1 byte")
	}
	if c.maxAggregateQueuedAppFrames < 1 || c.maxAggregateQueuedAppFrames > maxAggregateFramesServer {
		return invalidConfig("MaxAggregateQueuedApplicationFrames", c.maxAggregateQueuedAppFrames, "must be in range 1..262144")
	}
	if err := c.workers.validate(); err != nil {
		return err
	}
	return c.session.validate()
}

func (c workerConfig) validate() error {
	if c.applicationWorkers < 1 || c.applicationWorkers > maxApplicationWorkers {
		return invalidConfig("ApplicationWorkersPerOwner", c.applicationWorkers, "must be in range 1..512")
	}
	if c.completionWorkers < 1 || c.completionWorkers > maxCompletionWorkers {
		return invalidConfig("CompletionWorkersPerOwner", c.completionWorkers, "must be in range 1..256")
	}
	if c.onCloseWorkers < 1 || c.onCloseWorkers > maxOnCloseWorkers {
		return invalidConfig("OnCloseWorkersPerOwner", c.onCloseWorkers, "must be in range 1..128")
	}
	if err := validateWorkerIdleGrace("ApplicationWorkerIdleGrace", c.applicationIdleGrace); err != nil {
		return err
	}
	if err := validateWorkerIdleGrace("CompletionWorkerIdleGrace", c.completionIdleGrace); err != nil {
		return err
	}
	if err := validateWorkerIdleGrace("OnCloseWorkerIdleGrace", c.onCloseIdleGrace); err != nil {
		return err
	}
	if c.onCloseEventCredits < 1 || c.onCloseEventCredits > maxOnCloseEventCredits {
		return invalidConfig("OnCloseEventCreditsPerOwner", c.onCloseEventCredits, "must be in range 1..4096")
	}
	if c.aggregateOutstandingRequests < 1 || c.aggregateOutstandingRequests > maxRequestsOwner {
		return invalidConfig("AggregateOutstandingRequestsPerOwner", c.aggregateOutstandingRequests, "must be in range 1..131072")
	}
	if c.aggregateRetainedApplication < minimumRetainedBytes || c.aggregateRetainedApplication > maxRetainedApplicationOwner {
		return invalidConfig("AggregateRetainedApplicationBytesPerOwner", c.aggregateRetainedApplication, "must be in range 10 bytes..2 GiB")
	}
	if c.aggregateRetainedCompletion < minimumRetainedBytes || c.aggregateRetainedCompletion > maxRetainedCompletionOwner {
		return invalidConfig("AggregateRetainedCompletionBytesPerOwner", c.aggregateRetainedCompletion, "must be in range 10 bytes..1 GiB")
	}
	return nil
}

func (c sessionConfig) validate() error {
	for _, item := range []struct {
		name  string
		value time.Duration
	}{
		{"RequestTimeout", c.requestTimeout},
		{"FrameAssemblyTimeout", c.frameAssemblyTimeout},
		{"UnbindTimeout", c.unbindTimeout},
		{"WriteTimeout", c.writeTimeout},
		{"EnquireLinkResponseTimeout", c.enquireLinkResponseTimeout},
	} {
		if err := validatePositiveDuration(item.name, item.value); err != nil {
			return err
		}
	}
	if c.enquireLinkIdleInterval < 0 {
		return invalidConfig("EnquireLinkIdleInterval", c.enquireLinkIdleInterval, "must be zero (disable) or positive")
	}
	for _, item := range []struct {
		name  string
		value uint64
	}{
		{"MaxQueuedOutgoingBytes", c.maxQueuedOutgoingBytes},
		{"MaxQueuedIncomingBytes", c.maxQueuedIncomingBytes},
		{"MaxQueuedControlBytes", c.maxQueuedControlBytes},
	} {
		if item.value == 0 {
			return invalidConfig(item.name, item.value, "must be at least 1 byte")
		}
	}
	if c.applicationConcurrency < 1 || c.applicationConcurrency > maxSessionCallbackWorkers {
		return invalidConfig("ApplicationConcurrencyPerSession", c.applicationConcurrency, "must be in range 1..32")
	}
	if c.completionConcurrency < 1 || c.completionConcurrency > maxSessionCallbackWorkers {
		return invalidConfig("CompletionConcurrencyPerSession", c.completionConcurrency, "must be in range 1..32")
	}
	if c.maxIncomingFrames < 1 || c.maxIncomingFrames > maxApplicationFramesSession {
		return invalidConfig("IncomingApplicationFramesPerSession", c.maxIncomingFrames, "must be in range 1..1024")
	}
	if c.maxOutgoingFrames < 1 || c.maxOutgoingFrames > maxApplicationFramesSession {
		return invalidConfig("OutgoingApplicationFramesPerSession", c.maxOutgoingFrames, "must be in range 1..1024")
	}
	if c.maxOutstandingRequests < 1 || c.maxOutstandingRequests > maxRequestsSession {
		return invalidConfig("OutstandingRequestsPerSession", c.maxOutstandingRequests, "must be in range 1..512")
	}
	if c.maxRetainedApplication < minimumRetainedBytes || c.maxRetainedApplication > maxRetainedApplicationSess {
		return invalidConfig("RetainedApplicationBytesPerSession", c.maxRetainedApplication, "must be in range 10 bytes..64 MiB")
	}
	if c.maxRetainedCompletion < minimumRetainedBytes || c.maxRetainedCompletion > maxRetainedCompletionSess {
		return invalidConfig("RetainedCompletionBytesPerSession", c.maxRetainedCompletion, "must be in range 10 bytes..64 MiB")
	}
	return nil
}

func validateSystemID(field, systemID string) error {
	if strings.IndexByte(systemID, 0) >= 0 {
		return invalidConfig(field, "<redacted>", "must not contain a NUL byte")
	}
	if size := len([]byte(systemID)); size > maxSystemIDBytes {
		return invalidConfig(field, size, "Pinex system_id is limited to 19 bytes")
	}
	return nil
}

func validatePositiveDuration(field string, value time.Duration) error {
	if value <= 0 {
		return invalidConfig(field, value, "must be a positive finite duration")
	}
	return nil
}

func validateWorkerIdleGrace(field string, value time.Duration) error {
	if value < time.Millisecond || value > 10*time.Minute {
		return invalidConfig(field, value, "must be in range 1ms..10m")
	}
	return nil
}

func validateTCPAddress(field, address string, allowEmptyHost bool) error {
	if address == "" {
		return invalidConfig(field, address, "must not be empty")
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return invalidConfig(field, address, "must be a valid host:port TCP address")
	}
	if host == "" && !allowEmptyHost {
		return invalidConfig(field, address, "host must not be empty for a Client endpoint")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > math.MaxUint16 || (!allowEmptyHost && port == 0) {
		return invalidConfig(field, address, "port must be numeric and in the supported TCP range")
	}
	return nil
}

// runtimeGOMAXPROCS and maxInt are isolated to make the approved CPU formula testable.
func runtimeGOMAXPROCS() int { return runtimeGOMAXPROCSImpl() }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// scaledWorkerDefault computes min(max(floor, multiplier*P), cap) without overflow.
func scaledWorkerDefault(p, multiplier, floor, cap int) int {
	if p < 1 {
		p = 1
	}
	if p > cap/multiplier {
		return cap
	}
	return minInt(maxInt(floor, p*multiplier), cap)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
