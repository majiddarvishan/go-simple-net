package neta

import (
	"sync"
	"time"
)

// ownerCallbackDispatcher owns three physically isolated callback execution
// domains for one Client or Server instance. Constructing it creates no worker
// goroutines; each class grows only when dispatchable work arrives.
type ownerCallbackDispatcher struct {
	application *callbackWorkerPool
	completion  *callbackWorkerPool
	onClose     *callbackWorkerPool
}

func newOwnerCallbackDispatcher(config ownerConfig) *ownerCallbackDispatcher {
	return &ownerCallbackDispatcher{
		application: newCallbackWorkerPool(callbackApplication, config.applicationWorkers, config.applicationWorkerIdleGrace),
		completion:  newCallbackWorkerPool(callbackCompletion, config.completionWorkers, config.completionWorkerIdleGrace),
		onClose:     newCallbackWorkerPool(callbackOnClose, config.onCloseWorkers, config.onCloseWorkerIdleGrace),
	}
}

func (d *ownerCallbackDispatcher) enqueueApplication(task callbackTask) bool {
	return d != nil && d.application.enqueue(task)
}

func (d *ownerCallbackDispatcher) enqueueCompletion(task callbackTask) bool {
	return d != nil && d.completion.enqueue(task)
}

func (d *ownerCallbackDispatcher) enqueueOnClose(task callbackTask) bool {
	return d != nil && d.onClose.enqueue(task)
}

// cancelUnstarted is called while Session.dispatchMu is held. Workers never
// hold a pool lock while acquiring that mutex, so this preserves one
// cancellation/start linearization point without a lock-order cycle.
func (d *ownerCallbackDispatcher) cancelUnstarted(session *Session, mask callbackClassMask) {
	if d == nil {
		return
	}
	if mask&callbackMaskApplication != 0 {
		d.application.cancelUnstarted(session)
	}
	if mask&callbackMaskCompletion != 0 {
		d.completion.cancelUnstarted(session)
	}
}

type callbackWorkerPool struct {
	class      callbackClass
	maxWorkers uint32
	idleGrace  time.Duration

	mu sync.Mutex

	sessions map[*Session]*callbackSessionQueue

	readyHead  *callbackSessionQueue
	readyTail  *callbackSessionQueue
	readyCount uint32

	idleHead  *callbackWorker
	idleTail  *callbackWorker
	idleCount uint32

	workerCount     uint32
	activeCallbacks uint32
	createdWorkers  uint64
	retiredWorkers  uint64
}

type callbackSessionQueue struct {
	session *Session

	tasks []callbackTask
	head  int
	active uint32

	dispatching bool
	ready       bool
	readyPrev   *callbackSessionQueue
	readyNext   *callbackSessionQueue
}

func (q *callbackSessionQueue) queued() int {
	if q == nil {
		return 0
	}
	return len(q.tasks) - q.head
}

func (q *callbackSessionQueue) append(task callbackTask) {
	q.tasks = append(q.tasks, task)
}

func (q *callbackSessionQueue) pop() callbackTask {
	task := q.tasks[q.head]
	q.tasks[q.head] = callbackTask{}
	q.head++
	if q.head == len(q.tasks) {
		q.tasks = nil
		q.head = 0
	} else if q.head >= 64 && q.head*2 >= len(q.tasks) {
		remaining := copy(q.tasks, q.tasks[q.head:])
		clear(q.tasks[remaining:])
		q.tasks = q.tasks[:remaining]
		q.head = 0
	}
	return task
}

func (q *callbackSessionQueue) drain() []callbackTask {
	if q.queued() == 0 {
		q.tasks = nil
		q.head = 0
		return nil
	}
	cancelled := make([]callbackTask, q.queued())
	copy(cancelled, q.tasks[q.head:])
	clear(q.tasks)
	q.tasks = nil
	q.head = 0
	return cancelled
}

type callbackWorker struct {
	pool *callbackWorkerPool
	wake chan struct{}

	idle     bool
	idlePrev *callbackWorker
	idleNext *callbackWorker
	retiring bool
}

type callbackWorkerPoolSnapshot struct {
	Workers         uint32
	ActiveCallbacks uint32
	IdleWorkers     uint32
	ReadySessions   uint32
	QueuedCallbacks uint32
	CreatedWorkers  uint64
	RetiredWorkers  uint64
}

func newCallbackWorkerPool(class callbackClass, maxWorkers uint32, idleGrace time.Duration) *callbackWorkerPool {
	return &callbackWorkerPool{
		class:      class,
		maxWorkers: maxWorkers,
		idleGrace:  idleGrace,
		sessions:   make(map[*Session]*callbackSessionQueue),
	}
}

