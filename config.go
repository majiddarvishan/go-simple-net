package neta

import (
	"fmt"
	"math"
	"net"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRequestTimeout             = 30 * time.Second
	defaultFrameAssemblyTimeout       = 30 * time.Second
	defaultConnectTimeout             = 30 * time.Second
	defaultBindResponseTimeout        = 30 * time.Second
	defaultPreBindHandshakeTimeout    = 30 * time.Second
	defaultUnbindTimeout              = 10 * time.Second
	defaultWriteTimeout               = 30 * time.Second
	defaultEnquireLinkIdleInterval    = 30 * time.Second
	defaultEnquireLinkResponseTimeout = 10 * time.Second
	defaultWorkerIdleGrace            = 30 * time.Second

	defaultQueuedApplicationBytes    = uint64(4 << 20)
	defaultQueuedControlBytes        = uint64(4 << 10)
	defaultAggregateQueuedFrameBytes = uint64(256 << 20)
	defaultRetainedApplicationBytes  = uint64(16 << 20)
	defaultOwnerApplicationBytes     = uint64(512 << 20)
	defaultRetainedCompletionBytes   = uint64(16 << 20)
	defaultOwnerCompletionBytes      = uint64(256 << 20)

	defaultBoundSessions       = uint32(1024)
	defaultPendingPreBind      = uint32(128)
	defaultApplicationFrames   = uint32(256)
	defaultAggregateFrames     = uint32(65536)
	defaultSessionRequests     = uint32(128)
	defaultOwnerRequests       = uint32(32768)
	defaultOnCloseEventCredits = uint32(1024)

	minRetainedBytes = uint64(10)
	maxFrameSize     = uint64(1 << 20) // Total wire frame, including the 10-byte header.
)

const (
	maxApplicationWorkers         = uint32(512)
	maxCompletionWorkers          = uint32(256)
	maxOnCloseWorkers             = uint32(128)
	maxSessionCallbackConcurrency = uint32(32)
	maxOnCloseEventCredits        = uint32(4096)
	maxSessionApplicationFrames   = uint32(1024)
	maxAggregateApplicationFrames = uint32(262144)
	maxSessionRequests            = uint32(512)
	maxOwnerRequests              = uint32(131072)
	maxSessionRetainedBytes       = uint64(64 << 20)
	maxOwnerApplicationBytes      = uint64(2 << 30)
	maxOwnerCompletionBytes       = uint64(1 << 30)
	maxWorkerIdleGrace            = 10 * time.Minute
)

type sessionConfig struct {
	protocolVersion              ProtocolVersion
	requestTimeout               time.Duration
	frameAssemblyTimeout         time.Duration
	unbindTimeout                time.Duration
	writeTimeout                 time.Duration
	enquireLinkIdleInterval      time.Duration
	enquireLinkResponseTimeout   time.Duration
	maxQueuedOutgoingBytes       uint64
	maxQueuedIncomingBytes       uint64
	maxQueuedControlBytes        uint64
	applicationConcurrency       uint32
	completionConcurrency        uint32
	maxIncomingApplicationFrames uint32
	maxOutgoingApplicationFrames uint32
	maxOutstandingRequests       uint32
	maxRetainedApplicationBytes  uint64
	maxRetainedCompletionBytes   uint64
	invalidOption                string
}

type ownerConfig struct {
	applicationWorkers          uint32
	completionWorkers           uint32
	onCloseWorkers              uint32
	applicationWorkerIdleGrace  time.Duration
	completionWorkerIdleGrace   time.Duration
	onCloseWorkerIdleGrace      time.Duration
	onCloseEventCredits         uint32
	maxOutstandingRequests      uint32
	maxRetainedApplicationBytes uint64
	maxRetainedCompletionBytes  uint64
}

type clientConfig struct {
	address             string
	systemID            string
	systemIDSet         bool
	bindType            BindType
	connectTimeout      time.Duration
	bindResponseTimeout time.Duration
	autoReconnect       bool
	onBound             OnBoundHandler
	onClose             OnCloseHandler
	session             sessionConfig
	owner               ownerConfig
	invalidOption       string
}

