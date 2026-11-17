package neta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func (s *Session) readerLoop() {
	defer s.wg.Done()
	for {
		frame, err := s.readFrame()
		if err != nil {
			if !s.isClosing() {
				s.failRead(err)
			}
			return
		}
		if err := s.handleFrame(frame); err != nil {
			s.beginClose(err, true)
			return
		}
	}
}

func (s *Session) readFrame() (wire.Frame, error) {
	var headerBytes [wire.HeaderSize]byte
	if _, err := io.ReadFull(s.conn, headerBytes[:1]); err != nil {
		return wire.Frame{}, err
	}

	s.mu.Lock()
	bound := s.state == SessionStateBound || s.state == SessionStateUnbinding
	s.mu.Unlock()
	if bound {
		if err := s.conn.SetReadDeadline(time.Now().Add(s.config.frameAssemblyTimeout)); err != nil {
			return wire.Frame{}, err
		}
	}

	if _, err := io.ReadFull(s.conn, headerBytes[1:]); err != nil {
		return wire.Frame{}, err
	}
	header, err := wire.ParseHeader(headerBytes[:])
	if err != nil {
		return wire.Frame{}, err
	}

	switch header.Command {
	case wire.CommandUnbindRequest, wire.CommandUnbindResponse, wire.CommandEnquireLinkRequest, wire.CommandEnquireLinkResponse:
		if header.Length != wire.HeaderSize {
			return wire.Frame{}, fmt.Errorf("%w: control command must not carry a Body", ErrProtocol)
		}
	}
	bodyLength := int(header.Length) - wire.HeaderSize
	var body []byte
	if bodyLength > 0 {
		body = make([]byte, bodyLength)
		if _, err := io.ReadFull(s.conn, body); err != nil {
			return wire.Frame{}, err
		}
	}
	if bound {
		if err := s.conn.SetReadDeadline(time.Time{}); err != nil {
			return wire.Frame{}, err
		}
	}
	switch header.Command {
	case wire.CommandUnbindRequest, wire.CommandUnbindResponse, wire.CommandEnquireLinkRequest, wire.CommandEnquireLinkResponse:
		if len(body) != 0 {
			return wire.Frame{}, fmt.Errorf("%w: control command has a non-empty Body", ErrProtocol)
		}
	}
	return wire.Frame{Header: header, Body: body}, nil
}

func (s *Session) handleFrame(frame wire.Frame) error {
	if frame.Header.Sequence == 0 {
		return ErrProtocol
	}
	s.mu.Lock()
	state := s.state
	role := s.role
	s.lastActivity = time.Now()
	s.mu.Unlock()
	s.signalControl()

	switch state {
	case SessionStateNew:
		if role != sessionRoleServer || frame.Header.Command != wire.CommandBindRequest {
			return ErrProtocol
		}
		return s.handlePreBindRequest(frame)
	case SessionStateBinding:
		if role != sessionRoleClient || frame.Header.Command != wire.CommandBindResponse {
			return ErrProtocol
		}
		return s.handleClientBindResponse(frame)
	case SessionStateBound, SessionStateUnbinding:
		return s.handleBoundFrame(frame, role)
	default:
		return ErrClosed
	}
}

