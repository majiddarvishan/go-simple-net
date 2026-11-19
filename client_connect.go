package neta

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

const (
	reconnectBaseWindow = time.Second
	reconnectMaxWindow  = 30 * time.Second
	reconnectStableAge  = 30 * time.Second
)

type clientReconnectEvent struct {
	session       *Session
	boundDuration time.Duration
	dialFailure   bool
}

// Connect performs one TCP Dial and neta-cpp Bind attempt. It returns only after
// the Client is Bound or the attempt fails. Context cancellation after a
// successful return does not affect the Session.
func (c *Client) Connect(ctx context.Context) error {
	if c == nil {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.connectGate:
	}
	defer func() { c.connectGate <- struct{}{} }()

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrClosed
	}
	current := c.current
	c.mu.Unlock()
	if current != nil && current.State() == SessionStateBound {
		return nil
	}

	err, retryable := c.connectAttempt(ctx)
	if err != nil {
		if retryable && c.config.autoReconnect {
			c.startReconnectSupervisor()
			c.signalReconnect(clientReconnectEvent{dialFailure: true})
		}
		return err
	}
	if c.config.autoReconnect {
		c.startReconnectSupervisor()
	}
	return nil
}

// connectAttempt runs while the caller owns connectGate. Only Dial failures
// are retryable before the first Bound generation; Bind rejection and Bind
// timeout are deliberately terminal for that attempt.
func (c *Client) connectAttempt(ctx context.Context) (error, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		c.mu.Lock()
		if c.stopped {
			c.mu.Unlock()
			return ErrClosed, false
		}
		current := c.current
		c.mu.Unlock()
		if current == nil {
			break
		}
		if current.State() == SessionStateBound {
			return nil, false
		}
		select {
		case <-ctx.Done():
			return ctx.Err(), false
		case <-current.Done():
		}
		c.mu.Lock()
		if c.current == current {
			c.current = nil
		}
		c.mu.Unlock()
	}

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return ErrClosed, false
	}
	c.generation++
	generation := c.generation
	attemptCtx, cancel := context.WithCancel(ctx)
	attemptDone := make(chan struct{})
	c.connectCancel = cancel
	c.connectDone = attemptDone
	c.mu.Unlock()

	defer func() {
		cancel()
		c.mu.Lock()
		if c.connectDone == attemptDone {
			c.connectCancel = nil
			c.connectDone = nil
		}
		close(attemptDone)
		c.mu.Unlock()
	}()

	dialCtx, dialCancel := context.WithTimeout(attemptCtx, c.config.connectTimeout)
	conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", c.config.address)
	dialContextErr := dialCtx.Err()
	dialCancel()

	if ctx.Err() != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return ctx.Err(), false
	}
	if attemptCtx.Err() != nil {
		if conn != nil {
			_ = conn.Close()
		}
		c.mu.Lock()
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return ErrClosed, false
		}
		return context.Canceled, false
	}
	if errors.Is(dialContextErr, context.DeadlineExceeded) {
		if conn != nil {
			_ = conn.Close()
		}
		return newOperationError("dial", ErrConnectTimeout), true
	}
	if dialErr != nil {
		return newOperationError("dial", dialErr), true
	}
	if conn == nil {
		return newOperationError("dial", ErrConnectTimeout), true
	}

	handlers := sessionHandlers{
		onBound:   c.config.onBound,
		onRequest: c.config.onRequest,
		onClose:   c.config.onClose,
		onBoundInternal: func(session *Session) {
			c.clientSessionBound(generation, session)
		},
		onDoneInternal: func(session *Session, cause error) {
			c.clientSessionDone(generation, session, cause)
		},
	}
	session, err := newClientBindingSessionWithResources(
		conn,
		c.config.session,
		wire.Bind{
			Type:     wire.BindType(c.config.bindType),
			SystemID: c.config.systemID,
		},
		c.config.bindResponseTimeout,
		c.callbacks,
		handlers,
		c.resources,
	)
	if err != nil {
		return err, false
	}

	if err := session.waitBound(attemptCtx); err != nil {
		if ctx.Err() != nil {
			session.Close()
			<-session.Done()
			return ctx.Err(), false
		}
		c.mu.Lock()
		stopped := c.stopped
		c.mu.Unlock()
		if stopped || errors.Is(attemptCtx.Err(), context.Canceled) {
			session.Close()
			<-session.Done()
			return ErrClosed, false
		}
		cause := session.Err()
		if cause == nil {
			cause = err
		}
		return cause, false
	}

	if session.State() != SessionStateBound {
		cause := session.Err()
		if cause == nil {
			cause = ErrClosed
		}
		return cause, false
	}
	c.mu.Lock()
	currentGeneration := !c.stopped &&
		c.generation == generation &&
		c.boundGeneration == generation &&
		c.current == session
	c.mu.Unlock()
	if !currentGeneration {
		session.Close()
		<-session.Done()
		return ErrClosed, false
	}
	return nil, false
}

