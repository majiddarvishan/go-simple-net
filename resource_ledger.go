package neta

import "sync"

// ownerResourceLimits are immutable admission ceilings owned by one Client or
// Server instance. A Client has no aggregate queued-Application-frame ceiling;
// a Server has the Q-010 aggregate byte and WORKER-007 frame ceilings.
type ownerResourceLimits struct {
	maxOutstandingRequests              uint32
	maxRetainedApplicationBytes         uint64
	maxRetainedCompletionBytes          uint64
	maxOnCloseEventCredits              uint32
	maxAggregateQueuedApplicationBytes  uint64
	maxAggregateQueuedApplicationFrames uint32
	hasAggregateApplicationQueue        bool
}

type ownerResourceUsage struct {
	outstandingRequests      uint32
	retainedApplicationBytes uint64
	retainedCompletionBytes  uint64
	onCloseEventCredits      uint32
	queuedApplicationBytes   uint64
	queuedApplicationFrames  uint32
}

// ownerResourceLedger is shared by every Session belonging to one Client or
// Server. Its lock protects only small counter transactions; callers must never
// hold it while doing network I/O or invoking user code.
type ownerResourceLedger struct {
	mu     sync.Mutex
	limits ownerResourceLimits
	used   ownerResourceUsage
}

func newClientResourceLedger(config ownerConfig) *ownerResourceLedger {
	return newOwnerResourceLedger(ownerResourceLimits{
		maxOutstandingRequests:      config.maxOutstandingRequests,
		maxRetainedApplicationBytes: config.maxRetainedApplicationBytes,
		maxRetainedCompletionBytes:  config.maxRetainedCompletionBytes,
		maxOnCloseEventCredits:      config.onCloseEventCredits,
	})
}

func newServerResourceLedger(owner ownerConfig, server serverConfig) *ownerResourceLedger {
	return newOwnerResourceLedger(ownerResourceLimits{
		maxOutstandingRequests:              owner.maxOutstandingRequests,
		maxRetainedApplicationBytes:         owner.maxRetainedApplicationBytes,
		maxRetainedCompletionBytes:          owner.maxRetainedCompletionBytes,
		maxOnCloseEventCredits:              owner.onCloseEventCredits,
		maxAggregateQueuedApplicationBytes:  server.maxAggregateQueuedApplicationBytes,
		maxAggregateQueuedApplicationFrames: server.maxAggregateQueuedApplicationFrames,
		hasAggregateApplicationQueue:        true,
	})
}

func newOwnerResourceLedger(limits ownerResourceLimits) *ownerResourceLedger {
	return &ownerResourceLedger{limits: limits}
}

// reserveRequest atomically reserves one Request credit for the full
// response-wait/terminal-Completion lifecycle.
func (l *ownerResourceLedger) reserveRequest() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.used.outstandingRequests >= l.limits.maxOutstandingRequests {
		return false
	}
	l.used.outstandingRequests++
	return true
}

func (l *ownerResourceLedger) releaseRequest() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.used.outstandingRequests == 0 {
		return false
	}
	l.used.outstandingRequests--
	return true
}