func (p *callbackWorkerPool) enqueue(task callbackTask) bool {
	if p == nil || task.session == nil || task.run == nil {
		return false
	}

	p.mu.Lock()
	queue := p.sessions[task.session]
	if queue == nil {
		queue = &callbackSessionQueue{session: task.session}
		p.sessions[task.session] = queue
	}
	queue.append(task)
	p.makeReadyLocked(queue)
	p.wakeOrGrowLocked()
	p.mu.Unlock()
	return true
}

func (p *callbackWorkerPool) cancelUnstarted(session *Session) {
	if p == nil || session == nil {
		return
	}

	p.mu.Lock()
	queue := p.sessions[session]
	if queue == nil {
		p.mu.Unlock()
		return
	}
	p.removeReadyLocked(queue)
	cancelled := queue.drain()
	p.cleanupSessionQueueLocked(queue)
	p.mu.Unlock()

	for _, task := range cancelled {
		if task.cancel != nil {
			task.cancel()
		}
	}
}

func (p *callbackWorkerPool) makeReadyLocked(queue *callbackSessionQueue) {
	if queue == nil || queue.ready || queue.dispatching || queue.queued() == 0 {
		return
	}
	if queue.active >= p.sessionConcurrency(queue.session) {
		return
	}

	queue.ready = true
	queue.readyPrev = p.readyTail
	queue.readyNext = nil
	if p.readyTail == nil {
		p.readyHead = queue
	} else {
		p.readyTail.readyNext = queue
	}
	p.readyTail = queue
	p.readyCount++
}

func (p *callbackWorkerPool) removeReadyLocked(queue *callbackSessionQueue) {
	if queue == nil || !queue.ready {
		return
	}
	if queue.readyPrev == nil {
		p.readyHead = queue.readyNext
	} else {
		queue.readyPrev.readyNext = queue.readyNext
	}
	if queue.readyNext == nil {
		p.readyTail = queue.readyPrev
	} else {
		queue.readyNext.readyPrev = queue.readyPrev
	}
	queue.readyPrev = nil
	queue.readyNext = nil
	queue.ready = false
	p.readyCount--
}

func (p *callbackWorkerPool) popReadyLocked() *callbackSessionQueue {
	queue := p.readyHead
	if queue != nil {
		p.removeReadyLocked(queue)
	}
	return queue
}

func (p *callbackWorkerPool) sessionConcurrency(session *Session) uint32 {
	if session == nil {
		return 1
	}
	switch p.class {
	case callbackApplication:
		if session.config.applicationConcurrency != 0 {
			return session.config.applicationConcurrency
		}
	case callbackCompletion:
		if session.config.completionConcurrency != 0 {
			return session.config.completionConcurrency
		}
	case callbackOnClose:
		return 1
	}
	return 1
}

func (p *callbackWorkerPool) cleanupSessionQueueLocked(queue *callbackSessionQueue) {
	if queue == nil || queue.queued() != 0 || queue.active != 0 || queue.dispatching || queue.ready {
		return
	}
	if p.sessions[queue.session] == queue {
		delete(p.sessions, queue.session)
	}
}

// wakeOrGrowLocked assigns currently-ready Session lanes to idle workers first,
// then lazily creates enough workers for the remaining ready lanes. Worker
// goroutine count never exceeds the configured owner/class ceiling.
func (p *callbackWorkerPool) wakeOrGrowLocked() {
	ready := p.readyCount
	for ready > 0 && p.idleHead != nil {
		worker := p.popIdleWorkerLocked()
		worker.idle = false
		select {
		case worker.wake <- struct{}{}:
		default:
		}
		ready--
	}
	for ready > 0 && p.workerCount < p.maxWorkers {
		p.spawnWorkerLocked()
		ready--
	}
}

func (p *callbackWorkerPool) spawnWorkerLocked() {
	worker := &callbackWorker{
		pool: p,
		wake: make(chan struct{}, 1),
	}
	p.workerCount++
	p.createdWorkers++
	go worker.run()
}

func (p *callbackWorkerPool) appendIdleWorkerLocked(worker *callbackWorker) {
	worker.idle = true
	worker.idlePrev = p.idleTail
	worker.idleNext = nil
	if p.idleTail == nil {
		p.idleHead = worker
	} else {
		p.idleTail.idleNext = worker
	}
	p.idleTail = worker
	p.idleCount++
}

func (p *callbackWorkerPool) removeIdleWorkerLocked(worker *callbackWorker) {
	if worker == nil || !worker.idle {
		return
	}
	if worker.idlePrev == nil {
		p.idleHead = worker.idleNext
	} else {
		worker.idlePrev.idleNext = worker.idleNext
	}
	if worker.idleNext == nil {
		p.idleTail = worker.idlePrev
	} else {
		worker.idleNext.idlePrev = worker.idlePrev
	}
	worker.idlePrev = nil
	worker.idleNext = nil
	worker.idle = false
	p.idleCount--
}

