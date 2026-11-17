package neta

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

type sessionRole uint8

const (
	sessionRoleClient sessionRole = iota + 1
	sessionRoleServer
)

type callbackClass uint8

const (
	callbackApplication callbackClass = iota + 1
	callbackCompletion
	callbackOnClose
)

type callbackClassMask uint8

const (
	callbackMaskApplication callbackClassMask = 1 << iota
	callbackMaskCompletion
)

type callbackTask struct {
	session *Session
	run     func()
	cancel  func()
}

// sessionCallbackDispatcher is deliberately private. Implementations must only
// enqueue work and must never invoke user code inline. Phase 5 supplies the
// bounded, owner-scoped worker pools.
type sessionCallbackDispatcher interface {
	enqueueApplication(callbackTask) bool
	enqueueCompletion(callbackTask) bool
	enqueueOnClose(callbackTask) bool
	cancelUnstarted(*Session, callbackClassMask)
}

type sessionHandlers struct {
	onBound   OnBoundHandler
	onRequest OnRequestHandler
	onClose   OnCloseHandler
}

type sessionIdentity struct {
	systemID string
	bindType wire.BindType
	role     sessionRole
}

type outboundFrame struct {
	bytes      []byte
	control    bool
	requestSeq uint32
	afterWrite func()
}

type pendingRequest struct {
	sequence   uint32
	completion CompletionHandler
	deadline   time.Time
	settled    bool
	writerClaimed bool
}

type bindDecision struct {
	serverSystemID string
	err            error
}

type bindRequestEvent struct {
	info     BindInfo
	sequence uint32
	decision chan bindDecision
}

// Session owns exactly one Reader and one serialized Writer for one TCP
// connection. All mutable protocol state is protected by mu.
type Session struct {
	config     sessionConfig
	conn       net.Conn
	remoteAddr net.Addr
	role       sessionRole
	dispatcher sessionCallbackDispatcher
	handlers   sessionHandlers

	mu                  sync.Mutex
	cond                *sync.Cond
	state               SessionState
	systemID            string
	bindType            wire.BindType
	sequence            wire.SequenceGenerator
	pending             map[uint32]*pendingRequest
	outQueue            []*outboundFrame
	queuedAppBytes      uint64
	queuedAppFrames     uint32
	queuedControlBytes  uint64
	activeWrite         *outboundFrame
	lastActivity        time.Time
	preBindDeadline     time.Time
	bindDeadline        time.Time
	unbindDeadline      time.Time
	enquireLinkDeadline time.Time
	bindSequence        uint32
	unbindSequence      uint32
	enquireLinkSequence uint32
	enquireLinkQueued   bool
	enquireLinkOutstanding bool
	pendingBind         *bindRequestEvent
	bindRequests        chan bindRequestEvent
	closeCause          error
	closeForced         bool
	started             bool

	wake      chan struct{}
	bound     chan struct{}
	boundOnce sync.Once
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newSession(conn net.Conn, config sessionConfig, identity sessionIdentity, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) *Session {
	s := &Session{
		config:       config,
		conn:         conn,
		role:         identity.role,
		dispatcher:   dispatcher,
		handlers:     handlers,
		state:        SessionStateNew,
		systemID:     identity.systemID,
		bindType:     identity.bindType,
		pending:      make(map[uint32]*pendingRequest),
		bindRequests: make(chan bindRequestEvent, 1),
		wake:         make(chan struct{}, 1),
		bound:        make(chan struct{}),
		done:         make(chan struct{}),
	}
	if conn != nil {
		s.remoteAddr = conn.RemoteAddr()
	}
	s.cond = sync.NewCond(&s.mu)
	if identity.role == sessionRoleClient || identity.role == sessionRoleServer {
		// A zero bind type is a valid uni-directional Bind. Callers that create
		// an already-bound Session must provide the negotiated type explicitly.
	}
	return s
}

func newBoundSession(conn net.Conn, config sessionConfig, identity sessionIdentity, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) *Session {
	s := newSession(conn, config, identity, dispatcher, handlers)
	s.mu.Lock()
	s.state = SessionStateBound
	s.lastActivity = time.Now()
	s.mu.Unlock()
	s.boundOnce.Do(func() { close(s.bound) })
	s.start()
	s.dispatchBound()
	return s
}

func newClientBindingSession(conn net.Conn, config sessionConfig, bind wire.Bind, bindTimeout time.Duration, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) (*Session, error) {
	if bindTimeout <= 0 {
		return nil, newConfigError("bind_response_timeout", bindTimeout.String(), "must be a positive duration")
	}
	sequence := uint32(1)
	frame, err := wire.EncodeBindRequest(sequence, bind)
	if err != nil {
		return nil, err
	}
	s := newSession(conn, config, sessionIdentity{bindType: bind.Type, role: sessionRoleClient}, dispatcher, handlers)
	s.mu.Lock()
	sequence = s.nextSequenceLocked()
	frame, err = wire.EncodeBindRequest(sequence, bind)
	if err == nil {
		s.state = SessionStateBinding
		s.bindSequence = sequence
		s.bindDeadline = time.Now().Add(bindTimeout)
		err = s.enqueueControlLocked(&outboundFrame{bytes: frame, control: true})
	}
	s.mu.Unlock()
	if err != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return nil, err
	}
	s.start()
	s.signalControl()
	return s, nil
}

