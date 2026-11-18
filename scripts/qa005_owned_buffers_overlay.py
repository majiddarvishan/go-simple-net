#!/usr/bin/env python3
"""Generate a test-only Go overlay that observes owned buffer storage.

F1/F2/F3/F4 backing storage is tracked by Session under its existing mutex.
This is an UNQUALIFIED diagnostic. Metadata/closures/Panic objects are excluded,
and time sampling can miss fast peaks. Never emit retained-attributed-B.
No repository source file is edited by this script.
V2 additionally tracks F1 during build/copy and rolls back rejected frames.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("qa005_charge_probe", ROOT / "qa005_memory_overlay.py")
charge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(charge)

ADAPTER_ID = "QA005-OWNED-BUFFER-OVERLAY-V3-UNQUALIFIED"
SOURCE_FILES = ("benchmark_test.go", "session.go", "session_internal.go", "session_transport.go", "resource_charges.go")
FIXTURES = ("qa005_qualification_benchmark_test.go", "benchmark_test.go")


def replace_once(content, needle, replacement, label):
    occurrences = content.count(needle)
    if occurrences != 1:
        raise ValueError(f"{label}: expected one anchor; found {occurrences}")
    return content.replace(needle, replacement, 1)


SESSION_HELPERS = """
// Test-overlay-only: counters represent live backing storage responsibility,
// not Go allocator buckets, policy reservation amounts or foreign object graphs.
func qa005AddOwnedLocked(s *Session, counter *uint64, amount uint64) {
    if amount == 0 { return }
    next, ok := checkedAddUint64(*counter, amount)
    total, totalOK := checkedAddUint64(s.qa005OwnedTotal, amount)
    if !ok || !totalOK { s.qa005OwnedFault = true; return }
    *counter = next
    s.qa005OwnedTotal = total
    if total > s.qa005OwnedPeak { s.qa005OwnedPeak = total }
}

