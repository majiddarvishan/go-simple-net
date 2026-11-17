package neta

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

type testSessionDispatcher struct {
	mu          sync.Mutex
	application []callbackTask
	completion  []callbackTask
	onClose     []callbackTask
}

func (d *testSessionDispatcher) enqueueApplication(task callbackTask) bool {
	d.mu.Lock()
	d.application = append(d.application, task)
	d.mu.Unlock()
	return true
}

func (d *testSessionDispatcher) enqueueCompletion(task callbackTask) bool {
	d.mu.Lock()
	d.completion = append(d.completion, task)
	d.mu.Unlock()
	return true
}

func (d *testSessionDispatcher) enqueueOnClose(task callbackTask) bool {
	d.mu.Lock()
	d.onClose = append(d.onClose, task)
	d.mu.Unlock()
	return true
}

func (d *testSessionDispatcher) cancelUnstarted(session *Session, mask callbackClassMask) {
	d.mu.Lock()
	var cancelled []callbackTask
	filter := func(tasks []callbackTask, enabled bool) []callbackTask {
		if !enabled {
			return tasks
		}
		kept := make([]callbackTask, 0, len(tasks))
		for _, task := range tasks {
			if task.session == session {
				cancelled = append(cancelled, task)
			} else {
				kept = append(kept, task)
			}
		}
		return kept
	}
	d.application = filter(d.application, mask&callbackMaskApplication != 0)
	d.completion = filter(d.completion, mask&callbackMaskCompletion != 0)
	d.mu.Unlock()
	for _, task := range cancelled {
		if task.cancel != nil {
			task.cancel()
		}
	}
}

func (d *testSessionDispatcher) takeApplication() (callbackTask, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.application) == 0 {
		return callbackTask{}, false
	}
	task := d.application[0]
	d.application[0] = callbackTask{}
	d.application = d.application[1:]
	return task, true
}

func (d *testSessionDispatcher) takeCompletion() (callbackTask, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.completion) == 0 {
		return callbackTask{}, false
	}
	task := d.completion[0]
	d.completion[0] = callbackTask{}
	d.completion = d.completion[1:]
	return task, true
}

func (d *testSessionDispatcher) takeOnClose() (callbackTask, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.onClose) == 0 {
		return callbackTask{}, false
	}
	task := d.onClose[0]
	d.onClose[0] = callbackTask{}
	d.onClose = d.onClose[1:]
	return task, true
}

func (d *testSessionDispatcher) completionCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.completion)
}

func waitForCompletionQueued(t *testing.T, dispatcher *testSessionDispatcher) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if dispatcher.completionCount() != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Completion callback was not queued")
}

func (d *testSessionDispatcher) applicationCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.application)
}

func waitForTask(t *testing.T, take func() (callbackTask, bool)) callbackTask {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if task, ok := take(); ok {
			return task
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("callback task was not enqueued")
	return callbackTask{}
}

func waitForApplicationQueued(t *testing.T, dispatcher *testSessionDispatcher) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if dispatcher.applicationCount() != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Application callback was not queued")
}

func readWireFrame(t *testing.T, conn net.Conn) wire.Frame {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var headerBytes [wire.HeaderSize]byte
	if _, err := io.ReadFull(conn, headerBytes[:]); err != nil {
		t.Fatalf("read frame header: %v", err)
	}
	header, err := wire.ParseHeader(headerBytes[:])
	if err != nil {
		t.Fatalf("parse frame header: %v", err)
	}
	body := make([]byte, int(header.Length)-wire.HeaderSize)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read frame body: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	return wire.Frame{Header: header, Body: body}
}

func writeWireFrame(t *testing.T, conn net.Conn, frame []byte) {
	t.Helper()
	if err := writeFullFrame(conn, frame, 2*time.Second); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func newTestBoundSession(local net.Conn, config sessionConfig, dispatcher *testSessionDispatcher, handlers sessionHandlers) *Session {
	return newTestBoundSessionWithResources(local, config, dispatcher, handlers, newClientResourceLedger(defaultOwnerConfig()))
}

func newTestBoundSessionWithResources(local net.Conn, config sessionConfig, dispatcher *testSessionDispatcher, handlers sessionHandlers, resources *ownerResourceLedger) *Session {
	return newBoundSessionWithResources(local, config, sessionIdentity{
		role: sessionRoleClient, bindType: wire.BindBiDirectional,
	}, dispatcher, handlers, resources)
}

func waitDone(t *testing.T, session *Session) {
	t.Helper()
	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Session.Done did not close")
	}
}

