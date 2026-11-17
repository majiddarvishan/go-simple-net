package neta

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func workerTestOwnerConfig(application, completion, onClose uint32, grace time.Duration) ownerConfig {
	config := defaultOwnerConfig()
	config.applicationWorkers = application
	config.completionWorkers = completion
	config.onCloseWorkers = onClose
	config.applicationWorkerIdleGrace = grace
	config.completionWorkerIdleGrace = grace
	config.onCloseWorkerIdleGrace = grace
	return config
}

func newWorkerTestSession(dispatcher *ownerCallbackDispatcher, applicationConcurrency, completionConcurrency uint32, resources *ownerResourceLedger, onClose OnCloseHandler) *Session {
	config := defaultSessionConfig()
	config.applicationConcurrency = applicationConcurrency
	config.completionConcurrency = completionConcurrency
	if resources == nil {
		resources = newClientResourceLedger(defaultOwnerConfig())
	}
	return newSessionWithResources(nil, config, sessionIdentity{role: sessionRoleClient}, dispatcher, sessionHandlers{
		onClose: onClose,
	}, resources)
}

func enqueueWorkerTestTask(t *testing.T, session *Session, class callbackClass, run, cancel func()) {
	t.Helper()
	if !session.enqueueCallback(class, callbackTask{session: session, run: run, cancel: cancel}) {
		t.Fatalf("failed to enqueue callback class %d", class)
	}
}

func waitForWorkerSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForWorkerPool(t *testing.T, pool *callbackWorkerPool, predicate func(callbackWorkerPoolSnapshot) bool, description string) callbackWorkerPoolSnapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := pool.snapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; last snapshot: %+v", description, pool.snapshot())
	return callbackWorkerPoolSnapshot{}
}

func TestOwnerCallbackDispatcherCreatesThreeIndependentLazyPools(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(3, 2, 1, 20*time.Millisecond))
	if dispatcher.application == dispatcher.completion ||
		dispatcher.application == dispatcher.onClose ||
		dispatcher.completion == dispatcher.onClose {
		t.Fatal("callback classes must have independent physical pools")
	}
	for name, pool := range map[string]*callbackWorkerPool{
		"application": dispatcher.application,
		"completion":  dispatcher.completion,
		"on-close":    dispatcher.onClose,
	} {
		if got := pool.snapshot(); got.Workers != 0 || got.CreatedWorkers != 0 {
			t.Fatalf("%s pool eagerly created workers: %+v", name, got)
		}
	}

	client, err := newTestClient(WithClientAddress("127.0.0.1:2775"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	server, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if client.callbacks == nil || server.callbacks == nil {
		t.Fatal("Client and Server constructors must create their own lazy callback owners")
	}
	if client.callbacks.application == server.callbacks.application ||
		client.callbacks.completion == server.callbacks.completion ||
		client.callbacks.onClose == server.callbacks.onClose {
		t.Fatal("independent Client and Server instances must not share worker pools")
	}
	if client.callbacks.application.snapshot().Workers != 0 ||
		server.callbacks.application.snapshot().Workers != 0 {
		t.Fatal("constructors must not eagerly create callback goroutines")
	}
}

func TestWorkerClassesRemainIsolatedWhenApplicationBlocks(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	resources := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 4})

	application := newWorkerTestSession(dispatcher, 1, 1, resources, nil)
	completion := newWorkerTestSession(dispatcher, 1, 1, resources, nil)
	onCloseStarted := make(chan struct{})
	onClose := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(onCloseStarted)
	})

	applicationStarted := make(chan struct{})
	releaseApplication := make(chan struct{})
	applicationDone := make(chan struct{})
	enqueueWorkerTestTask(t, application, callbackApplication, func() {
		close(applicationStarted)
		<-releaseApplication
		close(applicationDone)
	}, nil)
	waitForWorkerSignal(t, applicationStarted, "blocked Application callback")

	completionDone := make(chan struct{})
	enqueueWorkerTestTask(t, completion, callbackCompletion, func() {
		close(completionDone)
	}, nil)

	onClose.Close()
	waitDone(t, onClose)
	waitForWorkerSignal(t, completionDone, "Completion callback while Application is blocked")
	waitForWorkerSignal(t, onCloseStarted, "OnClose callback while Application is blocked")

	select {
	case <-applicationDone:
		t.Fatal("blocked Application callback unexpectedly returned")
	default:
	}

	close(releaseApplication)
	waitForWorkerSignal(t, applicationDone, "Application callback release")
	application.Close()
	completion.Close()
	waitDone(t, application)
	waitDone(t, completion)
}

