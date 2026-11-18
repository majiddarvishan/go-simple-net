package neta

import (
	"bytes"
	"fmt"
	"io"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

// One already-encoded Bind Frame must use the first sequence in the
// Session generator; the next outbound sequence must not reuse Bind's ID.
func TestClientInitialBindFrameIsExactAndConsumesSequence(t *testing.T) {
	for _, tc := range []struct {
		name string
		version ProtocolVersion
		encode func(uint32, wire.Bind) ([]byte, error)
	}{
		{name: "V1", version: ProtocolVersionV1, encode: wire.EncodeBindRequest},
		{name: "V2", version: ProtocolVersionV2, encode: wire.EncodeBindRequestV2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			config := defaultSessionConfig()
			config.protocolVersion = tc.version
			config.enquireLinkIdleInterval = 0
			bind := wire.Bind{Type: wire.BindBiDirectional, SystemID: "sequence-test"}
			s, err := newClientBindingSession(local, config, bind, time.Second, &testSessionDispatcher{}, sessionHandlers{})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			want, err := tc.encode(1, bind)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(want))
			if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(peer, got); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("encoded Bind differs: got %x want %x", got, want)
			}
			s.mu.Lock()
			next := s.nextSequenceLocked()
			s.mu.Unlock()
			if next != 2 {
				t.Fatalf("first post-Bind sequence = %d, want 2", next)
			}
		})
	}
}

// The constructor intentionally validates Bind before acquiring the
// connection. Invalid Bind must not silently close a caller-owned socket.
func TestInvalidClientBindRetainsCallerConnection(t *testing.T) {
	for _, version := range []ProtocolVersion{ProtocolVersionV1, ProtocolVersionV2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			local, peer := net.Pipe()
			defer local.Close()
			defer peer.Close()
			config := defaultSessionConfig()
			config.protocolVersion = version
			s, err := newClientBindingSession(local, config, wire.Bind{
				Type: wire.BindBiDirectional, SystemID: "invalid\x00id",
			}, time.Second, &testSessionDispatcher{}, sessionHandlers{})
			if err == nil || s != nil {
				t.Fatalf("invalid Bind returned session %v and error %v", s, err)
			}
			if !errors.Is(err, wire.ErrInvalidSystemID) {
				t.Fatalf("invalid Bind error = %v, want ErrInvalidSystemID", err)
			}
			if err := peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			var probe [1]byte
			_, err = peer.Read(probe[:])
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Fatalf("caller socket unexpectedly closed: read error %v", err)
			}
		})
	}
}