func TestSessionSendRequestCorrelatesResponse(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{})
	defer peer.Close()

	result := make(chan struct {
		response Response
		err      error
	}, 1)
	sequence, err := session.SendRequest([]byte("request"), func(response Response, err error) {
		result <- struct {
			response Response
			err      error
		}{response: response, err: err}
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	outbound := readWireFrame(t, peer)
	if outbound.Header.Command != wire.CommandStreamRequest || outbound.Header.Sequence != sequence {
		t.Fatalf("unexpected outbound frame: %+v", outbound.Header)
	}
	responseFrame, err := wire.EncodeStreamResponse(sequence, wire.StatusOK, []byte("response"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, responseFrame)

	task := waitForTask(t, dispatcher.takeCompletion)
	task.run()
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("Completion error: %v", got.err)
		}
		if string(got.response.Payload) != "response" || got.response.Sequence != sequence {
			t.Fatalf("unexpected response: %+v", got.response)
		}
	case <-time.After(time.Second):
		t.Fatal("Completion callback did not run")
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionOnBoundIsQueuedBeforeApplicationRequest(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	var order []string
	session, err := newClientBindingSession(local, defaultSessionConfig(), wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "client",
	}, time.Second, dispatcher, sessionHandlers{
		onBound:   func(*Session) { order = append(order, "bound") },
		onRequest: func(*Session, Request) { order = append(order, "request") },
	})
	if err != nil {
		t.Fatalf("newClientBindingSession: %v", err)
	}
	defer peer.Close()

	bindRequest := readWireFrame(t, peer)
	bindResponse, err := wire.EncodeBindResponse(bindRequest.Header.Sequence, wire.StatusOK, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "server",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, bindResponse)
	request, err := wire.EncodeStreamRequest(55, []byte("after-bind"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		dispatcher.mu.Lock()
		count := len(dispatcher.application)
		dispatcher.mu.Unlock()
		if count >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	first, ok := dispatcher.takeApplication()
	if !ok {
		t.Fatal("OnBound callback was not queued")
	}
	second, ok := dispatcher.takeApplication()
	if !ok {
		t.Fatal("OnRequest callback was not queued")
	}
	first.run()
	second.run()
	if len(order) != 2 || order[0] != "bound" || order[1] != "request" {
		t.Fatalf("callback order = %v", order)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionCompletionFIFOTracksSettlementOrder(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{})
	defer peer.Close()

	var completed []uint32
	first, err := session.SendRequest([]byte("first"), func(response Response, err error) {
		if err != nil {
			t.Errorf("first Completion: %v", err)
			return
		}
		completed = append(completed, response.Sequence)
	})
	if err != nil {
		t.Fatalf("first SendRequest: %v", err)
	}
	second, err := session.SendRequest([]byte("second"), func(response Response, err error) {
		if err != nil {
			t.Errorf("second Completion: %v", err)
			return
		}
		completed = append(completed, response.Sequence)
	})
	if err != nil {
		t.Fatalf("second SendRequest: %v", err)
	}

	_ = readWireFrame(t, peer)
	_ = readWireFrame(t, peer)
	secondResponse, err := wire.EncodeStreamResponse(second, wire.StatusOK, []byte("second-result"))
	if err != nil {
		t.Fatal(err)
	}
	firstResponse, err := wire.EncodeStreamResponse(first, wire.StatusOK, []byte("first-result"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, secondResponse)
	writeWireFrame(t, peer, firstResponse)

	firstTask := waitForTask(t, dispatcher.takeCompletion)
	secondTask := waitForTask(t, dispatcher.takeCompletion)
	firstTask.run()
	secondTask.run()
	if len(completed) != 2 || completed[0] != second || completed[1] != first {
		t.Fatalf("Completion order = %v, want [%d %d]", completed, second, first)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionUnbindDrainsApplicationFIFO(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{})
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("before-unbind")); err != nil {
		t.Fatalf("SendNoWait: %v", err)
	}
	unbindResult := make(chan error, 1)
	go func() { unbindResult <- session.Unbind(context.Background()) }()

	first := readWireFrame(t, peer)
	if first.Header.Command != wire.CommandStreamRequest || string(first.Body) != "before-unbind" {
		t.Fatalf("Application FIFO frame was not first: %+v", first.Header)
	}
	second := readWireFrame(t, peer)
	if second.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("Unbind did not follow the Application FIFO: %+v", second.Header)
	}
	response, err := wire.EncodeUnbindResponse(second.Header.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)

	select {
	case err := <-unbindResult:
		if err != nil {
			t.Fatalf("Unbind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Unbind did not complete")
	}
	waitDone(t, session)
}

func TestSessionUnbindStopsNewApplicationAdmission(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{
		onRequest: func(*Session, Request) {},
	})
	defer peer.Close()

	firstRequest, err := wire.EncodeStreamRequest(1, []byte("accepted-before-unbind"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, firstRequest)
	waitForApplicationQueued(t, dispatcher)

	unbindResult := make(chan error, 1)
	go func() { unbindResult <- session.Unbind(context.Background()) }()
	unbind := readWireFrame(t, peer)
	if unbind.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("expected Unbind Request, got 0x%x", unbind.Header.Command)
	}

	secondRequest, err := wire.EncodeStreamRequest(2, []byte("after-unbind"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, secondRequest)
	time.Sleep(20 * time.Millisecond)
	if got := dispatcher.applicationCount(); got != 1 {
		t.Fatalf("Application callback count after Unbind = %d, want 1", got)
	}

	response, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)
	select {
	case err := <-unbindResult:
		if err != nil {
			t.Fatalf("Unbind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Unbind did not complete")
	}
	waitDone(t, session)
}

func TestSessionReadsFragmentedHeaderAndBody(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	received := make(chan string, 1)
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{
		onRequest: func(_ *Session, request Request) { received <- string(request.Payload) },
	})
	defer peer.Close()

	frame, err := wire.EncodeStreamRequest(18, []byte("fragmented-payload"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range frame {
		if _, err := peer.Write(frame[i : i+1]); err != nil {
			t.Fatalf("write fragment %d: %v", i, err)
		}
	}
	task := waitForTask(t, dispatcher.takeApplication)
	task.run()
	select {
	case payload := <-received:
		if payload != "fragmented-payload" {
			t.Fatalf("payload = %q", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("fragmented Request callback did not run")
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionEnquireLinkDisabledWhenIdleIntervalIsZero(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	_ = peer.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
	var one [1]byte
	if _, err := peer.Read(one[:]); err == nil {
		t.Fatal("EnquireLink was sent while automatic probes were disabled")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("unexpected read error: %v", err)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionCancelsQueuedEnquireLinkAfterInboundActivity(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 40 * time.Millisecond
	config.enquireLinkResponseTimeout = 100 * time.Millisecond
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("writer-blocker")); err != nil {
		t.Fatalf("SendNoWait: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		queued := session.enquireLinkQueued && !session.enquireLinkWriting
		session.mu.Unlock()
		if queued {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	queued := session.enquireLinkQueued && !session.enquireLinkWriting
	session.mu.Unlock()
	if !queued {
		t.Fatal("EnquireLink was not queued behind the blocked Writer")
	}

	inbound, err := wire.EncodeStreamRequest(61, []byte("activity"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, inbound)
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		stillQueued := session.enquireLinkQueued
		session.mu.Unlock()
		if !stillQueued {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	stillQueued := session.enquireLinkQueued
	session.config.enquireLinkIdleInterval = 0
	session.mu.Unlock()
	if stillQueued {
		t.Fatal("inbound activity did not cancel the queued EnquireLink")
	}

	outbound := readWireFrame(t, peer)
	if outbound.Header.Command != wire.CommandStreamRequest || string(outbound.Body) != "writer-blocker" {
		t.Fatalf("unexpected outbound frame: %+v", outbound.Header)
	}
	_ = peer.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
	var one [1]byte
	if _, err := peer.Read(one[:]); err == nil {
		t.Fatal("cancelled EnquireLink was transmitted")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("unexpected read error: %v", err)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionCloseClearsTransportQueuesAndPendingState(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{})
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("writer-owned")); err != nil {
		t.Fatalf("SendNoWait: %v", err)
	}
	if _, err := session.SendRequest([]byte("queued-request"), func(Response, error) {}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	session.Close()
	waitDone(t, session)

	session.mu.Lock()
	defer session.mu.Unlock()
	if len(session.pending) != 0 || len(session.appQueue) != 0 || len(session.controlQueue) != 0 || len(session.barrierQueue) != 0 {
		t.Fatalf("Session retained internal queues: pending=%d app=%d control=%d barrier=%d", len(session.pending), len(session.appQueue), len(session.controlQueue), len(session.barrierQueue))
	}
	if session.activeWrite != nil || session.queuedAppBytes != 0 || session.queuedAppFrames != 0 || session.queuedControlBytes != 0 {
		t.Fatalf("Session retained transport charges: active=%v appBytes=%d appFrames=%d controlBytes=%d", session.activeWrite != nil, session.queuedAppBytes, session.queuedAppFrames, session.queuedControlBytes)
	}
}

func TestSessionOnCloseIsQueuedOnlyAfterDone(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	observedDone := make(chan bool, 1)
	session := newBoundSession(local, defaultSessionConfig(), sessionIdentity{
		role: sessionRoleClient, bindType: wire.BindBiDirectional,
	}, dispatcher, sessionHandlers{
		onClose: func(s *Session, _ error) {
			select {
			case <-s.Done():
				observedDone <- true
			default:
				observedDone <- false
			}
		},
	})
	defer peer.Close()

	session.Close()
	waitDone(t, session)
	task := waitForTask(t, dispatcher.takeOnClose)
	task.run()
	select {
	case done := <-observedDone:
		if !done {
			t.Fatal("OnClose ran before Session.Done")
		}
	case <-time.After(time.Second):
		t.Fatal("OnClose callback did not run")
	}
}

func TestSessionRejectsZeroSequence(t *testing.T) {
	local, peer := net.Pipe()
	session := newTestBoundSession(local, defaultSessionConfig(), &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	header := make([]byte, wire.HeaderSize)
	binary.BigEndian.PutUint32(header[0:4], uint32(wire.HeaderSize))
	header[4] = byte(wire.CommandStreamRequest)
	header[5] = 0
	binary.BigEndian.PutUint32(header[6:10], 0)
	writeWireFrame(t, peer, header)
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrProtocol) {
		t.Fatalf("expected ErrProtocol for zero sequence, got %v", session.Err())
	}
	if err := session.SendResponse(0, nil, 0); !errors.Is(err, ErrProtocol) {
		t.Fatalf("SendResponse with zero sequence = %v, want ErrProtocol", err)
	}
}

func TestSessionFrameAssemblyTimeoutIsAbsolute(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.frameAssemblyTimeout = 40 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	if _, err := peer.Write([]byte{0}); err != nil {
		t.Fatalf("write first Header byte: %v", err)
	}
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		for i := 0; i < wire.HeaderSize-1; i++ {
			time.Sleep(20 * time.Millisecond)
			if _, err := peer.Write([]byte{0}); err != nil {
				return
			}
		}
	}()
	waitDone(t, session)
	<-senderDone
	if !errors.Is(session.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected frame assembly deadline, got %v", session.Err())
	}
}

func TestSessionServerPreBindTimeout(t *testing.T) {
	local, peer := net.Pipe()
	session, err := newServerPreBindSession(local, defaultSessionConfig(), time.Now(), 40*time.Millisecond, &testSessionDispatcher{}, sessionHandlers{})
	if err != nil {
		t.Fatalf("newServerPreBindSession: %v", err)
	}
	defer peer.Close()
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrBindTimeout) {
		t.Fatalf("expected ErrBindTimeout, got %v", session.Err())
	}
}

func TestSessionClientBindTimeout(t *testing.T) {
	local, peer := net.Pipe()
	session, err := newClientBindingSession(local, defaultSessionConfig(), wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "client",
	}, 40*time.Millisecond, &testSessionDispatcher{}, sessionHandlers{})
	if err != nil {
		t.Fatalf("newClientBindingSession: %v", err)
	}
	defer peer.Close()
	_ = readWireFrame(t, peer)
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrBindTimeout) {
		t.Fatalf("expected ErrBindTimeout, got %v", session.Err())
	}
}

func TestSessionUnbindTimeoutIsAbsolute(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.unbindTimeout = 40 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	result := make(chan error, 1)
	go func() { result <- session.Unbind(context.Background()) }()
	frame := readWireFrame(t, peer)
	if frame.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("expected Unbind Request, got 0x%x", frame.Header.Command)
	}
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrUnbindTimeout) {
		t.Fatalf("expected ErrUnbindTimeout, got %v", session.Err())
	}
	select {
	case err := <-result:
		if !errors.Is(err, ErrUnbindTimeout) {
			t.Fatalf("Unbind result = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Unbind caller was not released")
	}
}

func TestSessionEnquireLinkResponseTimeout(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 10 * time.Millisecond
	config.enquireLinkResponseTimeout = 40 * time.Millisecond
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	probe := readWireFrame(t, peer)
	if probe.Header.Command != wire.CommandEnquireLinkRequest {
		t.Fatalf("expected EnquireLink Request, got 0x%x", probe.Header.Command)
	}
	waitDone(t, session)
	var operationErr *OperationError
	if !errors.As(session.Err(), &operationErr) || operationErr.Operation() != "enquire_link" || !errors.Is(session.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected enquire_link deadline OperationError, got %v", session.Err())
	}
}

func TestSessionClientBindHandshake(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session, err := newClientBindingSession(local, defaultSessionConfig(), wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "client",
	}, time.Second, dispatcher, sessionHandlers{})
	if err != nil {
		t.Fatalf("newClientBindingSession: %v", err)
	}
	defer peer.Close()

	request := readWireFrame(t, peer)
	bind, err := wire.DecodeBindRequest(request)
	if err != nil {
		t.Fatalf("DecodeBindRequest: %v", err)
	}
	if bind.SystemID != "client" || bind.Type != wire.BindBiDirectional {
		t.Fatalf("unexpected Bind Request: %+v", bind)
	}
	response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "server",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)
	if err := session.waitBound(context.Background()); err != nil {
		t.Fatalf("waitBound: %v", err)
	}
	if got := session.SystemID(); got != "server" {
		t.Fatalf("SystemID = %q, want server", got)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionServerPreBindHandshake(t *testing.T) {
	local, peer := net.Pipe()
	gate := make(chan struct{})
	wrapped := writeReturnGateConn{Conn: local, gate: gate}
	dispatcher := &testSessionDispatcher{}
	var order []string
	session, err := newServerPreBindSession(wrapped, defaultSessionConfig(), time.Now(), time.Second, dispatcher, sessionHandlers{
		onBound:   func(*Session) { order = append(order, "bound") },
		onRequest: func(*Session, Request) { order = append(order, "request") },
	})
	if err != nil {
		t.Fatalf("newServerPreBindSession: %v", err)
	}
	defer peer.Close()

	request, err := wire.EncodeBindRequest(41, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	event, err := session.waitBindRequest(context.Background())
	if err != nil {
		t.Fatalf("waitBindRequest: %v", err)
	}
	if event.info.SystemID != "peer" || event.sequence != 41 {
		t.Fatalf("unexpected BindInfo: %+v", event.info)
	}
	if err := session.completeBind(event, "server", nil); err != nil {
		t.Fatalf("completeBind: %v", err)
	}
	response := readWireFrame(t, peer)
	bind, err := wire.DecodeBindResponse(response)
	if err != nil {
		t.Fatalf("DecodeBindResponse: %v", err)
	}
	if response.Header.Sequence != 41 || bind.SystemID != "server" {
		t.Fatalf("unexpected Bind Response: header=%+v bind=%+v", response.Header, bind)
	}

	firstFrame, err := wire.EncodeStreamRequest(42, []byte("first-after-bind"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, firstFrame)
	if dispatcher.applicationCount() != 0 {
		t.Fatal("post-Bind Application callback escaped before Session became Bound")
	}
	close(gate)
	if err := session.waitBound(context.Background()); err != nil {
		t.Fatalf("waitBound: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && dispatcher.applicationCount() < 2 {
		time.Sleep(time.Millisecond)
	}
	first, ok := dispatcher.takeApplication()
	if !ok {
		t.Fatal("OnBound callback was not queued")
	}
	second, ok := dispatcher.takeApplication()
	if !ok {
		t.Fatal("post-Bind OnRequest callback was not queued")
	}
	first.run()
	second.run()
	if len(order) != 2 || order[0] != "bound" || order[1] != "request" {
		t.Fatalf("callback order = %v", order)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionPreBindForcedCloseUnblocksHandshake(t *testing.T) {
	local, peer := net.Pipe()
	session, err := newServerPreBindSession(local, defaultSessionConfig(), time.Now(), time.Second, &testSessionDispatcher{}, sessionHandlers{})
	if err != nil {
		t.Fatalf("newServerPreBindSession: %v", err)
	}
	defer peer.Close()

	request, err := wire.EncodeBindRequest(17, wire.Bind{Type: wire.BindBiDirectional, SystemID: "peer"})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	if _, err := session.waitBindRequest(context.Background()); err != nil {
		t.Fatalf("waitBindRequest: %v", err)
	}

	session.Close()
	waitDone(t, session)
}

func TestSessionForcedCloseCancelsUnstartedApplicationCallback(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	var called bool
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{
		onRequest: func(*Session, Request) { called = true },
	})
	defer peer.Close()

	request, err := wire.EncodeStreamRequest(7, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	waitForApplicationQueued(t, dispatcher)
	session.Close()
	waitDone(t, session)
	if called {
		t.Fatal("unstarted Application callback ran during Forced Close")
	}
	if dispatcher.applicationCount() != 0 {
		t.Fatal("unstarted Application callback was not cancelled")
	}
}

func TestSessionRequestTimeoutCancelsQueuedFrame(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.requestTimeout = 250 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, config, dispatcher, sessionHandlers{})
	defer peer.Close()

	var firstResult, secondResult error
	firstSequence, err := session.SendRequest([]byte("first"), func(_ Response, err error) { firstResult = err })
	if err != nil {
		t.Fatalf("first SendRequest: %v", err)
	}
	if _, err := session.SendRequest([]byte("second"), func(_ Response, err error) { secondResult = err }); err != nil {
		t.Fatalf("second SendRequest: %v", err)
	}
	writerDeadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(writerDeadline) {
		session.mu.Lock()
		pending := session.pending[firstSequence]
		claimed := pending != nil && pending.writerClaimed
		session.mu.Unlock()
		if claimed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	claimed := session.pending[firstSequence] != nil && session.pending[firstSequence].writerClaimed
	session.mu.Unlock()
	if !claimed {
		t.Fatal("Writer did not claim the first Request frame")
	}

	firstTask := waitForTask(t, dispatcher.takeCompletion)
	secondTask := waitForTask(t, dispatcher.takeCompletion)
	firstTask.run()
	secondTask.run()
	if !errors.Is(firstResult, ErrRequestTimeout) || !errors.Is(secondResult, ErrRequestTimeout) {
		t.Fatalf("request timeout results = %v, %v", firstResult, secondResult)
	}

	firstFrame := readWireFrame(t, peer)
	if firstFrame.Header.Command != wire.CommandStreamRequest || string(firstFrame.Body) != "first" {
		t.Fatalf("unexpected writer-owned frame: %+v", firstFrame.Header)
	}
	_ = peer.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
	var extra [1]byte
	if _, err := peer.Read(extra[:]); err == nil {
		t.Fatal("timed-out queued Request was transmitted")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("unexpected read error after first frame: %v", err)
	}
	session.Close()
	waitDone(t, session)
}

type writeReturnGateConn struct {
	net.Conn
	gate <-chan struct{}
}

func (c writeReturnGateConn) Write(payload []byte) (int, error) {
	n, err := c.Conn.Write(payload)
	if err == nil {
		<-c.gate
	}
	return n, err
}

func TestSessionEnquireLinkResponseMayArriveBeforeWriterReturns(t *testing.T) {
	local, peer := net.Pipe()
	gate := make(chan struct{})
	wrapped := writeReturnGateConn{Conn: local, gate: gate}
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 20 * time.Millisecond
	config.enquireLinkResponseTimeout = 80 * time.Millisecond
	dispatcher := &testSessionDispatcher{}
	session := newBoundSession(wrapped, config, sessionIdentity{
		role: sessionRoleClient, bindType: wire.BindBiDirectional,
	}, dispatcher, sessionHandlers{})
	defer peer.Close()

	probe := readWireFrame(t, peer)
	if probe.Header.Command != wire.CommandEnquireLinkRequest {
		t.Fatalf("expected EnquireLink Request, got 0x%x", probe.Header.Command)
	}
	response, err := wire.EncodeEnquireLinkResponse(probe.Header.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		seen := session.enquireLinkResponseSeen
		session.mu.Unlock()
		if seen {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	seen := session.enquireLinkResponseSeen
	session.mu.Unlock()
	if !seen {
		close(gate)
		t.Fatal("Reader did not observe the early EnquireLink Response")
	}

	close(gate)
	session.mu.Lock()
	session.config.enquireLinkIdleInterval = 0
	session.mu.Unlock()
	time.Sleep(2 * config.enquireLinkResponseTimeout)
	if session.State() != SessionStateBound {
		t.Fatalf("Session closed after a matching early response: %v", session.Err())
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionWriteTimeoutClosesSession(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.writeTimeout = 40 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("blocked-write")); err != nil {
		t.Fatalf("SendNoWait: %v", err)
	}
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrWriteTimeout) {
		t.Fatalf("expected ErrWriteTimeout, got %v", session.Err())
	}
}

func TestSessionLateResponseCannotResurrectTimedOutRequest(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.requestTimeout = 40 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	dispatcher := &testSessionDispatcher{}
	session := newTestBoundSession(local, config, dispatcher, sessionHandlers{})
	defer peer.Close()

	result := make(chan error, 2)
	sequence, err := session.SendRequest([]byte("timeout-race"), func(_ Response, err error) {
		result <- err
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	_ = readWireFrame(t, peer)
	waitForCompletionQueued(t, dispatcher)

	lateResponse, err := wire.EncodeStreamResponse(sequence, wire.StatusOK, []byte("too-late"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, lateResponse)
	task := waitForTask(t, dispatcher.takeCompletion)
	task.run()
	select {
	case err := <-result:
		if !errors.Is(err, ErrRequestTimeout) {
			t.Fatalf("Completion error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed-out Completion did not run")
	}
	time.Sleep(20 * time.Millisecond)
	if got := dispatcher.completionCount(); got != 0 {
		t.Fatalf("late Response created %d duplicate Completion tasks", got)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionForcedCloseCancelsQueuedCompletionCallback(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	called := make(chan struct{}, 1)
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{})
	defer peer.Close()

	sequence, err := session.SendRequest([]byte("pending"), func(Response, error) {
		called <- struct{}{}
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	_ = readWireFrame(t, peer)
	response, err := wire.EncodeStreamResponse(sequence, wire.StatusOK, []byte("done"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, response)
	waitForCompletionQueued(t, dispatcher)

	session.Close()
	waitDone(t, session)
	if got := dispatcher.completionCount(); got != 0 {
		t.Fatalf("queued Completion callbacks after Forced Close = %d", got)
	}
	select {
	case <-called:
		t.Fatal("unstarted Completion callback ran during Forced Close")
	default:
	}
}

func TestSessionDoneDoesNotWaitForRunningApplicationCallback(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	session := newTestBoundSession(local, defaultSessionConfig(), dispatcher, sessionHandlers{
		onRequest: func(*Session, Request) {
			close(started)
			<-release
			close(finished)
		},
	})
	defer peer.Close()

	request, err := wire.EncodeStreamRequest(27, []byte("blocking-callback"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	task := waitForTask(t, dispatcher.takeApplication)
	go task.run()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Application callback did not start")
	}

	session.Close()
	waitDone(t, session)
	select {
	case <-finished:
		t.Fatal("Session.Done waited for the running user callback")
	default:
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("running callback did not finish after release")
	}
}

func TestSessionSendResponseUsesPeerSequence(t *testing.T) {
	local, peer := net.Pipe()
	dispatcher := &testSessionDispatcher{}
	session := newBoundSession(local, defaultSessionConfig(), sessionIdentity{
		role: sessionRoleServer, bindType: wire.BindBiDirectional,
	}, dispatcher, sessionHandlers{
		onRequest: func(s *Session, request Request) {
			if err := s.SendResponse(request.Sequence, []byte("ack"), 0); err != nil {
				t.Errorf("SendResponse: %v", err)
			}
		},
	})
	defer peer.Close()

	request, err := wire.EncodeStreamRequest(99, []byte("incoming"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	task := waitForTask(t, dispatcher.takeApplication)
	task.run()
	response := readWireFrame(t, peer)
	if response.Header.Command != wire.CommandStreamResponse || response.Header.Sequence != 99 || string(response.Body) != "ack" {
		t.Fatalf("unexpected Stream Response: %+v body=%q", response.Header, response.Body)
	}
	session.Close()
	waitDone(t, session)
}

func TestWriteFullFrameHandlesShortWrites(t *testing.T) {
	local, peer := net.Pipe()
	writer := shortWriteConn{Conn: local, maxWrite: 2}
	payload := []byte("0123456789abcdef")
	result := make(chan error, 1)
	go func() { result <- writeFullFrame(writer, payload, time.Second) }()

	got := make([]byte, len(payload))
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(peer, got); err != nil {
		t.Fatalf("read short-written payload: %v", err)
	}
	if err := <-result; err != nil {
		t.Fatalf("writeFullFrame: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
	_ = local.Close()
	_ = peer.Close()
}

type shortWriteConn struct {
	net.Conn
	maxWrite int
}

func (c shortWriteConn) Write(payload []byte) (int, error) {
	if len(payload) > c.maxWrite {
		payload = payload[:c.maxWrite]
	}
	return c.Conn.Write(payload)
}

func TestSessionIgnoresResponseBeforeWriterClaimsRequest(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.requestTimeout = 2 * time.Second
	config.enquireLinkIdleInterval = 0
	dispatcher := &testSessionDispatcher{}
	var secondResponse Response
	var secondErr error
	secondCalls := 0
	session := newTestBoundSession(local, config, dispatcher, sessionHandlers{})
	defer peer.Close()

	firstSequence, err := session.SendRequest([]byte("writer-blocker"), func(Response, error) {})
	if err != nil {
		t.Fatalf("first SendRequest: %v", err)
	}
	secondSequence, err := session.SendRequest([]byte("queued-request"), func(response Response, err error) {
		secondCalls++
		secondResponse, secondErr = response, err
	})
	if err != nil {
		t.Fatalf("second SendRequest: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		first := session.pending[firstSequence]
		second := session.pending[secondSequence]
		ready := first != nil && first.writerClaimed && second != nil && !second.writerClaimed
		session.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	first := session.pending[firstSequence]
	second := session.pending[secondSequence]
	ready := first != nil && first.writerClaimed && second != nil && !second.writerClaimed
	session.mu.Unlock()
	if !ready {
		t.Fatal("expected first Request to own Writer while second remains queued")
	}

	forged, err := wire.EncodeStreamResponse(secondSequence, wire.StatusOK, []byte("forged-before-send"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, forged)
	time.Sleep(20 * time.Millisecond)
	if got := dispatcher.completionCount(); got != 0 {
		t.Fatalf("pre-send Response queued %d Completion callbacks", got)
	}
	session.mu.Lock()
	second = session.pending[secondSequence]
	stillPending := second != nil && !second.settled && !second.writerClaimed
	session.mu.Unlock()
	if !stillPending {
		t.Fatal("pre-send Response settled or removed the queued Request")
	}

	firstFrame := readWireFrame(t, peer)
	secondFrame := readWireFrame(t, peer)
	if firstFrame.Header.Sequence != firstSequence || secondFrame.Header.Sequence != secondSequence {
		t.Fatalf("outbound Sequences = %d, %d; want %d, %d",
			firstFrame.Header.Sequence, secondFrame.Header.Sequence, firstSequence, secondSequence)
	}

	actual, err := wire.EncodeStreamResponse(secondSequence, wire.StatusOK, []byte("actual-response"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, actual)
	task := waitForTask(t, dispatcher.takeCompletion)
	task.run()
	if secondCalls != 1 || secondErr != nil || string(secondResponse.Payload) != "actual-response" {
		t.Fatalf("Completion calls=%d response=%+v error=%v", secondCalls, secondResponse, secondErr)
	}
	if got := dispatcher.completionCount(); got != 0 {
		t.Fatalf("unexpected duplicate Completion callbacks: %d", got)
	}

	session.Close()
	waitDone(t, session)
}

func TestSessionCancelsQueuedEnquireLinkAfterOutboundActivity(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 40 * time.Millisecond
	config.enquireLinkResponseTimeout = 100 * time.Millisecond
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("outbound-activity")); err != nil {
		t.Fatalf("SendNoWait: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		queued := session.enquireLinkQueued && !session.enquireLinkWriting
		session.mu.Unlock()
		if queued {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	queued := session.enquireLinkQueued && !session.enquireLinkWriting
	session.mu.Unlock()
	if !queued {
		t.Fatal("EnquireLink was not queued behind the active Application Write")
	}

	outbound := readWireFrame(t, peer)
	if outbound.Header.Command != wire.CommandStreamRequest || string(outbound.Body) != "outbound-activity" {
		t.Fatalf("unexpected outbound frame: %+v body=%q", outbound.Header, outbound.Body)
	}
	session.mu.Lock()
	session.config.enquireLinkIdleInterval = 0
	session.mu.Unlock()

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		cancelled := !session.enquireLinkQueued && !session.enquireLinkWriting && !session.enquireLinkOutstanding
		session.mu.Unlock()
		if cancelled {
			break
		}
		time.Sleep(time.Millisecond)
	}
	session.mu.Lock()
	cancelled := !session.enquireLinkQueued && !session.enquireLinkWriting && !session.enquireLinkOutstanding
	session.mu.Unlock()
	if !cancelled {
		t.Fatal("successful outbound activity did not cancel the queued EnquireLink")
	}

	_ = peer.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
	var one [1]byte
	if _, err := peer.Read(one[:]); err == nil {
		t.Fatal("queued EnquireLink was transmitted after successful outbound activity")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("unexpected read error: %v", err)
	}
	session.Close()
	waitDone(t, session)
}

func TestSessionServerFirstPostBindFrameUsesAssemblyDeadline(t *testing.T) {
	local, peer := net.Pipe()
	gate := make(chan struct{})
	wrapped := writeReturnGateConn{Conn: local, gate: gate}
	config := defaultSessionConfig()
	config.frameAssemblyTimeout = 40 * time.Millisecond
	config.enquireLinkIdleInterval = 0
	session, err := newServerPreBindSession(wrapped, config, time.Now(), time.Second, &testSessionDispatcher{}, sessionHandlers{})
	if err != nil {
		t.Fatalf("newServerPreBindSession: %v", err)
	}
	defer peer.Close()

	bindRequest, err := wire.EncodeBindRequest(41, wire.Bind{
		Type: wire.BindBiDirectional, SystemID: "peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, bindRequest)
	event, err := session.waitBindRequest(context.Background())
	if err != nil {
		t.Fatalf("waitBindRequest: %v", err)
	}
	if err := session.completeBind(event, "server", nil); err != nil {
		t.Fatalf("completeBind: %v", err)
	}
	response := readWireFrame(t, peer)
	if response.Header.Command != wire.CommandBindResponse {
		t.Fatalf("expected Bind Response, got 0x%x", response.Header.Command)
	}
	session.mu.Lock()
	responseWriteHeld := session.bindResponseWriting
	session.mu.Unlock()
	if !responseWriteHeld {
		t.Fatal("expected server Bind Response Write to remain in flight")
	}

	frame, err := wire.EncodeStreamRequest(42, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(frame[:1]); err != nil {
		t.Fatalf("write first post-Bind Header byte: %v", err)
	}
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		for i := 1; i < len(frame); i++ {
			time.Sleep(20 * time.Millisecond)
			if _, err := peer.Write(frame[i : i+1]); err != nil {
				return
			}
		}
	}()

	deadline := time.Now().Add(150 * time.Millisecond)
	closing := false
	for time.Now().Before(deadline) {
		state := session.State()
		if state == SessionStateClosing || state == SessionStateClosed {
			closing = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(gate)
	if !closing {
		session.Close()
	}
	<-senderDone
	waitDone(t, session)
	if !closing {
		t.Fatal("first post-Bind Frame did not receive its absolute assembly deadline")
	}
	var operationErr *OperationError
	if !errors.As(session.Err(), &operationErr) || operationErr.Operation() != "frame assembly" ||
		!errors.Is(session.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected first post-Bind frame assembly timeout, got %v", session.Err())
	}
}

func TestSessionUnbindContextCancellationUsesUnbindTimeoutTerminalCause(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.unbindTimeout = time.Second
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- session.Unbind(ctx) }()
	request := readWireFrame(t, peer)
	if request.Header.Command != wire.CommandUnbindRequest {
		t.Fatalf("expected Unbind Request, got 0x%x", request.Header.Command)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Unbind return error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Unbind did not return after caller Context cancellation")
	}
	waitDone(t, session)
	if !errors.Is(session.Err(), ErrUnbindTimeout) {
		t.Fatalf("Session terminal cause = %v, want ErrUnbindTimeout", session.Err())
	}
}

func TestSessionIncomingEnquireLinkAnsweredWhenDisabled(t *testing.T) {
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	session := newTestBoundSession(local, config, &testSessionDispatcher{}, sessionHandlers{})
	defer peer.Close()

	request, err := wire.EncodeEnquireLinkRequest(77)
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer, request)
	response := readWireFrame(t, peer)
	if response.Header.Command != wire.CommandEnquireLinkResponse || response.Header.Sequence != 77 {
		t.Fatalf("unexpected EnquireLink Response: %+v", response.Header)
	}
	session.Close()
	waitDone(t, session)
}


func TestSessionServerAggregateOutgoingQueueIncludesWriterOwnedFrame(t *testing.T) {
	owner := newOwnerResourceLedger(ownerResourceLimits{
		maxRetainedApplicationBytes:         64,
		hasAggregateApplicationQueue:        true,
		maxAggregateQueuedApplicationBytes:  11,
		maxAggregateQueuedApplicationFrames: 1,
	})
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	local1, peer1 := net.Pipe()
	local2, peer2 := net.Pipe()
	first := newTestBoundSessionWithResources(local1, config, &testSessionDispatcher{}, sessionHandlers{}, owner)
	second := newTestBoundSessionWithResources(local2, config, &testSessionDispatcher{}, sessionHandlers{}, owner)
	defer peer1.Close()
	defer peer2.Close()

	if _, err := first.SendNoWait([]byte("x")); err != nil {
		t.Fatalf("first SendNoWait: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := owner.snapshot(); got.queuedApplicationFrames == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := owner.snapshot(); got.queuedApplicationBytes != 11 || got.queuedApplicationFrames != 1 {
		t.Fatalf("first Writer-owned Frame charge = %+v", got)
	}
	if _, err := second.SendNoWait([]byte("x")); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("second SendNoWait error = %v, want ErrQueueFull", err)
	}
	if second.State() != SessionStateBound {
		t.Fatalf("aggregate outgoing Queue Full changed second Session state to %v", second.State())
	}

	outbound := readWireFrame(t, peer1)
	if outbound.Header.Command != wire.CommandStreamRequest || string(outbound.Body) != "x" {
		t.Fatalf("unexpected first Frame: %+v body=%q", outbound.Header, outbound.Body)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := owner.snapshot(); got.queuedApplicationFrames == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := owner.snapshot(); got.queuedApplicationBytes != 0 || got.queuedApplicationFrames != 0 || got.retainedApplicationBytes != 0 {
		t.Fatalf("owner charges after full Write = %+v", got)
	}
	first.Close()
	second.Close()
	waitDone(t, first)
	waitDone(t, second)
}

func TestSessionOutgoingRetainedApplicationBudgetIncludesWriterOwnedFrame(t *testing.T) {
	owner := newOwnerResourceLedger(ownerResourceLimits{
		maxRetainedApplicationBytes:         11,
		hasAggregateApplicationQueue:        true,
		maxAggregateQueuedApplicationBytes:  1024,
		maxAggregateQueuedApplicationFrames: 8,
	})
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	sessionConn, peer := net.Pipe()
	session := newTestBoundSessionWithResources(sessionConn, config, &testSessionDispatcher{}, sessionHandlers{}, owner)
	defer peer.Close()

	if _, err := session.SendNoWait([]byte("x")); err != nil {
		t.Fatalf("first SendNoWait: %v", err)
	}
	if got := owner.snapshot().retainedApplicationBytes; got != 11 {
		t.Fatalf("retained Application bytes = %d, want 11", got)
	}
	if _, err := session.SendNoWait([]byte("y")); !errors.Is(err, ErrCapacity) {
		t.Fatalf("second SendNoWait error = %v, want ErrCapacity", err)
	}
	if session.State() != SessionStateBound {
		t.Fatalf("retained-byte capacity failure changed Session state to %v", session.State())
	}
	_ = readWireFrame(t, peer)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if owner.snapshot().retainedApplicationBytes == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := owner.snapshot().retainedApplicationBytes; got != 0 {
		t.Fatalf("retained Application bytes after full Write = %d", got)
	}
	if _, err := session.SendNoWait([]byte("y")); err != nil {
		t.Fatalf("SendNoWait after release: %v", err)
	}
	_ = readWireFrame(t, peer)
	session.Close()
	waitDone(t, session)
}

func TestSessionRequestCreditIsOwnerScopedAndHeldThroughCompletionReturn(t *testing.T) {
	owner := newOwnerResourceLedger(ownerResourceLimits{
		maxOutstandingRequests:      1,
		maxRetainedApplicationBytes: 1024,
	})
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	local1, peer1 := net.Pipe()
	local2, peer2 := net.Pipe()
	dispatcher1 := &testSessionDispatcher{}
	first := newTestBoundSessionWithResources(local1, config, dispatcher1, sessionHandlers{}, owner)
	second := newTestBoundSessionWithResources(local2, config, &testSessionDispatcher{}, sessionHandlers{}, owner)
	defer peer1.Close()
	defer peer2.Close()

	sequence, err := first.SendRequest([]byte("request"), func(Response, error) {})
	if err != nil {
		t.Fatalf("first SendRequest: %v", err)
	}
	outbound := readWireFrame(t, peer1)
	if outbound.Header.Sequence != sequence {
		t.Fatalf("outbound Sequence = %d, want %d", outbound.Header.Sequence, sequence)
	}
	if _, err := second.SendRequest([]byte("blocked"), func(Response, error) {}); !errors.Is(err, ErrCapacity) {
		t.Fatalf("second SendRequest error = %v, want ErrCapacity", err)
	}
	if got := owner.snapshot().outstandingRequests; got != 1 {
		t.Fatalf("owner Request credits = %d, want 1", got)
	}

	response, err := wire.EncodeStreamResponse(sequence, wire.StatusOK, []byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	writeWireFrame(t, peer1, response)
	task := waitForTask(t, dispatcher1.takeCompletion)
	if got := owner.snapshot().outstandingRequests; got != 1 {
		t.Fatalf("Request credit released before Completion callback return: %d", got)
	}
	task.run()
	if got := owner.snapshot().outstandingRequests; got != 0 {
		t.Fatalf("Request credits after Completion return = %d, want 0", got)
	}
	if _, err := second.SendRequest([]byte("now-admitted"), func(Response, error) {}); err != nil {
		t.Fatalf("SendRequest after credit release: %v", err)
	}
	_ = readWireFrame(t, peer2)
	first.Close()
	second.Close()
	waitDone(t, first)
	waitDone(t, second)
}