func qa005SubtractOwnedLocked(s *Session, counter *uint64, amount uint64) {
    if amount == 0 { return }
    next, ok := checkedSubUint64(*counter, amount)
    total, totalOK := checkedSubUint64(s.qa005OwnedTotal, amount)
    if !ok || !totalOK { s.qa005OwnedFault = true; return }
    *counter = next
    s.qa005OwnedTotal = total
}
"""


def patch_internal(content):
    content = replace_once(
        content, "\tafterWrite         func()\n",
        "\tafterWrite         func()\n\tqa005Observed      bool // overlay-only, one charge per tracked allocation\n\tqa005Enqueued      bool // overlay-only, detect duplicate queue admission\n",
        "outbound frame identity")
    content = replace_once(
        content, "\tactiveWrite               *outboundFrame\n",
        "\tactiveWrite               *outboundFrame\n"
        "\tqa005OwnedOutgoingApp     uint64\n"
        "\tqa005OwnedIncomingApp     uint64\n"
        "\tqa005OwnedIncomingReply   uint64\n"
        "\tqa005OwnedControl         uint64\n"
        "\tqa005OwnedFault           bool\n"
        "\tqa005OwnedTotal           uint64\n"
        "\tqa005OwnedPeak            uint64\n\tqa005AfterAllocation      func() // deterministic overlay-only close interleaving\n",
        "session storage counters")
    content = replace_once(
        content, "\ts.queuedControlBytes = next\n\tif item.barrier {",
        "\ts.queuedControlBytes = next\n"
        "\tif item.qa005Observed { s.qa005OwnedFault = true } else {\n"
        "\t\tqa005AddOwnedLocked(s, &s.qa005OwnedControl, uint64(cap(item.bytes)))\n"
        "\t\titem.qa005Observed = true\n"
        "\t}\n\tif item.barrier {",
        "control admission")
    content = replace_once(
        content, "\ts.appQueue = append(s.appQueue, item)\n\ts.cond.Signal()",
        "\ts.appQueue = append(s.appQueue, item)\n"
        "\tif item.qa005Enqueued { s.qa005OwnedFault = true } else if !item.qa005Observed {\n"
        "\t\tqa005AddOwnedLocked(s, &s.qa005OwnedOutgoingApp, uint64(cap(item.bytes)))\n"
        "\t\titem.qa005Observed = true\n"
        "\t}\n"
        "\titem.qa005Enqueued = true\n\ts.cond.Signal()",
        "application queue")
    content = replace_once(
        content,
        "func (s *Session) releaseFrameChargeLocked(item *outboundFrame) {\n"
        "\tif item == nil {\n\t\treturn\n\t}\n\tif item.control {",
        "func (s *Session) releaseFrameChargeLocked(item *outboundFrame) {\n"
        "\tif item == nil {\n\t\treturn\n\t}\n"
        "\tif item.qa005Observed {\n"
        "\t\tif item.control {\n"
        "\t\t\tqa005SubtractOwnedLocked(s, &s.qa005OwnedControl, uint64(cap(item.bytes)))\n"
        "\t\t} else {\n"
        "\t\t\tqa005SubtractOwnedLocked(s, &s.qa005OwnedOutgoingApp, uint64(cap(item.bytes)))\n"
        "\t\t}\n"
        "\t\titem.qa005Observed = false\n"
        "\t}\n\tif item.control {",
        "writer/close release")
    return content.rstrip() + "\n" + SESSION_HELPERS


def patch_charges(content):
    content = replace_once(
        content, "\tbytes            uint64\n\tqueueReleased",
        "\tbytes            uint64\n\tqa005BodyBytes    uint64\n\tqueueReleased",
        "inbound application charge")
    content = replace_once(
        content, "\tbytes    uint64\n\treleased bool",
        "\tbytes    uint64\n\tqa005BodyBytes uint64\n\treleased bool",
        "inbound completion charge")
    content = replace_once(
        content, "\tc.retainedReleased = true\n\tif next, ok := checkedSubUint64(",
        "\tc.retainedReleased = true\n"
        "\tqa005SubtractOwnedLocked(s, &s.qa005OwnedIncomingApp, c.qa005BodyBytes)\n"
        "\tc.qa005BodyBytes = 0\n"
        "\tif next, ok := checkedSubUint64(",
        "application retained return")
    content = replace_once(
        content, "\tc.released = true\n\tif next, ok := checkedSubUint64(",
        "\tc.released = true\n"
        "\tqa005SubtractOwnedLocked(s, &s.qa005OwnedIncomingReply, c.qa005BodyBytes)\n"
        "\tc.qa005BodyBytes = 0\n"
        "\tif next, ok := checkedSubUint64(",
        "completion return")
    return content



def patch_session(content):
    """Register allocated F1 while holding Session.mu before copying the frame.

    Ownership starts at make, not after FIFO admission. An unqueued frame is
    released by its Send* call's defer; successful admission transfers the same
    observation to the outboundFrame. All changes exist only in the Go overlay.
    """
    builder = ("func buildStreamFrame(command wire.CommandID, status wire.CommandStatus, "
               "sequence uint32, payload []byte) ([]byte, error) {")
    content = replace_once(
        content, builder,
        builder.replace("payload []byte)", "payload []byte, qa005Owner ...*Session)"),
        "frame builder optional observer")
    content = replace_once(
        content, "\tframe := make([]byte, length)\n",
        "\tvar frame []byte\n"
        "\tif len(qa005Owner) == 0 || qa005Owner[0] == nil {\n"
        "\t\tframe = make([]byte, length)\n"
        "\t} else {\n"
        "\t\towner := qa005Owner[0]\n"
        "\t\towner.mu.Lock()\n"
        "\t\tframe = make([]byte, length)\n"
        "\t\tqa005AddOwnedLocked(owner, &owner.qa005OwnedOutgoingApp, uint64(cap(frame)))\n"
        "\t\tafter := owner.qa005AfterAllocation\n"
        "\t\towner.mu.Unlock()\n"
        "\t\tif after != nil { after() }\n"
        "\t}\n",
        "registered frame allocation")
    starts = (
        "func (s *Session) SendRequest(",
        "func (s *Session) SendNoWait(",
        "func (s *Session) SendResponse(",
    )
    end = "func (s *Session) applicationSendErrorLocked("
    for index, marker in enumerate(starts):
        begin = content.find(marker)
        after = content.find(starts[index+1] if index+1 < len(starts) else end, begin+len(marker))
        if begin < 0 or after < 0:
            raise ValueError("missing send function boundary: " + marker)
        segment = content[begin:after]
        call = "frame, err := buildStreamFrame("
        if segment.count(call) != 1:
            raise ValueError("send builder call changed: " + marker)
        line_end = segment.index("\n", segment.index(call))
        line = segment[segment.index(call):line_end]
        if not line.endswith("payload)"):
            raise ValueError("unexpected send builder call: " + marker)
        segment = segment.replace(line, line[:-1] + ", s)", 1)
        segment = replace_once(
            segment, "\n\n\ts.mu.Lock()\n\tif err != nil {",
            "\n\tqa005Transferred := false\n"
            "\tif err == nil {\n"
            "\t\tdefer func() {\n"
            "\t\t\tif !qa005Transferred {\n"
            "\t\t\t\ts.mu.Lock()\n"
            "\t\t\t\tqa005SubtractOwnedLocked(s, &s.qa005OwnedOutgoingApp, uint64(cap(frame)))\n"
            "\t\t\t\ts.mu.Unlock()\n"
            "\t\t\t}\n"
            "\t\t}()\n"
            "\t}\n\n\ts.mu.Lock()\n\tif err != nil {",
            "unqueued frame rollback " + marker)
        segment = replace_once(
            segment, "\titem := &outboundFrame{bytes: frame,",
            "\titem := &outboundFrame{bytes: frame, qa005Observed: true,",
            "enqueue ownership transfer " + marker)
        signal = "\ts.signalControl()"
        if segment.count(signal) != 1:
            raise ValueError("unexpected send success boundary: " + marker)
        before_signal = segment.index(signal)
        last_finish = segment.rfind("\ts.finishAdmissionLocked()\n", 0, before_signal)
        if last_finish < 0:
            raise ValueError("missing final admission completion: " + marker)
        segment = (segment[:last_finish] + "\tqa005Transferred = true\n" +
                   segment[last_finish:])
        content = content[:begin] + segment + content[after:]
    return content


def patch_transport(content):
    return replace_once(
        content,
        "\t\tbody = make([]byte, bodyLength)\n\t\tif _, err := io.ReadFull(s.conn, body);",
        "\t\tbody = make([]byte, bodyLength)\n"
        "\t\tif applicationCharge != nil || completionCharge != nil {\n"
        "\t\t\ts.mu.Lock()\n"
        "\t\t\tif applicationCharge != nil {\n"
        "\t\t\t\tapplicationCharge.qa005BodyBytes = uint64(cap(body))\n"
        "\t\t\t\tqa005AddOwnedLocked(s, &s.qa005OwnedIncomingApp, uint64(cap(body)))\n"
        "\t\t\t} else {\n"
        "\t\t\t\tcompletionCharge.qa005BodyBytes = uint64(cap(body))\n"
        "\t\t\t\tqa005AddOwnedLocked(s, &s.qa005OwnedIncomingReply, uint64(cap(body)))\n"
        "\t\t\t}\n"
        "\t\t\ts.mu.Unlock()\n"
        "\t\t}\n"
        "\t\tif _, err := io.ReadFull(s.conn, body);",
        "inbound body allocation")


BENCHMARK_HELPERS = """
// Overlay-only snapshot: briefly lock Server list, then each Session separately.
// The resulting global sample is NOT an atomic heap snapshot.
// Overlay-only ownership set: Server lock is released before Session locks.
func qa005OwnedSessions(env *benchmarkEnv) []*Session {
    if env.qa005OwnedRegistry != nil { return env.qa005OwnedRegistry }
    env.server.mu.Lock()
    sessions := make([]*Session, 0, len(env.server.sessions)+len(env.sessions))
    for session := range env.server.sessions { sessions = append(sessions, session) }
    env.server.mu.Unlock()
    return append(sessions, env.sessions...)
}

