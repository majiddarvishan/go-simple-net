package neta

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func TestClientBindConfigurationRequiresSystemID(t *testing.T) {
	_, err := NewClient(WithClientAddress("127.0.0.1:9000"))
	requireConfigError(t, err, "system_id")
}

func TestClientBindConfigurationValidatesWireConstraints(t *testing.T) {
	cases := []struct {
		name     string
		systemID string
		bindType BindType
		field    string
	}{
		{name: "empty_system_id", systemID: "", bindType: BindTypeBiDirectional, field: "system_id"},
		{name: "too_long_system_id", systemID: strings.Repeat("x", 20), bindType: BindTypeBiDirectional, field: "system_id"},
		{name: "nul_system_id", systemID: "client\x00id", bindType: BindTypeBiDirectional, field: "system_id"},
		{name: "invalid_bind_type", systemID: "client", bindType: BindType(2), field: "bind_type"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(
				WithClientAddress("127.0.0.1:9000"),
				WithClientSystemID(test.systemID),
				WithClientBindType(test.bindType),
			)
			requireConfigError(t, err, test.field)
		})
	}
}

func TestClientBindTypeDefaultsToBiDirectionalAndCanBeConfigured(t *testing.T) {
	client, err := NewClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSystemID("client"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.config.bindType != BindTypeBiDirectional {
		t.Fatalf("default BindType = %d, want bi-directional", client.config.bindType)
	}

	client, err = NewClient(
		WithClientAddress("127.0.0.1:9000"),
		WithClientSystemID("client"),
		WithClientBindType(BindTypeUniDirectional),
	)
	if err != nil {
		t.Fatalf("NewClient with uni-directional Bind: %v", err)
	}
	if client.config.bindType != BindTypeUniDirectional {
		t.Fatalf("configured BindType = %d, want uni-directional", client.config.bindType)
	}
}

func TestClientConnectBindsConfiguredIdentityAndStopsGracefully(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer conn.Close()

		request := readWireFrame(t, conn)
		bind, err := wire.DecodeBindRequest(request)
		if err != nil {
			peerDone <- err
			return
		}
		if bind.SystemID != "client-01" || bind.Type != wire.BindUniDirectional {
			peerDone <- errors.New("Bind Request did not contain configured SystemID/BindType")
			return
		}
		response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
			Type: wire.BindUniDirectional, SystemID: "server-01",
		})
		if err != nil {
			peerDone <- err
			return
		}
		writeWireFrame(t, conn, response)

		unbind := readWireFrame(t, conn)
		if unbind.Header.Command != wire.CommandUnbindRequest {
			peerDone <- errors.New("expected Unbind Request")
			return
		}
		unbindResponse, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
		if err != nil {
			peerDone <- err
			return
		}
		writeWireFrame(t, conn, unbindResponse)
		peerDone <- nil
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("client-01"),
		WithClientBindType(BindTypeUniDirectional),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	session, ok := client.CurrentSession()
	if !ok {
		t.Fatal("Connect returned without a current Bound Session")
	}
	if session.SystemID() != "server-01" {
		t.Fatalf("Session.SystemID = %q, want server-01", session.SystemID())
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("peer did not complete graceful Unbind")
	}
}

