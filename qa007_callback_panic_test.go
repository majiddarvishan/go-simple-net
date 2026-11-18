package neta

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func qa007Await(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func qa007Bound(t *testing.T, owner ownerConfig, resources *ownerResourceLedger, handlers sessionHandlers) (*Session, net.Conn) {
	t.Helper()
	local, peer := net.Pipe()
	session := newBoundSessionWithResources(
		local, defaultSessionConfig(),
		sessionIdentity{role: sessionRoleClient, bindType: wire.BindBiDirectional},
		newOwnerCallbackDispatcher(owner), handlers, resources,
	)
	if session == nil {
		_ = peer.Close()
		t.Fatal("eligible Session unexpectedly rejected")
	}
	t.Cleanup(func() {
		session.Close()
		_ = peer.Close()
		qa007Await(t, session.Done(), "Session.Done")
	})
	return session, peer
}

func TestQA007OnBoundAndOnClosePanicsAreRecovered(t *testing.T) {
	owner := defaultOwnerConfig()
	closeEntered := make(chan struct{})
	session, _ := qa007Bound(t, owner, newClientResourceLedger(owner), sessionHandlers{
		onBound: func(*Session) { panic("bound failure") },
		onClose: func(*Session, error) {
			select {
			case <-closeEntered:
			default:
				close(closeEntered)
			}
			panic("close failure")
		},
	})
	qa007Await(t, session.Done(), "panic-triggered Done")
	qa007Await(t, closeEntered, "OnClose")
	deadline := time.Now().Add(3 * time.Second)
	for {
		diagnostics := session.Diagnostics().CallbackPanics
		if diagnostics.OnClose != nil {
			if !errors.Is(session.Err(), ErrCallbackPanic) {
				t.Fatalf("Err = %v, want ErrCallbackPanic", session.Err())
			}
			if diagnostics.First == nil || diagnostics.First.Callback != "OnBound" {
				t.Fatalf("first = %+v, want OnBound", diagnostics.First)
			}
			if diagnostics.OnClose.Callback != "OnClose" || diagnostics.OnClose.Stack == "" {
				t.Fatalf("OnClose record incomplete: %+v", diagnostics.OnClose)
			}
			if len(diagnostics.Recent) < 2 || diagnostics.Recent[0].Sequence >= diagnostics.Recent[1].Sequence {
				t.Fatalf("Recent not ordered: %+v", diagnostics.Recent)
			}
			diagnostics.First.Callback = "corrupted snapshot"
			if session.Diagnostics().CallbackPanics.First.Callback != "OnBound" {
				t.Fatal("snapshot mutated internal diagnostics")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("OnClose panic not recorded")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestQA007ForcedClosePreservesAllCompletionAndCredit(t *testing.T) {
	owner := defaultOwnerConfig()
	owner.completionWorkers = 1
	ledger := newClientResourceLedger(owner)
	session, _ := qa007Bound(t, owner, ledger, sessionHandlers{})
	first := make(chan struct{})
	second := make(chan struct{})
	firstSequence, err := session.SendRequest([]byte("first"), func(response Response, err error) {
		if !errors.Is(err, ErrClosed) {
			t.Errorf("first Completion error = %v", err)
		}
		close(first)
		panic("completion panicked")
	})
	if err != nil {
		t.Fatal(err)
	}
	secondSequence, err := session.SendRequest([]byte("second"), func(response Response, err error) {
		if !errors.Is(err, ErrClosed) {
			t.Errorf("second Completion error = %v", err)
		}
		close(second)
	})
	if err != nil || secondSequence == firstSequence {
		t.Fatalf("second admission = (%d, %v), first=%d", secondSequence, err, firstSequence)
	}
	session.Close()
	qa007Await(t, session.Done(), "forced close")
	qa007Await(t, first, "first Completion")
	qa007Await(t, second, "second Completion")
	deadline := time.Now().Add(3 * time.Second)
	for ledger.snapshot().outstandingRequests != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if remaining := ledger.snapshot().outstandingRequests; remaining != 0 {
		t.Fatalf("owner Request credits retained: %d", remaining)
	}
	if session.Diagnostics().CallbackPanics.First == nil {
		t.Fatal("Completion panic not recorded")
	}
}

func TestQA007ApplicationStartClaimCancelsUnstartedAndRecordsLatePanic(t *testing.T) {
	owner := defaultOwnerConfig()
	owner.applicationWorkers = 1
	session, _ := qa007Bound(t, owner, newClientResourceLedger(owner), sessionHandlers{})
	started := make(chan struct{})
	unblock := make(chan struct{})
	cancelled := make(chan struct{})
	var called atomic.Bool
	if !session.enqueueCallback(callbackApplication, callbackTask{
		session: session, name: "OnRequest",
		run: func() {
			close(started)
			<-unblock
			panic("panic after Done")
		},
	}) {
		t.Fatal("first Application not admitted")
	}
	qa007Await(t, started, "first Application start")
	if !session.enqueueCallback(callbackApplication, callbackTask{
		session: session, name: "OnRequest",
		run: func() { called.Store(true) },
		cancel: func() { close(cancelled) },
	}) {
		t.Fatal("second Application not admitted")
	}
	session.Close()
	qa007Await(t, cancelled, "unstarted Application cancellation")
	qa007Await(t, session.Done(), "Done independent of active callback")
	close(unblock)
	deadline := time.Now().Add(3 * time.Second)
	for session.Diagnostics().CallbackPanics.First == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if called.Load() {
		t.Fatal("an unclaimed Application callback ran after Close")
	}
	record := session.Diagnostics().CallbackPanics.First
	if record == nil || record.Callback != "OnRequest" || record.Stack == "" {
		t.Fatalf("late panic record = %+v", record)
	}
	if !errors.Is(session.Err(), ErrClosed) {
		t.Fatalf("late panic replaced first close cause: %v", session.Err())
	}
}

func TestQA007OnCloseCreditReservedBeforeAdmission(t *testing.T) {
	owner := defaultOwnerConfig()
	owner.onCloseEventCredits = 1
	ledger := newClientResourceLedger(owner)
	started := make(chan struct{})
	release := make(chan struct{})
	var callbacks atomic.Int32
	handlers := sessionHandlers{onClose: func(*Session, error) {
		if callbacks.Add(1) == 1 {
			close(started)
			<-release
		}
	}}
	session, _ := qa007Bound(t, owner, ledger, handlers)
	rejectedConn, rejectedPeer := net.Pipe()
	rejected := newBoundSessionWithResources(rejectedConn, defaultSessionConfig(),
		sessionIdentity{role: sessionRoleClient, bindType: wire.BindBiDirectional},
		newOwnerCallbackDispatcher(owner), handlers, ledger)
	_ = rejectedPeer.Close()
	if rejected != nil {
		t.Fatal("saturated admission did not reject")
	}
	session.Close()
	qa007Await(t, session.Done(), "Done with OnClose blocked")
	qa007Await(t, started, "OnClose start")
	if used := ledger.snapshot().onCloseEventCredits; used != 1 {
		t.Fatalf("OnClose credit not held while running: %d", used)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for ledger.snapshot().onCloseEventCredits != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if used := ledger.snapshot().onCloseEventCredits; used != 0 {
		t.Fatalf("OnClose credit leak: %d", used)
	}
}

type qa007HostileStringer struct{}

func (qa007HostileStringer) String() string { panic("user stringer invoked") }

func TestQA007SafePanicValueAndBoundedRecentRing(t *testing.T) {
	_, text, _ := safePanicText(qa007HostileStringer{})
	if !strings.Contains(text, "non-primitive") {
		t.Fatalf("unsafe rendering: %q", text)
	}
	_, bounded, truncated := safePanicText(strings.Repeat("é", 600))
	if !truncated || len(bounded) > maxPanicValueTextBytes {
		t.Fatalf("UTF-8 bound failed: %d, %v", len(bounded), truncated)
	}
	owner := defaultOwnerConfig()
	session, _ := qa007Bound(t, owner, newClientResourceLedger(owner), sessionHandlers{})
	session.config.callbackPanicRingCapacity = 1
	for i := 0; i < 3; i++ {
		session.recordCallbackPanic("test", i)
	}
	history := session.Diagnostics().CallbackPanics
	if history.First == nil || len(history.Recent) != 1 || history.Dropped != 2 {
		t.Fatalf("ring history = %+v", history)
	}
	if history.First.Sequence != 1 || history.Recent[0].Sequence != 3 {
		t.Fatalf("recording order = %+v", history)
	}
}

func TestQA007PanicConfigValidation(t *testing.T) {
	config := defaultSessionConfig()
	for _, capacity := range []uint32{0, 33} {
		config.callbackPanicRingCapacity = capacity
		if err := validateSessionConfig(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("ring %d: %v", capacity, err)
		}
	}
	config = defaultSessionConfig()
	for _, budget := range []uint64{0, 63 << 10, (2 << 20) + 1} {
		config.callbackPanicByteBudget = budget
		if err := validateSessionConfig(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("budget %d: %v", budget, err)
		}
	}
}
