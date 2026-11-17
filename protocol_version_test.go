package neta

import "testing"

func TestProtocolVersionDefaultsToV2(t *testing.T) {
	if got := defaultSessionConfig().protocolVersion; got != ProtocolVersionV2 {
		t.Fatalf("default Session protocol version = %d, want V2", got)
	}
	client, err := NewClient(
		WithClientAddress("127.0.0.1:2775"),
		WithClientSystemID("version-client"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := client.config.session.protocolVersion; got != ProtocolVersionV2 {
		t.Fatalf("Client default version = %d, want V2", got)
	}
	server, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if got := server.config.session.protocolVersion; got != ProtocolVersionV2 {
		t.Fatalf("Server default version = %d, want V2", got)
	}
}

func TestProtocolVersionExplicitV1AndSessionImmutability(t *testing.T) {
	client, err := NewClient(
		WithClientAddress("127.0.0.1:2775"),
		WithClientSystemID("version-client"),
		WithClientSessionOptions(WithProtocolVersion(ProtocolVersionV1)),
	)
	if err != nil {
		t.Fatalf("NewClient(V1): %v", err)
	}
	server, err := NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerSessionOptions(WithProtocolVersion(ProtocolVersionV1)),
	)
	if err != nil {
		t.Fatalf("NewServer(V1): %v", err)
	}
	if client.config.session.protocolVersion != ProtocolVersionV1 ||
		server.config.session.protocolVersion != ProtocolVersionV1 {
		t.Fatal("explicit V1 not inherited by Client and Server")
	}
	config := client.config.session
	session := newSession(nil, config, sessionIdentity{role: sessionRoleClient}, nil, sessionHandlers{})
	if got := session.ProtocolVersion(); got != ProtocolVersionV1 {
		t.Fatalf("Session ProtocolVersion = %d, want V1", got)
	}
	config.protocolVersion = ProtocolVersionV2
	if got := session.ProtocolVersion(); got != ProtocolVersionV1 {
		t.Fatalf("Session ProtocolVersion changed after creation: %d", got)
	}
}

func TestProtocolVersionValidationAndLastOptionWins(t *testing.T) {
	for _, version := range []ProtocolVersion{0, 3, 255} {
		_, err := NewClient(
			WithClientAddress("127.0.0.1:2775"),
			WithClientSystemID("version-client"),
			WithClientSessionOptions(WithProtocolVersion(version)),
		)
		requireConfigError(t, err, "protocol_version")
		_, err = NewServer(
			WithServerListenAddress("127.0.0.1:0"),
			WithServerSessionOptions(WithProtocolVersion(version)),
		)
		requireConfigError(t, err, "protocol_version")
	}
	client, err := NewClient(
		WithClientAddress("127.0.0.1:2775"),
		WithClientSystemID("version-client"),
		WithClientSessionOptions(
			WithProtocolVersion(0),
			WithProtocolVersion(ProtocolVersionV1),
		),
	)
	if err != nil {
		t.Fatalf("last protocol-version option should win: %v", err)
	}
	if got := client.config.session.protocolVersion; got != ProtocolVersionV1 {
		t.Fatalf("last version = %d, want V1", got)
	}
}
