package main

import (
	"context"
	"testing"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func TestServerV1RejectsPositionalArgs(t *testing.T) {
	if err := run([]string{"unexpected"}); err == nil {
		t.Fatal("expected argument rejection")
	}
}

func TestV1OnlyServerRejectsV2ClientWithoutIdentityCollision(t *testing.T) {
	server, err := neta.NewServer(
		neta.WithServerListenAddress("127.0.0.1:0"),
		neta.WithServerSessionOptions(neta.WithProtocolVersion(neta.ProtocolVersionV1)),
	)
	if err != nil { t.Fatal(err) }
	if err := server.Start(); err != nil { t.Fatal(err) }
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil { t.Errorf("Stop: %v", err) }
	}()
	for _, tc := range []struct { version neta.ProtocolVersion; id string; valid bool }{
		{neta.ProtocolVersionV1, "v1-accepted", true},
		{neta.ProtocolVersionV2, "v2-rejected", false},
	} {
		client, err := neta.NewClient(
			neta.WithClientAddress(server.Addr().String()),
			neta.WithClientSystemID(tc.id),
			neta.WithClientSessionOptions(neta.WithProtocolVersion(tc.version)),
		)
		if err != nil { t.Fatal(err) }
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = client.Connect(ctx)
		cancel()
		if (err==nil) != tc.valid {
			t.Errorf("version=%d id=%q expected accepted=%v got error=%v", tc.version, tc.id, tc.valid, err)
		}
		stopCtx,stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Stop(stopCtx); err != nil { t.Logf("Client.Stop after Bind result: %v", err) }
		stopCancel()
	}
}
