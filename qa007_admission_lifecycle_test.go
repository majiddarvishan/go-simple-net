package neta

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func waitQA007OwnerOnCloseReleased(t *testing.T, ledger *ownerResourceLedger) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for ledger.snapshot().onCloseEventCredits != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := ledger.snapshot().onCloseEventCredits; got != 0 {
		t.Fatalf("owner OnClose credit leaked: %d", got)
	}
}

func TestQA007BindValidatorPanicClosesOnlyOffenderAndReleasesCredit(t *testing.T) {
	closed := make(chan *Session, 4)
	server := newLifecycleTestServer(t,
		WithServerOnCloseEventCredits(1),
		WithServerOnCloseHandler(func(session *Session, _ error) {
			closed <- session
		}),
		WithBindValidator(func(info BindInfo) error {
			if info.SystemID == "panic-validator" {
				panic("BindValidator failure")
			}
			return nil
		}),
	)
	bad, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("Dial bad peer: %v", err)
	}
	defer bad.Close()
	request, err := wire.EncodeBindRequest(1, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "panic-validator",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, bad, request)

	var failed *Session
	select {
	case failed = <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("recovered BindValidator panic did not dispatch OnClose")
	}
	if !errors.Is(failed.Err(), ErrCallbackPanic) {
		t.Fatalf("validator panic terminal cause = %v", failed.Err())
	}
	if diagnostic := failed.Diagnostics().CallbackPanics.First; diagnostic == nil ||
		diagnostic.Callback != "BindValidator" || diagnostic.Stack == "" {
		t.Fatalf("validator panic diagnostics = %+v", diagnostic)
	}
	waitQA007OwnerOnCloseReleased(t, server.resources)

	good, response := bindLifecyclePeer(t, server.Addr().String(), "healthy-peer")
	if response.Header.Status != wire.StatusOK {
		t.Fatalf("healthy peer Bind status after panic = %d", response.Header.Status)
	}
	_ = good.Close()
	select {
	case recovered := <-closed:
		if errors.Is(recovered.Err(), ErrCallbackPanic) {
			t.Fatal("unaffected healthy Session inherited panic terminal cause")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("healthy Session OnClose was lost")
	}
	waitQA007OwnerOnCloseReleased(t, server.resources)
}

func TestQA007PreBindTimeoutReleasesOnCloseOwnerCredit(t *testing.T) {
	owner := defaultOwnerConfig()
	owner.onCloseEventCredits = 1
	ledger := newOwnerResourceLedger(ownerResourceLimits{maxOnCloseEventCredits: 1})
	closed := make(chan *Session, 1)
	local, peer := net.Pipe()
	defer peer.Close()

	session, err := newServerPreBindSessionWithResources(
		local, defaultSessionConfig(), time.Now(), 25*time.Millisecond,
		newOwnerCallbackDispatcher(owner),
		sessionHandlers{onClose: func(s *Session, _ error) { closed <- s }},
		ledger,
	)
	if err != nil {
		t.Fatalf("server pre-Bind admission: %v", err)
	}
	if got := ledger.snapshot().onCloseEventCredits; got != 1 {
		t.Fatalf("pre-Bind Session did not pre-reserve credit: %d", got)
	}
	defer session.Close()
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrBindTimeout) {
		t.Fatalf("pre-Bind terminal error = %v, want ErrBindTimeout", session.Err())
	}
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("pre-Bind Timeout failed to notify OnClose")
	}
	waitQA007OwnerOnCloseReleased(t, ledger)
}