func newServerPreBindSession(conn net.Conn, config sessionConfig, acceptedAt time.Time, preBindTimeout time.Duration, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) (*Session, error) {
	if preBindTimeout <= 0 {
		return nil, newConfigError("prebind_handshake_timeout", preBindTimeout.String(), "must be a positive duration")
	}
	if acceptedAt.IsZero() {
		acceptedAt = time.Now()
	}
	s := newSession(conn, config, sessionIdentity{role: sessionRoleServer}, dispatcher, handlers)
	s.preBindDeadline = acceptedAt.Add(preBindTimeout)
	s.start()
	s.signalControl()
	return s, nil
}

func (s *Session) start() {
	s.mu.Lock()
	if s.started || s.state >= SessionStateClosing {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(3)
	s.mu.Unlock()

	go s.readerLoop()
	go s.writerLoop()
	go s.controlLoop()
}

func (s *Session) waitBound(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-s.bound:
		return nil
	case <-s.done:
		return s.Err()
	case <-ctx.Done():
		s.beginClose(ctx.Err(), true)
		<-s.done
		return ctx.Err()
	}
}

func (s *Session) waitBindRequest(ctx context.Context) (bindRequestEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case event := <-s.bindRequests:
		return event, nil
	case <-s.done:
		return bindRequestEvent{}, s.Err()
	case <-ctx.Done():
		return bindRequestEvent{}, ctx.Err()
	}
}

// completeBind is called by the Server orchestration layer after its
// BindValidator and duplicate-SystemID admission checks have completed.
func (s *Session) completeBind(event bindRequestEvent, serverSystemID string, admissionErr error) error {
	s.mu.Lock()
	if s.state != SessionStateNew || s.pendingBind != &event {
		s.mu.Unlock()
		return ErrClosing
	}
	s.pendingBind = nil
	s.mu.Unlock()

	select {
	case event.decision <- bindDecision{serverSystemID: serverSystemID, err: admissionErr}:
		return nil
	case <-s.done:
		return ErrClosed
	}
}

func (s *Session) markBound(systemID string, bindType wire.BindType) {
	s.mu.Lock()
	if s.state != SessionStateBinding {
		s.mu.Unlock()
		return
	}
	s.state = SessionStateBound
	s.systemID = systemID
	s.bindType = bindType
	s.lastActivity = time.Now()
	s.preBindDeadline = time.Time{}
	s.bindDeadline = time.Time{}
	s.mu.Unlock()

	s.boundOnce.Do(func() { close(s.bound) })
	s.signalControl()
	s.dispatchBound()
}

func (s *Session) dispatchBound() {
	if s.handlers.onBound == nil {
		return
	}
	task := callbackTask{
		session: s,
		run:     func() { s.handlers.onBound(s) },
		cancel:  func() {},
	}
	if !s.enqueueCallback(callbackApplication, task) {
		s.beginClose(ErrCapacity, true)
	}
}

func (s *Session) enqueueCallback(class callbackClass, task callbackTask) bool {
	if s.dispatcher == nil {
		if task.cancel != nil {
			task.cancel()
		}
		return false
	}
	var accepted bool
	switch class {
	case callbackApplication:
		accepted = s.dispatcher.enqueueApplication(task)
	case callbackCompletion:
		accepted = s.dispatcher.enqueueCompletion(task)
	case callbackOnClose:
		accepted = s.dispatcher.enqueueOnClose(task)
	}
	if !accepted && task.cancel != nil {
		task.cancel()
	}
	return accepted
}

func (s *Session) signalControl() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Session) nextSequenceLocked() uint32 {
	// At most the bounded per-Session Request population plus three control
	// sequences can be live. The extra attempts cover a wraparound collision.
	for attempts := 0; attempts <= len(s.pending)+4; attempts++ {
		sequence := s.sequence.Next()
		if _, exists := s.pending[sequence]; exists {
			continue
		}
		if sequence == s.bindSequence && s.state == SessionStateBinding {
			continue
		}
		if sequence == s.unbindSequence && s.state == SessionStateUnbinding {
			continue
		}
		if sequence == s.enquireLinkSequence && (s.enquireLinkQueued || s.enquireLinkOutstanding) {
			continue
		}
		return sequence
	}
	return 0
}

