package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	neta "github.com/majiddarvishan/neta-go"
)

func TestClientConfigPrecedenceAndStrictJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.json")
	if err := os.WriteFile(path, []byte(`{"address":"127.0.0.1:5555","system_id":"json-id","protocol_version":"v1","auto_reconnect":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := parseClientArgs([]string{"-config", path, "-addr", "127.0.0.1:6666", "-system-id", "cli-id", "-protocol", "v2", "-auto-reconnect=false"})
	if err != nil {
		t.Fatal(err)
	}
	if *c.Address != "127.0.0.1:6666" || *c.SystemID != "cli-id" || *c.ProtocolVersion != "v2" || *c.AutoReconnect {
		t.Fatalf("CLI overrides JSON incorrectly: %+v", c)
	}
	opts, err := clientOptions(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := neta.NewClient(opts...); err != nil {
		t.Fatalf("valid overrides: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"unexpected":42}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseClientArgs([]string{"-config", path}); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict unknown-field rejection, got: %v", err)
	}
}

func TestClientConfigErrors(t *testing.T) {
	for _, tc := range [][]string{
		{"-protocol", "v3"},
		{"-request-timeout", "bad-duration"},
		{"-system-id", strings.Repeat("x", 21)},
		{"-protocol", "v1", "-system-id", strings.Repeat("x", 20)},
		{"-max-outstanding", "0"},
	} {
		c, err := parseClientArgs(tc)
		if err != nil {
			continue
		}
		opts, err := clientOptions(c)
		if err == nil {
			_, err = neta.NewClient(opts...)
		}
		if err == nil {
			t.Errorf("expected failure for %v", tc)
		}
	}
}

func TestClientDefaultsRequireNoOptionalConfig(t *testing.T) {
	c, err := parseClientArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.AutoReconnect != nil || c.ProtocolVersion != nil || c.RequestTimeout != nil {
		t.Fatalf("optional defaults were overridden: %+v", c)
	}
	opts, err := clientOptions(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := neta.NewClient(opts...); err != nil {
		t.Fatal(err)
	}
}