// Reset each Session HWM at the start of the measured traffic window.
func qa005ResetOwnedHighwaters(env *benchmarkEnv) {
    sessions := qa005OwnedSessions(env)
    env.qa005OwnedRegistry = sessions
    env.qa005OwnedIndex = make(map[*Session]struct{}, len(sessions))
    for _, session := range sessions {
        env.qa005OwnedIndex[session] = struct{}{}
    }
    for _, session := range sessions {
        session.mu.Lock()
        session.qa005OwnedPeak = session.qa005OwnedTotal
        session.mu.Unlock()
    }
}

// A Session removed from the Server's current map may still own callback bodies.
func qa005OwnedRegistryComplete(env *benchmarkEnv) bool {
    if env.qa005OwnedRegistry == nil { return false }
    env.server.mu.Lock()
    defer env.server.mu.Unlock()
    for session := range env.server.sessions {
        if _, known := env.qa005OwnedIndex[session]; !known { return false }
    }
    return true
}

// Sum of independently reached per-Session HWM values. This is an UPPER BOUND
// over tracked sessions and NOT a simultaneous global peak or retained-attributed-B.
func qa005OwnedPeakUpper(env *benchmarkEnv) (uint64, bool) {
    if !qa005OwnedRegistryComplete(env) { return 0, false }
    var upper uint64
    for _, session := range qa005OwnedSessions(env) {
        session.mu.Lock()
        peak := session.qa005OwnedPeak
        fault := session.qa005OwnedFault || session.resourceAccountingFailed
        session.mu.Unlock()
        if fault { return 0, false }
        next, ok := checkedAddUint64(upper, peak)
        if !ok { return 0, false }
        upper = next
    }
    return upper, true
}

