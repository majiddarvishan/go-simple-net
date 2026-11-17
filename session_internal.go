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
// enqueue work and must never invoke user code inline. Owner dispatchers use
// bounded, owner-scoped worker pools.
// sessionCallbackDispatcher must provide nonblocking enqueue operations. Enqueue
// must never execute task.run inline and must preserve per-Session FIFO order
// within each callback class. cancelUnstarted removes only queued tasks and
// invokes their cancel functions exactly once; it never waits for running work.
type sessionCallbackDispatcher interface {
	enqueueApplication(callbackTask) bool
	enqueueCompletion(callbackTask) bool
	enqueueOnClose(callbackTask) bool
	cancelUnstarted(*Session, callbackClassMask)
}

type sessionHandlers struct {
	onBound         OnBoundHandler
	onRequest       OnRequestHandler
	onClose         OnCloseHandler
	onBoundInternal func(*Session)
	onDoneInternal  func(*Session, error)
}

type sessionIdentity struct {
	systemID string
	bindType wire.BindType
	role     sessionRole
}

type outboundFrame struct {
	bytes              []byte
	applicationCharged bool
	control            bool
	barrier            bool
	requestSeq         uint32
	controlSeq         uint32
	bindRequest        bool
	bindResponse       bool
	unbindRequest      bool
	afterWrite         func()
}

