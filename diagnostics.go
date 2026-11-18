package neta

import (
	"reflect"
	"runtime/debug"
	"strconv"
	"time"
	"unicode/utf8"
)

const maxPanicValueTextBytes = 1024

// PanicRecord is an immutable, snapshot-friendly rendering of one recovered
// user callback panic. The original recovered value is not exported.
type PanicRecord struct {
	Callback       string
	ValueType      string
	ValueText      string
	ValueTruncated bool
	Stack          string
	Time           time.Time
	Sequence       uint64
}

// CallbackPanicDiagnostics describes callback panics recorded so far.
// First and OnClose are independent of the bounded Recent records.
type CallbackPanicDiagnostics struct {
	First   *PanicRecord
	Recent  []PanicRecord
	OnClose *PanicRecord
	Dropped uint64
}

// SessionDiagnostics is a coherent snapshot of one Session's diagnostics.
type SessionDiagnostics struct {
	CallbackPanics CallbackPanicDiagnostics
}

type panicRecentEntry struct {
	record PanicRecord
	raw    any
	bytes  uint64
}

type callbackPanicStore struct {
	first      *PanicRecord
	firstRaw   any
	onClose    *PanicRecord
	onCloseRaw any
	recent     []panicRecentEntry
	recentBytes uint64
	dropped    uint64
	sequence   uint64
}

// Diagnostics returns an independent snapshot. No Session state, network or
// worker lock is acquired, and healthy Sessions allocate no panic history.
func (s *Session) Diagnostics() SessionDiagnostics {
	if s == nil {
		return SessionDiagnostics{}
	}
	s.panicMu.Lock()
	defer s.panicMu.Unlock()
	var snapshot SessionDiagnostics
	history := &s.panicLog
	if history.first != nil {
		first := *history.first
		snapshot.CallbackPanics.First = &first
	}
	if history.onClose != nil {
		onClose := *history.onClose
		snapshot.CallbackPanics.OnClose = &onClose
	}
	if len(history.recent) > 0 {
		snapshot.CallbackPanics.Recent = make([]PanicRecord, len(history.recent))
		for i := range history.recent {
			snapshot.CallbackPanics.Recent[i] = history.recent[i].record
		}
	}
	snapshot.CallbackPanics.Dropped = history.dropped
	return snapshot
}

// safePanicText intentionally never calls the value's String or Error methods,
// nor fmt's Formatter methods. Non-primitive values receive an opaque marker.
func safePanicText(value any) (valueType, valueText string, truncated bool) {
	if value == nil {
		return "<nil>", "<nil>", false
	}
	t := reflect.TypeOf(value)
	valueType = t.String()
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		valueText = v.String()
	case reflect.Bool:
		valueText = strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		valueText = strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		valueText = strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		valueText = strconv.FormatFloat(v.Float(), 'g', -1, t.Bits())
	default:
		valueText = "<non-primitive panic value>"
	}
	if len(valueText) > maxPanicValueTextBytes {
		end := maxPanicValueTextBytes
		for end > 0 && !utf8.ValidString(valueText[:end]) {
			end--
		}
		valueText = valueText[:end]
		truncated = true
	}
	return valueType, valueText, truncated
}

func (s *Session) recordCallbackPanic(callback string, recovered any) {
	now := time.Now()
	stack := string(debug.Stack())
	valueType, valueText, truncated := safePanicText(recovered)
	record := PanicRecord{
		Callback: callback, ValueType: valueType, ValueText: valueText,
		ValueTruncated: truncated, Stack: stack, Time: now,
	}
	// This is a record-storage budget, not a total Session memory bound:
	// the First and OnClose records (and their raw values) are exempt.
	size := uint64(len(callback) + len(valueType) + len(valueText) + len(stack))
	s.panicMu.Lock()
	defer s.panicMu.Unlock()
	store := &s.panicLog
	store.sequence++
	record.Sequence = store.sequence
	if store.first == nil {
		first := record
		store.first = &first
		store.firstRaw = recovered
	}
	if callback == "OnClose" && store.onClose == nil {
		onClose := record
		store.onClose = &onClose
		store.onCloseRaw = recovered
	}
	if size > s.config.callbackPanicByteBudget {
		store.dropped++
		return
	}
	if store.recent == nil {
		store.recent = make([]panicRecentEntry, 0, s.config.callbackPanicRingCapacity)
	}
	for len(store.recent) >= int(s.config.callbackPanicRingCapacity) ||
		store.recentBytes+size > s.config.callbackPanicByteBudget {
		if len(store.recent) == 0 {
			break
		}
		store.recentBytes -= store.recent[0].bytes
		copy(store.recent, store.recent[1:])
		store.recent[len(store.recent)-1] = panicRecentEntry{}
		store.recent = store.recent[:len(store.recent)-1]
		store.dropped++
	}
	store.recent = append(store.recent, panicRecentEntry{record: record, raw: recovered, bytes: size})
	store.recentBytes += size
}

// invokeUserCallback is the panic boundary around user code, including late
// callbacks that may outlive Done. Transport teardown never waits for it.
func (s *Session) invokeUserCallback(name string, fn func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.recordCallbackPanic(name, recovered)
			if name != "OnClose" {
				s.beginClose(ErrCallbackPanic, true)
			}
		}
	}()
	fn()
}
