package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestBindV2OmitsNULAndPreservesLegacyV1(t *testing.T) {
	bind := Bind{Type: BindBiDirectional, SystemID: "node"}
	v1, err := EncodeBindRequest(7, bind)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := EncodeBindRequestV2(7, bind)
	if err != nil {
		t.Fatal(err)
	}
	if len(v1) != len(v2)+1 || binary.BigEndian.Uint32(v1[:4]) != binary.BigEndian.Uint32(v2[:4])+1 {
		t.Fatalf("V1/V2 total lengths must differ by one: %x / %x", v1, v2)
	}
	if v1[len(v1)-1] != 0 || v2[len(v2)-1] == 0 {
		t.Fatalf("V1 must terminate SystemID and V2 must not: v1=%x v2=%x", v1, v2)
	}
	if !bytes.Equal(v1[4:len(v1)-1], v2[4:]) {
		t.Fatalf("V2 differs from V1 beyond the NUL and length: %x / %x", v1, v2)
	}
	f1, err := DecodeFrame(v1)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := DecodeFrame(v2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBindRequest(f2); !errors.Is(err, ErrMalformedBindBody) {
		t.Fatalf("V1 must reject V2 Bind: %v", err)
	}
	if _, err := DecodeBindRequestV2(f1); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V2 must reject V1 Bind: %v", err)
	}
	if got, err := DecodeBindRequestV2(f2); err != nil || got != bind {
		t.Fatalf("V2 decode got %+v, %v", got, err)
	}
}

func TestBindResponseV2AndOmittedLegacyBody(t *testing.T) {
	bind := Bind{Type: BindBiDirectional, SystemID: ""}
	v1, err := EncodeBindResponse(7, StatusOK, bind)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := EncodeBindResponseV2(7, StatusOK, bind)
	if err != nil {
		t.Fatal(err)
	}
	if len(v1) != len(v2)+1 || len(v2) != HeaderSize+1 {
		t.Fatalf("unexpected V1/V2 BindResp sizes: %d/%d", len(v1), len(v2))
	}
	frame, err := DecodeFrame(v2)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeBindResponseV2(frame); err != nil || got != bind {
		t.Fatalf("V2 BindResp decode got %+v, %v", got, err)
	}
	omitted, err := EncodeFrame(CommandBindResponse, StatusOK, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	omittedFrame, err := DecodeFrame(omitted)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeBindResponseV2(omittedFrame); err != nil || got != bind {
		t.Fatalf("omitted BindResp decode got %+v, %v", got, err)
	}
}

func TestBindV2Validation(t *testing.T) {
	for _, bind := range []Bind{
		{Type: BindType(4), SystemID: "node"},
		{Type: BindBiDirectional, SystemID: "x\x00y"},
		{Type: BindBiDirectional, SystemID: "012345678901234567890"},
	} {
		if _, err := EncodeBindRequestV2(1, bind); err == nil {
			t.Fatalf("invalid V2 Bind accepted: %+v", bind)
		}
	}
	frame, err := DecodeFrame([]byte{0, 0, 0, 10, byte(CommandBindRequest), 0, 0, 0, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBindRequestV2(frame); !errors.Is(err, ErrMalformedBindBody) {
		t.Fatalf("empty V2 request body must be rejected, got %v", err)
	}
}

func TestBindSystemIDLengthLimitsRemainVersionSpecific(t *testing.T) {
	systemID19 := "1234567890123456789"
	systemID20 := "12345678901234567890"
	systemID21 := "123456789012345678901"

	// Legacy V1 still caps the ID at 19 bytes and requires the terminator.
	v1, err := EncodeBindRequest(101, Bind{Type: BindBiDirectional, SystemID: systemID19})
	if err != nil {
		t.Fatalf("V1 19-byte Bind: %v", err)
	}
	if len(v1) != HeaderSize+1+19+1 || v1[len(v1)-1] != 0 {
		t.Fatalf("V1 19-byte Bind missing terminator or wrong size: %x", v1)
	}
	if _, err := EncodeBindRequest(102, Bind{Type: BindBiDirectional, SystemID: systemID20}); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V1 accepted 20-byte SystemID: %v", err)
	}
	if _, err := EncodeBindResponse(103, StatusOK, Bind{Type: BindBiDirectional, SystemID: systemID20}); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V1 response accepted 20-byte SystemID: %v", err)
	}

	// V2 uses all 20 bytes, with no NUL, for both Bind directions.
	bind := Bind{Type: BindBiDirectional, SystemID: systemID20}
	v2Request, err := EncodeBindRequestV2(104, bind)
	if err != nil {
		t.Fatalf("V2 request at 20-byte limit: %v", err)
	}
	if len(v2Request) != HeaderSize+1+20 || v2Request[len(v2Request)-1] != '0' {
		t.Fatalf("V2 request size or last byte unexpected: %x", v2Request)
	}
	v2RequestFrame, err := DecodeFrame(v2Request)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeBindRequestV2(v2RequestFrame); err != nil || got != bind {
		t.Fatalf("V2 request roundtrip = %+v, %v", got, err)
	}
	v2Response, err := EncodeBindResponseV2(105, StatusOK, bind)
	if err != nil {
		t.Fatalf("V2 response at 20-byte limit: %v", err)
	}
	if len(v2Response) != HeaderSize+1+20 {
		t.Fatalf("V2 response length = %d, want %d", len(v2Response), HeaderSize+1+20)
	}
	v2ResponseFrame, err := DecodeFrame(v2Response)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeBindResponseV2(v2ResponseFrame); err != nil || got != bind {
		t.Fatalf("V2 response roundtrip = %+v, %v", got, err)
	}

	// Both V2 encoding and decoding reject the 21st byte.
	if _, err := EncodeBindRequestV2(106, Bind{Type: BindBiDirectional, SystemID: systemID21}); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V2 request accepted 21 bytes: %v", err)
	}
	if _, err := EncodeBindResponseV2(107, StatusOK, Bind{Type: BindBiDirectional, SystemID: systemID21}); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V2 response accepted 21 bytes: %v", err)
	}
	tooLongBody := append([]byte{byte(BindBiDirectional)}, systemID21...)
	tooLongRequest, err := EncodeFrame(CommandBindRequest, StatusOK, 108, tooLongBody)
	if err != nil {
		t.Fatal(err)
	}
	tooLongRequestFrame, err := DecodeFrame(tooLongRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBindRequestV2(tooLongRequestFrame); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V2 decoder accepted 21-byte request: %v", err)
	}
	tooLongResponse, err := EncodeFrame(CommandBindResponse, StatusOK, 109, tooLongBody)
	if err != nil {
		t.Fatal(err)
	}
	tooLongResponseFrame, err := DecodeFrame(tooLongResponse)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBindResponseV2(tooLongResponseFrame); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V2 decoder accepted 21-byte response: %v", err)
	}

	// Legacy decoder cannot accept a V1 packet with 20 bytes, even if it
	// supplies its required NUL terminator.
	invalidV1Body := append(append([]byte{byte(BindBiDirectional)}, systemID20...), 0)
	invalidV1, err := EncodeFrame(CommandBindRequest, StatusOK, 110, invalidV1Body)
	if err != nil {
		t.Fatal(err)
	}
	invalidV1Frame, err := DecodeFrame(invalidV1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBindRequest(invalidV1Frame); !errors.Is(err, ErrInvalidSystemID) {
		t.Fatalf("V1 decoder accepted 20-byte request: %v", err)
	}
}