type pendingRequest struct {
	completion     CompletionHandler
	deadline       time.Time
	settled        bool
	writerClaimed  bool
	requestCredit  bool
	creditReleased bool
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

type inboundFrame struct {
	frame                 wire.Frame
	applicationCharge     *applicationReadCharge
	completionCharge      *completionReadCharge
	ignoreRequest         bool
	ignoreResponse        bool
	completionCapacityErr bool
}

// Session owns exactly one Reader and one serialized Writer for one TCP
// connection. All mutable protocol state is protected by mu.
type Session struct {
	config     sessionConfig
	conn       net.Conn
	remoteAddr net.Addr
	role       sessionRole
	resources  *ownerResourceLedger
	dispatcher sessionCallbackDispatcher
	handlers   sessionHandlers

	mu                        sync.Mutex
	dispatchMu                sync.Mutex
	cond                      *sync.Cond
	state                     SessionState
	systemID                  string
	bindType                  wire.BindType
	sequence                  wire.SequenceGenerator
	pending                   map[uint32]*pendingRequest
	appQueue                  []*outboundFrame
	controlQueue              []*outboundFrame
	barrierQueue              []*outboundFrame
	queuedAppBytes            uint64
	queuedAppFrames           uint32
	retainedApplicationBytes  uint64
	retainedCompletionBytes   uint64
	queuedIncomingBytes       uint64
	queuedIncomingFrames      uint32
	outstandingRequestCredits uint32
	admissionCount            uint32
	resourceAccountingFailed  bool
	queuedControlBytes        uint64
	activeWrite               *outboundFrame
	lastActivity              time.Time
	preBindDeadline           time.Time
	bindDeadline              time.Time
	unbindDeadline            time.Time
	enquireLinkDeadline       time.Time
	bindSequence              uint32
	unbindSequence            uint32
	enquireLinkSequence       uint32
	bindRequestWriting        bool
	bindRequestSent           bool
	bindResponseWriting       bool
	bindResponseSent          bool
	unbindRequestWriting      bool
	unbindRequestSent         bool
	enquireLinkWriting        bool
	enquireLinkQueued         bool
	enquireLinkOutstanding    bool
	enquireLinkResponseSeen   bool
	pendingBind               *bindRequestEvent
	bindRequests              chan bindRequestEvent
	closeCause                error
	closeForced               bool
	started                   bool

	wake      chan struct{}
	bound     chan struct{}
	boundOnce sync.Once
	done      chan struct{}
	closing   chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newSession(conn net.Conn, config sessionConfig, identity sessionIdentity, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) *Session {
	return newSessionWithResources(conn, config, identity, dispatcher, handlers, newClientResourceLedger(defaultOwnerConfig()))
}

func newSessionWithResources(conn net.Conn, config sessionConfig, identity sessionIdentity, dispatcher sessionCallbackDispatcher, handlers sessionHandlers, resources *ownerResourceLedger) *Session {
	if resources == nil {
		resources = newClientResourceLedger(defaultOwnerConfig())
	}
	s := &Session{
		config:       config,
		conn:         conn,
		role:         identity.role,
		resources:    resources,
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
		closing:      make(chan struct{}),
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
	return newBoundSessionWithResources(conn, config, identity, dispatcher, handlers, newClientResourceLedger(defaultOwnerConfig()))
}

func newBoundSessionWithResources(conn net.Conn, config sessionConfig, identity sessionIdentity, dispatcher sessionCallbackDispatcher, handlers sessionHandlers, resources *ownerResourceLedger) *Session {
	if conn == nil {
		return nil
	}
	s := newSessionWithResources(conn, config, identity, dispatcher, handlers, resources)
	s.mu.Lock()
	s.state = SessionStateBound
	s.lastActivity = time.Now()
	s.mu.Unlock()
	s.boundOnce.Do(func() { close(s.bound) })
	s.dispatchBound()
	s.start()
	return s
}

func newClientBindingSession(conn net.Conn, config sessionConfig, bind wire.Bind, bindTimeout time.Duration, dispatcher sessionCallbackDispatcher, handlers sessionHandlers) (*Session, error) {
	if conn == nil {
		return nil, newConfigError("connection", "<nil>", "must be non-nil")
	}
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
		err = s.enqueueControlLocked(&outboundFrame{bytes: frame, control: true, bindRequest: true})
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
	resources := newServerResourceLedger(defaultOwnerConfig(), defaultServerConfig())
	return newServerPreBindSessionWithResources(conn, config, acceptedAt, preBindTimeout, dispatcher, handlers, resources)
}

func newServerPreBindSessionWithResources(conn net.Conn, config sessionConfig, acceptedAt time.Time, preBindTimeout time.Duration, dispatcher sessionCallbackDispatcher, handlers sessionHandlers, resources *ownerResourceLedger) (*Session, error) {
	s, err := newServerPreBindSessionUnstarted(conn, config, acceptedAt, preBindTimeout, dispatcher, handlers, resources)
	if err != nil {
		return nil, err
	}
	s.start()
	s.signalControl()
	return s, nil
}

// newServerPreBindSessionUnstarted prepares a Pre-Bind Session without starting
// its Reader/Writer/Control goroutines. Server registers ownership before start.
func newServerPreBindSessionUnstarted(conn net.Conn, config sessionConfig, acceptedAt time.Time, preBindTimeout time.Duration, dispatcher sessionCallbackDispatcher, handlers sessionHandlers, resources *ownerResourceLedger) (*Session, error) {
	if conn == nil {
		return nil, newConfigError("connection", "<nil>", "must be non-nil")
	}
	if preBindTimeout <= 0 {
		return nil, newConfigError("prebind_handshake_timeout", preBindTimeout.String(), "must be a positive duration")
	}
	if acceptedAt.IsZero() {
		acceptedAt = time.Now()
	}
	s := newSessionWithResources(conn, config, sessionIdentity{role: sessionRoleServer}, dispatcher, handlers, resources)
	s.preBindDeadline = acceptedAt.Add(preBindTimeout)
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
	case <-s.closing:
		if err := s.Err(); err != nil {
			return bindRequestEvent{}, err
		}
		return bindRequestEvent{}, ErrClosed
	case <-ctx.Done():
		s.beginClose(ctx.Err(), true)
		<-s.done
		return bindRequestEvent{}, ctx.Err()
	}
}

// completeBind is called by the Server orchestration layer after its
// BindValidator and duplicate-SystemID admission checks have completed.
func (s *Session) completeBind(event bindRequestEvent, serverSystemID string, admissionErr error) error {
	s.mu.Lock()
	if s.state != SessionStateNew || s.pendingBind == nil || s.pendingBind.decision != event.decision {
		s.mu.Unlock()
		return ErrClosing
	}
	if !s.preBindDeadline.IsZero() && !time.Now().Before(s.preBindDeadline) {
		s.mu.Unlock()
		s.beginClose(ErrBindTimeout, true)
		return ErrBindTimeout
	}
	s.pendingBind = nil
	s.mu.Unlock()

	select {
	case event.decision <- bindDecision{serverSystemID: serverSystemID, err: admissionErr}:
		return nil
	case <-s.closing:
		return ErrClosed
	}
}

func (s *Session) markBound(systemID string, bindType wire.BindType) {
	s.dispatchMu.Lock()
	s.mu.Lock()
	if s.state != SessionStateBinding {
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		return
	}
	s.state = SessionStateBound
	s.systemID = systemID
	s.bindType = bindType
	s.lastActivity = time.Now()
	s.preBindDeadline = time.Time{}
	s.bindDeadline = time.Time{}
	s.mu.Unlock()

	if s.handlers.onBoundInternal != nil {
		s.handlers.onBoundInternal(s)
	}
	s.boundOnce.Do(func() { close(s.bound) })
	accepted := true
	if s.handlers.onBound != nil {
		task := callbackTask{
			session: s,
			run:     func() { s.handlers.onBound(s) },
			cancel:  func() {},
		}
		accepted = s.enqueueCallbackWithDispatchLock(callbackApplication, task)
	}
	s.dispatchMu.Unlock()
	s.signalControl()
	if !accepted {
		s.beginClose(ErrCapacity, true)
	}
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
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	return s.enqueueCallbackWithDispatchLock(class, task)
}

// enqueueCallbackWithDispatchLock requires dispatchMu. It serializes callback
// admission with Unbind and Forced Close state transitions.
func (s *Session) enqueueCallbackWithDispatchLock(class callbackClass, task callbackTask) bool {
	s.mu.Lock()
	forcedClose := s.state >= SessionStateClosing && s.closeForced
	s.mu.Unlock()
	if forcedClose && class != callbackOnClose {
		if task.cancel != nil {
			task.cancel()
		}
		return false
	}
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
		if !s.isSequenceInUseLocked(sequence) {
			return sequence
		}
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
	if item.barrier {
		s.barrierQueue = append(s.barrierQueue, item)
	} else {
		s.controlQueue = append(s.controlQueue, item)
	}
	s.cond.Signal()
	return nil
}

func (s *Session) reserveIncomingApplicationLocked(frameBytes uint64) (*applicationReadCharge, error) {
	if frameBytes == 0 {
		return nil, ErrProtocol
	}
	if s.queuedIncomingFrames >= s.config.maxIncomingApplicationFrames {
		return nil, ErrCapacity
	}
	nextQueueBytes, ok := checkedAddUint64(s.queuedIncomingBytes, frameBytes)
	if !ok || nextQueueBytes > s.config.maxQueuedIncomingBytes {
		return nil, ErrCapacity
	}
	nextRetainedBytes, ok := checkedAddUint64(s.retainedApplicationBytes, frameBytes)
	if !ok || nextRetainedBytes > s.config.maxRetainedApplicationBytes {
		return nil, ErrCapacity
	}
	if s.resources == nil {
		return nil, ErrCapacity
	}
	if err := s.resources.reserveApplicationFrame(frameBytes); err != nil {
		// Incoming overload always closes only this Session; callers do not
		// expose the Server-wide Queue Full distinction to the peer.
		return nil, ErrCapacity
	}
	s.queuedIncomingBytes = nextQueueBytes
	s.queuedIncomingFrames++
	s.retainedApplicationBytes = nextRetainedBytes
	return &applicationReadCharge{session: s, bytes: frameBytes}, nil
}

func (s *Session) reserveCompletionLocked(frameBytes uint64) *completionReadCharge {
	if frameBytes == 0 {
		return nil
	}
	nextRetainedBytes, ok := checkedAddUint64(s.retainedCompletionBytes, frameBytes)
	if !ok || nextRetainedBytes > s.config.maxRetainedCompletionBytes {
		return nil
	}
	if s.resources == nil || !s.resources.reserveRetainedCompletion(frameBytes) {
		return nil
	}
	s.retainedCompletionBytes = nextRetainedBytes
	return &completionReadCharge{session: s, bytes: frameBytes}
}

func (s *Session) reserveOutgoingApplicationLocked(frameBytes uint64) error {
	if frameBytes == 0 {
		return ErrProtocol
	}
	if s.queuedAppFrames >= s.config.maxOutgoingApplicationFrames {
		return ErrQueueFull
	}
	nextQueueBytes, ok := checkedAddUint64(s.queuedAppBytes, frameBytes)
	if !ok || nextQueueBytes > s.config.maxQueuedOutgoingBytes {
		return ErrQueueFull
	}
	nextRetainedBytes, ok := checkedAddUint64(s.retainedApplicationBytes, frameBytes)
	if !ok || nextRetainedBytes > s.config.maxRetainedApplicationBytes {
		return ErrCapacity
	}
	if s.resources == nil {
		return ErrCapacity
	}
	if err := s.resources.reserveApplicationFrame(frameBytes); err != nil {
		return err
	}
	s.queuedAppBytes = nextQueueBytes
	s.queuedAppFrames++
	s.retainedApplicationBytes = nextRetainedBytes
	return nil
}

// releaseOutgoingApplicationReservationLocked releases a reservation that has
// not yet been transferred to an outboundFrame.
func (s *Session) releaseOutgoingApplicationReservationLocked(frameBytes uint64) {
	if frameBytes == 0 {
		return
	}
	if next, ok := checkedSubUint64(s.queuedAppBytes, frameBytes); ok {
		s.queuedAppBytes = next
	} else {
		s.resourceAccountingFailed = true
	}
	if s.queuedAppFrames > 0 {
		s.queuedAppFrames--
	} else {
		s.resourceAccountingFailed = true
	}
	if next, ok := checkedSubUint64(s.retainedApplicationBytes, frameBytes); ok {
		s.retainedApplicationBytes = next
	} else {
		s.resourceAccountingFailed = true
	}
	queueReleased := s.resources != nil && s.resources.releaseQueuedApplication(frameBytes, 1)
	retainedReleased := s.resources != nil && s.resources.releaseRetainedApplication(frameBytes)
	if !queueReleased || !retainedReleased {
		s.resourceAccountingFailed = true
	}
}

func (s *Session) enqueueApplicationLocked(item *outboundFrame) error {
	if item == nil || item.control || len(item.bytes) == 0 || !item.applicationCharged {
		return ErrProtocol
	}
	s.appQueue = append(s.appQueue, item)
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
			s.resourceAccountingFailed = true
		}
		return
	}
	if !item.applicationCharged {
		return
	}
	s.releaseOutgoingApplicationReservationLocked(uint64(len(item.bytes)))
	item.applicationCharged = false
}

