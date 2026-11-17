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

func newLifecycleTestServer(t *testing.T, options ...ServerOption) *Server {
	t.Helper()
	all := []ServerOption{
		WithServerListenAddress("127.0.0.1:0"),
		WithServerSessionOptions(WithEnquireLinkIdleInterval(0)),
	}
	all = append(all, options...)
	server, err := NewServer(all...)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.Addr() != nil {
		t.Fatal("Addr must be nil before Start")
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if server.Addr() == nil {
		t.Fatal("Addr is nil after Start")
	}
	t.Cleanup(func() {
		_ = server.Stop(context.Background())
	})
	return server
}

func bindLifecyclePeer(t *testing.T, address, systemID string) (net.Conn, wire.Frame) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatalf("Dial %s: %v", address, err)
	}
	request, err := wire.EncodeBindRequest(1, wire.Bind{
		Type:     wire.BindBiDirectional,
		SystemID: systemID,
	})
	if err != nil {
		_ = conn.Close()
		t.Fatalf("EncodeBindRequest: %v", err)
	}
	writeWireFrame(t, conn, request)
	response := readWireFrame(t, conn)
	return conn, response
}

func waitServerPendingSockets(t *testing.T, server *Server, want uint32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		server.mu.Lock()
		got := server.pendingSockets
		server.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	server.mu.Lock()
	got := server.pendingSockets
	server.mu.Unlock()
	t.Fatalf("pending sockets = %d, want %d", got, want)
}

func waitServerBoundCount(t *testing.T, server *Server, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		server.mu.Lock()
		got := len(server.boundIDs)
		server.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	server.mu.Lock()
	got := len(server.boundIDs)
	server.mu.Unlock()
	t.Fatalf("bound IDs = %d, want %d", got, want)
}

func receiveBoundSession(t *testing.T, bound <-chan *Session) *Session {
	t.Helper()
	select {
	case session := <-bound:
		return session
	case <-time.After(time.Second):
		t.Fatal("OnBound callback was not delivered")
		return nil
	}
}

func TestServerStartBindAndGracefulStop(t *testing.T) {
	bound := make(chan *Session, 1)
	server := newLifecycleTestServer(t, WithServerOnBoundHandler(func(session *Session) {
		bound <- session
	}))

	conn, response := bindLifecyclePeer(t, server.Addr().String(), "peer-a")
	defer conn.Close()
	if response.Header.Command != wire.CommandBindResponse || response.Header.Status != wire.StatusOK {
		t.Fatalf("Bind Response = %+v, want successful Bind Response", response.Header)
	}
	session := receiveBoundSession(t, bound)
	if session.State() != SessionStateBound || session.SystemID() != "peer-a" {
		t.Fatalf("Session state/identity = %s/%q", session.State(), session.SystemID())
	}
	waitServerBoundCount(t, server, 1)
	waitServerPendingSockets(t, server, 0)

	stopResult := make(chan error, 1)
	go func() { stopResult <- server.Stop(context.Background()) }()
	unbind := readWireFrame(t, conn)
	if unbind.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("Server Stop sent command 0x%x, want Unbind Request", unbind.Header.Command)
	}
	responseBytes, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, conn, responseBytes)
	select {
	case err := <-stopResult:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Server Stop did not finish after Unbind Response")
	}
	waitDone(t, session)
	if err := server.Start(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Start after Stop = %v, want ErrClosed", err)
	}
}

func TestServerRejectsDuplicateSystemIDWithoutEvictingExistingSession(t *testing.T) {
	bound := make(chan *Session, 2)
	server := newLifecycleTestServer(t, WithServerOnBoundHandler(func(session *Session) {
		bound <- session
	}))

	firstConn, firstResponse := bindLifecyclePeer(t, server.Addr().String(), "duplicate-peer")
	defer firstConn.Close()
	if firstResponse.Header.Status != wire.StatusOK {
		t.Fatalf("first Bind status = 0x%x, want success", firstResponse.Header.Status)
	}
	firstSession := receiveBoundSession(t, bound)

	secondConn, secondResponse := bindLifecyclePeer(t, server.Addr().String(), "duplicate-peer")
	defer secondConn.Close()
	if secondResponse.Header.Command != wire.CommandBindResponse || secondResponse.Header.Status != wire.StatusFailed {
		t.Fatalf("duplicate Bind response = %+v, want rfail", secondResponse.Header)
	}
	if firstSession.State() != SessionStateBound {
		t.Fatalf("existing Session was evicted: state=%s", firstSession.State())
	}
	waitServerBoundCount(t, server, 1)

	_ = firstConn.Close()
	waitDone(t, firstSession)
	waitServerBoundCount(t, server, 0)

	thirdConn, thirdResponse := bindLifecyclePeer(t, server.Addr().String(), "duplicate-peer")
	defer thirdConn.Close()
	if thirdResponse.Header.Status != wire.StatusOK {
		t.Fatalf("Bind after prior owner closed status = 0x%x, want success", thirdResponse.Header.Status)
	}
	_ = receiveBoundSession(t, bound)
}

