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
	frameBytes, err := streamFrameSize(payload)
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
	if err := s.reserveOutgoingApplicationLocked(frameBytes); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	if err := s.reserveRequestCreditLocked(); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.mu.Unlock()
		return 0, err
	}
	s.admissionCount++
	s.mu.Unlock()

	// Resource reservations precede allocation/copy. The in-flight admission
	// count keeps Done behind this operation if Close races the snapshot.
	frame, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload)

	s.mu.Lock()
	if err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.releaseRequestAdmissionLocked()
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	if err := s.applicationSendErrorLocked(); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.releaseRequestAdmissionLocked()
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	sequence := s.nextSequenceLocked()
	if sequence == 0 {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.releaseRequestAdmissionLocked()
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, ErrCapacity
	}
	binary.BigEndian.PutUint32(frame[6:10], sequence)
	pending := &pendingRequest{completion: completion, requestCredit: true}
	item := &outboundFrame{bytes: frame, requestSeq: sequence, applicationCharged: true}
	if err := s.enqueueApplicationLocked(item); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.releaseRequestAdmissionLocked()
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	// RequestTimeout starts at successful FIFO admission, not before the
	// payload snapshot or while capacity is still being reserved.
	pending.deadline = time.Now().Add(s.config.requestTimeout)
	s.pending[sequence] = pending
	s.finishAdmissionLocked()
	s.mu.Unlock()

	s.signalControl()
	return sequence, nil
}

// SendNoWait admits a one-way Stream Request without creating pending response
// state or a Completion callback.
func (s *Session) SendNoWait(payload []byte) (uint32, error) {
	frameBytes, err := streamFrameSize(payload)
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
	if err := s.reserveOutgoingApplicationLocked(frameBytes); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	s.admissionCount++
	s.mu.Unlock()

	frame, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload)

	s.mu.Lock()
	if err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	if err := s.applicationSendErrorLocked(); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	sequence := s.nextSequenceLocked()
	if sequence == 0 {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, ErrCapacity
	}
	binary.BigEndian.PutUint32(frame[6:10], sequence)
	item := &outboundFrame{bytes: frame, applicationCharged: true}
	if err := s.enqueueApplicationLocked(item); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return 0, err
	}
	s.finishAdmissionLocked()
	s.mu.Unlock()
	s.signalControl()
	return sequence, nil
}

// SendResponse admits a Stream Response using the original peer Request
// Sequence. Payload is snapshotted before the method returns successfully.
func (s *Session) SendResponse(sequence uint32, payload []byte, status uint8) error {
	if sequence == 0 {
		return ErrProtocol
	}
	frameBytes, err := streamFrameSize(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if err := s.applicationSendErrorLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.role == sessionRoleClient && s.bindType == wire.BindUniDirectional {
		s.mu.Unlock()
		return ErrProtocol
	}
	if err := s.reserveOutgoingApplicationLocked(frameBytes); err != nil {
		s.mu.Unlock()
		return err
	}
	s.admissionCount++
	s.mu.Unlock()

	frame, err := buildStreamFrame(wire.CommandStreamResponse, wire.CommandStatus(status), sequence, payload)

	s.mu.Lock()
	if err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return err
	}
	if err := s.applicationSendErrorLocked(); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return err
	}
	item := &outboundFrame{bytes: frame, applicationCharged: true}
	if err := s.enqueueApplicationLocked(item); err != nil {
		s.releaseOutgoingApplicationReservationLocked(frameBytes)
		s.finishAdmissionLocked()
		s.mu.Unlock()
		return err
	}
	s.finishAdmissionLocked()
	s.mu.Unlock()
	s.signalControl()
	return nil
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
		// The caller's earlier Context wins the wait, but the Session's
		// terminal lifecycle classification remains the approved Unbind timeout.
		s.beginClose(ErrUnbindTimeout, true)
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

// State returns the Session's current lifecycle state.
func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// SystemID returns the peer identity accepted during Bind, or an empty string
// before that identity has been published.
func (s *Session) SystemID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.systemID
}

// RemoteAddr returns the peer's TCP address.
func (s *Session) RemoteAddr() net.Addr { return s.remoteAddr }

func (s *Session) completeRequest(sequence uint32, response Response, resultErr error) {
	s.completeRequestWithCharge(sequence, response, resultErr, nil)
}

func (s *Session) completeRequestWithCharge(sequence uint32, response Response, resultErr error, charge *completionReadCharge) {
	s.dispatchMu.Lock()
	s.mu.Lock()
	pending := s.pending[sequence]
	if pending == nil || pending.settled || !pending.writerClaimed {
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		charge.release()
		return
	}
	pending.settled = true
	task := s.completionTaskWithChargeLocked(sequence, pending, response, resultErr, charge)
	s.mu.Unlock()

	accepted := s.enqueueCallbackWithDispatchLock(callbackCompletion, task)
	s.dispatchMu.Unlock()
	if !accepted {
		s.beginClose(ErrCapacity, true)
	}
	s.signalControl()
}

func (s *Session) completionTaskLocked(sequence uint32, pending *pendingRequest, response Response, resultErr error) callbackTask {
	return s.completionTaskWithChargeLocked(sequence, pending, response, resultErr, nil)
}

func (s *Session) completionTaskWithChargeLocked(sequence uint32, pending *pendingRequest, response Response, resultErr error, charge *completionReadCharge) callbackTask {
	return callbackTask{
		session: s,
		run: func() {
			defer s.releasePending(sequence, pending)
			defer charge.release()
			pending.completion(response, resultErr)
		},
		cancel: func() {
			charge.release()
			s.releasePending(sequence, pending)
		},
	}
}

func (s *Session) releasePending(sequence uint32, pending *pendingRequest) {
	s.mu.Lock()
	if current := s.pending[sequence]; current == pending {
		delete(s.pending, sequence)
	}
	s.releaseRequestCreditLocked(pending)
	s.mu.Unlock()
	s.signalControl()
}

func streamFrameSize(payload []byte) (uint64, error) {
	if len(payload) > wire.MaxBodySize {
		return 0, ErrFrameTooLarge
	}
	return uint64(wire.HeaderSize) + uint64(len(payload)), nil
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