// reserveApplicationFrame reserves retained Application ownership and, for a
// Server owner, the combined queued-Application byte/frame budget in one owner
// transaction. Queue exhaustion remains distinct from retained-byte exhaustion.
func (l *ownerResourceLedger) reserveApplicationFrame(bytes uint64) error {
	if l == nil || bytes == 0 {
		return ErrCapacity
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	nextRetained, ok := checkedAddUint64(l.used.retainedApplicationBytes, bytes)
	if !ok || nextRetained > l.limits.maxRetainedApplicationBytes {
		return ErrCapacity
	}

	var nextQueued uint64
	var nextFrames uint64
	if l.limits.hasAggregateApplicationQueue {
		nextQueued, ok = checkedAddUint64(l.used.queuedApplicationBytes, bytes)
		if !ok || nextQueued > l.limits.maxAggregateQueuedApplicationBytes {
			return ErrQueueFull
		}
		nextFrames = uint64(l.used.queuedApplicationFrames) + 1
		if nextFrames > uint64(l.limits.maxAggregateQueuedApplicationFrames) {
			return ErrQueueFull
		}
	}

	l.used.retainedApplicationBytes = nextRetained
	if l.limits.hasAggregateApplicationQueue {
		l.used.queuedApplicationBytes = nextQueued
		l.used.queuedApplicationFrames = uint32(nextFrames)
	}
	return nil
}

func (l *ownerResourceLedger) reserveRetainedApplication(bytes uint64) bool {
	if l == nil || bytes == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	next, ok := checkedAddUint64(l.used.retainedApplicationBytes, bytes)
	if !ok || next > l.limits.maxRetainedApplicationBytes {
		return false
	}
	l.used.retainedApplicationBytes = next
	return true
}

func (l *ownerResourceLedger) releaseRetainedApplication(bytes uint64) bool {
	if l == nil || bytes == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	next, ok := checkedSubUint64(l.used.retainedApplicationBytes, bytes)
	if !ok {
		return false
	}
	l.used.retainedApplicationBytes = next
	return true
}

func (l *ownerResourceLedger) reserveRetainedCompletion(bytes uint64) bool {
	if l == nil || bytes == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	next, ok := checkedAddUint64(l.used.retainedCompletionBytes, bytes)
	if !ok || next > l.limits.maxRetainedCompletionBytes {
		return false
	}
	l.used.retainedCompletionBytes = next
	return true
}

func (l *ownerResourceLedger) releaseRetainedCompletion(bytes uint64) bool {
	if l == nil || bytes == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	next, ok := checkedSubUint64(l.used.retainedCompletionBytes, bytes)
	if !ok {
		return false
	}
	l.used.retainedCompletionBytes = next
	return true
}

// reserveQueuedApplication atomically reserves the Server-wide combined
// incoming+outgoing Application queue budget. It is a no-op for Client owners.
func (l *ownerResourceLedger) reserveQueuedApplication(bytes uint64, frames uint32) bool {
	if l == nil || bytes == 0 || frames == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.limits.hasAggregateApplicationQueue {
		return true
	}
	nextBytes, ok := checkedAddUint64(l.used.queuedApplicationBytes, bytes)
	if !ok || nextBytes > l.limits.maxAggregateQueuedApplicationBytes {
		return false
	}
	nextFrames := uint64(l.used.queuedApplicationFrames) + uint64(frames)
	if nextFrames > uint64(l.limits.maxAggregateQueuedApplicationFrames) {
		return false
	}
	l.used.queuedApplicationBytes = nextBytes
	l.used.queuedApplicationFrames = uint32(nextFrames)
	return true
}

func (l *ownerResourceLedger) releaseQueuedApplication(bytes uint64, frames uint32) bool {
	if l == nil || bytes == 0 || frames == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.limits.hasAggregateApplicationQueue {
		return true
	}
	nextBytes, ok := checkedSubUint64(l.used.queuedApplicationBytes, bytes)
	if !ok || frames > l.used.queuedApplicationFrames {
		return false
	}
	l.used.queuedApplicationBytes = nextBytes
	l.used.queuedApplicationFrames -= frames
	return true
}

// reserveOnCloseEvent is nonblocking. It must succeed before an eligible
// Session is admitted. The credit covers live, queued and running OnClose
// responsibilities, and is released only after the callback returns.
func (l *ownerResourceLedger) reserveOnCloseEvent() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.used.onCloseEventCredits >= l.limits.maxOnCloseEventCredits {
		return false
	}
	l.used.onCloseEventCredits++
	return true
}

func (l *ownerResourceLedger) releaseOnCloseEvent() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.used.onCloseEventCredits == 0 {
		return false
	}
	l.used.onCloseEventCredits--
	return true
}

func (l *ownerResourceLedger) snapshot() ownerResourceUsage {
	if l == nil {
		return ownerResourceUsage{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.used
}