type serverConfig struct {
	listenAddress                       string
	bindValidator                       BindValidator
	onBound                             OnBoundHandler
	onRequest                           OnRequestHandler
	onClose                             OnCloseHandler
	preBindHandshakeTimeout             time.Duration
	maxBoundSessions                    uint32
	maxPendingPreBindSessions           uint32
	maxAggregateQueuedApplicationBytes  uint64
	maxAggregateQueuedApplicationFrames uint32
	session                             sessionConfig
	owner                               ownerConfig
	invalidOption                       string
}

func defaultSessionConfig() sessionConfig {
	return sessionConfig{
		protocolVersion:              ProtocolVersionV2,
		requestTimeout:               defaultRequestTimeout,
		frameAssemblyTimeout:         defaultFrameAssemblyTimeout,
		unbindTimeout:                defaultUnbindTimeout,
		writeTimeout:                 defaultWriteTimeout,
		enquireLinkIdleInterval:      defaultEnquireLinkIdleInterval,
		enquireLinkResponseTimeout:   defaultEnquireLinkResponseTimeout,
		maxQueuedOutgoingBytes:       defaultQueuedApplicationBytes,
		maxQueuedIncomingBytes:       defaultQueuedApplicationBytes,
		maxQueuedControlBytes:        defaultQueuedControlBytes,
		applicationConcurrency:       1,
		completionConcurrency:        1,
		maxIncomingApplicationFrames: defaultApplicationFrames,
		maxOutgoingApplicationFrames: defaultApplicationFrames,
		maxOutstandingRequests:       defaultSessionRequests,
		maxRetainedApplicationBytes:  defaultRetainedApplicationBytes,
		maxRetainedCompletionBytes:   defaultRetainedCompletionBytes,
	}
}

func defaultWorkerCounts(procs int) (application, completion, onClose uint32) {
	if procs < 1 {
		procs = 1
	}
	if procs >= 32 {
		return 128, 64, 32
	}
	application = uint32(4 * procs)
	if application < 4 {
		application = 4
	}
	completion = uint32(2 * procs)
	if completion < 2 {
		completion = 2
	}
	onClose = uint32(procs)
	if onClose < 2 {
		onClose = 2
	}
	return application, completion, onClose
}

func defaultOwnerConfig() ownerConfig {
	application, completion, onClose := defaultWorkerCounts(runtime.GOMAXPROCS(0))
	return ownerConfig{
		applicationWorkers:          application,
		completionWorkers:           completion,
		onCloseWorkers:              onClose,
		applicationWorkerIdleGrace:  defaultWorkerIdleGrace,
		completionWorkerIdleGrace:   defaultWorkerIdleGrace,
		onCloseWorkerIdleGrace:      defaultWorkerIdleGrace,
		onCloseEventCredits:         defaultOnCloseEventCredits,
		maxOutstandingRequests:      defaultOwnerRequests,
		maxRetainedApplicationBytes: defaultOwnerApplicationBytes,
		maxRetainedCompletionBytes:  defaultOwnerCompletionBytes,
	}
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		bindType:            BindTypeBiDirectional,
		connectTimeout:      defaultConnectTimeout,
		bindResponseTimeout: defaultBindResponseTimeout,
		autoReconnect:       false,
		session:             defaultSessionConfig(),
		owner:               defaultOwnerConfig(),
	}
}

func defaultServerConfig() serverConfig {
	return serverConfig{
		preBindHandshakeTimeout:             defaultPreBindHandshakeTimeout,
		maxBoundSessions:                    defaultBoundSessions,
		maxPendingPreBindSessions:           defaultPendingPreBind,
		maxAggregateQueuedApplicationBytes:  defaultAggregateQueuedFrameBytes,
		maxAggregateQueuedApplicationFrames: defaultAggregateFrames,
		session:                             defaultSessionConfig(),
		owner:                               defaultOwnerConfig(),
	}
}

