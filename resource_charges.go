package neta

import "sync"

// applicationReadCharge follows one inbound Stream Request from its validated
// Header reservation through callback return/cancellation. Queue-only counters
// are released when the callback starts; retained-payload bytes follow the
// owned Frame until the callback actually returns.
type applicationReadCharge struct {
	session          *Session
	bytes            uint64
	queueReleased    bool
	retainedReleased bool
}

func (c *applicationReadCharge) releaseQueue() {
	if c == nil || c.session == nil {
		return
	}
	s := c.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.queueReleased {
		return
	}
	c.queueReleased = true
	if next, ok := checkedSubUint64(s.queuedIncomingBytes, c.bytes); ok {
		s.queuedIncomingBytes = next
	} else {
		s.resourceAccountingFailed = true
	}
	if s.queuedIncomingFrames > 0 {
		s.queuedIncomingFrames--
	} else {
		s.resourceAccountingFailed = true
	}
	if s.resources == nil || !s.resources.releaseQueuedApplication(c.bytes, 1) {
		s.resourceAccountingFailed = true
	}
}

func (c *applicationReadCharge) releaseRetained() {
	if c == nil || c.session == nil {
		return
	}
	s := c.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.retainedReleased {
		return
	}
	c.retainedReleased = true
	if next, ok := checkedSubUint64(s.retainedApplicationBytes, c.bytes); ok {
		s.retainedApplicationBytes = next
	} else {
		s.resourceAccountingFailed = true
	}
	if s.resources == nil || !s.resources.releaseRetainedApplication(c.bytes) {
		s.resourceAccountingFailed = true
	}
}

func (c *applicationReadCharge) releaseAll() {
	if c == nil {
		return
	}
	c.releaseQueue()
	c.releaseRetained()
}

// completionReadCharge accounts a matched inbound Response from before Body
// allocation until its Completion callback returns or is cancelled.
type completionReadCharge struct {
	session  *Session
	bytes    uint64
	released bool
}

func (c *completionReadCharge) release() {
	if c == nil || c.session == nil {
		return
	}
	s := c.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.released {
		return
	}
	c.released = true
	if next, ok := checkedSubUint64(s.retainedCompletionBytes, c.bytes); ok {
		s.retainedCompletionBytes = next
	} else {
		s.resourceAccountingFailed = true
	}
	if s.resources == nil || !s.resources.releaseRetainedCompletion(c.bytes) {
		s.resourceAccountingFailed = true
	}
}

// onCloseEventCharge owns one pre-admission reserved owner credit. Its
// release is idempotent across admission rollback and callback completion.
type onCloseEventCharge struct {
	owner *ownerResourceLedger
	once  sync.Once
}

func (c *onCloseEventCharge) release() {
	if c == nil || c.owner == nil {
		return
	}
	c.once.Do(func() {
		_ = c.owner.releaseOnCloseEvent()
	})
}

func (f inboundFrame) releaseAll() {
	f.applicationCharge.releaseAll()
	f.completionCharge.release()
}
