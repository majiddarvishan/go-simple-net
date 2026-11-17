package neta

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func requireConfigError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected configuration error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("errors.Is(err, ErrInvalidConfig) = false: %v", err)
	}
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("errors.As(err, *ConfigError) = false: %T", err)
	}
	if configErr.Field() != field {
		t.Fatalf("field = %q, want %q (error: %v)", configErr.Field(), field, err)
	}
}

func TestApprovedDefaults(t *testing.T) {
	session := defaultSessionConfig()
	if session.requestTimeout != 30*time.Second ||
		session.frameAssemblyTimeout != 30*time.Second ||
		session.unbindTimeout != 10*time.Second ||
		session.writeTimeout != 30*time.Second ||
		session.enquireLinkIdleInterval != 30*time.Second ||
		session.enquireLinkResponseTimeout != 10*time.Second {
		t.Fatalf("unexpected Session timeout defaults: %+v", session)
	}
	if session.maxQueuedOutgoingBytes != 4<<20 ||
		session.maxQueuedIncomingBytes != 4<<20 ||
		session.maxQueuedControlBytes != 4<<10 {
		t.Fatalf("unexpected Session queue-byte defaults: %+v", session)
	}
	if session.applicationConcurrency != 1 || session.completionConcurrency != 1 ||
		session.maxIncomingApplicationFrames != 256 || session.maxOutgoingApplicationFrames != 256 ||
		session.maxOutstandingRequests != 128 {
		t.Fatalf("unexpected Session count defaults: %+v", session)
	}
	if session.maxRetainedApplicationBytes != 16<<20 || session.maxRetainedCompletionBytes != 16<<20 {
		t.Fatalf("unexpected Session retained-byte defaults: %+v", session)
	}

	client := defaultClientConfig()
	if client.connectTimeout != 30*time.Second || client.bindResponseTimeout != 30*time.Second || client.autoReconnect {
		t.Fatalf("unexpected Client defaults: %+v", client)
	}
	owner := client.owner
	if owner.onCloseEventCredits != 1024 || owner.maxOutstandingRequests != 32768 ||
		owner.maxRetainedApplicationBytes != 512<<20 || owner.maxRetainedCompletionBytes != 256<<20 {
		t.Fatalf("unexpected owner defaults: %+v", owner)
	}
	if owner.applicationWorkerIdleGrace != 30*time.Second ||
		owner.completionWorkerIdleGrace != 30*time.Second ||
		owner.onCloseWorkerIdleGrace != 30*time.Second {
		t.Fatalf("unexpected worker idle grace defaults: %+v", owner)
	}

	server := defaultServerConfig()
	if server.preBindHandshakeTimeout != 30*time.Second ||
		server.maxBoundSessions != 1024 ||
		server.maxPendingPreBindSessions != 128 ||
		server.maxAggregateQueuedApplicationBytes != 256<<20 ||
		server.maxAggregateQueuedApplicationFrames != 65536 {
		t.Fatalf("unexpected Server defaults: %+v", server)
	}
}

func TestDefaultWorkerCounts(t *testing.T) {
	tests := []struct {
		procs int
		app   uint32
		comp  uint32
		close uint32
	}{
		{0, 4, 2, 2},
		{1, 4, 2, 2},
		{2, 8, 4, 2},
		{4, 16, 8, 4},
		{8, 32, 16, 8},
		{16, 64, 32, 16},
		{31, 124, 62, 31},
		{32, 128, 64, 32},
		{1024, 128, 64, 32},
	}
	for _, test := range tests {
		app, comp, closeWorkers := defaultWorkerCounts(test.procs)
		if app != test.app || comp != test.comp || closeWorkers != test.close {
			t.Errorf("P=%d: got (%d,%d,%d), want (%d,%d,%d)", test.procs, app, comp, closeWorkers, test.app, test.comp, test.close)
		}
	}
}

func TestConstructorsRequireEndpointsAndDoNotOpenNetworking(t *testing.T) {
	if _, err := newTestClient(); err == nil {
		t.Fatal("NewClient without an endpoint must fail")
	} else {
		requireConfigError(t, err, "address")
	}
	if _, err := NewServer(); err == nil {
		t.Fatal("NewServer without a listen address must fail")
	} else {
		requireConfigError(t, err, "listen_address")
	}
	client, err := newTestClient(WithClientAddress("127.0.0.1:9000"))
	if err != nil || client == nil {
		t.Fatalf("NewClient: client=%v err=%v", client, err)
	}
	server, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil || server == nil {
		t.Fatalf("NewServer: server=%v err=%v", server, err)
	}
}