func TestWorkerRoundRobinGivesEachEligibleSessionOneTurn(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	first := newWorkerTestSession(dispatcher, 2, 1, nil, nil)
	second := newWorkerTestSession(dispatcher, 1, 1, nil, nil)

	starts := make(chan string, 4)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	enqueueWorkerTestTask(t, first, callbackApplication, func() {
		starts <- "first-1"
		close(firstStarted)
		<-releaseFirst
	}, nil)
	waitForWorkerSignal(t, firstStarted, "first Session's first callback")

	enqueueWorkerTestTask(t, first, callbackApplication, func() { starts <- "first-2" }, nil)
	enqueueWorkerTestTask(t, first, callbackApplication, func() { starts <- "first-3" }, nil)
	enqueueWorkerTestTask(t, second, callbackApplication, func() { starts <- "second-1" }, nil)

	close(releaseFirst)
	got := make([]string, 0, 4)
	for range 4 {
		select {
		case value := <-starts:
			got = append(got, value)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out collecting callback start order: %v", got)
		}
	}
	want := []string{"first-1", "first-2", "second-1", "first-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round-robin start order = %v, want %v", got, want)
		}
	}
	first.Close()
	second.Close()
	waitDone(t, first)
	waitDone(t, second)
}

func TestWorkerOwnerCeilingIsSharedAcrossSessionGenerations(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	oldGeneration := newWorkerTestSession(dispatcher, 4, 1, nil, nil)
	newGeneration := newWorkerTestSession(dispatcher, 4, 1, nil, nil)

	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	newStarted := make(chan struct{})
	enqueueWorkerTestTask(t, oldGeneration, callbackApplication, func() {
		close(oldStarted)
		<-releaseOld
	}, nil)
	waitForWorkerSignal(t, oldStarted, "old-generation callback")

	enqueueWorkerTestTask(t, newGeneration, callbackApplication, func() {
		close(newStarted)
	}, nil)
	select {
	case <-newStarted:
		t.Fatal("new Session generation bypassed the Client/Server owner worker ceiling")
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseOld)
	waitForWorkerSignal(t, newStarted, "new-generation callback after old callback returned")
	oldGeneration.Close()
	newGeneration.Close()
	waitDone(t, oldGeneration)
	waitDone(t, newGeneration)
}

func TestWorkerPerSessionConcurrencyIsAnIndependentLimit(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(4, 2, 1, 20*time.Millisecond))
	session := newWorkerTestSession(dispatcher, 1, 1, nil, nil)

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	enqueueWorkerTestTask(t, session, callbackApplication, func() {
		close(firstStarted)
		<-releaseFirst
	}, nil)
	waitForWorkerSignal(t, firstStarted, "first serial callback")

	enqueueWorkerTestTask(t, session, callbackApplication, func() {
		close(secondStarted)
	}, nil)
	select {
	case <-secondStarted:
		t.Fatal("per-Session concurrency=1 allowed overlapping Application callbacks")
	case <-time.After(25 * time.Millisecond):
	}

	if got := dispatcher.application.snapshot().ActiveCallbacks; got != 1 {
		t.Fatalf("active callbacks while first is blocked = %d, want 1", got)
	}
	close(releaseFirst)
	waitForWorkerSignal(t, secondStarted, "second callback after first returned")
	session.Close()
	waitDone(t, session)
	waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
	}, "serial callbacks to settle")
}