func (s *Session) reserveRequestCreditLocked() error {
	if s.outstandingRequestCredits >= s.config.maxOutstandingRequests {
		return ErrCapacity
	}
	if s.resources == nil || !s.resources.reserveRequest() {
		return ErrCapacity
	}
	s.outstandingRequestCredits++
	return nil
}

func (s *Session) releaseRequestAdmissionLocked() {
	if s.outstandingRequestCredits > 0 {
		s.outstandingRequestCredits--
	} else {
		s.resourceAccountingFailed = true
	}
	if s.resources == nil || !s.resources.releaseRequest() {
		s.resourceAccountingFailed = true
	}
}

func (s *Session) releaseRequestCreditLocked(pending *pendingRequest) {
	if pending == nil || !pending.requestCredit || pending.creditReleased {
		return
	}
	pending.creditReleased = true
	s.releaseRequestAdmissionLocked()
}

func (s *Session) finishAdmissionLocked() {
	if s.admissionCount == 0 {
		s.resourceAccountingFailed = true
		return
	}
	s.admissionCount--
	s.cond.Broadcast()
}

func (s *Session) removeQueuedControlSequenceLocked(sequence uint32) bool {
	if sequence == 0 {
		return false
	}
	for i, item := range s.controlQueue {
		if item.controlSeq != sequence {
			continue
		}
		s.releaseFrameChargeLocked(item)
		copy(s.controlQueue[i:], s.controlQueue[i+1:])
		s.controlQueue[len(s.controlQueue)-1] = nil
		s.controlQueue = s.controlQueue[:len(s.controlQueue)-1]
		s.cond.Broadcast()
		return true
	}
	return false
}

