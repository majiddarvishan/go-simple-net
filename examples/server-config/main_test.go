package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	neta "github.com/majiddarvishan/neta-go"
)

func TestServerConfigOverrideAndRejectUnknown(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(p, []byte(`{"listen_address":"127.0.0.1:5000","max_bound_sessions":2,"protocol_version":"v1"}`), 0600); err != nil { t.Fatal(err) }
	cfg, err := parseServerArgs([]string{"-config", p, "-listen", "127.0.0.1:0", "-max-bound", "3", "-protocol", "v2"})
	if err != nil { t.Fatal(err) }
	if *cfg.ListenAddress != "127.0.0.1:0" || *cfg.MaxBoundSessions != 3 || *cfg.ProtocolVersion != "v2" {
		t.Fatalf("CLI/JSON precedence failed: %+v", cfg)
	}
	opts, err := serverOptions(cfg)
	if err != nil { t.Fatal(err) }
	if _, err := neta.NewServer(opts...); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(`{"not_a_setting":4}`), 0600); err != nil { t.Fatal(err) }
	if _, err := parseServerArgs([]string{"-config", p}); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict JSON unknown key failure: %v", err)
	}
}

func TestServerConfigInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"-protocol", "v3"},
		{"-frame-timeout", "unexpected"},
		{"-max-bound", "0"},
		{"-app-concurrency", "99"},
		{"-onclose-credits", "0"},
	} {
		cfg, err := parseServerArgs(args)
		if err != nil { continue }
		opts, err := serverOptions(cfg)
		if err == nil { _, err = neta.NewServer(opts...) }
		if err == nil { t.Errorf("expected invalid %v to fail", args) }
	}
}

func TestServerDefaultOptions(t *testing.T) {
	cfg, err := parseServerArgs(nil)
	if err != nil { t.Fatal(err) }
	if cfg.ProtocolVersion != nil || cfg.ApplicationWorkers != nil || cfg.MaxBoundSessions != nil {
		t.Fatalf("unset options should use library defaults: %+v", cfg)
	}
	opts, err := serverOptions(cfg)
	if err != nil { t.Fatal(err) }
	if _, err := neta.NewServer(opts...); err != nil { t.Fatal(err) }
}