func (p *callbackWorkerPool) popIdleWorkerLocked() *callbackWorker {
	worker := p.idleHead
	if worker != nil {
		p.removeIdleWorkerLocked(worker)
	}
	return worker
}

// nextCandidate either removes one ready Session from the round-robin head or
// atomically registers this worker as idle. The latter closes the lost-wakeup
// window between checking readiness and waiting for new work.
func (p *callbackWorkerPool) nextCandidate(worker *callbackWorker) *callbackSessionQueue {
	p.mu.Lock()
	queue := p.popReadyLocked()
	if queue != nil {
		queue.dispatching = true
		p.mu.Unlock()
		return queue
	}
	if !worker.idle {
		p.appendIdleWorkerLocked(worker)
	}
	p.mu.Unlock()
	return nil
}

// dispatchCandidate acquires Session.dispatchMu before the pool lock. A
// Forced Close that acquires dispatchMu first drains the queued task and wins;
// otherwise this function marks the task active before releasing dispatchMu.
func (p *callbackWorkerPool) dispatchCandidate(queue *callbackSessionQueue) {
	session := queue.session
	session.dispatchMu.Lock()
	p.mu.Lock()

	queue.dispatching = false
	if queue.queued() == 0 {
		p.cleanupSessionQueueLocked(queue)
		p.mu.Unlock()
		session.dispatchMu.Unlock()
		return
	}
	if queue.active >= p.sessionConcurrency(session) {
		p.makeReadyLocked(queue)
		p.wakeOrGrowLocked()
		p.mu.Unlock()
		session.dispatchMu.Unlock()
		return
	}

	task := queue.pop()
	queue.active++
	p.activeCallbacks++
	p.makeReadyLocked(queue)
	p.wakeOrGrowLocked()

	p.mu.Unlock()
	session.dispatchMu.Unlock()

	defer p.finishCallback(queue)
	task.run()
}

func (p *callbackWorkerPool) finishCallback(queue *callbackSessionQueue) {
	p.mu.Lock()
	if queue.active == 0 || p.activeCallbacks == 0 {
		p.mu.Unlock()
		panic("neta: callback worker accounting underflow")
	}
	queue.active--
	p.activeCallbacks--
	p.makeReadyLocked(queue)
	p.cleanupSessionQueueLocked(queue)
	p.wakeOrGrowLocked()
	p.mu.Unlock()
}

func (p *callbackWorkerPool) retireIfIdle(worker *callbackWorker) bool {
	p.mu.Lock()
	if !worker.idle {
		p.mu.Unlock()
		return false
	}
	p.removeIdleWorkerLocked(worker)
	worker.retiring = true
	p.mu.Unlock()
	return true
}

func (p *callbackWorkerPool) workerExited(worker *callbackWorker) {
	p.mu.Lock()
	p.removeIdleWorkerLocked(worker)
	if p.workerCount == 0 {
		p.mu.Unlock()
		panic("neta: callback worker count underflow")
	}
	p.workerCount--
	if worker.retiring {
		p.retiredWorkers++
	}
	p.wakeOrGrowLocked()
	p.mu.Unlock()
}

func stopAndDrainWorkerTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (worker *callbackWorker) run() {
	p := worker.pool
	defer p.workerExited(worker)

	// One timer is reused for every idle period of this worker.
	timer := time.NewTimer(p.idleGrace)
	stopAndDrainWorkerTimer(timer)

	for {
		if queue := p.nextCandidate(worker); queue != nil {
			p.dispatchCandidate(queue)
			continue
		}

		stopAndDrainWorkerTimer(timer)
		timer.Reset(p.idleGrace)
		select {
		case <-worker.wake:
			stopAndDrainWorkerTimer(timer)
		case <-timer.C:
			if p.retireIfIdle(worker) {
				return
			}
			// Enqueue won the timer race and already signalled this worker.
			select {
			case <-worker.wake:
			default:
			}
		}
	}
}

func (p *callbackWorkerPool) snapshot() callbackWorkerPoolSnapshot {
	if p == nil {
		return callbackWorkerPoolSnapshot{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot := callbackWorkerPoolSnapshot{
		Workers:         p.workerCount,
		ActiveCallbacks: p.activeCallbacks,
		IdleWorkers:     p.idleCount,
		ReadySessions:   p.readyCount,
		CreatedWorkers:  p.createdWorkers,
		RetiredWorkers:  p.retiredWorkers,
	}
	for _, queue := range p.sessions {
		snapshot.QueuedCallbacks += uint32(queue.queued())
	}
	return snapshot
}
