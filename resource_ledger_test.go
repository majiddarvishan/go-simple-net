package neta

import (
	"sync"
	"testing"
)

func TestOwnerResourceLedgerRequestCreditsAreBoundedAndReleased(t *testing.T) {
	ledger := newOwnerResourceLedger(ownerResourceLimits{maxOutstandingRequests: 2})
	if !ledger.reserveRequest() || !ledger.reserveRequest() {
		t.Fatal("expected the two available Request credits to be admitted")
	}
	if ledger.reserveRequest() {
		t.Fatal("third Request credit exceeded the owner limit")
	}
	if got := ledger.snapshot().outstandingRequests; got != 2 {
		t.Fatalf("outstanding Requests = %d, want 2", got)
	}
	if !ledger.releaseRequest() || !ledger.reserveRequest() {
		t.Fatal("released Request credit was not reusable")
	}
	if !ledger.releaseRequest() || !ledger.releaseRequest() {
		t.Fatal("expected both Request credits to release")
	}
	if ledger.releaseRequest() {
		t.Fatal("Request credit underflow must be rejected")
	}
}

func TestOwnerResourceLedgerRetainedByteClassesAreIndependent(t *testing.T) {
	ledger := newOwnerResourceLedger(ownerResourceLimits{
		maxRetainedApplicationBytes: 100,
		maxRetainedCompletionBytes:  50,
	})
	if !ledger.reserveRetainedApplication(100) {
		t.Fatal("exact Application byte limit should be admitted")
	}
	if ledger.reserveRetainedApplication(1) {
		t.Fatal("Application bytes exceeded the owner limit")
	}
	if !ledger.reserveRetainedCompletion(50) {
		t.Fatal("Completion bytes must have an independent limit")
	}
	if ledger.reserveRetainedCompletion(1) {
		t.Fatal("Completion bytes exceeded the owner limit")
	}
	if !ledger.releaseRetainedApplication(100) || !ledger.releaseRetainedCompletion(50) {
		t.Fatal("expected retained-byte reservations to release")
	}
	if ledger.releaseRetainedApplication(1) {
		t.Fatal("Application byte underflow must be rejected")
	}
	if got := ledger.snapshot(); got.retainedApplicationBytes != 0 || got.retainedCompletionBytes != 0 {
		t.Fatalf("retained bytes after release = %+v", got)
	}
}

func TestOwnerResourceLedgerServerAggregateQueueCombinesDirections(t *testing.T) {
	ledger := newOwnerResourceLedger(ownerResourceLimits{
		hasAggregateApplicationQueue:        true,
		maxAggregateQueuedApplicationBytes:  100,
		maxAggregateQueuedApplicationFrames: 2,
	})
	if !ledger.reserveQueuedApplication(40, 1) {
		t.Fatal("first direction reservation failed")
	}
	if !ledger.reserveQueuedApplication(60, 1) {
		t.Fatal("second direction should consume remaining shared capacity")
	}
	if ledger.reserveQueuedApplication(1, 1) {
		t.Fatal("aggregate byte limit was exceeded")
	}
	if ledger.reserveQueuedApplication(1, 1) {
		t.Fatal("aggregate frame limit was exceeded")
	}
	if got := ledger.snapshot(); got.queuedApplicationBytes != 100 || got.queuedApplicationFrames != 2 {
		t.Fatalf("aggregate queue usage = %+v", got)
	}
	if !ledger.releaseQueuedApplication(40, 1) || !ledger.releaseQueuedApplication(60, 1) {
		t.Fatal("aggregate queue reservations did not release")
	}
	if ledger.releaseQueuedApplication(1, 1) {
		t.Fatal("aggregate queue underflow must be rejected")
	}
}