func validateSessionConfig(c sessionConfig) error {
	if c.protocolVersion != ProtocolVersionV1 && c.protocolVersion != ProtocolVersionV2 {
		return newConfigError("protocol_version", fmt.Sprint(c.protocolVersion), "must be ProtocolVersionV1 or ProtocolVersionV2")
	}
	durations := []struct {
		name string
		v    time.Duration
	}{
		{"request_timeout", c.requestTimeout},
		{"frame_assembly_timeout", c.frameAssemblyTimeout},
		{"unbind_timeout", c.unbindTimeout},
		{"write_timeout", c.writeTimeout},
		{"enquire_link_response_timeout", c.enquireLinkResponseTimeout},
	}
	for _, item := range durations {
		if item.v <= 0 {
			return newConfigError(item.name, item.v.String(), "must be a positive duration")
		}
	}
	if c.enquireLinkIdleInterval < 0 {
		return newConfigError("enquire_link_idle_interval", c.enquireLinkIdleInterval.String(), "must be zero or a positive duration")
	}
	if err := validatePositiveUint64("max_queued_outgoing_bytes", c.maxQueuedOutgoingBytes); err != nil {
		return err
	}
	if err := validatePositiveUint64("max_queued_incoming_bytes", c.maxQueuedIncomingBytes); err != nil {
		return err
	}
	if err := validatePositiveUint64("max_queued_control_bytes", c.maxQueuedControlBytes); err != nil {
		return err
	}
	if err := validateRangeUint32("application_callback_concurrency", c.applicationConcurrency, 1, maxSessionCallbackConcurrency); err != nil {
		return err
	}
	if err := validateRangeUint32("completion_callback_concurrency", c.completionConcurrency, 1, maxSessionCallbackConcurrency); err != nil {
		return err
	}
	if err := validateRangeUint32("max_incoming_application_frames", c.maxIncomingApplicationFrames, 1, maxSessionApplicationFrames); err != nil {
		return err
	}
	if err := validateRangeUint32("max_outgoing_application_frames", c.maxOutgoingApplicationFrames, 1, maxSessionApplicationFrames); err != nil {
		return err
	}
	if err := validateRangeUint32("max_outstanding_requests_per_session", c.maxOutstandingRequests, 1, maxSessionRequests); err != nil {
		return err
	}
	if err := validateRangeUint64("max_retained_application_bytes_per_session", c.maxRetainedApplicationBytes, minRetainedBytes, maxSessionRetainedBytes); err != nil {
		return err
	}
	if err := validateRangeUint64("max_retained_completion_bytes_per_session", c.maxRetainedCompletionBytes, minRetainedBytes, maxSessionRetainedBytes); err != nil {
		return err
	}
	if c.invalidOption != "" {
		return newConfigError(c.invalidOption, "<nil>", "must be a non-zero SessionOption")
	}
	return nil
}

func validateOwnerConfig(c ownerConfig) error {
	if err := validateRangeUint32("application_workers_per_owner", c.applicationWorkers, 1, maxApplicationWorkers); err != nil {
		return err
	}
	if err := validateRangeUint32("completion_workers_per_owner", c.completionWorkers, 1, maxCompletionWorkers); err != nil {
		return err
	}
	if err := validateRangeUint32("onclose_workers_per_owner", c.onCloseWorkers, 1, maxOnCloseWorkers); err != nil {
		return err
	}
	for _, item := range []struct {
		name string
		v    time.Duration
	}{
		{"application_worker_idle_grace", c.applicationWorkerIdleGrace},
		{"completion_worker_idle_grace", c.completionWorkerIdleGrace},
		{"onclose_worker_idle_grace", c.onCloseWorkerIdleGrace},
	} {
		if item.v < time.Millisecond || item.v > maxWorkerIdleGrace {
			return newConfigError(item.name, item.v.String(), "must be between 1ms and 10m inclusive")
		}
	}
	if err := validateRangeUint32("onclose_event_credits_per_owner", c.onCloseEventCredits, 1, maxOnCloseEventCredits); err != nil {
		return err
	}
	if err := validateRangeUint32("max_outstanding_requests_per_owner", c.maxOutstandingRequests, 1, maxOwnerRequests); err != nil {
		return err
	}
	if err := validateRangeUint64("max_retained_application_bytes_per_owner", c.maxRetainedApplicationBytes, minRetainedBytes, maxOwnerApplicationBytes); err != nil {
		return err
	}
	if err := validateRangeUint64("max_retained_completion_bytes_per_owner", c.maxRetainedCompletionBytes, minRetainedBytes, maxOwnerCompletionBytes); err != nil {
		return err
	}
	return nil
}

