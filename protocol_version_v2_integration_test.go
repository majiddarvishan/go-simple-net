package neta

import (
	"context"
	"testing"
	"time"
)

func TestSessionSelectsBindVersionFromClient(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version ProtocolVersion
	}{
		{name: "legacy_v1", version: ProtocolVersionV1},
		{name: "new_v2", version: ProtocolVersionV2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selected := make(chan ProtocolVersion, 1)
			server, err := NewServer(
				WithServerListenAddress("127.0.0.1:0"),
				WithServerOnBoundHandler(func(session *Session) {
					selected <- session.ProtocolVersion()
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
				WithClientSystemID("v-test"),
				WithClientSessionOptions(WithProtocolVersion(tt.version)),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Stop(ctx) }()
			if err := client.Connect(ctx); err != nil {
				t.Fatalf("Connect(%s): %v", tt.name, err)
			}
			session, ok := client.CurrentSession()
			if !ok || session.ProtocolVersion() != tt.version {
				t.Fatalf("Client selected unexpected version (session=%v)", ok)
			}
			select {
			case got := <-selected:
				if got != tt.version {
					t.Fatalf("Server selected %d, want %d", got, tt.version)
				}
			case <-ctx.Done():
				t.Fatal("Server OnBound was not observed")
			}
		})
	}
}

func TestV1OnlyServerRejectsV2ClientBind(t *testing.T) {
	server, err := NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerSessionOptions(WithProtocolVersion(ProtocolVersionV1)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = server.Stop(ctx) }()
	client, err := NewClient(
		WithClientAddress(server.Addr().String()),
		WithClientSystemID("v2-client"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Stop(ctx) }()
	if err := client.Connect(ctx); err == nil {
		t.Fatal("V1-only Server accepted a V2 Bind")
	}
}