func (c *Client) clientSessionBound(generation uint64, session *Session) {
	c.mu.Lock()
	if c.stopped || c.generation != generation {
		c.mu.Unlock()
		session.Close()
		return
	}
	c.current = session
	c.boundGeneration = generation
	c.boundAt = time.Now()
	c.mu.Unlock()
}

func (c *Client) clientSessionDone(generation uint64, session *Session, cause error) {
	c.mu.Lock()
	wasBound := c.boundGeneration == generation
	var boundDuration time.Duration
	if wasBound {
		boundDuration = time.Since(c.boundAt)
		c.boundGeneration = 0
		c.boundAt = time.Time{}
	}
	retry := wasBound && !c.stopped && c.config.autoReconnect && retryableBoundSessionClose(cause)
	c.mu.Unlock()

	// finishClose invokes this hook immediately before closing Session.Done.
	// Keep the retiring generation visible internally until Done is published,
	// so a manual Connect cannot overlap its replacement Bind with old teardown.
	go func() {
		<-session.Done()
		c.mu.Lock()
		if c.current == session && c.generation == generation {
			c.current = nil
		}
		c.mu.Unlock()
		if retry {
			c.startReconnectSupervisor()
			c.signalReconnect(clientReconnectEvent{
				session:       session,
				boundDuration: boundDuration,
			})
		}
	}()
}

func retryableBoundSessionClose(cause error) bool {
	if cause == nil ||
		errors.Is(cause, ErrClosed) ||
		errors.Is(cause, ErrClosing) ||
		errors.Is(cause, ErrBindRejected) ||
		errors.Is(cause, ErrBindTimeout) ||
		errors.Is(cause, ErrUnbindTimeout) {
		return false
	}
	var operation *OperationError
	if !errors.As(cause, &operation) {
		return false
	}
	switch operation.Operation() {
	case "read", "write", "frame assembly", "enquire_link":
		return true
	default:
		return false
	}
}

func (c *Client) startReconnectSupervisor() {
	c.mu.Lock()
	if c.stopped || !c.config.autoReconnect || c.reconnectStarted {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	c.reconnectStarted = true
	c.reconnectCancel = cancel
	c.reconnectDone = done
	c.mu.Unlock()

	go c.reconnectLoop(ctx, done)
}

func (c *Client) reconnectLoop(ctx context.Context, done chan struct{}) {
	defer func() {
		c.mu.Lock()
		if c.reconnectDone == done {
			close(done)
			c.reconnectDone = nil
			c.reconnectCancel = nil
			c.reconnectStarted = false
		} else {
			close(done)
		}
		c.mu.Unlock()
	}()

	window := reconnectBaseWindow
	pendingRetry := false
	for {
		if !pendingRetry {
			event, ok := c.waitReconnectEvent(ctx)
			if !ok {
				return
			}
			if event.boundDuration >= reconnectStableAge {
				window = reconnectBaseWindow
			}
			pendingRetry = true
		}

		if !c.waitReconnectDelay(ctx, &window) {
			return
		}
		if c.hasBoundSession() {
			pendingRetry = false
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-c.connectGate:
		}
		err, retryable := c.connectAttempt(ctx)
		c.connectGate <- struct{}{}
		if err == nil {
			pendingRetry = false
			continue
		}
		if !retryable {
			return
		}
		window = nextReconnectWindow(window)
		pendingRetry = true
	}
}

func (c *Client) waitReconnectEvent(ctx context.Context) (clientReconnectEvent, bool) {
	select {
	case <-ctx.Done():
		return clientReconnectEvent{}, false
	case event := <-c.reconnectWake:
		if event.session != nil {
			select {
			case <-ctx.Done():
				return clientReconnectEvent{}, false
			case <-event.session.Done():
			}
		}
		return event, true
	}
}

func (c *Client) waitReconnectDelay(ctx context.Context, window *time.Duration) bool {
	delay := fullJitter(*window)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case event := <-c.reconnectWake:
			if event.session != nil {
				select {
				case <-ctx.Done():
					return false
				case <-event.session.Done():
				}
			}
			if event.boundDuration >= reconnectStableAge {
				*window = reconnectBaseWindow
			}
		case <-timer.C:
			return true
		}
	}
}

func (c *Client) signalReconnect(event clientReconnectEvent) {
	select {
	case c.reconnectWake <- event:
		return
	default:
	}
	// Coalesce stale events while preserving the newest generation signal.
	select {
	case <-c.reconnectWake:
	default:
	}
	select {
	case c.reconnectWake <- event:
	default:
	}
}

func (c *Client) hasBoundSession() bool {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return false
	}
	session := c.current
	c.mu.Unlock()
	return session != nil && session.State() == SessionStateBound
}

func fullJitter(window time.Duration) time.Duration {
	if window <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(window)))
}

func nextReconnectWindow(window time.Duration) time.Duration {
	if window >= reconnectMaxWindow/2 {
		return reconnectMaxWindow
	}
	return window * 2
}