func TestEnquireLinkIdleZeroIsTheOnlyDurationZeroException(t *testing.T) {
	_, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(WithEnquireLinkIdleInterval(0)),
	)
	if err != nil {
		t.Fatalf("explicit zero idle interval should disable probes: %v", err)
	}
	_, err = newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(WithRequestTimeout(0)),
	)
	requireConfigError(t, err, "request_timeout")
	_, err = NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerPreBindHandshakeTimeout(0),
	)
	requireConfigError(t, err, "prebind_handshake_timeout")
}

func TestFunctionalOptionsApplyBeforeValidationAndRemainIndependent(t *testing.T) {
	client, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientApplicationWorkers(maxApplicationWorkers+1),
		WithClientApplicationWorkers(maxApplicationWorkers),
		WithClientSessionOptions(
			WithEnquireLinkIdleInterval(0),
			WithRequestTimeout(45*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("last option should win before final validation: %v", err)
	}
	if client.config.owner.applicationWorkers != maxApplicationWorkers {
		t.Fatalf("worker override = %d", client.config.owner.applicationWorkers)
	}
	if client.config.session.enquireLinkIdleInterval != 0 ||
		client.config.session.requestTimeout != 45*time.Second ||
		client.config.session.enquireLinkResponseTimeout != 10*time.Second {
		t.Fatalf("independent Session options were not preserved: %+v", client.config.session)
	}
}

func TestCrossFieldCapacitiesAreIndependent(t *testing.T) {
	_, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientApplicationWorkers(1),
		WithClientMaxOutstandingRequests(1),
		WithClientMaxRetainedApplicationBytes(10),
		WithClientSessionOptions(
			WithApplicationCallbackConcurrency(32),
			WithMaxOutstandingRequestsPerSession(512),
			WithMaxRetainedApplicationBytesPerSession(64<<20),
		),
	)
	if err != nil {
		t.Fatalf("independent owner and Session ceilings must be accepted: %v", err)
	}
	_, err = NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerMaxAggregateQueuedApplicationFrames(1),
		WithServerSessionOptions(WithMaxIncomingApplicationFrames(1024)),
	)
	if err != nil {
		t.Fatalf("aggregate frame cap may be below theoretical Session capacity: %v", err)
	}
}

func TestNilOptionsAreRejected(t *testing.T) {
	_, err := newTestClient(WithClientAddress("127.0.0.1:9000"), ClientOption{})
	requireConfigError(t, err, "client_options[2]")
	_, err = NewServer(WithServerListenAddress("127.0.0.1:0"), ServerOption{})
	requireConfigError(t, err, "server_options[1]")
	_, err = newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(SessionOption{}),
	)
	requireConfigError(t, err, "session_options[0]")
}

func TestWorker009CHardMaximums(t *testing.T) {
	clientCases := []struct {
		name string
		opts []ClientOption
	}{
		{"application_workers", []ClientOption{WithClientApplicationWorkers(513)}},
		{"completion_workers", []ClientOption{WithClientCompletionWorkers(257)}},
		{"onclose_workers", []ClientOption{WithClientOnCloseWorkers(129)}},
		{"application_idle_grace_low", []ClientOption{WithClientApplicationWorkerIdleGrace(time.Millisecond - 1)}},
		{"application_idle_grace_high", []ClientOption{WithClientApplicationWorkerIdleGrace(10*time.Minute + 1)}},
		{"onclose_credits", []ClientOption{WithClientOnCloseEventCredits(4097)}},
		{"owner_requests", []ClientOption{WithClientMaxOutstandingRequests(131073)}},
		{"owner_application_bytes", []ClientOption{WithClientMaxRetainedApplicationBytes((2 << 30) + 1)}},
		{"owner_completion_bytes", []ClientOption{WithClientMaxRetainedCompletionBytes((1 << 30) + 1)}},
	}
	for _, test := range clientCases {
		t.Run(test.name, func(t *testing.T) {
			options := append([]ClientOption{WithClientAddress("127.0.0.1:9000")}, test.opts...)
			_, err := newTestClient(options...)
			if err == nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid config, got %v", err)
			}
		})
	}

	sessionCases := []struct {
		name string
		opt  SessionOption
	}{
		{"application_concurrency", WithApplicationCallbackConcurrency(33)},
		{"completion_concurrency", WithCompletionCallbackConcurrency(33)},
		{"incoming_frames", WithMaxIncomingApplicationFrames(1025)},
		{"outgoing_frames", WithMaxOutgoingApplicationFrames(1025)},
		{"session_requests", WithMaxOutstandingRequestsPerSession(513)},
		{"session_application_bytes", WithMaxRetainedApplicationBytesPerSession((64 << 20) + 1)},
		{"session_completion_bytes", WithMaxRetainedCompletionBytesPerSession((64 << 20) + 1)},
	}
	for _, test := range sessionCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := newTestClient(
				WithClientAddress("127.0.0.1:9000"),
				WithClientSessionOptions(test.opt),
			)
			if err == nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid config, got %v", err)
			}
		})
	}

	_, err := NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerMaxAggregateQueuedApplicationFrames(262145),
	)
	requireConfigError(t, err, "max_aggregate_queued_application_frames")
}

