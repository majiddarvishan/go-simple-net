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
		{Type: BindBiDirectional, SystemID: "01234567890123456789"},
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
