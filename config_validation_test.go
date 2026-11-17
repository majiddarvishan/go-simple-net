package neta

import (
	"errors"
	"testing"
	"time"
)

func TestTCPEndpointSyntaxValidation(t *testing.T) {
	for _, address := range []string{"missing-port", "host:", ":9000", "[::1]9000", "2001:db8::1:9000"} {
		t.Run("client/"+address, func(t *testing.T) {
			_, err := NewClient(WithClientAddress(address))
			if err == nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid TCP endpoint %q, got %v", address, err)
			}
		})
	}
	for _, address := range []string{"missing-port", "host:"} {
		t.Run("server/"+address, func(t *testing.T) {
			_, err := NewServer(WithServerListenAddress(address))
			if err == nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid TCP endpoint %q, got %v", address, err)
			}
		})
	}
	for _, address := range []string{"127.0.0.1:9000", "localhost:9000", "[::1]:9000"} {
		if _, err := NewClient(WithClientAddress(address)); err != nil {
			t.Errorf("valid Client endpoint %q rejected: %v", address, err)
		}
	}
	for _, address := range []string{"127.0.0.1:0", ":0", "[::]:9000"} {
		if _, err := NewServer(WithServerListenAddress(address)); err != nil {
			t.Errorf("valid Server listen endpoint %q rejected: %v", address, err)
		}
	}
}

func TestAllRequiredPositiveLimitsRejectZero(t *testing.T) {
	clientCases := []struct {
		name  string
		opt   ClientOption
		field string
	}{
		{"application_workers", WithClientApplicationWorkers(0), "application_workers_per_owner"},
		{"completion_workers", WithClientCompletionWorkers(0), "completion_workers_per_owner"},
		{"onclose_workers", WithClientOnCloseWorkers(0), "onclose_workers_per_owner"},
		{"onclose_credits", WithClientOnCloseEventCredits(0), "onclose_event_credits_per_owner"},
		{"owner_requests", WithClientMaxOutstandingRequests(0), "max_outstanding_requests_per_owner"},
	}
	for _, test := range clientCases {
		t.Run("client/"+test.name, func(t *testing.T) {
			_, err := NewClient(WithClientAddress("127.0.0.1:9000"), test.opt)
			requireConfigError(t, err, test.field)
		})
	}

	sessionCases := []struct {
		name  string
		opt   SessionOption
		field string
	}{
		{"application_concurrency", WithApplicationCallbackConcurrency(0), "application_callback_concurrency"},
		{"completion_concurrency", WithCompletionCallbackConcurrency(0), "completion_callback_concurrency"},
		{"incoming_frames", WithMaxIncomingApplicationFrames(0), "max_incoming_application_frames"},
		{"outgoing_frames", WithMaxOutgoingApplicationFrames(0), "max_outgoing_application_frames"},
		{"session_requests", WithMaxOutstandingRequestsPerSession(0), "max_outstanding_requests_per_session"},
		{"outgoing_bytes", WithMaxQueuedOutgoingBytes(0), "max_queued_outgoing_bytes"},
		{"incoming_bytes", WithMaxQueuedIncomingBytes(0), "max_queued_incoming_bytes"},
		{"control_bytes", WithMaxQueuedControlBytes(0), "max_queued_control_bytes"},
	}
	for _, test := range sessionCases {
		t.Run("session/"+test.name, func(t *testing.T) {
			_, err := NewClient(
				WithClientAddress("127.0.0.1:9000"),
				WithClientSessionOptions(test.opt),
			)
			requireConfigError(t, err, test.field)
		})
	}

	serverCases := []struct {
		name  string
		opt   ServerOption
		field string
	}{
		{"bound_sessions", WithServerMaxBoundSessions(0), "max_bound_sessions"},
		{"pending_prebind", WithServerMaxPendingPreBindSessions(0), "max_pending_prebind_sessions"},
		{"aggregate_queue_bytes", WithServerMaxAggregateQueuedApplicationBytes(0), "max_aggregate_queued_application_bytes"},
		{"aggregate_queue_frames", WithServerMaxAggregateQueuedApplicationFrames(0), "max_aggregate_queued_application_frames"},
	}
	for _, test := range serverCases {
		t.Run("server/"+test.name, func(t *testing.T) {
			_, err := NewServer(WithServerListenAddress("127.0.0.1:0"), test.opt)
			requireConfigError(t, err, test.field)
		})
	}
}