func (s *Session) handlePreBindRequest(frame wire.Frame) error {
	if frame.Header.Sequence == 0 {
		return ErrProtocol
	}
	bind, err := wire.DecodeBindRequest(frame)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	event := bindRequestEvent{
		info: BindInfo{
			SystemID: bind.SystemID, BindType: uint8(bind.Type), RemoteAddr: s.remoteAddr,
		},
		sequence: frame.Header.Sequence,
		decision: make(chan bindDecision, 1),
	}

	s.mu.Lock()
	if s.state != SessionStateNew || s.pendingBind != nil {
		s.mu.Unlock()
		return ErrClosing
	}
	s.pendingBind = &event
	s.mu.Unlock()

	select {
	case s.bindRequests <- event:
	case <-s.closing:
		return ErrClosed
	}

	var decision bindDecision
	select {
	case decision = <-event.decision:
	case <-s.closing:
		return ErrClosed
	}

	status := wire.StatusOK
	cause := error(nil)
	if decision.err != nil {
		status = wire.StatusFailed
		cause = fmt.Errorf("%w: %v", ErrBindRejected, decision.err)
	}

	response, err := wire.EncodeBindResponse(frame.Header.Sequence, status, wire.Bind{
		Type: bind.Type, SystemID: decision.serverSystemID,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}

	s.mu.Lock()
	if s.state != SessionStateNew {
		s.mu.Unlock()
		return ErrClosing
	}
	s.state = SessionStateBinding
	s.bindSequence = frame.Header.Sequence
	s.bindType = bind.Type
	s.systemID = bind.SystemID
	err = s.enqueueControlLocked(&outboundFrame{
		bytes:   response,
		control: true,
		afterWrite: func() {
			if cause != nil {
				s.beginClose(cause, true)
				return
			}
			s.markBound(bind.SystemID, bind.Type)
		},
	})
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) handleClientBindResponse(frame wire.Frame) error {
	s.mu.Lock()
	sequence := s.bindSequence
	s.mu.Unlock()
	if frame.Header.Sequence != sequence {
		return ErrProtocol
	}
	if frame.Header.Status != wire.StatusOK {
		s.beginClose(ErrBindRejected, true)
		return nil
	}
	bind, err := wire.DecodeBindResponse(frame)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	s.markBound(bind.SystemID, bind.Type)
	return nil
}

func (s *Session) handleBoundFrame(frame wire.Frame, role sessionRole) error {
	switch frame.Header.Command {
	case wire.CommandStreamRequest:
		if s.handlers.onRequest == nil {
			return nil
		}
		s.dispatchMu.Lock()
		s.mu.Lock()
		currentState := s.state
		currentBindType := s.bindType
		s.mu.Unlock()
		if currentState != SessionStateBound {
			s.dispatchMu.Unlock()
			return nil
		}
		if role == sessionRoleClient && currentBindType == wire.BindUniDirectional {
			s.dispatchMu.Unlock()
			return ErrProtocol
		}
		payload := frame.Body
		task := callbackTask{
			session: s,
			run: func() {
				s.handlers.onRequest(s, Request{Sequence: frame.Header.Sequence, Payload: payload})
			},
			cancel: func() {},
		}
		accepted := s.enqueueCallbackWithDispatchLock(callbackApplication, task)
		s.dispatchMu.Unlock()
		if !accepted {
			s.beginClose(ErrCapacity, true)
		}
		return nil

	case wire.CommandStreamResponse:
		s.completeRequest(frame.Header.Sequence, Response{
			Sequence: frame.Header.Sequence,
			Status:   uint8(frame.Header.Status),
			Payload:  frame.Body,
		}, nil)
		return nil

	case wire.CommandEnquireLinkRequest:
		response, err := wire.EncodeEnquireLinkResponse(frame.Header.Sequence)
		if err != nil {
			return err
		}
		s.mu.Lock()
		if s.isClosingLocked() {
			s.mu.Unlock()
			return ErrClosed
		}
		err = s.enqueueControlLocked(&outboundFrame{bytes: response, control: true})
		s.mu.Unlock()
		if err != nil {
			s.beginClose(err, true)
		}
		return nil

	case wire.CommandEnquireLinkResponse:
		s.mu.Lock()
		if frame.Header.Sequence == s.enquireLinkSequence {
			if s.enquireLinkQueued {
				s.enquireLinkResponseSeen = true
			} else if s.enquireLinkOutstanding {
				s.enquireLinkOutstanding = false
				s.enquireLinkDeadline = time.Time{}
			}
		}
		s.mu.Unlock()
		s.signalControl()
		return nil

	case wire.CommandUnbindRequest:
		response, err := wire.EncodeUnbindResponse(frame.Header.Sequence)
		if err != nil {
			return err
		}
		s.dispatchMu.Lock()
		s.mu.Lock()
		if s.isClosingLocked() {
			s.mu.Unlock()
			s.dispatchMu.Unlock()
			return ErrClosed
		}
		if s.state == SessionStateBound {
			s.state = SessionStateUnbinding
			s.unbindDeadline = time.Now().Add(s.config.unbindTimeout)
			s.removeQueuedControlSequenceLocked(s.enquireLinkSequence)
			s.clearProbeLocked()
		}
		err = s.enqueueControlLocked(&outboundFrame{
			bytes:      response,
			control:    true,
			barrier:    true,
			afterWrite: func() { s.beginClose(nil, false) },
		})
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		if err != nil {
			s.beginClose(err, true)
		}
		s.signalControl()
		return nil

	case wire.CommandUnbindResponse:
		s.mu.Lock()
		matches := s.state == SessionStateUnbinding && frame.Header.Sequence == s.unbindSequence
		s.mu.Unlock()
		if matches {
			s.beginClose(nil, false)
		}
		return nil

	default:
		return ErrProtocol
	}
}

func (s *Session) writerLoop() {
	defer s.wg.Done()
	for {
		s.mu.Lock()
		for len(s.controlQueue) == 0 && len(s.appQueue) == 0 && len(s.barrierQueue) == 0 && !s.isClosingLocked() {
			s.cond.Wait()
		}
		if s.isClosingLocked() {
			s.mu.Unlock()
			return
		}

		var item *outboundFrame
		switch {
		case len(s.controlQueue) > 0:
			item = s.controlQueue[0]
			s.controlQueue[0] = nil
			s.controlQueue = s.controlQueue[1:]
		case len(s.appQueue) > 0:
			item = s.appQueue[0]
			s.appQueue[0] = nil
			s.appQueue = s.appQueue[1:]
		case len(s.barrierQueue) > 0:
			// Unbind is a FIFO barrier: it cannot pass any previously admitted
			// Application Stream frame, but it uses independent control reserve.
			item = s.barrierQueue[0]
			s.barrierQueue[0] = nil
			s.barrierQueue = s.barrierQueue[1:]
		}
		s.activeWrite = item
		if item.requestSeq != 0 {
			if pending := s.pending[item.requestSeq]; pending != nil {
				pending.writerClaimed = true
			}
		}
		s.mu.Unlock()

		err := writeFullFrame(s.conn, item.bytes, s.config.writeTimeout)

		s.mu.Lock()
		s.releaseFrameChargeLocked(item)
		if s.activeWrite == item {
			s.activeWrite = nil
		}
		if err == nil {
			s.lastActivity = time.Now()
		}
		s.mu.Unlock()
		s.signalControl()

		if err != nil {
			if errors.Is(err, ErrWriteTimeout) {
				s.beginClose(newOperationError("write", ErrWriteTimeout), true)
			} else {
				s.beginClose(newOperationError("write", err), true)
			}
			return
		}
		if item.afterWrite != nil {
			item.afterWrite()
		}
	}
}

func writeFullFrame(conn net.Conn, frame []byte, timeout time.Duration) error {
	if timeout <= 0 {
		return ErrWriteTimeout
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer conn.SetWriteDeadline(time.Time{})

	for written := 0; written < len(frame); {
		n, err := conn.Write(frame[written:])
		if n > 0 {
			written += n
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("%w: %v", ErrWriteTimeout, err)
			}
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (s *Session) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isClosingLocked()
}

func (s *Session) waitForClose(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-s.done:
		return s.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}