func TestWorkerForcedCloseCancelsQueuedButNotRunningCallbacks(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	session := newWorkerTestSession(dispatcher, 2, 2, nil, nil)

	applicationStarted := make(chan struct{})
	completionStarted := make(chan struct{})
	releaseApplication := make(chan struct{})
	releaseCompletion := make(chan struct{})
	applicationDone := make(chan struct{})
	completionDone := make(chan struct{})
	var applicationCancelled atomic.Int32
	var completionCancelled atomic.Int32
	var applicationSecondRan atomic.Int32
	var completionSecondRan atomic.Int32

	enqueueWorkerTestTask(t, session, callbackApplication, func() {
		close(applicationStarted)
		<-releaseApplication
		close(applicationDone)
	}, nil)
	enqueueWorkerTestTask(t, session, callbackCompletion, func() {
		close(completionStarted)
		<-releaseCompletion
		close(completionDone)
	}, nil)
	waitForWorkerSignal(t, applicationStarted, "running Application callback")
	waitForWorkerSignal(t, completionStarted, "running Completion callback")

	enqueueWorkerTestTask(t, session, callbackApplication, func() {
		applicationSecondRan.Add(1)
	}, func() {
		applicationCancelled.Add(1)
	})
	enqueueWorkerTestTask(t, session, callbackCompletion, func() {
		completionSecondRan.Add(1)
	}, func() {
		completionCancelled.Add(1)
	})

	session.Close()
	waitDone(t, session)
	if applicationCancelled.Load() != 1 || completionCancelled.Load() != 1 {
		t.Fatalf("queued callback cancellations: Application=%d Completion=%d, want one each",
			applicationCancelled.Load(), completionCancelled.Load())
	}
	if applicationSecondRan.Load() != 0 || completionSecondRan.Load() != 0 {
		t.Fatal("a queued callback ran after Forced Close cancellation")
	}
	if dispatcher.application.snapshot().ActiveCallbacks != 1 ||
		dispatcher.completion.snapshot().ActiveCallbacks != 1 {
		t.Fatal("Forced Close released a started callback's active permit before return")
	}

	close(releaseApplication)
	close(releaseCompletion)
	waitForWorkerSignal(t, applicationDone, "started Application callback return")
	waitForWorkerSignal(t, completionDone, "started Completion callback return")
	for name, pool := range map[string]*callbackWorkerPool{
		"application": dispatcher.application,
		"completion":  dispatcher.completion,
	} {
		waitForWorkerPool(t, pool, func(s callbackWorkerPoolSnapshot) bool {
			return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
		}, name+" callback release")
	}
}

func TestWorkerOnCloseRunsAfterDoneAndSkipsAtEventCapacity(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	resources := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 1})
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan struct{})
	secondRan := make(chan struct{})

	first := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(firstStarted)
		<-releaseFirst
		close(firstDone)
	})
	second := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(secondRan)
	})

	first.Close()
	waitDone(t, first)
	waitForWorkerSignal(t, firstStarted, "OnClose after Done")
	if got := resources.snapshot().onCloseEventCredits; got != 1 {
		t.Fatalf("OnClose event credits while callback runs = %d, want 1", got)
	}

	second.Close()
	waitDone(t, second)
	select {
	case <-secondRan:
		t.Fatal("OnClose ran despite owner event-credit saturation")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseFirst)
	waitForWorkerSignal(t, firstDone, "first OnClose return")
	waitForWorkerPool(t, dispatcher.onClose, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
	}, "OnClose worker release")
	if got := resources.snapshot().onCloseEventCredits; got != 0 {
		t.Fatalf("OnClose event credit after callback return = %d, want 0", got)
	}
}

