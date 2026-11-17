package neta

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func newClientWithBoundTestSession(t *testing.T, session *Session) *Client {
	t.Helper()
	client, err := NewClient(WithClientAddress("127.0.0.1:1"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.mu.Lock()
	client.current = session
	client.generation = 1
	client.mu.Unlock()
	return client
}

func TestClientCurrentSessionReturnsOnlyBoundGeneration(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	session := newTestBoundSession(local, defaultSessionConfig(), &testSessionDispatcher{}, sessionHandlers{})
	client := newClientWithBoundTestSession(t, session)

	got, ok := client.CurrentSession()
	if !ok || got != session {
		t.Fatalf("CurrentSession = (%p, %t), want (%p, true)", got, ok, session)
	}

	session.Close()
	waitDone(t, session)
	got, ok = client.CurrentSession()
	if ok || got != nil {
		t.Fatalf("CurrentSession after close = (%p, %t), want (nil, false)", got, ok)
	}
	client.Stop(context.Background())
}

func TestClientStopGracefullyUnbindsAndIsIdempotent(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	session := newTestBoundSession(local, defaultSessionConfig(), &testSessionDispatcher{}, sessionHandlers{})
	client := newClientWithBoundTestSession(t, session)

	const callers = 4
	results := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		go func() {
			ready.Done()
			<-start
			results <- client.Stop(context.Background())
		}()
	}
	ready.Wait()
	close(start)

	unbind := readWireFrame(t, peer)
	if unbind.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("Stop sent command 0x%x, want Unbind Request", unbind.Header.Command)
	}
	response, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)

	for i := 0; i < callers; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("Stop caller %d: %v", i, err)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent Stop callers did not complete")
		}
	}
	waitDone(t, session)
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("CurrentSession returned a generation after terminal Stop")
	}
}

func TestClientStopContextExpiryForcesCurrentSession(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	session := newTestBoundSession(local, defaultSessionConfig(), &testSessionDispatcher{}, sessionHandlers{})
	client := newClientWithBoundTestSession(t, session)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- client.Stop(ctx) }()

	unbind := readWireFrame(t, peer)
	if unbind.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("expected Unbind Request, got 0x%x", unbind.Header.Command)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop error = %v, want context deadline", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not force-close an unresponsive Session")
	}
	waitDone(t, session)
}

func TestClientStopBeforeConnectIsTerminalAndIdempotent(t *testing.T) {
	client, err := NewClient(WithClientAddress("127.0.0.1:1"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := client.Stop(context.Background()); err != nil {
		t.Fatalf("repeated Stop: %v", err)
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("CurrentSession returned a generation after Stop")
	}
	client.mu.Lock()
	stopped := client.stopped
	client.mu.Unlock()
	if !stopped {
		t.Fatal("Stop did not make Client terminal")
	}
}
