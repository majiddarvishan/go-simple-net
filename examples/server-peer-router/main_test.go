package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func TestConfigDefaultJSONCLIAndValidation(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Peers) != 2 || cfg.Routes["east"] != "peer-east-01" {
		t.Fatalf("unexpected default routing: %+v", cfg)
	}
	name := filepath.Join(t.TempDir(), "server.json")
	content := []byte(`{"listen_address":"127.0.0.1:0","max_bound_sessions":3,"max_inflight_dispatches":1}`)
	if err := os.WriteFile(name, content, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = parseConfig([]string{"-config", name, "-max-bound", "4", "-max-inflight", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxBoundSessions != 4 || cfg.MaxInflight != 2 || cfg.ListenAddress != "127.0.0.1:0" {
		t.Fatalf("CLI did not override JSON: %+v", cfg)
	}
	if err := os.WriteFile(name, []byte(`{"unrecognized":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseConfig([]string{"-config", name}); err == nil {
		t.Fatal("unknown JSON field must be rejected")
	}
}

func TestConfigRejectsUnsafeOrUnroutablePeers(t *testing.T) {
	cases := []struct {
		name string
		edit func(*demoConfig)
	}{
		{"duplicate bind", func(c *demoConfig) { c.Peers[1].BindSystemID = c.Peers[0].BindSystemID }},
		{"duplicate peer", func(c *demoConfig) { c.Peers[1].ID = c.Peers[0].ID }},
		{"unknown route", func(c *demoConfig) { c.Routes["east"] = "not-a-peer" }},
		{"invalid route", func(c *demoConfig) { c.Routes["east west"] = "peer-east-01" }},
		{"overlong bind", func(c *demoConfig) { c.Peers[0].BindSystemID = "this-exceeds-20-bytes-peer" }},
		{"too few slots", func(c *demoConfig) { c.MaxBoundSessions = 1 }},
		{"zero dispatch", func(c *demoConfig) { c.MaxInflight = 0 }},
		{"bad request timeout", func(c *demoConfig) { c.RequestTimeout = "0s" }},
		{"oversubscribed workers", func(c *demoConfig) { c.ApplicationWorkers = 513 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaults()
			tt.edit(&cfg)
			if err := validateConfig(cfg); err == nil {
				t.Fatal("expected config validation error")
			}
		})
	}
}

func connectTestPeer(t *testing.T, addr, bindID string) *neta.Client {
	t.Helper()
	client, err := neta.NewClient(
		neta.WithClientAddress(addr),
		neta.WithClientSystemID(bindID),
		neta.WithClientBindType(neta.BindTypeBiDirectional),
		neta.WithClientOnRequestHandler(func(s *neta.Session, request neta.Request) {
			_ = s.SendResponse(request.Sequence, append([]byte("ack:"), request.Payload...), 0)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect %s: %v", bindID, err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Stop(ctx)
	})
	return client
}

func waitPeerOnline(t *testing.T, registry *peerRegistry, route string) *neta.Session {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, session, err := registry.lookup(route); err == nil {
			return session
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %q did not get a Bound peer Session", route)
	return nil
}

func TestTwoPeersBoundAndRoutedWithoutFallback(t *testing.T) {
	cfg := defaults()
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.RequestTimeout = "250ms"
	cfg.MaxInflight = 1
	registry := newPeerRegistry(cfg)
	srv, err := buildServer(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})
	addr := srv.Addr().String()
	a := connectTestPeer(t, addr, "peer-east")
	_ = connectTestPeer(t, addr, "peer-west")
	east := waitPeerOnline(t, registry, "east")
	west := waitPeerOnline(t, registry, "west")
	if east == west {
		t.Fatal("two peer routes selected the same Session")
	}
	if _, _, err := registry.lookup("missing"); !errors.Is(err, errUnknownRoute) {
		t.Fatalf("missing route: got %v", err)
	}
	if len(registry.peers()) != 2 {
		t.Fatal("registry must list both known peers")
	}

	completion := make(chan struct {
		response neta.Response
		err error
	}, 1)
	release := make(chan struct{})
	peerID, seq, err := registry.dispatch("east", []byte("from-peer-router"), func(response neta.Response, resultErr error) {
		defer func() { <-release }()
		completion <- struct {
			response neta.Response
			err error
		}{response, resultErr}
	})
	if err != nil || seq == 0 || peerID != "peer-east-01" {
		t.Fatalf("dispatch got peer=%q seq=%d err=%v", peerID, seq, err)
	}
	// The first Completion retains the bounded global in-flight credit until
	// the application callback returns; another route must not bypass it.
	if _, _, err := registry.dispatch("west", []byte("second"), func(neta.Response, error) {}); !errors.Is(err, errDispatchBusy) {
		t.Fatalf("bounded inflight: got %v", err)
	}
	select {
	case got := <-completion:
		if got.err != nil || got.response.Sequence != seq || string(got.response.Payload) != "ack:from-peer-router" {
			t.Fatalf("unexpected bidirectional Completion: %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bidirectional Request did not receive its Response")
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_ = a.Stop(ctx)
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := registry.lookup("east"); errors.Is(err, errPeerOffline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, _, err := registry.lookup("east"); !errors.Is(err, errPeerOffline) {
		t.Fatalf("offline route must fail closed, got %v", err)
	}
	if _, _, err := registry.lookup("west"); err != nil {
		t.Fatalf("unrelated peer must remain available: %v", err)
	}
}

func TestBindRejectsUnknownAndUnidirectionalPeers(t *testing.T) {
	registry := newPeerRegistry(defaults())
	if err := registry.validateBind(neta.BindInfo{SystemID: "unknown", BindType: uint8(neta.BindTypeBiDirectional)}); err == nil {
		t.Fatal("unknown identity accepted")
	}
	if err := registry.validateBind(neta.BindInfo{SystemID: "peer-east", BindType: uint8(neta.BindTypeUniDirectional)}); err == nil {
		t.Fatal("unidirectional peer accepted")
	}
	if err := registry.validateBind(neta.BindInfo{SystemID: "peer-east", BindType: uint8(neta.BindTypeBiDirectional)}); err != nil {
		t.Fatal(err)
	}
}