func TestWorkerIdleRetirementReusesThenReturnsToZero(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 80*time.Millisecond))
	session := newWorkerTestSession(dispatcher, 1, 1, nil, nil)

	firstDone := make(chan struct{})
	enqueueWorkerTestTask(t, session, callbackApplication, func() { close(firstDone) }, nil)
	waitForWorkerSignal(t, firstDone, "first callback")
	firstIdle := waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.IdleWorkers == 1
	}, "first worker to become idle")
	if firstIdle.CreatedWorkers != 1 {
		t.Fatalf("created workers after first callback = %d, want 1", firstIdle.CreatedWorkers)
	}

	secondDone := make(chan struct{})
	enqueueWorkerTestTask(t, session, callbackApplication, func() { close(secondDone) }, nil)
	waitForWorkerSignal(t, secondDone, "callback reusing idle worker")
	secondIdle := waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.IdleWorkers == 1
	}, "reused worker to become idle")
	if secondIdle.CreatedWorkers != 1 || secondIdle.RetiredWorkers != 0 {
		t.Fatalf("worker was not reused before grace expiry: %+v", secondIdle)
	}

	retired := waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.Workers == 0
	}, "zero warm-idle floor")
	if retired.RetiredWorkers != 1 {
		t.Fatalf("retired workers = %d, want 1", retired.RetiredWorkers)
	}

	thirdDone := make(chan struct{})
	enqueueWorkerTestTask(t, session, callbackApplication, func() { close(thirdDone) }, nil)
	waitForWorkerSignal(t, thirdDone, "worker recreation after idle retirement")
	recreated := waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.CreatedWorkers == 2 && s.IdleWorkers == 1
	}, "replacement worker to become idle")
	if recreated.Workers != 1 {
		t.Fatalf("worker count after lazy recreation = %d, want 1", recreated.Workers)
	}
	session.Close()
	waitDone(t, session)
}

func TestWorkerIdleTimeoutNeverRetiresRunningCallback(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 5*time.Millisecond))
	session := newWorkerTestSession(dispatcher, 1, 1, nil, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	enqueueWorkerTestTask(t, session, callbackApplication, func() {
		close(started)
		<-release
		close(done)
	}, nil)
	waitForWorkerSignal(t, started, "blocked callback")
	time.Sleep(25 * time.Millisecond)
	snapshot := dispatcher.application.snapshot()
	if snapshot.Workers != 1 || snapshot.ActiveCallbacks != 1 || snapshot.RetiredWorkers != 0 {
		t.Fatalf("idle retirement interrupted a running callback: %+v", snapshot)
	}

	close(release)
	waitForWorkerSignal(t, done, "blocked callback return")
	waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.Workers == 0 && s.RetiredWorkers == 1
	}, "worker retirement after callback return")
	session.Close()
	waitDone(t, session)
}

func TestWorkerConcurrentEnqueueAndIdleRetirementNeverStrandsWork(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(4, 1, 1, time.Millisecond))
	session := newWorkerTestSession(dispatcher, 4, 1, nil, nil)

	const taskCount = 160
	var enqueuers sync.WaitGroup
	var ran atomic.Int32
	var cancelled atomic.Int32
	done := make(chan struct{}, taskCount)

	for i := 0; i < taskCount; i++ {
		enqueuers.Add(1)
		go func() {
			defer enqueuers.Done()
			task := callbackTask{
				session: session,
				run: func() {
					ran.Add(1)
					done <- struct{}{}
				},
				cancel: func() {
					cancelled.Add(1)
					done <- struct{}{}
				},
			}
			if !session.enqueueCallback(callbackApplication, task) {
				t.Error("callback admission unexpectedly failed")
			}
		}()
		if i%8 == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}

	enqueuers.Wait()
	for i := 0; i < taskCount; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("callback %d was stranded during worker retirement; snapshot=%+v", i, dispatcher.application.snapshot())
		}
	}
	if got := ran.Load(); got != taskCount {
		t.Fatalf("callbacks run = %d, want %d (cancelled=%d)", got, taskCount, cancelled.Load())
	}
	if got := cancelled.Load(); got != 0 {
		t.Fatalf("callbacks unexpectedly cancelled without Forced Close: %d", got)
	}
	waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.Workers == 0 && s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
	}, "all workers to retire after churn")
	session.Close()
	waitDone(t, session)
}