func TestAllMandatoryTimeoutsRejectZero(t *testing.T) {
	clientCases := []struct {
		name  string
		opt   ClientOption
		field string
	}{
		{"connect", WithConnectTimeout(0), "connect_timeout"},
		{"bind_response", WithBindResponseTimeout(0), "bind_response_timeout"},
	}
	for _, test := range clientCases {
		t.Run("client/"+test.name, func(t *testing.T) {
			_, err := NewClient(WithClientAddress("127.0.0.1:9000"), test.opt)
			requireConfigError(t, err, test.field)
		})
	}

	sessionCases := []struct {
		name  string
		opt   SessionOption
		field string
	}{
		{"request", WithRequestTimeout(0), "request_timeout"},
		{"frame_assembly", WithFrameAssemblyTimeout(0), "frame_assembly_timeout"},
		{"unbind", WithUnbindTimeout(0), "unbind_timeout"},
		{"write", WithWriteTimeout(0), "write_timeout"},
		{"enquire_link_response", WithEnquireLinkResponseTimeout(0), "enquire_link_response_timeout"},
	}
	for _, test := range sessionCases {
		t.Run("session/"+test.name, func(t *testing.T) {
			_, err := NewClient(
				WithClientAddress("127.0.0.1:9000"),
				WithClientSessionOptions(test.opt),
			)
			requireConfigError(t, err, test.field)
		})
	}
}

func TestEachWorkerIdleGraceHasIndependentInclusiveRange(t *testing.T) {
	valid := []struct {
		name string
		opt  ClientOption
	}{
		{"application_min", WithClientApplicationWorkerIdleGrace(time.Millisecond)},
		{"completion_min", WithClientCompletionWorkerIdleGrace(time.Millisecond)},
		{"onclose_min", WithClientOnCloseWorkerIdleGrace(time.Millisecond)},
		{"application_max", WithClientApplicationWorkerIdleGrace(10 * time.Minute)},
		{"completion_max", WithClientCompletionWorkerIdleGrace(10 * time.Minute)},
		{"onclose_max", WithClientOnCloseWorkerIdleGrace(10 * time.Minute)},
	}
	for _, test := range valid {
		t.Run("valid/"+test.name, func(t *testing.T) {
			if _, err := NewClient(WithClientAddress("127.0.0.1:9000"), test.opt); err != nil {
				t.Fatalf("inclusive idle grace boundary rejected: %v", err)
			}
		})
	}
	invalid := []struct {
		name string
		opt  ClientOption
	}{
		{"application_zero", WithClientApplicationWorkerIdleGrace(0)},
		{"completion_zero", WithClientCompletionWorkerIdleGrace(0)},
		{"onclose_zero", WithClientOnCloseWorkerIdleGrace(0)},
		{"application_over", WithClientApplicationWorkerIdleGrace(10*time.Minute + 1)},
		{"completion_over", WithClientCompletionWorkerIdleGrace(10*time.Minute + 1)},
		{"onclose_over", WithClientOnCloseWorkerIdleGrace(10*time.Minute + 1)},
	}
	for _, test := range invalid {
		t.Run("invalid/"+test.name, func(t *testing.T) {
			_, err := NewClient(WithClientAddress("127.0.0.1:9000"), test.opt)
			if err == nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid idle grace, got %v", err)
			}
		})
	}
}

func TestSessionOptionSliceIsSnapshotted(t *testing.T) {
	options := []SessionOption{WithRequestTimeout(45 * time.Second)}
	clientOption := WithClientSessionOptions(options...)
	options[0] = WithRequestTimeout(0)

	client, err := NewClient(WithClientAddress("127.0.0.1:9000"), clientOption)
	if err != nil {
		t.Fatalf("mutating caller option slice changed the captured options: %v", err)
	}
	if client.config.session.requestTimeout != 45*time.Second {
		t.Fatalf("request timeout = %s, want 45s", client.config.session.requestTimeout)
	}
}

func TestOwnerAndSessionBudgetsDoNotImposeProductConstraints(t *testing.T) {
	_, err := NewServer(
		WithServerListenAddress(":0"),
		WithServerMaxAggregateQueuedApplicationBytes(1),
		WithServerMaxAggregateQueuedApplicationFrames(1),
		WithServerMaxOutstandingRequests(1),
		WithServerMaxRetainedApplicationBytes(10),
		WithServerMaxRetainedCompletionBytes(10),
		WithServerApplicationWorkers(1),
		WithServerSessionOptions(
			WithMaxQueuedIncomingBytes(4<<20),
			WithMaxIncomingApplicationFrames(1024),
			WithMaxOutstandingRequestsPerSession(512),
			WithMaxRetainedApplicationBytesPerSession(64<<20),
			WithMaxRetainedCompletionBytesPerSession(64<<20),
			WithApplicationCallbackConcurrency(32),
		),
	)
	if err != nil {
		t.Fatalf("independent aggregate and Session budgets must not be rejected by product constraints: %v", err)
	}
}
