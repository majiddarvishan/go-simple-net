package neta

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

// An attacker can send only the Header of a Bind whose declared Body is
// impossible for V1 and V2. Do not allocate or block waiting for that Body.
func TestOversizedBindHeaderIsRejectedBeforeReadingBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		server bool
		cmd    wire.CommandID
	}{
		{name: "ServerRequest", server: true, cmd: wire.CommandBindRequest},
		{name: "ClientResponse", cmd: wire.CommandBindResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			config := defaultSessionConfig()
			config.enquireLinkIdleInterval = 0
			dispatcher := &testSessionDispatcher{}
			var session *Session
			var err error
			if tc.server {
				session, err = newServerPreBindSession(
					local, config, time.Now(), 5*time.Second, dispatcher, sessionHandlers{},
				)
			} else {
				session, err = newClientBindingSession(
					local, config, wire.Bind{
						Type: wire.BindBiDirectional, SystemID: "test-peer",
					}, 5*time.Second, dispatcher, sessionHandlers{},
				)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if !tc.server {
				_ = readWireFrame(t, peer)
			}
			header, err := (wire.Header{
				Length: uint32(wire.HeaderSize + 22),
				Command: tc.cmd,
				Status: wire.StatusOK,
				Sequence: 1,
			}).MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			writeWireFrame(t, peer, header[:])
			select {
			case <-session.Done():
			case <-time.After(time.Second):
				t.Fatal("oversized Bind Header was not rejected before reading the Body")
			}
			if !errors.Is(session.Err(), ErrProtocol) {
				t.Fatalf("oversized Bind close error = %v, want ErrProtocol", session.Err())
			}
		})
	}
}