func TestWorkerConcurrentEnqueueAndForcedCloseSettlesEachTaskOnce(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(4, 1, 1, 10*time.Millisecond))
	session := newWorkerTestSession(dispatcher, 4, 1, nil, nil)

	const taskCount = 128
	var settled atomic.Int32
	var duplicates atomic.Int32
	var enqueuers sync.WaitGroup
	var once [taskCount]atomic.Bool
	start := make(chan struct{})

	for i := 0; i < taskCount; i++ {
		enqueuers.Add(1)
		go func(index int) {
			defer enqueuers.Done()
			<-start
			task := callbackTask{
				session: session,
				run: func() {
					if !once[index].CompareAndSwap(false, true) {
						duplicates.Add(1)
					}
					time.Sleep(time.Microsecond)
					settled.Add(1)
				},
				cancel: func() {
					if !once[index].CompareAndSwap(false, true) {
						duplicates.Add(1)
					}
					settled.Add(1)
				},
			}
			session.enqueueCallback(callbackApplication, task)
		}(i)
	}
	close(start)
	time.Sleep(time.Millisecond)
	session.Close()
	enqueuers.Wait()
	waitDone(t, session)
	waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0 && s.ReadySessions == 0
	}, "all admitted and cancelled callbacks to settle")

	if got := settled.Load(); got != taskCount {
		t.Fatalf("callbacks settled = %d, want %d", got, taskCount)
	}
	if got := duplicates.Load(); got != 0 {
		t.Fatalf("callbacks run/cancelled more than once: %d", got)
	}
}

