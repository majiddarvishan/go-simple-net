package neta

import (
	"errors"
	"math"
	"testing"
	"time"
)

func validClientOptions() []ClientOption {
	return []ClientOption{
		WithClientAddress("127.0.0.1:9000"),
		WithClientSystemID("neta-client"),
	}
}

func validServerOptions() []ServerOption {
	return []ServerOption{WithServerListenAddress("127.0.0.1:0")}
}

func TestConstructorsRequireNetworkEndpoints(t *testing.T) {
	if _, err := NewClient(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewClient() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewServer(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewServer() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewClient(WithClientAddress("127.0.0.1:9000")); err != nil {
		t.Fatalf("empty SystemID is a valid Pinex C-Octet string: %v", err)
	}
}

func TestDefaultsAndIndependentOverrides(t *testing.T) {
	clientOptions := append(validClientOptions(), WithClientSessionOptions(WithRequestTimeout(7*time.Second)))
	client, err := NewClient(clientOptions...)
	if err != nil {
		t.Fatal(err)
	}
	if client.config.session.requestTimeout != 7*time.Second {
		t.Fatalf("RequestTimeout = %s, want 7s", client.config.session.requestTimeout)
	}
	if client.config.session.frameAssemblyTimeout != 30*time.Second {
		t.Fatalf("FrameAssemblyTimeout changed with unrelated option: %s", client.config.session.frameAssemblyTimeout)
	}
	if client.config.autoReconnect {
		t.Fatal("AutoReconnect default must be false")
	}
	if client.config.bindType != BindTypeBiDirection {
		t.Fatalf("BindType default = %d, want bi-directional", client.config.bindType)
	}

	serverOptions := append(validServerOptions(), WithServerSessionOptions(WithEnquireLinkIdleInterval(0)))
	server, err := NewServer(serverOptions...)
	if err != nil {
		t.Fatal(err)
	}
	if server.config.session.enquireLinkIdleInterval != 0 {
		t.Fatalf("EnquireLinkIdleInterval = %s, want explicit zero", server.config.session.enquireLinkIdleInterval)
	}
	if server.config.session.enquireLinkResponseTimeout != 10*time.Second {
		t.Fatalf("response timeout = %s, want 10s", server.config.session.enquireLinkResponseTimeout)
	}
}

func TestWorkerDefaultFormulaAndCaps(t *testing.T) {
	cases := []struct{ p, app, completion, onClose int }{
		{1, 4, 2, 2},
		{2, 8, 4, 2},
		{4, 16, 8, 4},
		{8, 32, 16, 8},
		{16, 64, 32, 16},
		{32, 128, 64, 32},
		{128, 128, 64, 32},
	}
	for _, tc := range cases {
		if got := scaledWorkerDefault(tc.p, 4, 4, 128); got != tc.app {
			t.Errorf("app workers at P=%d: got %d want %d", tc.p, got, tc.app)
		}
		if got := scaledWorkerDefault(tc.p, 2, 2, 64); got != tc.completion {
			t.Errorf("completion workers at P=%d: got %d want %d", tc.p, got, tc.completion)
		}
		if got := scaledWorkerDefault(tc.p, 1, 2, 32); got != tc.onClose {
			t.Errorf("OnClose workers at P=%d: got %d want %d", tc.p, got, tc.onClose)
		}
	}
	if got := scaledWorkerDefault(math.MaxInt, 4, 4, 128); got != 128 {
		t.Fatalf("overflow-safe capped worker default = %d, want 128", got)
	}
}

func TestApprovedWorker009CHardMaximums(t *testing.T) {
	valid := []ClientOption{
		WithClientApplicationWorkers(512),
		WithClientCompletionWorkers(256),
		WithClientOnCloseWorkers(128),
		WithClientOnCloseEventCredits(4096),
		WithClientOutstandingRequestCredits(131072),
		WithClientRetainedApplicationBytes(2 * bytesPerGiB),
		WithClientRetainedCompletionBytes(1 * bytesPerGiB),
	}
	options := append(validClientOptions(), valid...)
	if _, err := NewClient(options...); err != nil {
		t.Fatalf("approved maximums should be accepted: %v", err)
	}

	badCases := [][]ClientOption{
		{WithClientApplicationWorkers(513)},
		{WithClientCompletionWorkers(257)},
		{WithClientOnCloseWorkers(129)},
		{WithClientOnCloseEventCredits(4097)},
		{WithClientOutstandingRequestCredits(131073)},
		{WithClientRetainedApplicationBytes(2*bytesPerGiB + 1)},
		{WithClientRetainedCompletionBytes(1*bytesPerGiB + 1)},
	}
	for i, bad := range badCases {
		if _, err := NewClient(append(validClientOptions(), bad...)...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("bad owner config case %d error = %v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestSessionHardMaximumsAndMinimumRetainedBytes(t *testing.T) {
	maxes := []SessionOption{
		WithApplicationConcurrency(32),
		WithCompletionConcurrency(32),
		WithMaxIncomingApplicationFrames(1024),
		WithMaxOutgoingApplicationFrames(1024),
		WithMaxOutstandingRequests(512),
		WithMaxRetainedApplicationBytes(64 * bytesPerMiB),
		WithMaxRetainedCompletionBytes(64 * bytesPerMiB),
		WithApplicationConcurrency(32),
		WithRequestTimeout(time.Nanosecond),
		WithFrameAssemblyTimeout(time.Nanosecond),
		WithUnbindTimeout(time.Nanosecond),
		WithWriteTimeout(time.Nanosecond),
		WithEnquireLinkResponseTimeout(time.Nanosecond),
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(maxes...))...); err != nil {
		t.Fatalf("session hard maxima should be accepted: %v", err)
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(WithMaxRetainedApplicationBytes(9)))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("retained bytes below 10 error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(WithMaxIncomingApplicationFrames(1025)))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("incoming frames above hard max error = %v, want ErrInvalidConfig", err)
	}
}

func TestCrossFieldCapacitiesRemainIndependent(t *testing.T) {
	// Aggregate limits may be lower than per-Session limits; concurrency is
	// intersected with the owner worker ceiling at dispatch, not rejected here.
	options := append(validServerOptions(),
		WithServerApplicationWorkers(1),
		WithServerOutstandingRequestCredits(1),
		WithServerRetainedApplicationBytes(minimumRetainedBytes),
		WithServerMaxAggregateQueuedApplicationFrames(1),
		WithServerSessionOptions(
			WithApplicationConcurrency(32),
			WithMaxOutstandingRequests(512),
			WithMaxIncomingApplicationFrames(1024),
		),
	)
	if _, err := NewServer(options...); err != nil {
		t.Fatalf("independent aggregate/session limits should be accepted: %v", err)
	}
}

func TestOnlyEnquireLinkIdleIntervalAllowsZero(t *testing.T) {
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(WithRequestTimeout(0)))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("RequestTimeout=0 error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(WithEnquireLinkIdleInterval(-time.Second)))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("negative idle interval error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(WithEnquireLinkIdleInterval(0)))...); err != nil {
		t.Fatalf("EnquireLinkIdleInterval=0 should disable probes: %v", err)
	}
}

func TestSystemIDMatchesPinexCStringBounds(t *testing.T) {
	for _, tc := range []struct {
		id  string
		bad bool
	}{
		{"", false},
		{"valid-system-id-123", false},
		{"12345678901234567890", true},
		{"bad\x00id", true},
	} {
		options := append(validClientOptions()[:1], WithClientSystemID(tc.id))
		_, err := NewClient(options...)
		if (err != nil) != tc.bad {
			t.Errorf("SystemID %q error=%v, bad=%v", tc.id, err, tc.bad)
		}
	}
}

func TestNilOptionsAreRejected(t *testing.T) {
	if _, err := NewClient(append(validClientOptions(), nil)...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil ClientOption error = %v, want ErrInvalidConfig", err)
	}
	if _, err := NewServer(append(validServerOptions(), WithServerSessionOptions(nil))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil SessionOption error = %v, want ErrInvalidConfig", err)
	}
}

func TestTCPAddressValidation(t *testing.T) {
	for _, address := range []string{"localhost", "127.0.0.1:0", "[::1]9000", "host:notaport", "host:65536"} {
		options := append(validClientOptions(), WithClientAddress(address))
		if _, err := NewClient(options...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Client address %q error=%v, want ErrInvalidConfig", address, err)
		}
	}
	if _, err := NewServer(WithServerListenAddress(":0")); err != nil {
		t.Fatalf("ephemeral server listen address should be valid: %v", err)
	}
}

func TestApprovedDefaultsAreCompleteAndFinite(t *testing.T) {
	client, err := NewClient(validClientOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(validServerOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	for name, cfg := range map[string]sessionConfig{
		"client": client.config.session,
		"server": server.config.session,
	} {
		if cfg.requestTimeout != 30*time.Second || cfg.frameAssemblyTimeout != 30*time.Second || cfg.unbindTimeout != 10*time.Second || cfg.writeTimeout != 30*time.Second {
			t.Errorf("%s timeout defaults are incorrect: %+v", name, cfg)
		}
		if cfg.enquireLinkIdleInterval != 30*time.Second || cfg.enquireLinkResponseTimeout != 10*time.Second {
			t.Errorf("%s Enquire Link defaults are incorrect: %+v", name, cfg)
		}
		if cfg.maxQueuedOutgoingBytes != 4*bytesPerMiB || cfg.maxQueuedIncomingBytes != 4*bytesPerMiB || cfg.maxQueuedControlBytes != 4*bytesPerKiB {
			t.Errorf("%s queue-byte defaults are incorrect: %+v", name, cfg)
		}
		if cfg.applicationConcurrency != 1 || cfg.completionConcurrency != 1 || cfg.maxIncomingFrames != 256 || cfg.maxOutgoingFrames != 256 || cfg.maxOutstandingRequests != 128 {
			t.Errorf("%s concurrency/count defaults are incorrect: %+v", name, cfg)
		}
		if cfg.maxRetainedApplication != 16*bytesPerMiB || cfg.maxRetainedCompletion != 16*bytesPerMiB {
			t.Errorf("%s retained-byte defaults are incorrect: %+v", name, cfg)
		}
	}

	if client.config.autoReconnect || client.config.connectTimeout != 30*time.Second || client.config.bindResponseTimeout != 30*time.Second {
		t.Errorf("Client lifecycle defaults are incorrect: %+v", client.config)
	}
	if client.config.workers.onCloseEventCredits != 1024 || client.config.workers.aggregateOutstandingRequests != 32768 || client.config.workers.aggregateRetainedApplication != 512*bytesPerMiB || client.config.workers.aggregateRetainedCompletion != 256*bytesPerMiB {
		t.Errorf("Client owner resource defaults are incorrect: %+v", client.config.workers)
	}
	if server.config.maxBoundSessions != 1024 || server.config.maxPendingPreBindSessions != 128 || server.config.preBindHandshakeTimeout != 30*time.Second || server.config.maxAggregateQueuedAppBytes != 256*bytesPerMiB || server.config.maxAggregateQueuedAppFrames != 65536 {
		t.Errorf("Server admission defaults are incorrect: %+v", server.config)
	}
	if server.config.workers.onCloseEventCredits != 1024 || server.config.workers.aggregateOutstandingRequests != 32768 || server.config.workers.aggregateRetainedApplication != 512*bytesPerMiB || server.config.workers.aggregateRetainedCompletion != 256*bytesPerMiB {
		t.Errorf("Server owner resource defaults are incorrect: %+v", server.config.workers)
	}
}

func TestWorkerIdleGraceRange(t *testing.T) {
	valid := []ClientOption{
		WithClientApplicationWorkerIdleGrace(time.Millisecond),
		WithClientCompletionWorkerIdleGrace(10 * time.Minute),
		WithClientOnCloseWorkerIdleGrace(time.Millisecond),
	}
	if _, err := NewClient(append(validClientOptions(), valid...)...); err != nil {
		t.Fatalf("idle grace boundary values should be accepted: %v", err)
	}
	invalid := [][]ClientOption{
		{WithClientApplicationWorkerIdleGrace(time.Millisecond - 1)},
		{WithClientCompletionWorkerIdleGrace(10*time.Minute + 1)},
		{WithClientOnCloseWorkerIdleGrace(0)},
	}
	for i, options := range invalid {
		if _, err := NewClient(append(validClientOptions(), options...)...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("idle grace invalid case %d error=%v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestServerAdmissionRanges(t *testing.T) {
	if _, err := NewServer(append(validServerOptions(),
		WithServerMaxAggregateQueuedApplicationFrames(262144),
		WithServerMaxAggregateQueuedApplicationBytes(math.MaxUint64),
		WithServerMaxBoundSessions(1),
		WithServerMaxPendingPreBindSessions(1),
	)...); err != nil {
		t.Fatalf("valid Server admission boundaries rejected: %v", err)
	}
	invalid := [][]ServerOption{
		{WithServerMaxAggregateQueuedApplicationFrames(0)},
		{WithServerMaxAggregateQueuedApplicationFrames(262145)},
		{WithServerMaxAggregateQueuedApplicationBytes(0)},
		{WithServerMaxBoundSessions(0)},
		{WithServerMaxPendingPreBindSessions(0)},
		{WithServerPreBindHandshakeTimeout(0)},
	}
	for i, options := range invalid {
		if _, err := NewServer(append(validServerOptions(), options...)...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("invalid Server admission case %d error=%v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestSessionMinimumsAndDurationRules(t *testing.T) {
	minimum := []SessionOption{
		WithApplicationConcurrency(1),
		WithCompletionConcurrency(1),
		WithMaxIncomingApplicationFrames(1),
		WithMaxOutgoingApplicationFrames(1),
		WithMaxOutstandingRequests(1),
		WithMaxRetainedApplicationBytes(10),
		WithMaxRetainedCompletionBytes(10),
		WithMaxQueuedIncomingBytes(1),
		WithMaxQueuedOutgoingBytes(1),
		WithMaxQueuedControlBytes(1),
		WithRequestTimeout(time.Nanosecond),
		WithFrameAssemblyTimeout(time.Nanosecond),
		WithUnbindTimeout(time.Nanosecond),
		WithWriteTimeout(time.Nanosecond),
		WithEnquireLinkResponseTimeout(time.Nanosecond),
	}
	if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(minimum...))...); err != nil {
		t.Fatalf("approved minimum values should be accepted: %v", err)
	}
	invalid := []SessionOption{
		WithApplicationConcurrency(0),
		WithCompletionConcurrency(0),
		WithMaxIncomingApplicationFrames(0),
		WithMaxOutgoingApplicationFrames(0),
		WithMaxOutstandingRequests(0),
		WithMaxRetainedApplicationBytes(9),
		WithMaxRetainedCompletionBytes(9),
		WithMaxQueuedIncomingBytes(0),
		WithMaxQueuedOutgoingBytes(0),
		WithMaxQueuedControlBytes(0),
		WithFrameAssemblyTimeout(-time.Nanosecond),
		WithUnbindTimeout(0),
		WithWriteTimeout(-time.Nanosecond),
		WithEnquireLinkResponseTimeout(0),
	}
	for i, option := range invalid {
		if _, err := NewClient(append(validClientOptions(), WithClientSessionOptions(option))...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("invalid Session option case %d error=%v, want ErrInvalidConfig", i, err)
		}
	}
}

func TestFunctionalOptionsApplyBeforeValidationAndAreIndependent(t *testing.T) {
	// The first value is invalid in isolation, but the later option replaces it
	// before the constructor validates the completed effective configuration.
	client, err := NewClient(append(validClientOptions(),
		WithClientSessionOptions(WithRequestTimeout(0), WithRequestTimeout(2*time.Second)),
	)...)
	if err != nil {
		t.Fatalf("last explicit option should win before validation: %v", err)
	}
	if client.config.session.requestTimeout != 2*time.Second {
		t.Fatalf("effective RequestTimeout=%s, want 2s", client.config.session.requestTimeout)
	}
}

func TestConfigErrorSupportsErrorsAs(t *testing.T) {
	_, err := NewClient(append(validClientOptions(), WithClientApplicationWorkers(0))...)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("errors.Is(%v, ErrInvalidConfig) = false", err)
	}
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("errors.As(%v, *ConfigError) = false", err)
	}
	if configErr.Field != "ApplicationWorkersPerOwner" {
		t.Fatalf("ConfigError.Field=%q, want ApplicationWorkersPerOwner", configErr.Field)
	}
}

func TestClientAndServerSystemIDWireBounds(t *testing.T) {
	for _, systemID := range []string{"", "1234567890123456789"} {
		if _, err := NewClient(append(validClientOptions(), WithClientSystemID(systemID))...); err != nil {
			t.Errorf("Client SystemID %q should be valid: %v", systemID, err)
		}
		if _, err := NewServer(append(validServerOptions(), WithServerSystemID(systemID))...); err != nil {
			t.Errorf("Server SystemID %q should be valid: %v", systemID, err)
		}
	}
	for _, systemID := range []string{"12345678901234567890", "bad\x00id"} {
		if _, err := NewClient(append(validClientOptions(), WithClientSystemID(systemID))...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Client SystemID %q error=%v, want ErrInvalidConfig", systemID, err)
		}
		if _, err := NewServer(append(validServerOptions(), WithServerSystemID(systemID))...); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Server SystemID %q error=%v, want ErrInvalidConfig", systemID, err)
		}
	}
}

func TestBindTypeValidation(t *testing.T) {
	for _, bindType := range []uint8{BindTypeUniDirection, BindTypeBiDirection} {
		if _, err := NewClient(append(validClientOptions(), WithClientBindType(bindType))...); err != nil {
			t.Errorf("BindType %d should be valid: %v", bindType, err)
		}
	}
	if _, err := NewClient(append(validClientOptions(), WithClientBindType(2))...); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("BindType=2 error=%v, want ErrInvalidConfig", err)
	}
}
