package neta

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

type phase8PinexPeerProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func startPhase8PinexServer(b *testing.B, binary, address string) *phase8PinexPeerProcess {
	b.Helper()
	cmd := exec.Command(binary, "server-echo", address)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.Fatalf("Pinex peer stdout: %v", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		b.Fatalf("start Pinex C++ Server: %v", err)
	}
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "READY" {
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		b.Fatalf("Pinex C++ Server readiness = %q, read error=%v, process error=%v", ready, err, waitErr)
	}
	peer := &phase8PinexPeerProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		peer.err = cmd.Wait()
		close(peer.done)
	}()
	return peer
}

func (p *phase8PinexPeerProcess) wait(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.err
	case <-timer.C:
		return errors.New("Pinex C++ Server process did not exit after graceful Unbind")
	}
}

func (p *phase8PinexPeerProcess) killAndWait() {
	if p == nil {
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

func phase8FreeTCPAddress(b *testing.B) string {
	b.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		b.Fatal(err)
	}
	return address
}

func phase8ClientWorkersRetired(client *Client) bool {
	return client.callbacks.application.snapshot().Workers == 0 &&
		client.callbacks.completion.snapshot().Workers == 0 &&
		client.callbacks.onClose.snapshot().Workers == 0
}

func BenchmarkPhase8PinexServer(b *testing.B) {
	binary := os.Getenv("NETA_PINEX_PEER_BIN")
	if binary == "" {
		b.Skip("set NETA_PINEX_PEER_BIN to benchmark against the pinned Pinex C++ Server")
	}
	if _, err := os.Stat(binary); err != nil {
		b.Fatalf("Pinex peer binary %q: %v", binary, err)
	}

	for _, size := range []int{64, 1024, 4096, 64 << 10, wire.MaxBodySize} {
		producers := []int{1, 4, 16}
		if size == wire.MaxBodySize {
			producers = []int{1}
		}
		for _, producerCount := range producers {
			name := fmt.Sprintf("payload=%s/producers=%d", phase8PayloadName(size), producerCount)
			b.Run(name, func(b *testing.B) {
				address := phase8FreeTCPAddress(b)
				peer := startPhase8PinexServer(b, binary, address)

				var client *Client
				b.Cleanup(func() {
					if client != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						_ = client.Stop(ctx)
						cancel()
						deadline := time.Now().Add(2 * time.Second)
						for !phase8ClientWorkersRetired(client) && time.Now().Before(deadline) {
							time.Sleep(time.Millisecond)
						}
					}
					peer.killAndWait()
				})

				var err error
				client, err = NewClient(
					WithClientAddress(address),
					WithClientSystemID("phase8-pinex-client"),
					WithClientApplicationWorkerIdleGrace(10*time.Millisecond),
					WithClientCompletionWorkerIdleGrace(10*time.Millisecond),
					WithClientOnCloseWorkerIdleGrace(10*time.Millisecond),
				)
				if err != nil {
					b.Fatalf("NewClient: %v", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err = client.Connect(ctx)
				cancel()
				if err != nil {
					b.Fatalf("Go Client Connect to Pinex C++ Server: %v", err)
				}
				session, ok := client.CurrentSession()
				if !ok {
					b.Fatal("Go Client has no Bound Session")
				}

				fakeServer := &Server{callbacks: newOwnerCallbackDispatcher(defaultOwnerConfig())}
				env := &phase8BenchEnv{
					server:   fakeServer,
					clients:  []*Client{client},
					sessions: []*Session{session},
					errors:   make(chan error, 1),
				}
				phase8RunRequestResponse(b, env, phase8Payload(size), producerCount)

				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := client.Stop(stopCtx); err != nil {
					stopCancel()
					b.Fatalf("Go Client graceful Stop: %v", err)
				}
				stopCancel()
				if err := peer.wait(5 * time.Second); err != nil {
					peer.killAndWait()
					b.Fatalf("Pinex C++ Server graceful exit: %v", err)
				}
				deadline := time.Now().Add(2 * time.Second)
				for !phase8ClientWorkersRetired(client) && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if !phase8ClientWorkersRetired(client) {
					b.Errorf("Go Client callback workers did not retire after idle grace")
				}
			})
		}
	}
}
