package neta

import (
	"context"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func (s *Session) controlLoop() {
	defer s.wg.Done()
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		now := time.Now()
		var completionTasks []callbackTask
		dispatchFailed := false
		s.dispatchMu.Lock()
		var closeCause error
		var forcedClose bool

		s.mu.Lock()
		if s.isClosingLocked() {
			s.mu.Unlock()
			return
		}

		if !s.preBindDeadline.IsZero() && !now.Before(s.preBindDeadline) &&
			(s.state == SessionStateNew || s.state == SessionStateBinding) {
			closeCause, forcedClose = ErrBindTimeout, true
		}
		if closeCause == nil && !s.bindDeadline.IsZero() && !now.Before(s.bindDeadline) && s.state == SessionStateBinding {
			closeCause, forcedClose = ErrBindTimeout, true
		}
		if closeCause == nil && !s.unbindDeadline.IsZero() && !now.Before(s.unbindDeadline) && s.state == SessionStateUnbinding {
			closeCause, forcedClose = ErrUnbindTimeout, true
		}
		if closeCause == nil && s.state == SessionStateBound && s.enquireLinkOutstanding && !s.enquireLinkDeadline.IsZero() && !now.Before(s.enquireLinkDeadline) {
			closeCause, forcedClose = newOperationError("enquire_link", context.DeadlineExceeded), true
		}

		if closeCause == nil {
			for sequence, pending := range s.pending {
				if pending.settled || now.Before(pending.deadline) {
					continue
				}
				pending.settled = true
				s.removeQueuedRequestLocked(sequence)
				completionTasks = append(completionTasks, s.completionTaskLocked(
					sequence, pending, Response{Sequence: sequence}, ErrRequestTimeout,
				))
			}
		}

		if closeCause == nil && s.state == SessionStateBound &&
			s.config.enquireLinkIdleInterval > 0 &&
			!s.enquireLinkQueued && !s.enquireLinkOutstanding &&
			!now.Before(s.lastActivity.Add(s.config.enquireLinkIdleInterval)) {
			sequence := s.nextSequenceLocked()
			if sequence == 0 {
				closeCause, forcedClose = ErrCapacity, true
			} else {
				frame, err := wire.EncodeEnquireLinkRequest(sequence)
				if err != nil {
					closeCause, forcedClose = err, true
				} else {
					s.enquireLinkSequence = sequence
					s.enquireLinkQueued = true
					item := &outboundFrame{
						bytes:      frame,
						control:    true,
						controlSeq: sequence,
						afterWrite: func() {
							s.mu.Lock()
							if s.state == SessionStateBound && s.enquireLinkQueued {
								s.enquireLinkQueued = false
								if s.enquireLinkResponseSeen {
									s.enquireLinkResponseSeen = false
									s.enquireLinkOutstanding = false
									s.enquireLinkDeadline = time.Time{}
								} else {
									s.enquireLinkOutstanding = true
									s.enquireLinkDeadline = time.Now().Add(s.config.enquireLinkResponseTimeout)
								}
							}
							s.mu.Unlock()
							s.signalControl()
						},
					}
					if err := s.enqueueControlLocked(item); err != nil {
						s.enquireLinkQueued = false
						closeCause, forcedClose = err, true
					}
				}
			}
		}

		next := s.nextDeadlineLocked()
		s.mu.Unlock()

		for _, task := range completionTasks {
			if !s.enqueueCallbackWithDispatchLock(callbackCompletion, task) {
				dispatchFailed = true
				break
			}
		}
		s.dispatchMu.Unlock()
		if dispatchFailed {
			s.beginClose(ErrCapacity, true)
			return
		}
		if closeCause != nil {
			s.beginClose(closeCause, forcedClose)
			return
		}

		delay := time.Hour
		if !next.IsZero() {
			delay = time.Until(next)
			if delay < 0 {
				delay = 0
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)

		select {
		case <-timer.C:
		case <-s.wake:
		case <-s.done:
			return
		}
	}
}

func (s *Session) nextDeadlineLocked() time.Time {
	var next time.Time
	consider := func(candidate time.Time) {
		if candidate.IsZero() {
			return
		}
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	if s.state == SessionStateNew || s.state == SessionStateBinding {
		consider(s.preBindDeadline)
		consider(s.bindDeadline)
	}
	if s.state == SessionStateUnbinding {
		consider(s.unbindDeadline)
	}
	if s.state == SessionStateBound && s.enquireLinkOutstanding {
		consider(s.enquireLinkDeadline)
	}
	for _, pending := range s.pending {
		if !pending.settled {
			consider(pending.deadline)
		}
	}
	if s.state == SessionStateBound && s.config.enquireLinkIdleInterval > 0 &&
		!s.enquireLinkQueued && !s.enquireLinkOutstanding {
		consider(s.lastActivity.Add(s.config.enquireLinkIdleInterval))
	}
	return next
}

func (s *Session) isSequenceInUseLocked(sequence uint32) bool {
	if _, exists := s.pending[sequence]; exists {
		return true
	}
	if s.state == SessionStateBinding && s.bindSequence == sequence {
		return true
	}
	if s.state == SessionStateUnbinding && s.unbindSequence == sequence {
		return true
	}
	return s.enquireLinkQueued && s.enquireLinkSequence == sequence ||
		s.enquireLinkOutstanding && s.enquireLinkSequence == sequence
}

func (s *Session) clearProbeLocked() {
	s.enquireLinkQueued = false
	s.enquireLinkOutstanding = false
	s.enquireLinkDeadline = time.Time{}
}
