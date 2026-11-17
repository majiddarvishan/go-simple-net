package neta_test

import (
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go"
)

func TestExternalPackageCanUseOnlyProvidedFunctionalOptions(t *testing.T) {
	client, err := neta.NewClient(
		neta.WithClientAddress("[::1]:9000"),
		neta.WithClientSystemID(""),
		neta.WithClientBindType(neta.BindTypeUniDirection),
		neta.WithConnectTimeout(time.Second),
		neta.WithBindResponseTimeout(time.Second),
		neta.WithAutoReconnect(false),
		neta.WithClientSessionOptions(neta.WithRequestTimeout(time.Second), neta.WithEnquireLinkIdleInterval(0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil Client")
	}

	server, err := neta.NewServer(
		neta.WithServerListenAddress(":0"),
		neta.WithServerSystemID("neta-server"),
		neta.WithServerBindValidator(func(info neta.BindInfo) error { return nil }),
		neta.WithServerOnBound(func(*neta.Session) {}),
		neta.WithServerOnRequest(func(*neta.Session, neta.Request) {}),
		neta.WithServerOnClose(func(*neta.Session, error) {}),
		neta.WithServerSessionOptions(neta.WithCompletionConcurrency(2)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if server == nil {
		t.Fatal("NewServer returned nil Server")
	}

	var _ neta.ClientOption = neta.WithClientAddress("localhost:1")
	var _ neta.ServerOption = neta.WithServerListenAddress(":0")
	var _ neta.SessionOption = neta.WithRequestTimeout(time.Second)
	var _ neta.CompletionHandler = func(neta.Response, error) {}
	var _ neta.OnBoundHandler = func(*neta.Session) {}
	var _ neta.OnRequestHandler = func(*neta.Session, neta.Request) {}
	var _ neta.OnCloseHandler = func(*neta.Session, error) {}
	var _ neta.BindValidator = func(neta.BindInfo) error { return nil }
}