func (s *Session) enqueueControlLocked(item *outboundFrame) error {
	if item == nil || !item.control || len(item.bytes) == 0 {
		return ErrProtocol
	}
	next, ok := checkedAddUint64(s.queuedControlBytes, uint64(len(item.bytes)))
	if !ok || next > s.config.maxQueuedControlBytes {
		return ErrCapacity
	}
	s.queuedControlBytes = next
	s.outQueue = append(s.outQueue, item)
	s.cond.Signal()
	return nil
}

func (s *Session) enqueueApplicationLocked(item *outboundFrame) error {
	if item == nil || item.control || len(item.bytes) == 0 {
		return ErrProtocol
	}
	if s.queuedAppFrames >= s.config.maxOutgoingApplicationFrames {
		return ErrQueueFull
	}
	next, ok := checkedAddUint64(s.queuedAppBytes, uint64(len(item.bytes)))
	if !ok || next > s.config.maxQueuedOutgoingBytes {
		return ErrQueueFull
	}
	s.queuedAppBytes = next
	s.queuedAppFrames++
	s.outQueue = append(s.outQueue, item)
	s.cond.Signal()
	return nil
}

func (s *Session) releaseFrameChargeLocked(item *outboundFrame) {
	if item == nil {
		return
	}
	if item.control {
		if next, ok := checkedSubUint64(s.queuedControlBytes, uint64(len(item.bytes))); ok {
			s.queuedControlBytes = next
		} else {
			s.queuedControlBytes = 0
		}
		return
	}
	if next, ok := checkedSubUint64(s.queuedAppBytes, uint64(len(item.bytes))); ok {
		s.queuedAppBytes = next
	} else {
		s.queuedAppBytes = 0
	}
	if s.queuedAppFrames > 0 {
		s.queuedAppFrames--
	}
}

func (s *Session) removeQueuedRequestLocked(sequence uint32) bool {
	for i, item := range s.outQueue {
		if item.requestSeq != sequence {
			continue
		}
		s.releaseFrameChargeLocked(item)
		copy(s.outQueue[i:], s.outQueue[i+1:])
		s.outQueue[len(s.outQueue)-1] = nil
		s.outQueue = s.outQueue[:len(s.outQueue)-1]
		s.cond.Broadcast()
		return true
	}
	return false
}

func (s *Session) isClosingLocked() bool {
	return s.state >= SessionStateClosing
}

func (s *Session) closeErrorLocked() error {
	if s.closeCause != nil {
		return s.closeCause
	}
	return ErrClosed
}

func (s *Session) beginClose(cause error, forced bool) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.state = SessionStateClosing
		s.closeCause = cause
		s.closeForced = forced

		var gracefulTasks []callbackTask
		if !forced {
			terminalErr := cause
			if terminalErr == nil {
				terminalErr = ErrClosed
			}
			for sequence, pending := range s.pending {
				if pending.settled {
					continue
				}
				pending.settled = true
				gracefulTasks = append(gracefulTasks, s.completionTaskLocked(sequence, pending, Response{Sequence: sequence}, terminalErr))
			}
		}

		for _, item := range s.outQueue {
			s.releaseFrameChargeLocked(item)
		}
		s.outQueue = nil
		s.preBindDeadline = time.Time{}
		s.bindDeadline = time.Time{}
		s.unbindDeadline = time.Time{}
		s.enquireLinkDeadline = time.Time{}
		s.pendingBind = nil
		s.cond.Broadcast()
		conn := s.conn
		s.mu.Unlock()

		if conn != nil {
			_ = conn.Close()
		}
		s.signalControl()

		if forced && s.dispatcher != nil {
			s.dispatcher.cancelUnstarted(s, callbackMaskApplication|callbackMaskCompletion)
		}
		for _, task := range gracefulTasks {
			if !s.enqueueCallback(callbackCompletion, task) {
				// The task's cancellation hook has already released its internal
				// Request entry. The terminal engine must still finish teardown.
			}
		}
		go s.finishClose()
	})
}

func (s *Session) finishClose() {
	s.wg.Wait()
	s.mu.Lock()
	s.state = SessionStateClosed
	cause := s.closeCause
	s.pending = make(map[uint32]*pendingRequest)
	s.outQueue = nil
	s.activeWrite = nil
	s.mu.Unlock()

	close(s.done)
	if s.handlers.onClose != nil {
		task := callbackTask{
			session: s,
			run:     func() { s.handlers.onClose(s, cause) },
			cancel:  func() {},
		}
		// OnClose is best-effort and is eligible only after Done closes.
		s.enqueueCallback(callbackOnClose, task)
	}
}

func (s *Session) failRead(err error) {
	if err == nil {
		err = ErrProtocol
	}
	var terminal error
	if errors.Is(err, context.DeadlineExceeded) {
		terminal = newOperationError("frame assembly", context.DeadlineExceeded)
	} else {
		terminal = newOperationError("read", err)
	}
	s.beginClose(terminal, true)
}
