package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"sync"
	"testing"
)

func TestHeaderGoldenVector(t *testing.T) {
	got, err := EncodeFrame(CommandStreamRequest, StatusOK, 0x01020304, []byte{0xaa, 0xbb})
	if err != nil {
		t.Fatal(err)
	}
	const want = "0000000c020001020304aabb"
	if hex.EncodeToString(got) != want {
		t.Fatalf("frame = %x, want %s", got, want)
	}
	decoded, err := DecodeFrame(got)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.Sequence != 0x01020304 || decoded.Header.Status != StatusOK || !bytes.Equal(decoded.Body, []byte{0xaa, 0xbb}) {
		t.Fatalf("decoded frame mismatch: %+v", decoded)
	}
}

func TestBindGoldenVectors(t *testing.T) {
	request, err := EncodeBindRequest(7, Bind{Type: BindBiDirectional, SystemID: "node"})
	if err != nil {
		t.Fatal(err)
	}
	const wantRequest = "00000010010000000007016e6f646500"
	if hex.EncodeToString(request) != wantRequest {
		t.Fatalf("bind request = %x", request)
	}
	response, err := EncodeBindResponse(7, StatusFailed, Bind{Type: BindUniDirectional, SystemID: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	const wantResponse = "0000000f81ff000000070073727600"
	if hex.EncodeToString(response) != wantResponse {
		t.Fatalf("bind response = %x", response)
	}
	frame, err := DecodeFrame(request)
	if err != nil {
		t.Fatal(err)
	}
	bind, err := DecodeBindRequest(frame)
	if err != nil {
		t.Fatal(err)
	}
	if bind.Type != BindBiDirectional || bind.SystemID != "node" {
		t.Fatalf("decoded bind = %+v", bind)
	}
}

func TestControlPDUAreHeaderOnlyAndEchoSequence(t *testing.T) {
	cases := []struct {
		name    string
		encode  func(uint32) ([]byte, error)
		command CommandID
	}{
		{"unbind request", EncodeUnbindRequest, CommandUnbindRequest},
		{"unbind response", EncodeUnbindResponse, CommandUnbindResponse},
		{"enquire request", EncodeEnquireLinkRequest, CommandEnquireLinkRequest},
		{"enquire response", EncodeEnquireLinkResponse, CommandEnquireLinkResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.encode(0x12345678)
			if err != nil {
				t.Fatal(err)
			}
			frame, err := DecodeFrame(got)
			if err != nil {
				t.Fatal(err)
			}
			if frame.Header.Command != tc.command || frame.Header.Sequence != 0x12345678 || len(frame.Body) != 0 {
				t.Fatalf("unexpected frame: %+v", frame)
			}
		})
	}
}

func TestFrameSizeBoundaries(t *testing.T) {
	max := bytes.Repeat([]byte{0x5a}, MaxBodySize)
	if _, err := EncodeStreamRequest(1, max); err != nil {
		t.Fatalf("maximum frame rejected: %v", err)
	}
	if _, err := EncodeStreamRequest(1, append(max, 0)); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("oversize frame error = %v", err)
	}
	header := make([]byte, HeaderSize)
	header[3] = 0x10
	header[4] = byte(CommandStreamRequest)
	if _, err := ParseHeader(header); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("oversize header error = %v", err)
	}
}

func TestDecodeFrameRejectsInvalidAndIncompleteFrames(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		target error
	}{
		{"short", []byte{1, 2}, ErrShortHeader},
		{"short length", []byte{0, 0, 0, 9, 2, 0, 0, 0, 0, 1}, ErrInvalidLength},
		{"unknown command", []byte{0, 0, 0, 10, 0x7f, 0, 0, 0, 0, 1}, ErrUnknownCommand},
		{"truncated body", []byte{0, 0, 0, 11, 2, 0, 0, 0, 0, 1}, ErrInvalidLength},
		{"extra bytes", []byte{0, 0, 0, 10, 2, 0, 0, 0, 0, 1, 0}, ErrInvalidLength},
		{"control body", []byte{0, 0, 0, 11, 3, 0, 0, 0, 0, 1, 0}, ErrInvalidControlBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeFrame(tc.data)
			if !errors.Is(err, tc.target) {
				t.Fatalf("error = %v, want %v", err, tc.target)
			}
		})
	}
}

func TestBindValidation(t *testing.T) {
	for _, bind := range []Bind{
		{Type: 2, SystemID: "x"},
		{Type: BindBiDirectional, SystemID: string(bytes.Repeat([]byte{'x'}, 20))},
		{Type: BindBiDirectional, SystemID: "a\x00b"},
	} {
		if _, err := EncodeBindRequest(1, bind); err == nil {
			t.Fatalf("invalid bind accepted: %+v", bind)
		}
	}
	if _, err := DecodeBindRequest(Frame{Header: Header{Command: CommandBindRequest}, Body: []byte{1, 'x'}}); !errors.Is(err, ErrMalformedBindBody) {
		t.Fatalf("malformed body error = %v", err)
	}
}

func TestSequenceGeneratorSkipsZeroOnWrap(t *testing.T) {
	var s SequenceGenerator
	s.value.value.Store(math.MaxUint32 - 1)
	if got := s.Next(); got != math.MaxUint32 {
		t.Fatalf("got %d", got)
	}
	if got := s.Next(); got != 1 {
		t.Fatalf("wrap yielded %d, want 1", got)
	}
}

func TestSequenceGeneratorConcurrentUniqueness(t *testing.T) {
	var s SequenceGenerator
	const n = 1000
	got := make(chan uint32, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got <- s.Next()
		}()
	}
	wg.Wait()
	close(got)
	seen := make(map[uint32]bool, n)
	for v := range got {
		if v == 0 || seen[v] {
			t.Fatalf("invalid/duplicate sequence %d", v)
		}
		seen[v] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d unique values, want %d", len(seen), n)
	}
}
