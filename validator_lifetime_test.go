package neta

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

// An expired handshake frees the socket slot, but must not free the slot
// occupied by a blocked user validator. Repeated connections cannot create
// an unbounded population of stuck goroutines.
func TestBlockedBindValidatorLifetimeIsBoundedAfterSessionTimeout(t *testing.T) {
	const limit = uint32(2)
	release := make(chan struct{})
	defer close(release)
	var entered atomic.Uint32

	server := newLifecycleTestServer(t,
		WithServerMaxPendingPreBindSessions(limit),
		WithServerPreBindHandshakeTimeout(70*time.Millisecond),
		WithBindValidator(func(BindInfo) error {
			entered.Add(1)
			<-release
			return nil
		}),
	)
	request, err := wire.EncodeBindRequest(1, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "blocked-validator",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		conn, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
		if err != nil {
			t.Fatalf("Dial attempt %d: %v", i, err)
		}
		writeWireFrame(t, conn, request)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var b [1]byte
		_, err = conn.Read(b[:])
		_ = conn.Close()
		if err == nil {
			t.Fatalf("attempt %d unexpectedly received a Bind Response", i)
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			t.Fatalf("attempt %d was not closed at handshake expiry: %v", i, err)
		}
		waitServerPendingSockets(t, server, 0)
	}
	if got := entered.Load(); got != limit {
		t.Fatalf("validator call population = %d, want lifetime cap %d", got, limit)
	}
	if got := len(server.validatorSlots); got != int(limit) {
		t.Fatalf("retained validator permits = %d, want %d", got, limit)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Stop should not wait for blocked user validator: %v", err)
	}
}

func TestServerCannotReserveSystemIDAfterSessionCleanup(t *testing.T) {
	server, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	retired := &Session{}
	if err := server.reserveBoundIdentity(retired, "retired-peer"); !errors.Is(err, ErrClosing) {
		t.Fatalf("reserve retired Session = %v, want ErrClosing", err)
	}
	if got := len(server.pendingIDs); got != 0 {
		t.Fatalf("late reservation leaked %d identities", got)
	}
}

// A validator may return after the Session has already completed teardown.
// Its result must not reserve the same identity forever or reject a replacement.
func TestLateBindValidatorCannotResurrectTimedOutIdentity(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var calls atomic.Uint32
	server := newLifecycleTestServer(t,
		WithServerPreBindHandshakeTimeout(70*time.Millisecond),
		WithBindValidator(func(BindInfo) error {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil
		}),
	)
	conn, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	request, err := wire.EncodeBindRequest(1, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "late-peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, conn, request)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first validator did not enter")
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	_, err = conn.Read(b[:])
	if err == nil {
		t.Fatal("expired Session unexpectedly sent a Bind Response")
	}
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		t.Fatalf("expired Session remained open: %v", err)
	}
	waitServerPendingSockets(t, server, 0)
	close(release)
	// The lifetime permit is returned only after the old handler completes,
	// including any stale reservation attempt.
	deadline := time.Now().Add(time.Second)
	for len(server.validatorSlots) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(server.validatorSlots) != 0 {
		t.Fatal("expired validator did not release its permit")
	}
	server.mu.Lock()
	pending := len(server.pendingIDs)
	server.mu.Unlock()
	if pending != 0 {
		t.Fatalf("late validator resurrected %d pending IDs", pending)
	}
	second, response := bindLifecyclePeer(t, server.Addr().String(), "late-peer")
	defer second.Close()
	if response.Header.Status != wire.StatusOK {
		t.Fatalf("replacement Bind status = 0x%x, want successful Bind", response.Header.Status)
	}
}