func qa005SampleOwnedBuffers(env *benchmarkEnv) (uint64, bool) {
    if !qa005OwnedRegistryComplete(env) { return 0, false }
    sessions := qa005OwnedSessions(env)
    var total uint64
    for _, session := range sessions {
        session.mu.Lock()
        amounts := [4]uint64{
            session.qa005OwnedOutgoingApp, session.qa005OwnedIncomingApp,
            session.qa005OwnedIncomingReply, session.qa005OwnedControl,
        }
        valid := !session.qa005OwnedFault && !session.resourceAccountingFailed
        session.mu.Unlock()
        if !valid { return 0, false }
        for _, amount := range amounts {
            next, ok := checkedAddUint64(total, amount)
            if !ok { return 0, false }
            total = next
        }
    }
    return total, true
}


// The end of one queue is not the end of inbound Application responsibility:
// its backing Body survives callback start and is freed at callback return.
// A matched inbound Response similarly survives until Completion release.
func TestQA005OwnedOverlayApplicationCompletionLifetimes(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    s.mu.Lock()
    if err := s.reserveOutgoingApplicationLocked(16); err != nil {
        s.mu.Unlock(); t.Fatal(err)
    }
    item := &outboundFrame{bytes: make([]byte, 16, 32), applicationCharged: true}
    if err := s.enqueueApplicationLocked(item); err != nil {
        s.mu.Unlock(); t.Fatal(err)
    }
    if s.qa005OwnedOutgoingApp != 32 {
        s.mu.Unlock(); t.Fatalf("outgoing owned = %d, want backing cap 32", s.qa005OwnedOutgoingApp)
    }
    // A writer claim only moves the existing frame pointer; no second charge.
    s.activeWrite = item
    if s.qa005OwnedOutgoingApp != 32 { s.mu.Unlock(); t.Fatal("double counted writer claim") }
    s.releaseFrameChargeLocked(item)
    s.activeWrite = nil
    if s.qa005OwnedOutgoingApp != 0 || item.qa005Observed || s.qa005OwnedTotal != 0 || s.qa005OwnedPeak < 32 || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("outgoing frame did not release exactly once")
    }

    app, err := s.reserveIncomingApplicationLocked(22)
    if err != nil { s.mu.Unlock(); t.Fatal(err) }
    app.qa005BodyBytes = 12
    qa005AddOwnedLocked(s, &s.qa005OwnedIncomingApp, 12)
    s.mu.Unlock()
    app.releaseQueue()
    s.mu.Lock()
    if s.qa005OwnedIncomingApp != 12 || app.retainedReleased {
        s.mu.Unlock(); t.Fatal("Application Body was lost at callback start")
    }
    s.mu.Unlock()
    app.releaseRetained()
    app.releaseRetained() // idempotent on callback completion
    s.mu.Lock()
    if s.qa005OwnedIncomingApp != 0 || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("Application Body was not released once")
    }

    completion := s.reserveCompletionLocked(22)
    if completion == nil { s.mu.Unlock(); t.Fatal("Completion reservation rejected") }
    completion.qa005BodyBytes = 12
    qa005AddOwnedLocked(s, &s.qa005OwnedIncomingReply, 12)
    if s.qa005OwnedIncomingReply != 12 { s.mu.Unlock(); t.Fatal("Completion Body missing") }
    s.mu.Unlock()
    completion.release()
    completion.release() // idempotent on terminal return
    s.mu.Lock()
    if s.qa005OwnedIncomingReply != 0 || s.qa005OwnedFault || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("Completion Body ownership or accounting invalid")
    }
    s.mu.Unlock()
}