func TestSessionReaderWriterAndDoneRemainIndependentOfApplicationCallback(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	resources := newClientResourceLedger(defaultOwnerConfig())
	local, peer := net.Pipe()
	defer peer.Close()

	applicationStarted := make(chan struct{})
	releaseApplication := make(chan struct{})
	applicationReturned := make(chan struct{})
	var callbacks atomic.Int32
	session := newBoundSessionWithResources(local, config, sessionIdentity{
		role: sessionRoleServer, bindType: wire.BindBiDirectional,
	}, dispatcher, sessionHandlers{
		onRequest: func(_ *Session, request Request) {
			callbacks.Add(1)
			if string(request.Payload) == "first" {
				close(applicationStarted)
				<-releaseApplication
				close(applicationReturned)
			}
		},
	}, resources)

	first, err := wire.EncodeStreamRequest(101, []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, first)
	waitForWorkerSignal(t, applicationStarted, "first inbound Application callback")

	second, err := wire.EncodeStreamRequest(102, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, second)
	probe, err := wire.EncodeEnquireLinkRequest(103)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, probe)
	probeResponse := readWireFrame(t, peer)
	if probeResponse.Header.Command != wire.CommandEnquireLinkResponse || probeResponse.Header.Sequence != 103 {
		t.Fatalf("Reader/Writer did not process control traffic while Application callback blocked: %+v", probeResponse.Header)
	}

	session.Close()
	waitDone(t, session)
	select {
	case <-applicationReturned:
		t.Fatal("Session Done waited for the running Application callback")
	default:
	}
	close(releaseApplication)
	waitForWorkerSignal(t, applicationReturned, "running Application callback return")

	if got := callbacks.Load(); got != 1 {
		t.Fatalf("Application callbacks executed = %d, want first only; second must be cancelled by Forced Close", got)
	}
	// The handler closes applicationReturned immediately before returning;
	// the worker's deferred resource release runs just after that signal.
	// Wait for that release instead of racing the callback epilogue under -race.
	deadline := time.Now().Add(2 * time.Second)
	for {
		usage := resources.snapshot()
		if usage.retainedApplicationBytes == 0 &&
			usage.queuedApplicationBytes == 0 &&
			usage.queuedApplicationFrames == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Application resources leaked after callback return/cancellation: %+v", usage)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSessionCompletionWorkerDoesNotBlockReaderWriterOrDone(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	ownerConfig := defaultOwnerConfig()
	resources := newClientResourceLedger(ownerConfig)
	local, peer := net.Pipe()
	defer peer.Close()

	completionStarted := make(chan struct{})
	releaseCompletion := make(chan struct{})
	completionReturned := make(chan struct{})
	session := newBoundSessionWithResources(local, config, sessionIdentity{
		role: sessionRoleClient, bindType: wire.BindBiDirectional,
	}, dispatcher, sessionHandlers{}, resources)

	sequence, err := session.SendRequest([]byte("request"), func(Response, error) {
		close(completionStarted)
		<-releaseCompletion
		close(completionReturned)
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	outbound := readWireFrame(t, peer)
	if outbound.Header.Command != wire.CommandStreamRequest || outbound.Header.Sequence != sequence {
		t.Fatalf("unexpected outbound Request: %+v", outbound.Header)
	}
	response, err := wire.EncodeStreamResponse(sequence, wire.StatusOK, []byte("response"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)
	waitForWorkerSignal(t, completionStarted, "Completion callback")

	if got := resources.snapshot(); got.outstandingRequests != 1 || got.retainedCompletionBytes == 0 {
		t.Fatalf("running Completion must retain Request and payload charges: %+v", got)
	}

	probe, err := wire.EncodeEnquireLinkRequest(104)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, probe)
	probeResponse := readWireFrame(t, peer)
	if probeResponse.Header.Command != wire.CommandEnquireLinkResponse || probeResponse.Header.Sequence != 104 {
		t.Fatalf("Reader/Writer did not progress while Completion callback blocked: %+v", probeResponse.Header)
	}

	session.Close()
	waitDone(t, session)
	select {
	case <-completionReturned:
		t.Fatal("Session Done waited for the running Completion callback")
	default:
	}
	if got := resources.snapshot().outstandingRequests; got != 1 {
		t.Fatalf("Forced Close released a started Completion Request credit early: %+v", resources.snapshot())
	}

	close(releaseCompletion)
	waitForWorkerSignal(t, completionReturned, "running Completion callback return")
	// The callback closes completionReturned immediately before returning.
	// Request and retained Completion credits are released by the worker's
	// deferred callback epilogue, so wait for that release instead of racing
	// the epilogue under -race.
	deadline := time.Now().Add(2 * time.Second)
	for {
		usage := resources.snapshot()
		if usage.outstandingRequests == 0 && usage.retainedCompletionBytes == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Completion resources leaked after callback return: %+v", usage)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWorkerOwnerPermitSurvivesSessionDoneUntilCallbackReturns(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	oldGeneration := newWorkerTestSession(dispatcher, 1, 1, nil, nil)
	newGeneration := newWorkerTestSession(dispatcher, 1, 1, nil, nil)

	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	oldReturned := make(chan struct{})
	newStarted := make(chan struct{})

	enqueueWorkerTestTask(t, oldGeneration, callbackApplication, func() {
		close(oldStarted)
		<-releaseOld
		close(oldReturned)
	}, nil)
	waitForWorkerSignal(t, oldStarted, "old-generation Application callback")

	// Session teardown must finish independently of the already-started user
	// callback. The callback nevertheless retains the shared owner permit.
	oldGeneration.Close()
	waitDone(t, oldGeneration)

	select {
	case <-oldReturned:
		t.Fatal("Session Done waited for the running old-generation callback")
	default:
	}

	enqueueWorkerTestTask(t, newGeneration, callbackApplication, func() {
		close(newStarted)
	}, nil)
	select {
	case <-newStarted:
		t.Fatal("new generation bypassed the permit held by a callback from the retired generation")
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseOld)
	waitForWorkerSignal(t, oldReturned, "old-generation callback return")
	waitForWorkerSignal(t, newStarted, "new-generation callback after old permit release")

	newGeneration.Close()
	waitDone(t, newGeneration)
	waitForWorkerPool(t, dispatcher.application, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
	}, "owner Application permit release")
}

func TestWorkerOnCloseEventCreditsCountQueuedAndRunning(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	resources := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 2})

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstReturned := make(chan struct{})
	secondStarted := make(chan struct{})
	secondReturned := make(chan struct{})
	thirdStarted := make(chan struct{})

	first := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(firstStarted)
		<-releaseFirst
		close(firstReturned)
	})
	second := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(secondStarted)
		close(secondReturned)
	})
	third := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(thirdStarted)
	})

	first.Close()
	waitDone(t, first)
	waitForWorkerSignal(t, firstStarted, "first running OnClose callback")

	second.Close()
	waitDone(t, second)
	waitForWorkerPool(t, dispatcher.onClose, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 1 && s.QueuedCallbacks == 1
	}, "second OnClose callback queued behind the first")
	if got := resources.snapshot().onCloseEventCredits; got != 2 {
		t.Fatalf("queued + running OnClose credits = %d, want 2", got)
	}

	third.Close()
	waitDone(t, third)
	select {
	case <-thirdStarted:
		t.Fatal("third OnClose ran after queued + running events exhausted the owner budget")
	default:
	}
	if got := resources.snapshot().onCloseEventCredits; got != 2 {
		t.Fatalf("OnClose credits changed after saturated event was skipped: %d, want 2", got)
	}

	close(releaseFirst)
	waitForWorkerSignal(t, firstReturned, "first OnClose callback return")
	waitForWorkerSignal(t, secondStarted, "queued second OnClose callback")
	waitForWorkerSignal(t, secondReturned, "second OnClose callback return")
	waitForWorkerPool(t, dispatcher.onClose, func(s callbackWorkerPoolSnapshot) bool {
		return s.ActiveCallbacks == 0 && s.QueuedCallbacks == 0
	}, "all admitted OnClose events to settle")
	if got := resources.snapshot().onCloseEventCredits; got != 0 {
		t.Fatalf("OnClose credits after both callbacks return = %d, want 0", got)
	}
}

