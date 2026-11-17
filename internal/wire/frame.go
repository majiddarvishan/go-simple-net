// Package wire implements the neta-c++ 1.5.0 wire format.
// It deliberately has no dependency on the public neta package or runtime.
package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	HeaderSize   = 10
	MaxFrameSize = 1 << 20
	MaxBodySize  = MaxFrameSize - HeaderSize
)

type CommandID uint8

const (
	CommandBindRequest        CommandID = 0x01
	CommandStreamRequest      CommandID = 0x02
	CommandUnbindRequest      CommandID = 0x03
	CommandEnquireLinkRequest CommandID = 0x04

	CommandBindResponse        CommandID = 0x81
	CommandStreamResponse      CommandID = 0x82
	CommandUnbindResponse      CommandID = 0x83
	CommandEnquireLinkResponse CommandID = 0x84
)

type CommandStatus uint8

const (
	StatusOK     CommandStatus = 0x00
	StatusFailed CommandStatus = 0xFF
)

var (
	ErrShortHeader        = errors.New("wire: frame shorter than header")
	ErrInvalidLength      = errors.New("wire: invalid frame length")
	ErrFrameTooLarge      = errors.New("wire: frame exceeds 1 MiB maximum")
	ErrUnknownCommand     = errors.New("wire: unknown command ID")
	ErrInvalidControlBody = errors.New("wire: control command must have an empty body")
)

// Header is the fixed 10-byte neta-c++ frame header.
type Header struct {
	Length   uint32
	Command  CommandID
	Status   CommandStatus
	Sequence uint32
}

// Frame owns its Body bytes. DecodeFrame copies Body so the returned Frame
// remains valid independently of the caller's input buffer.
type Frame struct {
	Header Header
	Body   []byte
}

func (h Header) MarshalBinary() ([HeaderSize]byte, error) {
	var out [HeaderSize]byte
	if h.Length < HeaderSize {
		return out, fmt.Errorf("%w: %d is below header size", ErrInvalidLength, h.Length)
	}
	if h.Length > MaxFrameSize {
		return out, fmt.Errorf("%w: %d", ErrFrameTooLarge, h.Length)
	}
	if !knownCommand(h.Command) {
		return out, fmt.Errorf("%w: 0x%02x", ErrUnknownCommand, uint8(h.Command))
	}
	binary.BigEndian.PutUint32(out[0:4], h.Length)
	out[4] = byte(h.Command)
	out[5] = byte(h.Status)
	binary.BigEndian.PutUint32(out[6:10], h.Sequence)
	return out, nil
}

func ParseHeader(src []byte) (Header, error) {
	if len(src) < HeaderSize {
		return Header{}, ErrShortHeader
	}
	h := Header{
		Length:   binary.BigEndian.Uint32(src[0:4]),
		Command:  CommandID(src[4]),
		Status:   CommandStatus(src[5]),
		Sequence: binary.BigEndian.Uint32(src[6:10]),
	}
	if h.Length < HeaderSize {
		return Header{}, fmt.Errorf("%w: %d is below header size", ErrInvalidLength, h.Length)
	}
	if h.Length > MaxFrameSize {
		return Header{}, fmt.Errorf("%w: %d", ErrFrameTooLarge, h.Length)
	}
	if !knownCommand(h.Command) {
		return Header{}, fmt.Errorf("%w: 0x%02x", ErrUnknownCommand, uint8(h.Command))
	}
	return h, nil
}

func EncodeFrame(command CommandID, status CommandStatus, sequence uint32, body []byte) ([]byte, error) {
	if len(body) > MaxBodySize {
		return nil, fmt.Errorf("%w: body is %d bytes", ErrFrameTooLarge, len(body))
	}
	length := HeaderSize + len(body)
	h := Header{Length: uint32(length), Command: command, Status: status, Sequence: sequence}
	header, err := h.MarshalBinary()
	if err != nil {
		return nil, err
	}
	frame := make([]byte, length)
	copy(frame[:HeaderSize], header[:])
	copy(frame[HeaderSize:], body)
	return frame, nil
}

// DecodeFrame decodes exactly one complete Frame. Partial TCP reads are
// handled by the future transport layer, not by this pure codec.
func DecodeFrame(src []byte) (Frame, error) {
	h, err := ParseHeader(src)
	if err != nil {
		return Frame{}, err
	}
	if uint64(len(src)) < uint64(h.Length) {
		return Frame{}, fmt.Errorf("%w: declared %d bytes, received %d", ErrInvalidLength, h.Length, len(src))
	}
	if uint64(len(src)) > uint64(h.Length) {
		return Frame{}, fmt.Errorf("%w: declared %d bytes, received %d", ErrInvalidLength, h.Length, len(src))
	}
	body := append([]byte(nil), src[HeaderSize:]...)
	if isEmptyBodyCommand(h.Command) && len(body) != 0 {
		return Frame{}, fmt.Errorf("%w: command 0x%02x has %d bytes", ErrInvalidControlBody, uint8(h.Command), len(body))
	}
	return Frame{Header: h, Body: body}, nil
}

func knownCommand(id CommandID) bool {
	switch id {
	case CommandBindRequest, CommandStreamRequest, CommandUnbindRequest, CommandEnquireLinkRequest,
		CommandBindResponse, CommandStreamResponse, CommandUnbindResponse, CommandEnquireLinkResponse:
		return true
	default:
		return false
	}
}

func isEmptyBodyCommand(id CommandID) bool {
	switch id {
	case CommandUnbindRequest, CommandUnbindResponse, CommandEnquireLinkRequest, CommandEnquireLinkResponse:
		return true
	default:
		return false
	}
}

// SequenceGenerator implements neta-c++'s per-Session incrementing sequence
// rule. Zero is skipped on wraparound. It is safe for concurrent callers.
type SequenceGenerator struct {
	value atomicUint32
}

func (s *SequenceGenerator) Next() uint32 {
	return nextSequence(&s.value)
}