func TestWorker009CInclusiveBoundaries(t *testing.T) {
	_, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientApplicationWorkers(512),
		WithClientCompletionWorkers(256),
		WithClientOnCloseWorkers(128),
		WithClientApplicationWorkerIdleGrace(10*time.Minute),
		WithClientCompletionWorkerIdleGrace(time.Millisecond),
		WithClientOnCloseEventCredits(4096),
		WithClientMaxOutstandingRequests(131072),
		WithClientMaxRetainedApplicationBytes(2<<30),
		WithClientMaxRetainedCompletionBytes(1<<30),
		WithClientSessionOptions(
			WithApplicationCallbackConcurrency(32),
			WithCompletionCallbackConcurrency(32),
			WithMaxIncomingApplicationFrames(1024),
			WithMaxOutgoingApplicationFrames(1024),
			WithMaxOutstandingRequestsPerSession(512),
			WithMaxRetainedApplicationBytesPerSession(64<<20),
			WithMaxRetainedCompletionBytesPerSession(64<<20),
		),
	)
	if err != nil {
		t.Fatalf("inclusive hard maxima should be accepted: %v", err)
	}
	_, err = NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerMaxAggregateQueuedApplicationFrames(262144),
	)
	if err != nil {
		t.Fatalf("aggregate frame hard maximum should be accepted: %v", err)
	}
}

func TestRetainedByteMinimumIsTenBytes(t *testing.T) {
	_, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientMaxRetainedApplicationBytes(9),
	)
	requireConfigError(t, err, "max_retained_application_bytes_per_owner")
	_, err = newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(WithMaxRetainedCompletionBytesPerSession(9)),
	)
	requireConfigError(t, err, "max_retained_completion_bytes_per_session")
}

func TestQueueByteLimitsArePositiveButIndependentOfFrameMaximum(t *testing.T) {
	_, err := newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(WithMaxQueuedOutgoingBytes(1)),
	)
	if err != nil {
		t.Fatalf("queue byte limits have positive minimum 1 and are not frame-size overrides: %v", err)
	}
	_, err = newTestClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSessionOptions(WithMaxQueuedIncomingBytes(0)),
	)
	requireConfigError(t, err, "max_queued_incoming_bytes")
}

func TestCheckedUint64Arithmetic(t *testing.T) {
	if got, ok := checkedAddUint64(1, 2); !ok || got != 3 {
		t.Fatalf("checked add = (%d,%t)", got, ok)
	}
	if _, ok := checkedAddUint64(math.MaxUint64, 1); ok {
		t.Fatal("checked add must reject overflow")
	}
	if got, ok := checkedSubUint64(3, 2); !ok || got != 1 {
		t.Fatalf("checked sub = (%d,%t)", got, ok)
	}
	if _, ok := checkedSubUint64(0, 1); ok {
		t.Fatal("checked sub must reject underflow")
	}
	if got, ok := checkedMulUint64(7, 8); !ok || got != 56 {
		t.Fatalf("checked mul = (%d,%t)", got, ok)
	}
	if _, ok := checkedMulUint64(math.MaxUint64, 2); ok {
		t.Fatal("checked mul must reject overflow")
	}
}

func TestConfigErrorDiagnostics(t *testing.T) {
	_, err := newTestClient(WithClientAddress(" "), WithConnectTimeout(0))
	requireConfigError(t, err, "address")
	if !strings.Contains(err.Error(), "non-empty host:port") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}
