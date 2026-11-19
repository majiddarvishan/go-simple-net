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
	if len(cfg.MessageEngines) != 2 || cfg.Routes["east"] != "me-east-01" {
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

func TestConfigRejectsUnsafeOrUnroutableMEs(t *testing.T) {
	cases := []struct {
		name string
		edit func(*demoConfig)
	}{
		{"duplicate bind", func(c *demoConfig) { c.MessageEngines[1].BindSystemID = c.MessageEngines[0].BindSystemID }},
		{"duplicate ME", func(c *demoConfig) { c.MessageEngines[1].ID = c.MessageEngines[0].ID }},
		{"unknown route", func(c *demoConfig) { c.Routes["east"] = "not-a-me" }},
		{"invalid route", func(c *demoConfig) { c.Routes["east west"] = "me-east-01" }},
		{"overlong bind", func(c *demoConfig) { c.MessageEngines[0].BindSystemID = "this-exceeds-20-bytes-me" }},
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

func connectTestME(t *testing.T, addr, bindID string) *neta.Client {
	t.Helper()
	client, err := neta.NewClient(
		neta.WithClientAddress(addr),
		neta.WithClientSystemID(bindID),
		neta.WithClientBindType(neta.BindTypeBiDirectional),
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

func waitMEOnline(t *testing.T, registry *meRegistry, route string) *neta.Session {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, session, err := registry.lookup(route); err == nil {
			return session
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %q did not get a Bound ME Session", route)
	return nil
}

func TestTwoMEsBoundAndRoutedWithoutFallback(t *testing.T) {
	cfg := defaults()
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.RequestTimeout = "250ms"
	cfg.MaxInflight = 1
	registry := newMERegistry(cfg)
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
	a := connectTestME(t, addr, "me-east")
	_ = connectTestME(t, addr, "me-west")
	east := waitMEOnline(t, registry, "east")
	west := waitMEOnline(t, registry, "west")
	if east == west {
		t.Fatal("two ME routes selected the same Session")
	}
	if _, _, err := registry.lookup("missing"); !errors.Is(err, errUnknownRoute) {
		t.Fatalf("missing route: got %v", err)
	}
	if len(registry.peers()) != 2 {
		t.Fatal("registry must list both known MEs")
	}

	completion := make(chan error, 1)
	meID, seq, err := registry.dispatch("east", []byte("from-smpp-internal-router"), func(_ neta.Response, e error) {
		completion <- e
	})
	if err != nil || seq == 0 || meID != "me-east-01" {
		t.Fatalf("dispatch got me=%q seq=%d err=%v", meID, seq, err)
	}
	// Client-side inbound Request delivery is NOT available in current
	// neta-go public API. Thus success means local admission only; the
	// completion must time out rather than invent an application ACK.
	if _, _, err := registry.dispatch("west", []byte("second"), func(neta.Response, error) {}); !errors.Is(err, errDispatchBusy) {
		t.Fatalf("bounded inflight: got %v", err)
	}
	select {
	case got := <-completion:
		if !errors.Is(got, neta.ErrRequestTimeout) {
			t.Fatalf("unexpected completion: %v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed-out unhandled Request was not settled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_ = a.Stop(ctx)
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := registry.lookup("east"); errors.Is(err, errMEOffline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, _, err := registry.lookup("east"); !errors.Is(err, errMEOffline) {
		t.Fatalf("offline route must fail closed, got %v", err)
	}
	if _, _, err := registry.lookup("west"); err != nil {
		t.Fatalf("unrelated ME must remain available: %v", err)
	}
}

func TestBindRejectsUnknownAndUnidirectionalMEs(t *testing.T) {
	registry := newMERegistry(defaults())
	if err := registry.validateBind(neta.BindInfo{SystemID: "unknown", BindType: uint8(neta.BindTypeBiDirectional)}); err == nil {
		t.Fatal("unknown identity accepted")
	}
	if err := registry.validateBind(neta.BindInfo{SystemID: "me-east", BindType: uint8(neta.BindTypeUniDirectional)}); err == nil {
		t.Fatal("unidirectional ME accepted")
	}
	if err := registry.validateBind(neta.BindInfo{SystemID: "me-east", BindType: uint8(neta.BindTypeBiDirectional)}); err != nil {
		t.Fatal(err)
	}
}
