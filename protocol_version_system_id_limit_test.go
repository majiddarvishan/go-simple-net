package neta

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClientSystemIDLimitByProtocolVersion(t *testing.T) {
	systemID19 := strings.Repeat("a", 19)
	systemID20 := strings.Repeat("b", 20)
	systemID21 := strings.Repeat("c", 21)
	for _, tc := range []struct {
		name    string
		id      string
		version ProtocolVersion
		valid   bool
	}{
		{name: "v1_19", id: systemID19, version: ProtocolVersionV1, valid: true},
		{name: "v1_20", id: systemID20, version: ProtocolVersionV1, valid: false},
		{name: "v2_19", id: systemID19, version: ProtocolVersionV2, valid: true},
		{name: "v2_20", id: systemID20, version: ProtocolVersionV2, valid: true},
		{name: "v2_21", id: systemID21, version: ProtocolVersionV2, valid: false},
		{name: "v2_nul", id: "abc\x00def", version: ProtocolVersionV2, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(
				WithClientAddress("127.0.0.1:2775"),
				WithClientSystemID(tc.id),
				WithClientSessionOptions(WithProtocolVersion(tc.version)),
			)
			if tc.valid {
				if err != nil || client == nil {
					t.Fatalf("NewClient(%s): client=%v, err=%v", tc.name, client, err)
				}
				return
			}
			requireConfigError(t, err, "system_id")
		})
	}
	if _, err := NewClient(
		WithClientAddress("127.0.0.1:2775"),
		WithClientSystemID(systemID20),
	); err != nil {
		t.Fatalf("default V2 must accept 20-byte SystemID: %v", err)
	}
	if _, err := NewClient(
		WithClientAddress("127.0.0.1:2775"),
		WithClientSystemID(systemID21),
	); err == nil {
		t.Fatal("default V2 accepted 21-byte SystemID")
	} else {
		requireConfigError(t, err, "system_id")
	}
}

func TestV2SystemIDTwentyBytesRoundTripOverTCP(t *testing.T) {
	systemID20 := strings.Repeat("z", 20)
	bound := make(chan string, 1)
	server, err := NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerOnBoundHandler(func(session *Session) {
			if session.ProtocolVersion() == ProtocolVersionV2 {
				bound <- session.SystemID()
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() { _ = server.Stop(ctx) }()

	client, err := NewClient(
		WithClientAddress(server.Addr().String()),
		WithClientSystemID(systemID20),
	)
	if err != nil {
		t.Fatalf("V2 NewClient with 20 bytes: %v", err)
	}
	defer func() { _ = client.Stop(ctx) }()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("V2 Client with 20-byte SystemID failed to Bind: %v", err)
	}
	session, ok := client.CurrentSession()
	if !ok || session.ProtocolVersion() != ProtocolVersionV2 {
		t.Fatalf("V2 Client did not publish a V2 Session: bound=%v", ok)
	}
	select {
	case got := <-bound:
		if got != systemID20 {
			t.Fatalf("Server observed SystemID = %q, want %q", got, systemID20)
		}
	case <-ctx.Done():
		t.Fatal("Server did not expose 20-byte V2 Bind identity")
	}
}
