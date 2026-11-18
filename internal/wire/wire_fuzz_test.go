package wire

import (
	"bytes"
	"testing"
)

func FuzzParseHeader(f *testing.F) {
	valid, _ := EncodeFrame(CommandStreamRequest, StatusOK, 1, []byte("seed"))
	f.Add(valid[:HeaderSize])
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 9, 2, 0, 0, 0, 0, 1})
	f.Add([]byte{0, 0, 0, 10, 0x7f, 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		header, err := ParseHeader(data)
		if err == nil && (header.Length < HeaderSize || header.Length > MaxFrameSize) {
			t.Fatalf("ParseHeader accepted out-of-range length %d", header.Length)
		}
	})
}

func FuzzDecodeFrame(f *testing.F) {
	valid, _ := EncodeFrame(CommandStreamRequest, StatusOK, 1, []byte{0, 1, 0xff})
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 10, 0x7f, 0, 0, 0, 0, 1})
	f.Add([]byte{0, 0, 0, 11, byte(CommandUnbindRequest), 0, 0, 0, 0, 1, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err == nil {
			if uint64(frame.Header.Length) != uint64(len(data)) {
				t.Fatalf("accepted frame length %d for %d input bytes", frame.Header.Length, len(data))
			}
			if len(frame.Body) > MaxBodySize {
				t.Fatalf("accepted body of %d bytes", len(frame.Body))
			}
		}
	})
}

func FuzzDecodeBindRequest(f *testing.F) {
	valid, _ := EncodeBindRequest(7, Bind{Type: BindBiDirectional, SystemID: "fuzz-client"})
	f.Add(valid)
	f.Add([]byte{0, 0, 0, 10, byte(CommandBindRequest), 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			return
		}
		bind, err := DecodeBindRequest(frame)
		if err == nil {
			if bind.Type != BindUniDirectional && bind.Type != BindBiDirectional {
				t.Fatalf("accepted invalid Bind type %d", bind.Type)
			}
			if len(bind.SystemID) > MaxSystemIDLength {
				t.Fatalf("accepted SystemID of %d bytes", len(bind.SystemID))
			}
		}
	})
}

func FuzzDecodeBindResponse(f *testing.F) {
	valid, _ := EncodeBindResponse(7, StatusOK, Bind{Type: BindBiDirectional, SystemID: "fuzz-server"})
	f.Add(valid)
	f.Add([]byte{0, 0, 0, 10, byte(CommandBindResponse), 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			return
		}
		bind, err := DecodeBindResponse(frame)
		if err == nil {
			if bind.Type != BindUniDirectional && bind.Type != BindBiDirectional {
				t.Fatalf("accepted invalid Bind type %d", bind.Type)
			}
			if len(bind.SystemID) > MaxSystemIDLength {
				t.Fatalf("accepted SystemID of %d bytes", len(bind.SystemID))
			}
		}
	})
}

func FuzzDecodeStreamRequest(f *testing.F) {
	valid, _ := EncodeStreamRequest(11, []byte{0, 1, 2, 0xfe, 0xff})
	f.Add(valid)
	f.Add([]byte{0, 0, 0, 10, byte(CommandStreamResponse), 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			return
		}
		_, _ = DecodeStreamRequest(frame)
	})
}

// Mutate V2 Bind requests independently of the legacy NUL-terminated V1 path.
// A successful decode must never yield an embedded NUL or an overlong ID.
func FuzzDecodeBindRequestV2(f *testing.F) {
	valid, _ := EncodeBindRequestV2(7, Bind{Type: BindBiDirectional, SystemID: "abcdefghijklmnopqrst"})
	legacy, _ := EncodeBindRequest(7, Bind{Type: BindBiDirectional, SystemID: "v1-client"})
	f.Add(valid)
	f.Add(legacy)
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 10, byte(CommandBindRequest), 0, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			return
		}
		bind, err := DecodeBindRequestV2(frame)
		if err == nil {
			if frame.Header.Command != CommandBindRequest ||
				(bind.Type != BindUniDirectional && bind.Type != BindBiDirectional) ||
				len(bind.SystemID) > MaxSystemIDLengthV2 ||
				bytes.IndexByte([]byte(bind.SystemID), 0) >= 0 {
				t.Fatalf("V2 Bind decoder accepted invalid request: %+v", bind)
			}
		}
	})
}

// An empty V2 Bind Response body remains valid for legacy default-value
// compatibility, but a nonempty V2 Bind body must be NUL free.
func FuzzDecodeBindResponseV2(f *testing.F) {
	valid, _ := EncodeBindResponseV2(7, StatusOK, Bind{Type: BindBiDirectional, SystemID: "abcdefghijklmnopqrst"})
	empty, _ := EncodeFrame(CommandBindResponse, StatusOK, 7, nil)
	legacy, _ := EncodeBindResponse(7, StatusOK, Bind{Type: BindBiDirectional, SystemID: "v1-server"})
	f.Add(valid)
	f.Add(empty)
	f.Add(legacy)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxFrameSize+1 {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			return
		}
		bind, err := DecodeBindResponseV2(frame)
		if err == nil {
			if frame.Header.Command != CommandBindResponse ||
				(bind.Type != BindUniDirectional && bind.Type != BindBiDirectional) ||
				len(bind.SystemID) > MaxSystemIDLengthV2 ||
				bytes.IndexByte([]byte(bind.SystemID), 0) >= 0 {
				t.Fatalf("V2 Bind decoder accepted invalid response: %+v", bind)
			}
		}
	})
}