// Overlay-only lifecycle sanity: an accepted control allocation is counted once,
// remains counted until release, and is not counted again at writer transfer.
func TestQA005OwnedOverlayControlLifecycle(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    s.mu.Lock()
    frame := &outboundFrame{control: true, bytes: make([]byte, 12, 32)}
    if err := s.enqueueControlLocked(frame); err != nil { s.mu.Unlock(); t.Fatal(err) }
    if s.qa005OwnedControl != 32 { s.mu.Unlock(); t.Fatalf("control bytes = %d, want 32", s.qa005OwnedControl) }
    if s.qa005OwnedFault { s.mu.Unlock(); t.Fatal("unexpected accounting fault") }
    s.releaseFrameChargeLocked(frame)
    if s.qa005OwnedControl != 0 || frame.qa005Observed { s.mu.Unlock(); t.Fatal("control storage must be released") }
    s.mu.Unlock()
}

// Regression: an F1 allocation is counted before Queue admission, and its
// Queue -> Writer transfer must not count it again.
func TestQA005OwnedOverlayPreQueueTransferAndRollback(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    payload := []byte("owned-frame")
    frame, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload, s)
    if err != nil { t.Fatal(err) }
    expected := uint64(cap(frame))
    s.mu.Lock()
    if s.qa005OwnedOutgoingApp != expected || s.qa005OwnedTotal != expected {
        s.mu.Unlock(); t.Fatal("F1 allocation not tracked before admission")
    }
    if err := s.reserveOutgoingApplicationLocked(uint64(len(frame))); err != nil {
        s.mu.Unlock(); t.Fatal(err)
    }
    item := &outboundFrame{bytes: frame, applicationCharged: true, qa005Observed: true}
    if err := s.enqueueApplicationLocked(item); err != nil {
        s.mu.Unlock(); t.Fatal(err)
    }
    if s.qa005OwnedOutgoingApp != expected || !item.qa005Enqueued || s.qa005OwnedFault {
        s.mu.Unlock(); t.Fatal("Queue must transfer, never duplicate, pre-owned F1")
    }
    s.activeWrite = item
    if s.qa005OwnedOutgoingApp != expected { s.mu.Unlock(); t.Fatal("Writer transfer double counted") }
    s.releaseFrameChargeLocked(item)
    s.activeWrite = nil
    s.releaseFrameChargeLocked(item)
    if s.qa005OwnedOutgoingApp != 0 || s.qa005OwnedTotal != 0 || s.qa005OwnedFault || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("F1 writer return/double release invalid")
    }
    s.mu.Unlock()
    orphan, err := buildStreamFrame(wire.CommandStreamRequest, wire.StatusOK, 0, payload, s)
    if err != nil { t.Fatal(err) }
    s.mu.Lock()
    if s.qa005OwnedOutgoingApp != uint64(cap(orphan)) {
        s.mu.Unlock(); t.Fatal("unqueued built frame missing")
    }
    qa005SubtractOwnedLocked(s, &s.qa005OwnedOutgoingApp, uint64(cap(orphan)))
    if s.qa005OwnedTotal != 0 || s.qa005OwnedFault {
        s.mu.Unlock(); t.Fatal("unqueued frame rollback failed")
    }
    s.mu.Unlock()
}