func (s *Session) removeQueuedRequestLocked(sequence uint32) bool {
	for i, item := range s.appQueue {
		if item.requestSeq != sequence {
			continue
		}
		s.releaseFrameChargeLocked(item)
		copy(s.appQueue[i:], s.appQueue[i+1:])
		s.appQueue[len(s.appQueue)-1] = nil
		s.appQueue = s.appQueue[:len(s.appQueue)-1]
		s.cond.Broadcast()
		return true
	}
	return false
}

func (s *Session) isClosingLocked() bool {
	return s.state >= SessionStateClosing
}

func (s *Session) beginClose(cause error, forced bool) {
	s.closeOnce.Do(func() {
		s.dispatchMu.Lock()
		s.mu.Lock()
		s.state = SessionStateClosing
		s.closeCause = cause
		s.closeForced = forced
		close(s.closing)

		var gracefulTasks []callbackTask
		if forced {
			for _, pending := range s.pending {
				if !pending.settled {
					pending.settled = true
					s.releaseRequestCreditLocked(pending)
				}
			}
		} else {
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

		for _, item := range s.appQueue {
			s.releaseFrameChargeLocked(item)
		}
		for _, item := range s.controlQueue {
			s.releaseFrameChargeLocked(item)
		}
		for _, item := range s.barrierQueue {
			s.releaseFrameChargeLocked(item)
		}
		s.appQueue = nil
		s.controlQueue = nil
		s.barrierQueue = nil
		s.preBindDeadline = time.Time{}
		s.bindDeadline = time.Time{}
		s.unbindDeadline = time.Time{}
		s.enquireLinkDeadline = time.Time{}
		s.clearProbeLocked()
		s.bindRequestWriting = false
		s.bindRequestSent = false
		s.bindResponseWriting = false
		s.bindResponseSent = false
		s.unbindRequestWriting = false
		s.unbindRequestSent = false
		s.pendingBind = nil
		s.cond.Broadcast()
		conn := s.conn
		s.mu.Unlock()

		if conn != nil {
			_ = conn.Close()
		}
		if forced && s.dispatcher != nil {
			s.dispatcher.cancelUnstarted(s, callbackMaskApplication|callbackMaskCompletion)
		}
		for _, task := range gracefulTasks {
			s.enqueueCallbackWithDispatchLock(callbackCompletion, task)
		}
		s.dispatchMu.Unlock()
		s.signalControl()
		go s.finishClose()
	})
}

func (s *Session) finishClose() {
	s.wg.Wait()
	s.mu.Lock()
	for s.admissionCount != 0 {
		s.cond.Wait()
	}
	s.state = SessionStateClosed
	cause := s.closeCause
	s.pending = make(map[uint32]*pendingRequest)
	s.appQueue = nil
	s.controlQueue = nil
	s.barrierQueue = nil
	s.activeWrite = nil
	s.mu.Unlock()

	if s.handlers.onDoneInternal != nil {
		s.handlers.onDoneInternal(s, cause)
	}
	close(s.done)
	if s.handlers.onClose != nil && s.resources != nil && s.resources.reserveOnCloseEvent() {
		credit := &onCloseEventCharge{owner: s.resources}
		task := callbackTask{
			session: s,
			run: func() {
				defer credit.release()
				s.handlers.onClose(s, cause)
			},
			cancel: credit.release,
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
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		terminal = newOperationError("frame assembly", context.DeadlineExceeded)
	} else {
		terminal = newOperationError("read", err)
	}
	s.beginClose(terminal, true)
}