func TestBlockedOnCloseDoesNotBlockApplicationOrCompletion(t *testing.T) {
	dispatcher := newOwnerCallbackDispatcher(workerTestOwnerConfig(1, 1, 1, 20*time.Millisecond))
	resources := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 2})

	onCloseStarted := make(chan struct{})
	releaseOnClose := make(chan struct{})
	onCloseReturned := make(chan struct{})
	closing := newWorkerTestSession(dispatcher, 1, 1, resources, func(*Session, error) {
		close(onCloseStarted)
		<-releaseOnClose
		close(onCloseReturned)
	})

	closing.Close()
	waitDone(t, closing)
	waitForWorkerSignal(t, onCloseStarted, "blocked OnClose callback")

	other := newWorkerTestSession(dispatcher, 1, 1, resources, nil)
	applicationDone := make(chan struct{})
	completionDone := make(chan struct{})
	enqueueWorkerTestTask(t, other, callbackApplication, func() { close(applicationDone) }, nil)
	enqueueWorkerTestTask(t, other, callbackCompletion, func() { close(completionDone) }, nil)

	waitForWorkerSignal(t, applicationDone, "Application callback while OnClose is blocked")
	waitForWorkerSignal(t, completionDone, "Completion callback while OnClose is blocked")
	select {
	case <-onCloseReturned:
		t.Fatal("blocked OnClose callback unexpectedly returned")
	default:
	}

	close(releaseOnClose)
	waitForWorkerSignal(t, onCloseReturned, "OnClose callback release")
	other.Close()
	waitDone(t, other)
}