// Forced Close racing a real SendNoWait after make must keep the built F1
// allocation counted until the Send method returns and its defer runs.
func TestQA005OwnedOverlayForcedCloseDuringBuild(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    allocated := make(chan struct{})
    resume := make(chan struct{})
    s.mu.Lock()
    s.state = SessionStateBound
    s.qa005AfterAllocation = func() { close(allocated); <-resume }
    s.mu.Unlock()
    results := make(chan error, 1)
    go func() {
        _, err := s.SendNoWait([]byte("close-race"))
        results <- err
    }()
    select {
    case <-allocated:
    case <-time.After(2*time.Second):
        t.Fatal("SendNoWait failed to allocate during test")
    }
    s.mu.Lock()
    if s.qa005OwnedOutgoingApp == 0 || s.admissionCount != 1 || s.qa005OwnedFault {
        s.mu.Unlock(); t.Fatal("built frame not owned during in-flight admission")
    }
    s.mu.Unlock()
    s.beginClose(ErrClosed, true)
    select {
    case <-s.Done(): t.Fatal("Done closed before in-flight Send release")
    default:
    }
    close(resume)
    select {
    case err := <-results:
        if !errors.Is(err, ErrClosed) { t.Fatalf("SendNoWait after Close = %v, want ErrClosed", err) }
    case <-time.After(2*time.Second):
        t.Fatal("in-flight SendNoWait did not return")
    }
    select {
    case <-s.Done():
    case <-time.After(2*time.Second): t.Fatal("Done did not follow admission cleanup")
    }
    s.mu.Lock()
    if s.qa005OwnedOutgoingApp != 0 || s.qa005OwnedTotal != 0 || s.qa005OwnedFault || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("Close-raced F1 allocation not released exactly once")
    }
    s.mu.Unlock()
}

// The Queue charge ends at callback start; both F2 and F3 must survive Done
// while a user callback remains active, then release when that callback returns.
func TestQA005OwnedOverlayCallbacksAfterDone(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    s.mu.Lock()
    app, err := s.reserveIncomingApplicationLocked(32)
    if err != nil { s.mu.Unlock(); t.Fatal(err) }
    completion := s.reserveCompletionLocked(32)
    if completion == nil { s.mu.Unlock(); t.Fatal("completion reservation rejected") }
    app.qa005BodyBytes = 23
    completion.qa005BodyBytes = 19
    qa005AddOwnedLocked(s, &s.qa005OwnedIncomingApp, app.qa005BodyBytes)
    qa005AddOwnedLocked(s, &s.qa005OwnedIncomingReply, completion.qa005BodyBytes)
    s.mu.Unlock()
    app.releaseQueue()
    entered := make(chan struct{},2)
    resume := make(chan struct{})
    var active sync.WaitGroup
    active.Add(2)
    go func() {
        defer active.Done()
        s.invokeUserCallback("OnRequest", func() {
            defer app.releaseRetained()
            entered <- struct{}{}
            <-resume
        })
    }()
    go func() {
        defer active.Done()
        s.invokeUserCallback("Completion", func() {
            defer completion.release()
            entered <- struct{}{}
            <-resume
        })
    }()
    for i:=0; i<2; i++ {
        select {
        case <-entered:
        case <-time.After(2*time.Second): t.Fatal("callback did not start")
        }
    }
    s.beginClose(ErrClosed,true)
    select {
    case <-s.Done():
    case <-time.After(2*time.Second): t.Fatal("Close did not complete internal teardown")
    }
    s.mu.Lock()
    if s.qa005OwnedIncomingApp != 23 || s.qa005OwnedIncomingReply != 19 || s.qa005OwnedTotal != 42 {
        s.mu.Unlock(); t.Fatal("Done prematurely released active callback payloads")
    }
    s.mu.Unlock()
    close(resume)
    active.Wait()
    s.mu.Lock()
    if s.qa005OwnedTotal != 0 || s.qa005OwnedFault || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("active callback retention did not finish on return")
    }
    s.mu.Unlock()
}

