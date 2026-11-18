package neta

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type failingAcceptListener struct {
	failure error
	closes  atomic.Int32
}

func (l *failingAcceptListener) Accept() (net.Conn, error) {
	return nil, l.failure
}

func (l *failingAcceptListener) Close() error {
	l.closes.Add(1)
	return nil
}

func (l *failingAcceptListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
}

func TestServerPublishesTerminalAcceptFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
	}{
		{name: "UnexpectedError", failure: errors.New("injected permanent accept failure")},
		{name: "UnexpectedListenerClose", failure: net.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
			if err != nil {
				t.Fatal(err)
			}
			if got := server.AcceptDone(); got != nil {
				t.Fatal("AcceptDone before Start must be nil")
			}
			listener := &failingAcceptListener{failure: tc.failure}
			ready := make(chan struct{})
			done := make(chan struct{})
			server.mu.Lock()
			server.listener = listener
			server.addr = listener.Addr()
			server.started = true
			server.acceptDone = done
			server.mu.Unlock()

			go server.acceptLoop(listener, ready, done)
			<-ready
			select {
			case <-server.AcceptDone():
			case <-time.After(time.Second):
				t.Fatal("fatal listener error did not close AcceptDone")
			}
			if err := server.AcceptErr(); !errors.Is(err, tc.failure) {
				t.Fatalf("AcceptErr = %v, want wrapped %v", err, tc.failure)
			}
			if err := server.Start(); !errors.Is(err, tc.failure) {
				t.Fatalf("Start after fatal Accept = %v, want saved failure", err)
			}
			if got := listener.closes.Load(); got != 1 {
				t.Fatalf("listener.Close calls = %d, want 1 after failure", got)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := server.Stop(ctx); err != nil {
				t.Fatalf("Stop after fatal Accept: %v", err)
			}
			if err := server.AcceptErr(); !errors.Is(err, tc.failure) {
				t.Fatalf("Stop erased AcceptErr: %v", err)
			}
		})
	}
}

func TestServerNormalStopClosesAcceptDoneWithoutError(t *testing.T) {
	server := newLifecycleTestServer(t)
	done := server.AcceptDone()
	if done == nil {
		t.Fatal("AcceptDone must be nonnil after Start")
	}
	select {
	case <-done:
		t.Fatal("AcceptDone closed while Server still accepting")
	default:
	}
	if err := server.AcceptErr(); err != nil {
		t.Fatalf("AcceptErr while healthy = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("normal Stop did not close AcceptDone")
	}
	if err := server.AcceptErr(); err != nil {
		t.Fatalf("normal Stop AcceptErr = %v, want nil", err)
	}
}
