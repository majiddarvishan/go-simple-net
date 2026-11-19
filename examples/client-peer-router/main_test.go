package main

import (
	"context"
	"testing"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func TestPeerClientFlags(t *testing.T) {
	cfg, err := parseClientArgs([]string{"-addr", "127.0.0.1:10001", "-system-id", "worker-a", "-window", "2s", "-greeting", ""})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.address != "127.0.0.1:10001" || cfg.systemID != "worker-a" || cfg.window != 2*time.Second || cfg.greeting != "" {
		t.Fatalf("parsed args: %+v", cfg)
	}
	if _, err := parseClientArgs([]string{"-window", "0s"}); err == nil {
		t.Fatal("zero window must fail")
	}
	if _, err := parseClientArgs([]string{"unexpected"}); err == nil {
		t.Fatal("positional arguments must fail")
	}
}

func TestPeerClientHandlesServerOriginatedRequest(t *testing.T) {
	bound := make(chan *neta.Session, 1)
	server, err := neta.NewServer(
		neta.WithServerListenAddress("127.0.0.1:0"),
		neta.WithServerOnBoundHandler(func(s *neta.Session) { bound <- s }),
		neta.WithServerOnRequestHandler(func(s *neta.Session, req neta.Request) {
			_ = s.SendResponse(req.Sequence, []byte("echo:"+string(req.Payload)), 0)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	clientDone := make(chan error, 1)
	go func() {
		clientDone <- run(ctx, []string{
			"-addr", server.Addr().String(),
			"-system-id", "worker-a", "-window", "2s",
		})
	}()
	var session *neta.Session
	select {
	case session = <-bound:
	case <-ctx.Done():
		t.Fatal("client never became Bound")
	}
	responseCh := make(chan struct {
		response neta.Response
		err error
	}, 1)
	seq, err := session.SendRequest([]byte("from-server"), func(resp neta.Response, e error) {
		responseCh <- struct {
			response neta.Response
			err error
		}{resp, e}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-responseCh:
		if result.err != nil || result.response.Sequence != seq ||
			result.response.Status != 0 || string(result.response.Payload) != "ack:from-server" {
			t.Fatalf("bad peer Completion: %+v", result)
		}
	case <-ctx.Done():
		t.Fatal("Server did not receive Client's Response")
	}
	select {
	case err := <-clientDone:
		if err != nil {
			t.Fatalf("client completion: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("client did not shut down after bounded window")
	}
}