// Panic recovery must run callback defers and release owned bodies while
// raw arbitrary recovered objects remain excluded from byte attribution.
func TestQA005OwnedOverlayPanicCleanup(t *testing.T) {
    s := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    s.mu.Lock()
    app, err := s.reserveIncomingApplicationLocked(30)
    if err != nil { s.mu.Unlock(); t.Fatal(err) }
    app.qa005BodyBytes = 20
    qa005AddOwnedLocked(s, &s.qa005OwnedIncomingApp, 20)
    s.mu.Unlock()
    app.releaseQueue()
    s.invokeUserCallback("OnRequest", func() {
        defer app.releaseRetained()
        panic("qa005-test-only")
    })
    s.mu.Lock()
    if s.qa005OwnedIncomingApp != 0 || s.qa005OwnedTotal != 0 || s.qa005OwnedFault || s.resourceAccountingFailed {
        s.mu.Unlock(); t.Fatal("callback Panic leaked retained body accounting")
    }
    s.mu.Unlock()
    select {
    case <-s.Done():
    case <-time.After(2*time.Second): t.Fatal("Panic did not terminate the Session")
    }
}


// A Server Session disappears from the live map at Close but its active
// callbacks can still own F2/F3. Frozen registry observes it; a newly admitted
// unregistered Session makes the diagnostic invalid, not silently zero.
func TestQA005OwnedOverlayFrozenRegistry(t *testing.T) {
    serverSession := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    clientSession := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    server := &Server{sessions: make(map[*Session]struct{})}
    server.sessions[serverSession] = struct{}{}
    env := &benchmarkEnv{server: server, sessions: []*Session{clientSession}}
    qa005ResetOwnedHighwaters(env)
    if len(env.qa005OwnedRegistry) != 2 || !qa005OwnedRegistryComplete(env) {
        t.Fatal("initial owner Session registry incomplete")
    }
    serverSession.mu.Lock()
    qa005AddOwnedLocked(serverSession, &serverSession.qa005OwnedIncomingApp, 17)
    serverSession.mu.Unlock()
    server.mu.Lock()
    delete(server.sessions, serverSession)
    server.mu.Unlock()
    value, valid := qa005SampleOwnedBuffers(env)
    if !valid || value != 17 { t.Fatalf("removed Session lost: value=%d valid=%v",value,valid) }
    fresh := newSession(nil, defaultSessionConfig(), sessionIdentity{}, nil, sessionHandlers{})
    server.mu.Lock()
    server.sessions[fresh] = struct{}{}
    server.mu.Unlock()
    if _, valid := qa005SampleOwnedBuffers(env); valid {
        t.Fatal("unregistered Server Session must invalidate memory observation")
    }
    serverSession.mu.Lock()
    qa005SubtractOwnedLocked(serverSession, &serverSession.qa005OwnedIncomingApp, 17)
    if serverSession.qa005OwnedTotal != 0 || serverSession.qa005OwnedFault {
        serverSession.mu.Unlock(); t.Fatal("frozen Session failed final release")
    }
    serverSession.mu.Unlock()
}