func validateClientConfig(c clientConfig) error {
	if c.invalidOption != "" {
		return newConfigError(c.invalidOption, "<nil>", "must be a non-zero ClientOption")
	}
	if strings.TrimSpace(c.address) == "" {
		return newConfigError("address", c.address, "must be a non-empty host:port endpoint")
	}
	if strings.TrimSpace(c.address) != c.address {
		return newConfigError("address", c.address, "must not contain leading or trailing whitespace")
	}
	if err := validateTCPAddress("address", c.address, true); err != nil {
		return err
	}
	if !c.systemIDSet {
		return newConfigError("system_id", "<unset>", "must be configured with WithClientSystemID")
	}
	if c.systemID == "" {
		return newConfigError("system_id", c.systemID, "must be non-empty")
	}
	if len(c.systemID) > 19 {
		return newConfigError("system_id", c.systemID, "must be at most 19 bytes")
	}
	if strings.IndexByte(c.systemID, 0) >= 0 {
		return newConfigError("system_id", c.systemID, "must not contain NUL")
	}
	if c.bindType != BindTypeUniDirectional && c.bindType != BindTypeBiDirectional {
		return newConfigError("bind_type", fmt.Sprint(c.bindType), "must be a supported BindType")
	}
	if c.connectTimeout <= 0 {
		return newConfigError("connect_timeout", c.connectTimeout.String(), "must be a positive duration")
	}
	if c.bindResponseTimeout <= 0 {
		return newConfigError("bind_response_timeout", c.bindResponseTimeout.String(), "must be a positive duration")
	}
	if err := validateSessionConfig(c.session); err != nil {
		return err
	}
	return validateOwnerConfig(c.owner)
}

func validateServerConfig(c serverConfig) error {
	if c.invalidOption != "" {
		return newConfigError(c.invalidOption, "<nil>", "must be a non-zero ServerOption")
	}
	if strings.TrimSpace(c.listenAddress) == "" {
		return newConfigError("listen_address", c.listenAddress, "must be a non-empty host:port endpoint")
	}
	if strings.TrimSpace(c.listenAddress) != c.listenAddress {
		return newConfigError("listen_address", c.listenAddress, "must not contain leading or trailing whitespace")
	}
	if err := validateTCPAddress("listen_address", c.listenAddress, false); err != nil {
		return err
	}
	if c.preBindHandshakeTimeout <= 0 {
		return newConfigError("prebind_handshake_timeout", c.preBindHandshakeTimeout.String(), "must be a positive duration")
	}
	if c.maxBoundSessions == 0 {
		return newConfigError("max_bound_sessions", "0", "must be positive")
	}
	if c.maxPendingPreBindSessions == 0 {
		return newConfigError("max_pending_prebind_sessions", "0", "must be positive")
	}
	if err := validatePositiveUint64("max_aggregate_queued_application_bytes", c.maxAggregateQueuedApplicationBytes); err != nil {
		return err
	}
	if err := validateRangeUint32("max_aggregate_queued_application_frames", c.maxAggregateQueuedApplicationFrames, 1, maxAggregateApplicationFrames); err != nil {
		return err
	}
	if err := validateSessionConfig(c.session); err != nil {
		return err
	}
	return validateOwnerConfig(c.owner)
}

func validateTCPAddress(field, address string, requireHost bool) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return newConfigError(field, address, "must be a valid TCP host:port endpoint")
	}
	if requireHost && host == "" {
		return newConfigError(field, address, "must include a remote host")
	}
	return nil
}

func validatePositiveUint64(field string, value uint64) error {
	if value == 0 {
		return newConfigError(field, "0", "must be positive")
	}
	return nil
}

func validateRangeUint32(field string, value, min, max uint32) error {
	if value < min || value > max {
		return newConfigError(field, fmt.Sprint(value), fmt.Sprintf("must be in range %d..%d", min, max))
	}
	return nil
}

func validateRangeUint64(field string, value, min, max uint64) error {
	if value < min || value > max {
		return newConfigError(field, fmt.Sprint(value), fmt.Sprintf("must be in range %d..%d", min, max))
	}
	return nil
}

func checkedAddUint64(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, false
	}
	return a + b, true
}

func checkedSubUint64(a, b uint64) (uint64, bool) {
	if b > a {
		return 0, false
	}
	return a - b, true
}

func checkedMulUint64(a, b uint64) (uint64, bool) {
	if a != 0 && b > math.MaxUint64/a {
		return 0, false
	}
	return a * b, true
}