func TestServerMaxBoundSessionsRejectsNewIdentity(t *testing.T) {
	bound := make(chan *Session, 1)
	server := newLifecycleTestServer(t,
		WithServerMaxBoundSessions(1),
		WithServerOnBoundHandler(func(session *Session) { bound <- session }),
	)

	firstConn, firstResponse := bindLifecyclePeer(t, server.Addr().String(), "first")
	defer firstConn.Close()
	if firstResponse.Header.Status != wire.StatusOK {
		t.Fatalf("first Bind status = 0x%x, want success", firstResponse.Header.Status)
	}
	firstSession := receiveBoundSession(t, bound)

	secondConn, secondResponse := bindLifecyclePeer(t, server.Addr().String(), "second")
	defer secondConn.Close()
	if secondResponse.Header.Status != wire.StatusFailed {
		t.Fatalf("second Bind status = 0x%x, want rfail at capacity", secondResponse.Header.Status)
	}
	if firstSession.State() != SessionStateBound {
		t.Fatalf("existing Session was affected by capacity rejection: %s", firstSession.State())
	}
	waitServerBoundCount(t, server, 1)
}

func TestServerPendingPreBindLimitRejectsBeforeSessionAllocation(t *testing.T) {
	server := newLifecycleTestServer(t, WithServerMaxPendingPreBindSessions(1))
	first, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("first Dial: %v", err)
	}
	defer first.Close()
	waitServerPendingSockets(t, server, 1)

	second, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("second Dial: %v", err)
	}
	defer second.Close()
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	if _, err := second.Read(one[:]); err == nil {
		t.Fatal("excess Pre-Bind connection was not closed")
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatalf("excess Pre-Bind connection remained open until read timeout: %v", err)
	}

	server.mu.Lock()
	sessionCount := len(server.sessions)
	pending := server.pendingSockets
	server.mu.Unlock()
	if sessionCount != 1 || pending != 1 {
		t.Fatalf("session/pending counts = %d/%d, want 1/1", sessionCount, pending)
	}
}

func TestServerStopClosesPreBindSessionsAndListener(t *testing.T) {
	server := newLifecycleTestServer(t)
	conn, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	waitServerPendingSockets(t, server, 1)

	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	if _, err := conn.Read(one[:]); err == nil {
		t.Fatal("Pre-Bind socket remained open after Stop")
	}
	waitServerPendingSockets(t, server, 0)
	if _, err := net.DialTimeout("tcp", server.Addr().String(), 100*time.Millisecond); err == nil {
		t.Fatal("listener accepted a new connection after Stop")
	}
}

func TestServerStopContextExpiryForcesBoundSessions(t *testing.T) {
	bound := make(chan *Session, 1)
	server := newLifecycleTestServer(t, WithServerOnBoundHandler(func(session *Session) {
		bound <- session
	}))
	conn, response := bindLifecyclePeer(t, server.Addr().String(), "slow-peer")
	defer conn.Close()
	if response.Header.Status != wire.StatusOK {
		t.Fatalf("Bind status = 0x%x, want success", response.Header.Status)
	}
	session := receiveBoundSession(t, bound)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- server.Stop(ctx) }()
	unbind := readWireFrame(t, conn)
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

func TestServerConcurrentDuplicateBindsHaveSingleWinner(t *testing.T) {
	bound := make(chan *Session, 2)
	server := newLifecycleTestServer(t, WithServerOnBoundHandler(func(session *Session) {
		bound <- session
	}))

	type dialResult struct {
		conn net.Conn
		err  error
	}
	connsReady := make(chan dialResult, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			conn, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
			if err != nil {
				connsReady <- dialResult{err: err}
				return
			}
			request, err := wire.EncodeBindRequest(1, wire.Bind{
				Type:     wire.BindBiDirectional,
				SystemID: "same-concurrent-id",
			})
			if err != nil {
				_ = conn.Close()
				connsReady <- dialResult{err: err}
				return
			}
			if err := writeFullFrame(conn, request, time.Second); err != nil {
				_ = conn.Close()
				connsReady <- dialResult{err: err}
				return
			}
			connsReady <- dialResult{conn: conn}
		}()
	}
	close(start)

	var conns []net.Conn
	for i := 0; i < 2; i++ {
		got := <-connsReady
		if got.err != nil {
			t.Fatalf("concurrent Bind Dial/write: %v", got.err)
		}
		conns = append(conns, got.conn)
	}
	var success, rejected int
	for _, conn := range conns {
		response := readWireFrame(t, conn)
		switch response.Header.Status {
		case wire.StatusOK:
			success++
		case wire.StatusFailed:
			rejected++
		default:
			t.Fatalf("unexpected Bind status 0x%x", response.Header.Status)
		}
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()
	if success != 1 || rejected != 1 {
		t.Fatalf("success/rejected = %d/%d, want 1/1", success, rejected)
	}
	_ = receiveBoundSession(t, bound)
	waitServerBoundCount(t, server, 1)
}