func TestClientConnectBindTimeoutDoesNotPublishSession(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = readWireFrame(t, conn)
		// Deliberately withhold Bind Response until the Client times out.
		time.Sleep(150 * time.Millisecond)
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("client-timeout"),
		WithBindResponseTimeout(40*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if !errors.Is(err, ErrBindTimeout) {
		t.Fatalf("Connect error = %v, want ErrBindTimeout", err)
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("timed-out Bind published a current Session")
	}
	_ = client.Stop(context.Background())
	<-peerDone
}

func TestClientBindRejectionIsNotAutomaticallyRetried(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer conn.Close()
		request := readWireFrame(t, conn)
		bind, err := wire.DecodeBindRequest(request)
		if err != nil {
			peerDone <- err
			return
		}
		response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusFailed, bind)
		if err != nil {
			peerDone <- err
			return
		}
		writeWireFrame(t, conn, response)
		peerDone <- nil
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("client-rejected"),
		WithAutoReconnect(true),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); !errors.Is(err, ErrBindRejected) {
		t.Fatalf("Connect error = %v, want ErrBindRejected", err)
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("rejected Bind published a current Session")
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("peer did not finish rejected Bind")
	}

	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(150 * time.Millisecond))
	if conn, err := listener.Accept(); err == nil {
		_ = conn.Close()
		t.Fatal("Bind rejection unexpectedly triggered an automatic retry")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("second Accept error = %v, want timeout", err)
	}
	_ = client.Stop(context.Background())
}

func TestClientAutoReconnectCreatesFreshSessionGeneration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan error, 1)
	releaseFirst := make(chan struct{})
	go func() {
		for attempt := 0; attempt < 2; attempt++ {
			conn, err := listener.Accept()
			if err != nil {
				peerDone <- err
				return
			}
			request := readWireFrame(t, conn)
			bind, err := wire.DecodeBindRequest(request)
			if err != nil {
				_ = conn.Close()
				peerDone <- err
				return
			}
			response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
				Type: bind.Type, SystemID: "server-reconnect",
			})
			if err != nil {
				_ = conn.Close()
				peerDone <- err
				return
			}
			writeWireFrame(t, conn, response)

			if attempt == 0 {
				// Let the test observe the first Bound generation before
				// simulating an unexpected transport loss.
				<-releaseFirst
				_ = conn.Close()
				continue
			}

			unbind := readWireFrame(t, conn)
			if unbind.Header.Command != wire.CommandUnbindRequest {
				_ = conn.Close()
				peerDone <- errors.New("expected Unbind Request on replacement generation")
				return
			}
			unbindResponse, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
			if err != nil {
				_ = conn.Close()
				peerDone <- err
				return
			}
			writeWireFrame(t, conn, unbindResponse)
			_ = conn.Close()
			peerDone <- nil
			return
		}
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("client-reconnect"),
		WithAutoReconnect(true),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("initial Connect: %v", err)
	}
	first, ok := client.CurrentSession()
	if !ok {
		t.Fatal("initial Connect did not publish a Bound Session")
	}
	close(releaseFirst)

	var replacement *Session
	reconnectDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(reconnectDeadline) {
		current, bound := client.CurrentSession()
		if bound && current != first {
			replacement = current
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Client did not reconnect within the test deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if replacement == nil {
		t.Fatal("reconnect did not publish a fresh Session generation")
	}
	if replacement.SystemID() != "server-reconnect" {
		t.Fatalf("replacement SystemID = %q", replacement.SystemID())
	}
	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("replacement peer did not complete graceful Unbind")
	}
}

func TestUniDirectionalClientDrainsAndIgnoresPeerRequests(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	requests := make(chan struct{}, 1)
	session := newBoundSessionWithResources(local, defaultSessionConfig(), sessionIdentity{
		role: sessionRoleClient, bindType: wire.BindUniDirectional,
	}, dispatcher, sessionHandlers{
		onRequest: func(*Session, Request) { requests <- struct{}{} },
	}, newClientResourceLedger(defaultOwnerConfig()))
	defer peer.Close()

	frame, err := wire.EncodeStreamRequest(1, []byte("peer-request"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, frame)
	select {
	case <-requests:
		t.Fatal("uni-directional Client dispatched a peer Request")
	case <-time.After(50 * time.Millisecond):
	}
	if session.State() != SessionStateBound {
		t.Fatalf("peer Request closed uni-directional Client: %s", session.State())
	}
	if err := session.SendResponse(1, []byte("not-allowed"), 0); !errors.Is(err, ErrProtocol) {
		t.Fatalf("SendResponse error = %v, want ErrProtocol", err)
	}
	session.Close()
	waitDone(t, session)
}
