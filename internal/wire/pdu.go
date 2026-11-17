package wire

import (
	"bytes"
	"errors"
	"fmt"
)

const MaxSystemIDLength = 19

var (
	ErrInvalidBindType   = errors.New("wire: invalid bind type")
	ErrInvalidSystemID   = errors.New("wire: invalid system ID")
	ErrMalformedBindBody = errors.New("wire: malformed bind body")
)

type BindType uint8

const (
	BindUniDirectional BindType = 0
	BindBiDirectional  BindType = 1
)

type Bind struct {
	Type     BindType
	SystemID string
}

func EncodeBindRequest(sequence uint32, bind Bind) ([]byte, error) {
	body, err := encodeBindBody(bind)
	if err != nil {
		return nil, err
	}
	return EncodeFrame(CommandBindRequest, StatusOK, sequence, body)
}

func EncodeBindResponse(sequence uint32, status CommandStatus, bind Bind) ([]byte, error) {
	body, err := encodeBindBody(bind)
	if err != nil {
		return nil, err
	}
	return EncodeFrame(CommandBindResponse, status, sequence, body)
}

func DecodeBindRequest(frame Frame) (Bind, error) {
	if frame.Header.Command != CommandBindRequest {
		return Bind{}, fmt.Errorf("%w: got command 0x%02x", ErrMalformedBindBody, uint8(frame.Header.Command))
	}
	return decodeBindBody(frame.Body)
}

func DecodeBindResponse(frame Frame) (Bind, error) {
	if frame.Header.Command != CommandBindResponse {
		return Bind{}, fmt.Errorf("%w: got command 0x%02x", ErrMalformedBindBody, uint8(frame.Header.Command))
	}
	// Pinex 1.5.0 marks Bind Response as omittable: an empty Body
	// deserializes to the PDU's default values (bi_direction, empty ID).
	if len(frame.Body) == 0 {
		return Bind{Type: BindBiDirectional}, nil
	}
	return decodeBindBody(frame.Body)
}

func encodeBindBody(bind Bind) ([]byte, error) {
	if bind.Type != BindUniDirectional && bind.Type != BindBiDirectional {
		return nil, fmt.Errorf("%w: %d", ErrInvalidBindType, bind.Type)
	}
	if len(bind.SystemID) > MaxSystemIDLength || bytes.IndexByte([]byte(bind.SystemID), 0) >= 0 {
		return nil, fmt.Errorf("%w: length must be 0..%d bytes and contain no NUL", ErrInvalidSystemID, MaxSystemIDLength)
	}
	body := make([]byte, 1, 1+len(bind.SystemID)+1)
	body[0] = byte(bind.Type)
	body = append(body, bind.SystemID...)
	body = append(body, 0)
	return body, nil
}

func decodeBindBody(body []byte) (Bind, error) {
	if len(body) < 2 {
		return Bind{}, ErrMalformedBindBody
	}
	bindType := BindType(body[0])
	if bindType != BindUniDirectional && bindType != BindBiDirectional {
		return Bind{}, fmt.Errorf("%w: %d", ErrInvalidBindType, bindType)
	}
	if body[len(body)-1] != 0 {
		return Bind{}, ErrMalformedBindBody
	}
	systemIDBytes := body[1 : len(body)-1]
	if len(systemIDBytes) > MaxSystemIDLength || bytes.IndexByte(systemIDBytes, 0) >= 0 {
		return Bind{}, ErrInvalidSystemID
	}
	return Bind{Type: bindType, SystemID: string(systemIDBytes)}, nil
}

func EncodeStreamRequest(sequence uint32, body []byte) ([]byte, error) {
	return EncodeFrame(CommandStreamRequest, StatusOK, sequence, body)
}

func EncodeStreamResponse(sequence uint32, status CommandStatus, body []byte) ([]byte, error) {
	return EncodeFrame(CommandStreamResponse, status, sequence, body)
}

func DecodeStreamRequest(frame Frame) ([]byte, error) {
	if frame.Header.Command != CommandStreamRequest {
		return nil, fmt.Errorf("wire: expected stream request, got command 0x%02x", uint8(frame.Header.Command))
	}
	return append([]byte(nil), frame.Body...), nil
}

func DecodeStreamResponse(frame Frame) ([]byte, error) {
	if frame.Header.Command != CommandStreamResponse {
		return nil, fmt.Errorf("wire: expected stream response, got command 0x%02x", uint8(frame.Header.Command))
	}
	return append([]byte(nil), frame.Body...), nil
}

func EncodeUnbindRequest(sequence uint32) ([]byte, error) {
	return EncodeFrame(CommandUnbindRequest, StatusOK, sequence, nil)
}

func EncodeUnbindResponse(sequence uint32) ([]byte, error) {
	return EncodeFrame(CommandUnbindResponse, StatusOK, sequence, nil)
}

func EncodeEnquireLinkRequest(sequence uint32) ([]byte, error) {
	return EncodeFrame(CommandEnquireLinkRequest, StatusOK, sequence, nil)
}

func EncodeEnquireLinkResponse(sequence uint32) ([]byte, error) {
	return EncodeFrame(CommandEnquireLinkResponse, StatusOK, sequence, nil)
}
