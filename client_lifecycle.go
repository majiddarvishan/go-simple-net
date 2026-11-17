package neta

import "context"

// CurrentSession returns the current Bound Session generation. It returns
// (nil, false) while disconnected, connecting, unbinding or stopped.
func (c *Client) CurrentSession() (*Session, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil, false
	}
	session := c.current
	c.mu.Unlock()

	if session == nil || session.State() != SessionStateBound {
		if session != nil {
			c.mu.Lock()
			if c.current == session {
				c.current = nil
			}
			c.mu.Unlock()
		}
		return nil, false
	}

	c.mu.Lock()
	current := !c.stopped && c.current == session
	c.mu.Unlock()
	if !current {
		return nil, false
	}
	return session, true
}

// Stop permanently stops the Client. It cancels an in-progress Connect and
// reconnect supervisor, Gracefully Unbinds the current Bound Session, and
// waits for internal Done publication rather than user callbacks.
func (c *Client) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	if c.stopDone != nil {
		done := c.stopDone
		c.mu.Unlock()
		select {
		case <-done:
			c.mu.Lock()
			err := c.stopErr
			c.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.stopDone = make(chan struct{})
	stopDone := c.stopDone
	c.stopped = true
	connectCancel := c.connectCancel
	connectDone := c.connectDone
	reconnectCancel := c.reconnectCancel
	c.mu.Unlock()

	if connectCancel != nil {
		connectCancel()
	}
	if reconnectCancel != nil {
		reconnectCancel()
	}

	var stopErr error
	if connectDone != nil {
		select {
		case <-connectDone:
		case <-ctx.Done():
			stopErr = ctx.Err()
			// Connect owns the attempt goroutine. Its context has been cancelled;
			// wait for it to settle before observing the final current generation.
			<-connectDone
		}
	}

	c.mu.Lock()
	session := c.current
	c.mu.Unlock()

	if session != nil {
		switch session.State() {
		case SessionStateBound, SessionStateUnbinding:
			if err := session.Unbind(ctx); err != nil && stopErr == nil {
				stopErr = err
			}
		default:
			session.Close()
		}

		select {
		case <-session.Done():
		case <-ctx.Done():
			stopErr = ctx.Err()
			session.Close()
			<-session.Done()
		}
	}

	if ctx.Err() != nil {
		stopErr = ctx.Err()
	}

	c.mu.Lock()
	if c.current == session {
		c.current = nil
	}
	c.stopErr = stopErr
	close(stopDone)
	c.mu.Unlock()
	return stopErr
}
