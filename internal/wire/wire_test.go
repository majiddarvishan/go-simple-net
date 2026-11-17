package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
)

func loadPinexGoldenFrames(t *testing.T) map[string][]byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/pinex-1.5.0/golden.txt")
	if err != nil {
		t.Fatalf("read Pinex golden fixtures: %v", err)
	}
	frames := make(map[string][]byte)
	for lineNo, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, encoded, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("golden fixture line %d has no '=' separator", lineNo+1)
		}
		frame, err := hex.DecodeString(encoded)
		if err != nil {
			t.Fatalf("decode golden fixture %q: %v", name, err)
		}
		if _, exists := frames[name]; exists {
			t.Fatalf("duplicate golden fixture %q", name)
		}
		frames[name] = frame
	}
	return frames
}

func TestPinex150CPPGeneratedGoldenFrames(t *testing.T) {
	golden := loadPinexGoldenFrames(t)

	bindRequest, err := EncodeBindRequest(7, Bind{Type: BindBiDirectional, SystemID: "node"})
	if err != nil {
		t.Fatal(err)
	}
	bindResponse, err := EncodeBindResponse(7, StatusFailed, Bind{Type: BindUniDirectional, SystemID: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	streamRequest, err := EncodeStreamRequest(0x01020304, []byte{0xaa, 0xbb})
	if err != nil {
		t.Fatal(err)
	}
	streamResponse, err := EncodeStreamResponse(42, StatusFailed, []byte{0x00, 0x01, 0x02, 0xfe, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	unbindRequest, err := EncodeUnbindRequest(0x12345678)
	if err != nil {
		t.Fatal(err)
	}
	unbindResponse, err := EncodeUnbindResponse(0x12345678)
	if err != nil {
		t.Fatal(err)
	}
	enquireRequest, err := EncodeEnquireLinkRequest(0x12345678)
	if err != nil {
		t.Fatal(err)
	}
	enquireResponse, err := EncodeEnquireLinkResponse(0x12345678)
	if err != nil {
		t.Fatal(err)
	}

	actual := map[string][]byte{
		"bind_request":          bindRequest,
		"bind_response":         bindResponse,
		"stream_request":        streamRequest,
		"stream_response":       streamResponse,
		"unbind_request":        unbindRequest,
		"unbind_response":       unbindResponse,
		"enquire_link_request":  enquireRequest,
		"enquire_link_response": enquireResponse,
	}
	for name, got := range actual {
		want, ok := golden[name]
		if !ok {
			t.Errorf("missing C++ golden fixture %q", name)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s = %x, C++ Pinex 1.5.0 fixture = %x", name, got, want)
		}
	}

	// Decode the C++-generated frames as well as comparing Go output.
	bindFrame, err := DecodeFrame(golden["bind_request"])
	if err != nil {
		t.Fatal(err)
	}
	decodedBind, err := DecodeBindRequest(bindFrame)
	if err != nil {
		t.Fatal(err)
	}
	if decodedBind != (Bind{Type: BindBiDirectional, SystemID: "node"}) {
		t.Fatalf("C++ Bind Request decoded as %+v", decodedBind)
	}

	omittedBindFrame, err := DecodeFrame(golden["bind_response_empty"])
	if err != nil {
		t.Fatal(err)
	}
	omittedBind, err := DecodeBindResponse(omittedBindFrame)
	if err != nil {
		t.Fatal(err)
	}
	if omittedBind != (Bind{Type: BindBiDirectional}) {
		t.Fatalf("C++ omitted Bind Response decoded as %+v", omittedBind)
	}

	streamFrame, err := DecodeFrame(golden["stream_response"])
	if err != nil {
		t.Fatal(err)
	}
	decodedStream, err := DecodeStreamResponse(streamFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decodedStream, []byte{0x00, 0x01, 0x02, 0xfe, 0xff}) {
		t.Fatalf("C++ Stream Response decoded as %x", decodedStream)
	}
}

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

func TestBindResponseAllowsOmittedBodyLikePinex(t *testing.T) {
	frameBytes := []byte{
		0x00, 0x00, 0x00, 0x0a,
		byte(CommandBindResponse), byte(StatusFailed),
		0x00, 0x00, 0x00, 0x07,
	}
	frame, err := DecodeFrame(frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	bind, err := DecodeBindResponse(frame)
	if err != nil {
		t.Fatal(err)
	}
	if bind.Type != BindBiDirectional || bind.SystemID != "" {
		t.Fatalf("omitted Bind Response decoded as %+v, want Pinex defaults", bind)
	}
}

func TestStreamPDUUseRawBody(t *testing.T) {
	payload := []byte{0x00, 0x01, 0x02, 0xfe, 0xff}
	requestBytes, err := EncodeStreamRequest(42, payload)
	if err != nil {
		t.Fatal(err)
	}
	requestFrame, err := DecodeFrame(requestBytes)
	if err != nil {
		t.Fatal(err)
	}
	requestBody, err := DecodeStreamRequest(requestFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(requestBody, payload) {
		t.Fatalf("request body = %x, want %x", requestBody, payload)
	}
	responseBytes, err := EncodeStreamResponse(42, StatusFailed, payload)
	if err != nil {
		t.Fatal(err)
	}
	responseFrame, err := DecodeFrame(responseBytes)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, err := DecodeStreamResponse(responseFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(responseBody, payload) {
		t.Fatalf("response body = %x, want %x", responseBody, payload)
	}
	if _, err := DecodeStreamRequest(responseFrame); err == nil {
		t.Fatal("DecodeStreamRequest accepted a Stream Response")
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
	header[1] = 0x10
	header[3] = 0x01
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
