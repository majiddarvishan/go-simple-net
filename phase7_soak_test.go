package neta

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func TestPhase7ReconnectSoak(t *testing.T) {
	rawDuration := os.Getenv("NETA_PHASE7_SOAK_SECONDS")
	if rawDuration == "" {
		t.Skip("set NETA_PHASE7_SOAK_SECONDS to run the bounded reconnect soak")
	}
	seconds, err := strconv.Atoi(rawDuration)
	if err != nil || seconds < 5 || seconds > 120 {
		t.Fatalf("NETA_PHASE7_SOAK_SECONDS must be between 5 and 120, got %q", rawDuration)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	var accepted atomic.Uint64
	var peerWG sync.WaitGroup
	acceptDone := make(chan struct{})
	peerErrors := make(chan error, 32)
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			peerWG.Add(1)
			go func(conn net.Conn) {
				defer peerWG.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				request, err := readPhase7Frame(conn)
				if err != nil {
					peerErrors <- err
					return
				}
				bind, err := wire.DecodeBindRequest(request)
				if err != nil {
					peerErrors <- err
					return
				}
				response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
					Type: bind.Type, SystemID: "soak-peer",
				})
				if err != nil {
					peerErrors <- err
					return
				}
				if err := writePhase7Full(conn, response); err != nil {
					// A concurrent terminal Stop may close the socket.
					return
				}

				_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
				next, err := readPhase7Frame(conn)
				if err == nil && next.Header.Command == wire.CommandUnbindRequest {
					unbindResponse, encodeErr := wire.EncodeUnbindResponse(next.Header.Sequence)
					if encodeErr == nil {
						_ = writePhase7Full(conn, unbindResponse)
					}
					return
				}
				var netErr net.Error
				if err != nil && !errors.As(err, &netErr) {
					peerErrors <- err
				}
				// A quiet bound peer is deliberately dropped after the read deadline.
			}(conn)
		}
	}()

	const clientsCount = 3
	clients := make([]*Client, 0, clientsCount)
	var bound atomic.Uint64
	boundEvents := make(chan struct{}, 1024)
	for i := 0; i < clientsCount; i++ {
		client, err := NewClient(
			WithClientAddress(listener.Addr().String()),
			WithClientSystemID("soak-client-"+itoa(i+1)),
			WithAutoReconnect(true),
			WithClientOnBoundHandler(func(*Session) {
				bound.Add(1)
				select {
				case boundEvents <- struct{}{}:
				default:
				}
			}),
		)
		if err != nil {
			t.Fatalf("NewClient %d: %v", i, err)
		}
		clients = append(clients, client)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+15)*time.Second)
	defer cancel()
	for i, client := range clients {
		if err := client.Connect(ctx); err != nil {
			t.Fatalf("initial Connect client %d: %v", i, err)
		}
	}

	soakTimer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer soakTimer.Stop()
	for {
		select {
		case <-boundEvents:
		case <-soakTimer.C:
			goto soakComplete
		case err := <-peerErrors:
			t.Fatalf("loopback peer: %v", err)
		case <-ctx.Done():
			t.Fatalf("soak deadline: %v", ctx.Err())
		}
	}

soakComplete:
	if got := bound.Load(); got < clientsCount+1 {
		t.Fatalf("only %d Bound generations observed across %d Clients", got, clientsCount)
	}

	for i, client := range clients {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := client.Stop(stopCtx)
		stopCancel()
		if err != nil {
			t.Fatalf("Stop client %d: %v", i, err)
		}
		if _, ok := client.CurrentSession(); ok {
			t.Fatalf("client %d still exposes a Session after Stop", i)
		}
		usage := client.resources.snapshot()
		if usage.outstandingRequests != 0 ||
			usage.retainedApplicationBytes != 0 ||
			usage.retainedCompletionBytes != 0 ||
			usage.onCloseEventCredits != 0 {
			t.Fatalf("client %d owner resource leak after Stop: %+v", i, usage)
		}
		app := client.callbacks.application.snapshot()
		completion := client.callbacks.completion.snapshot()
		onClose := client.callbacks.onClose.snapshot()
		if app.Workers > client.config.owner.applicationWorkers ||
			completion.Workers > client.config.owner.completionWorkers ||
			onClose.Workers > client.config.owner.onCloseWorkers {
			t.Fatalf("client %d exceeded callback worker ceilings: app=%+v completion=%+v onClose=%+v", i, app, completion, onClose)
		}
	}

	_ = listener.Close()
	<-acceptDone
	peerWG.Wait()
}
