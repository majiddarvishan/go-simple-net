package neta

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

// SendRequest snapshots payload, admits one Stream Request without waiting for
// network I/O, and reports its terminal result through completion.
func (s *Session) SendRequest(payload []byte, completion CompletionHandler) (uint32, error) {
	if completion == nil {
		return 0, fmt.Errorf("neta: SendRequest requires a non-nil CompletionHandler")
	}
	frame, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	if err := s.applicationSendErrorLocked(); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	if s.role == sessionRoleServer && s.bindType == wire.BindUniDirectional {
		s.mu.Unlock()
		return 0, ErrProtocol
	}
	if uint32(len(s.pending)) >= s.config.maxOutstandingRequests {
		s.mu.Unlock()
		return 0, ErrCapacity
	}
	sequence := s.nextSequenceLocked()
	if sequence == 0 {
		s.mu.Unlock()
		return 0, ErrCapacity
	}
	binary.BigEndian.PutUint32(frame[6:10], sequence)
	pending := &pendingRequest{
		completion: completion,
		deadline:   time.Now().Add(s.config.requestTimeout),
	}
	item := &outboundFrame{bytes: frame, requestSeq: sequence}
	if err := s.enqueueApplicationLocked(item); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	s.pending[sequence] = pending
	s.mu.Unlock()

	s.signalControl()
	return sequence, nil
}

// SendNoWait admits a one-way Stream Request without creating pending response
// state or a Completion callback.
func (s *Session) SendNoWait(payload []byte) (uint32, error) {
	frame, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	if err := s.applicationSendErrorLocked(); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	if s.role == sessionRoleServer && s.bindType == wire.BindUniDirectional {
		s.mu.Unlock()
		return 0, ErrProtocol
	}
	sequence := s.nextSequenceLocked()
	if sequence == 0 {
		s.mu.Unlock()
		return 0, ErrCapacity
	}
	binary.BigEndian.PutUint32(frame[6:10], sequence)
	if err := s.enqueueApplicationLocked(&outboundFrame{bytes: frame}); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	s.mu.Unlock()
	return sequence, nil
}

// SendResponse admits a Stream Response using the original peer Request
// Sequence. Payload is snapshotted before the method returns successfully.
func (s *Session) SendResponse(sequence uint32, payload []byte, status uint8) error {
	if sequence == 0 {
		return ErrProtocol
	}
	frame, err := buildStreamFrame(wire.CommandStreamResponse, wire.CommandStatus(status), sequence, payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if err := s.applicationSendErrorLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	err = s.enqueueApplicationLocked(&outboundFrame{bytes: frame})
	s.mu.Unlock()
	return err
}

func (s *Session) applicationSendErrorLocked() error {
	switch s.state {
	case SessionStateBound:
		return nil
	case SessionStateNew, SessionStateBinding:
		return ErrNotBound
	case SessionStateUnbinding:
		return ErrClosing
	default:
		return ErrClosed
	}
}

// Unbind performs the approved graceful Unbind sequence. It closes new
// Application admission atomically, drains already-admitted Frames in FIFO
// order, and waits for the matching Unbind Response or a bounded close.
func (s *Session) Unbind(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.dispatchMu.Lock()
	s.mu.Lock()
	var admissionErr error
	switch s.state {
	case SessionStateBound:
		sequence := s.nextSequenceLocked()
		if sequence == 0 {
			admissionErr = ErrCapacity
			break
		}
		frame, err := wire.EncodeUnbindRequest(sequence)
		if err != nil {
			admissionErr = err
			break
		}
		s.state = SessionStateUnbinding
		if s.enquireLinkQueued && !s.enquireLinkWriting {
			s.removeQueuedControlSequenceLocked(s.enquireLinkSequence)
		}
		s.clearProbeLocked()
		s.unbindSequence = sequence
		s.unbindDeadline = time.Now().Add(s.config.unbindTimeout)
		admissionErr = s.enqueueControlLocked(&outboundFrame{bytes: frame, control: true, barrier: true, unbindRequest: true})
	case SessionStateUnbinding:
		// Join the existing graceful Unbind operation.
	case SessionStateClosing, SessionStateClosed:
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		select {
		case <-s.done:
			return s.Err()
		default:
			return ErrClosing
		}
	default:
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		return ErrNotBound
	}
	s.mu.Unlock()
	s.dispatchMu.Unlock()
	if admissionErr != nil {
		s.beginClose(admissionErr, true)
		return admissionErr
	}
	s.signalControl()

	select {
	case <-s.done:
		return s.Err()
	case <-ctx.Done():
		s.beginClose(ctx.Err(), true)
		<-s.done
		return ctx.Err()
	}
}

// Close initiates Forced Close A and returns without waiting for internal
// teardown or arbitrary user callbacks.
func (s *Session) Close() {
	s.beginClose(ErrClosed, true)
}

// Done closes after transport loops, timers and internal Session state have
// been torn down and the terminal cause has become immutable.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns the terminal cause. It is immutable after Done closes.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCause
}

func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) SystemID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.systemID
}

func (s *Session) RemoteAddr() net.Addr { return s.remoteAddr }

func (s *Session) completeRequest(sequence uint32, response Response, resultErr error) {
	s.dispatchMu.Lock()
	s.mu.Lock()
	pending := s.pending[sequence]
	if pending == nil || pending.settled {
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		return
	}
	pending.settled = true
	if !pending.writerClaimed {
		s.removeQueuedRequestLocked(sequence)
	}
	task := s.completionTaskLocked(sequence, pending, response, resultErr)
	s.mu.Unlock()

	accepted := s.enqueueCallbackWithDispatchLock(callbackCompletion, task)
	s.dispatchMu.Unlock()
	if !accepted {
		s.beginClose(ErrCapacity, true)
	}
	s.signalControl()
}

func (s *Session) completionTaskLocked(sequence uint32, pending *pendingRequest, response Response, resultErr error) callbackTask {
	return callbackTask{
		session: s,
		run: func() {
			defer s.releasePending(sequence)
			pending.completion(response, resultErr)
		},
		cancel: func() { s.releasePending(sequence) },
	}
}

func (s *Session) releasePending(sequence uint32) {
	s.mu.Lock()
	delete(s.pending, sequence)
	s.mu.Unlock()
	s.signalControl()
}

func buildStreamFrame(command wire.CommandID, status wire.CommandStatus, sequence uint32, payload []byte) ([]byte, error) {
	if len(payload) > wire.MaxBodySize {
		return nil, ErrFrameTooLarge
	}
	length := wire.HeaderSize + len(payload)
	header, err := (wire.Header{
		Length: uint32(length), Command: command, Status: status, Sequence: sequence,
	}).MarshalBinary()
	if err != nil {
		if errors.Is(err, wire.ErrFrameTooLarge) {
			return nil, ErrFrameTooLarge
		}
		return nil, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	frame := make([]byte, length)
	copy(frame[:wire.HeaderSize], header[:])
	copy(frame[wire.HeaderSize:], payload)
	return frame, nil
}