"""


def patch_benchmark(content):
    patched = charge.patch_benchmark(content)
    patched = replace_once(
        patched, "type benchmarkEnv struct {\n",
        "type benchmarkEnv struct {\n"
        "\tqa005OwnedRegistry []*Session\n"
        "\tqa005OwnedIndex map[*Session]struct{}\n",
        "stable owned Session registry")
    patched = replace_once(
        patched, '    p := &qa005ChargeProbe{stop: make(chan struct{}), done: make(chan struct{})}',
        '    qa005ResetOwnedHighwaters(env)\n'
        '    p := &qa005ChargeProbe{stop: make(chan struct{}), done: make(chan struct{})}',
        "reset Session HWMs at traffic start")
    patched = replace_once(
        patched, "    samples uint64\n    invalid bool\n",
        "    samples uint64\n    invalid bool\n    ownedPeak uint64\n    ownedSamples uint64\n    ownedUpper uint64\n",
        "sampler state")
    patched = replace_once(
        patched, "        p.samples++\n",
        "        p.samples++\n"
        "        owned, valid := qa005SampleOwnedBuffers(env)\n"
        "        if !valid { p.invalid = true; return }\n"
        "        if owned > p.ownedPeak { p.ownedPeak = owned }\n"
        "        p.ownedSamples++\n",
        "sample observation")
    patched = replace_once(
        patched, "            case <-p.stop: observe(); return",
        "            case <-p.stop: observe(); upper, valid := qa005OwnedPeakUpper(env); if !valid { p.invalid = true }; p.ownedUpper = upper; return",
        "sample close peak upper")
    patched = replace_once(
        patched,
        '    b.ReportMetric(float64(p.samples), "probe-owner-charge-samples")\n',
        '    b.ReportMetric(float64(p.samples), "probe-owner-charge-samples")\n'
        '    b.ReportMetric(float64(p.ownedPeak), "probe-owned-buffer-peak-B")\n'
        '    b.ReportMetric(float64(p.ownedSamples), "probe-owned-buffer-samples")\n'
        '    b.ReportMetric(float64(p.ownedUpper), "probe-owned-session-hwm-upper-B")\n',
        "owned reporting")
    return patched.rstrip() + "\n" + BENCHMARK_HELPERS


PATCHES = {
    "benchmark_test.go": patch_benchmark,
    "session_internal.go": patch_internal,
    "session.go": patch_session,
    "session_transport.go": patch_transport,
    "resource_charges.go": patch_charges,
}


def prepare(source, output):
    source = source.resolve(strict=True)
    output = output.resolve()
    if source == output or source in output.parents:
        raise ValueError("overlay output must be outside source checkout")
    original = {name: (source / name).read_bytes() for name in
                (*FIXTURES, "session.go", "session_internal.go", "session_transport.go", "resource_charges.go")}
    edited = {name: PATCHES[name](original[name].decode("utf-8")).encode("utf-8")
              for name in SOURCE_FILES}
    output.mkdir(parents=True, exist_ok=False)
    paths = {}
    for name, contents in edited.items():
        target = output / name
        target.write_bytes(contents)
        paths[str(source / name)] = str(target)
    (output / "overlay.json").write_text(json.dumps({"Replace": paths}, sort_keys=True) + "\n")
    source_fixture = charge.digest([original[name] for name in FIXTURES])
    effective_fixture = charge.digest([edited.get(name, original[name]) for name in FIXTURES])
    original_sha = {name: hashlib.sha256(original[name]).hexdigest() for name in original}
    effective_sha = {name: hashlib.sha256(edited.get(name, original[name])).hexdigest() for name in original}
    metadata = {
        "adapter_id": ADAPTER_ID,
        "source_fixture_sha256": source_fixture,
        "effective_fixture_sha256": effective_fixture,
        "source_file_sha256": original_sha,
        "effective_file_sha256": effective_sha,
        "measurement_qualifies_retained_attributed_memory": False,
        "covered_categories": ["F1-outbound-app", "F2-inbound-request-body",
                               "F3-inbound-response-body", "F4-outbound-control"],
        "excluded_categories": ["metadata", "callbacks", "panic-raw-graphs",
                                "goroutine-stacks", "runtime-allocator-overhead",
                                "application-references", "unclassified-frame-bodies"],
        "primary_metric": "probe-owned-buffer-peak-B",
        "sampler_interval_ms": 5,
        "source_tree_mutated": False,
        "tracks_pre_enqueue_allocated_F1": True,
        "qualification": "DIAGNOSTIC_ONLY",
    }
    (output / "manifest.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
    return metadata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    a = parser.parse_args()
    print(json.dumps(prepare(a.source, a.output), sort_keys=True))


if __name__ == "__main__":
    main()