func TestOwnerResourceLedgerClientHasNoServerQueueBudget(t *testing.T) {
	ledger := newOwnerResourceLedger(ownerResourceLimits{
		maxOutstandingRequests: 1,
	})
	for i := 0; i < 4; i++ {
		if !ledger.reserveQueuedApplication(1<<20, 1) {
			t.Fatal("Client owner must not apply a Server aggregate queue limit")
		}
	}
	if got := ledger.snapshot().queuedApplicationBytes; got != 0 {
		t.Fatalf("Client should not maintain Server queue usage, got %d", got)
	}
}

func TestOwnerResourceLedgerOnCloseCreditsSkipAtCapacity(t *testing.T) {
	ledger := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 1})
	if !ledger.reserveOnCloseEvent() {
		t.Fatal("first OnClose event should be admitted")
	}
	if ledger.reserveOnCloseEvent() {
		t.Fatal("second OnClose event must be skipped at capacity")
	}
	if !ledger.releaseOnCloseEvent() || !ledger.reserveOnCloseEvent() {
		t.Fatal("OnClose event credit was not reusable after callback return")
	}
	if !ledger.releaseOnCloseEvent() || ledger.releaseOnCloseEvent() {
		t.Fatal("OnClose event credit underflow must be rejected")
	}
}

func TestOwnerResourceLedgerOwnersAreIndependent(t *testing.T) {
	first, second := newOwnerResourceLedger(ownerResourceLimits{maxOutstandingRequests: 1}),
		newOwnerResourceLedger(ownerResourceLimits{maxOutstandingRequests: 1})
	if !first.reserveRequest() || !second.reserveRequest() {
		t.Fatal("independent owners must have independent Request budgets")
	}
	if first.reserveRequest() || second.reserveRequest() {
		t.Fatal("each owner must enforce its own ceiling")
	}
}

func TestOwnerResourceLedgerConcurrentRequestAdmissionNeverOversubscribes(t *testing.T) {
	const limit = 37
	const attempts = 512
	ledger := newOwnerResourceLedger(ownerResourceLimits{maxOutstandingRequests: limit})
	var wg sync.WaitGroup
	var accepted sync.WaitGroup
	accepted.Add(attempts)
	var mu sync.Mutex
	successes := 0
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok := ledger.reserveRequest()
			if ok {
				mu.Lock()
				successes++
				mu.Unlock()
			}
			accepted.Done()
		}()
	}
	wg.Wait()
	accepted.Wait()
	if successes != limit {
		t.Fatalf("concurrent admissions = %d, want exactly %d", successes, limit)
	}
	if got := ledger.snapshot().outstandingRequests; got != limit {
		t.Fatalf("concurrent usage = %d, want %d", got, limit)
	}
	for i := 0; i < limit; i++ {
		if !ledger.releaseRequest() {
			t.Fatalf("release %d failed", i)
		}
	}
}

func TestConstructorsCreateIndependentOwnerResourceLedgers(t *testing.T) {
	firstClient, err := NewClient(WithClientAddress("127.0.0.1:9001"))
	if err != nil {
		t.Fatalf("NewClient(first): %v", err)
	}
	secondClient, err := NewClient(WithClientAddress("127.0.0.1:9002"))
	if err != nil {
		t.Fatalf("NewClient(second): %v", err)
	}
	if firstClient.resources == nil || secondClient.resources == nil || firstClient.resources == secondClient.resources {
		t.Fatal("each Client must own an independent resource ledger")
	}

	firstServer, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewServer(first): %v", err)
	}
	secondServer, err := NewServer(WithServerListenAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewServer(second): %v", err)
	}
	if firstServer.resources == nil || secondServer.resources == nil || firstServer.resources == secondServer.resources {
		t.Fatal("each Server must own an independent resource ledger")
	}
	if !firstServer.resources.limits.hasAggregateApplicationQueue {
		t.Fatal("Server ledger must enforce aggregate Application queue limits")
	}
	if firstClient.resources.limits.hasAggregateApplicationQueue {
		t.Fatal("Client ledger must not acquire a Server-only aggregate queue limit")
	}
}
