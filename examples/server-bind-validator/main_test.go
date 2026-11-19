package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func TestAllowlistValidation(t *testing.T) {
	for _, raw := range []string{"", "one,", ",one", "a,a", "123456789012345678901"} {
		if _, err := parseAllowlist(raw); err == nil {
			t.Errorf("expected failure for %q", raw)
		}
	}
	allowed, err := parseAllowlist("one,two")
	if err != nil || len(allowed) != 2 {
		t.Fatalf("unexpected allowlist: %v %v", allowed, err)
	}
}

func TestBindValidatorConfigErrors(t *testing.T) {
	for _, args := range [][]string{{"-protocol=v3"}, {"-allow=a,a"}, {"-protocol=v1", "-allow=12345678901234567890"}, {"garbage"}} {
		if err := run(args); err == nil {
			t.Errorf("expected failure for %v", args)
		}
	}
}

func TestBindRejectsUnauthorizedAndConcurrentDuplicate(t *testing.T) {
	server, err := neta.NewServer(
		neta.WithServerListenAddress("127.0.0.1:0"),
		neta.WithBindValidator(func(info neta.BindInfo) error {
			if info.SystemID != "approved" { return fmt.Errorf("demo identity denied") }
			return nil
		}),
	)
	if err != nil { t.Fatal(err) }
	if err := server.Start(); err != nil { t.Fatal(err) }
	defer func() {
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		defer cancel()
		if err:=server.Stop(ctx);err!=nil {t.Errorf("Stop: %v",err)}
	}()
	connect := func(id string, expectSuccess bool) *neta.Client {
		t.Helper()
		client, err := neta.NewClient(neta.WithClientAddress(server.Addr().String()), neta.WithClientSystemID(id))
		if err != nil { t.Fatal(err) }
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		err=client.Connect(ctx)
		cancel()
		if (err==nil)!=expectSuccess {t.Errorf("Bind id=%q accepted=%v err=%v",id,expectSuccess,err)}
		return client
	}
	first:=connect("approved",true)
	defer func() {
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		defer cancel()
		_ = first.Stop(ctx)
	}()
	duplicate:=connect("approved",false)
	defer func() {
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		defer cancel()
		_ = duplicate.Stop(ctx)
	}()
	missing:=connect("unauthorized",false)
	defer func() {
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		defer cancel()
		_ = missing.Stop(ctx)
	}()
}