func TestServerBoundTransitionCommitsReservedIdentityBySession(t *testing.T) {
	session := &Session{systemID: "peer"}
	server := &Server{
		sessions:       map[*Session]struct{}{session: {}},
		preBind:        map[*Session]struct{}{session: {}},
		pendingSockets: 1,
		pendingIDs:     map[string]*Session{"peer": session},
		boundIDs:       make(map[string]*Session),
		sessionIDs:     map[*Session]string{session: "peer"},
	}

	server.sessionBecameBound(session)

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.pendingIDs["peer"] != nil || server.boundIDs["peer"] != session {
		t.Fatalf("reserved identity was not committed to Bound registry: pending=%v bound=%v", server.pendingIDs["peer"], server.boundIDs["peer"])
	}
	if _, exists := server.preBind[session]; exists || server.pendingSockets != 0 {
		t.Fatalf("Pre-Bind accounting was not released: preBind=%t pending=%d", exists, server.pendingSockets)
	}
}

func TestServerDoneReleasesPendingIdentityBeforeSessionIdentityIsPublished(t *testing.T) {
	session := &Session{}
	server := &Server{
		sessions:       map[*Session]struct{}{session: {}},
		preBind:        map[*Session]struct{}{session: {}},
		pendingSockets: 1,
		pendingIDs:     map[string]*Session{"peer": session},
		boundIDs:       make(map[string]*Session),
		sessionIDs:     map[*Session]string{session: "peer"},
	}

	// The Session's public SystemID is still empty here: a close can win after
	// Bind admission reserves the peer identity but before the Reader publishes
	// the Bind state. Cleanup must use the reserved Session identity, not
	// Session.SystemID().
	server.sessionDone(session, ErrClosed)

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.sessions) != 0 || len(server.preBind) != 0 || len(server.pendingIDs) != 0 || len(server.boundIDs) != 0 {
		t.Fatalf("Session registry leaked entries: sessions=%d preBind=%d pendingIDs=%d boundIDs=%d", len(server.sessions), len(server.preBind), len(server.pendingIDs), len(server.boundIDs))
	}
	if server.pendingSockets != 0 || len(server.sessionIDs) != 0 {
		t.Fatalf("Session identity/accounting leaked: pending=%d sessionIDs=%d", server.pendingSockets, len(server.sessionIDs))
	}
}

func TestServerBindValidatorReceivesPeerAndCanRejectWithoutReservingID(t *testing.T) {
	var calls atomic.Uint32
	infos := make(chan BindInfo, 2)
	bound := make(chan *Session, 1)
	server := newLifecycleTestServer(t,
		WithBindValidator(func(info BindInfo) error {
			infos <- info
			if calls.Add(1) == 1 {
				return errors.New("test policy rejection")
			}
			return nil
		}),
		WithServerOnBoundHandler(func(session *Session) { bound <- session }),
	)

	firstConn, firstResponse := bindLifecyclePeer(t, server.Addr().String(), "validated-peer")
	defer firstConn.Close()
	if firstResponse.Header.Status != wire.StatusFailed {
		t.Fatalf("first Bind status = 0x%x, want rfail from validator", firstResponse.Header.Status)
	}
	firstInfo := <-infos
	if firstInfo.SystemID != "validated-peer" || firstInfo.BindType != uint8(wire.BindBiDirectional) || firstInfo.RemoteAddr == nil {
		t.Fatalf("BindValidator received incomplete peer info: %+v", firstInfo)
	}

	secondConn, secondResponse := bindLifecyclePeer(t, server.Addr().String(), "validated-peer")
	defer secondConn.Close()
	if secondResponse.Header.Status != wire.StatusOK {
		t.Fatalf("second Bind status = 0x%x, want success after validator accepts", secondResponse.Header.Status)
	}
	_ = receiveBoundSession(t, bound)
	waitServerBoundCount(t, server, 1)
}
