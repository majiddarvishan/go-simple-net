package neta

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Start opens the configured TCP listener and returns after the Accept loop is
// ready. A Server is terminal after Stop and cannot be restarted.
func (s *Server) Start() error {
	if s == nil {
		return ErrClosed
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return ErrClosed
	}
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	listener, err := net.Listen("tcp", s.config.listenAddress)
	if err != nil {
		return fmt.Errorf("neta: listen: %w", err)
	}

	ready := make(chan struct{})
	acceptDone := make(chan struct{})
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		_ = listener.Close()
		return ErrClosed
	}
	s.listener = listener
	s.addr = listener.Addr()
	s.acceptDone = acceptDone
	s.started = true
	s.mu.Unlock()

	go s.acceptLoop(listener, ready, acceptDone)
	<-ready
	return nil
}

// Addr returns the actual listener address, or nil before a successful Start.
func (s *Server) Addr() net.Addr {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) acceptLoop(listener net.Listener, ready chan<- struct{}, done chan<- struct{}) {
	close(ready)
	defer close(done)

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.Lock()
			stopping := s.stopping
			s.mu.Unlock()
			if stopping || errors.Is(err, net.ErrClosed) {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				timer := time.NewTimer(5 * time.Millisecond)
				<-timer.C
				continue
			}
			return
		}

		acceptedAt := time.Now()
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		if s.pendingSockets >= s.config.maxPendingPreBindSessions {
			s.mu.Unlock()
			_ = conn.Close()
			continue
		}
		s.pendingSockets++
		s.mu.Unlock()

		handlers := sessionHandlers{
			onBound:         s.config.onBound,
			onRequest:       s.config.onRequest,
			onClose:         s.config.onClose,
			onBoundInternal: s.sessionBecameBound,
			onDoneInternal:  s.sessionDone,
		}
		session, err := newServerPreBindSessionUnstarted(
			conn,
			s.config.session,
			acceptedAt,
			s.config.preBindHandshakeTimeout,
			s.callbacks,
			handlers,
			s.resources,
		)
		if err != nil {
			_ = conn.Close()
			s.releasePendingSocket()
			continue
		}

		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			_ = conn.Close()
			s.releasePendingSocket()
			return
		}
		s.sessions[session] = struct{}{}
		s.preBind[session] = struct{}{}
		s.mu.Unlock()

		session.start()
		session.signalControl()
		go s.handleBind(session)
	}
}

func (s *Server) releasePendingSocket() {
	s.mu.Lock()
	if s.pendingSockets > 0 {
		s.pendingSockets--
	}
	s.mu.Unlock()
}

// handleBind runs one bounded Pre-Bind admission flow per accepted Session.
// The number of these supervisors is bounded by MaxPendingPreBindSessions.
func (s *Server) handleBind(session *Session) {
	event, err := session.waitBindRequest(context.Background())
	if err != nil {
		return
	}

	var admissionErr error
	if s.config.bindValidator != nil {
		admissionErr = s.config.bindValidator(event.info)
	}
	if admissionErr == nil {
		admissionErr = s.reserveBoundIdentity(session, event.info.SystemID)
	}
	if err := session.completeBind(event, "", admissionErr); err != nil {
		session.Close()
	}
}

func (s *Server) reserveBoundIdentity(session *Session, systemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		return ErrClosing
	}
	if existing := s.boundIDs[systemID]; existing != nil {
		return fmt.Errorf("%w: duplicate SystemID %q", ErrBindRejected, systemID)
	}
	if existing := s.pendingIDs[systemID]; existing != nil {
		return fmt.Errorf("%w: duplicate SystemID %q", ErrBindRejected, systemID)
	}
	if uint32(len(s.boundIDs)+len(s.pendingIDs)) >= s.config.maxBoundSessions {
		return fmt.Errorf("%w: maximum bound Sessions reached", ErrBindRejected)
	}
	s.pendingIDs[systemID] = session
	return nil
}

// sessionBecameBound commits the previously reserved SystemID only after the
// successful Bind Response has been fully written.
func (s *Server) sessionBecameBound(session *Session) {
	systemID := session.SystemID()
	s.mu.Lock()
	if s.pendingIDs[systemID] == session {
		delete(s.pendingIDs, systemID)
		s.boundIDs[systemID] = session
	}
	if _, exists := s.preBind[session]; exists {
		delete(s.preBind, session)
		if s.pendingSockets > 0 {
			s.pendingSockets--
		}
	}
	s.mu.Unlock()
}

// sessionDone removes only entries still owned by this exact Session pointer.
// An old Session can never remove a newer Session using the same SystemID.
func (s *Server) sessionDone(session *Session, _ error) {
	systemID := session.SystemID()
	s.mu.Lock()
	delete(s.sessions, session)
	if _, exists := s.preBind[session]; exists {
		delete(s.preBind, session)
		if s.pendingSockets > 0 {
			s.pendingSockets--
		}
	}
	if s.pendingIDs[systemID] == session {
		delete(s.pendingIDs, systemID)
	}
	if s.boundIDs[systemID] == session {
		delete(s.boundIDs, systemID)
	}
	s.mu.Unlock()
}

// Stop is terminal and idempotent. It closes the listener before taking the
// Session snapshot, closes Pre-Bind Sessions and concurrently Gracefully
// Unbinds every Bound Session. Context expiry forces all remaining Sessions.
func (s *Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.stopDone != nil {
		done := s.stopDone
		s.mu.Unlock()
		select {
		case <-done:
			s.mu.Lock()
			err := s.stopErr
			s.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.stopDone = make(chan struct{})
	s.stopping = true
	listener := s.listener
	acceptDone := s.acceptDone
	s.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	if acceptDone != nil {
		<-acceptDone
	}

	s.mu.Lock()
	sessions := make([]*Session, 0, len(s.sessions))
	for session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, session := range sessions {
		wg.Add(1)
		go func(session *Session) {
			defer wg.Done()
			switch session.State() {
			case SessionStateBound, SessionStateUnbinding:
				_ = session.Unbind(ctx)
			default:
				session.Close()
			}
			<-session.Done()
		}(session)
	}

	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	var stopErr error
	select {
	case <-allDone:
	case <-ctx.Done():
		stopErr = ctx.Err()
		for _, session := range sessions {
			session.Close()
		}
		<-allDone
	}

	s.mu.Lock()
	s.stopErr = stopErr
	close(s.stopDone)
	s.mu.Unlock()
	return stopErr
}
