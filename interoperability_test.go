package neta

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func interopPeerBinary(t *testing.T) string {
	t.Helper()
	path := os.Getenv("NETA_PINEX_PEER_BIN")
	if path == "" {
		t.Skip("set NETA_PINEX_PEER_BIN to run live Pinex C++ interoperability")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Pinex peer binary %q: %v", path, err)
	}
	return path
}

type interopPeerProcess struct {
	done   <-chan error
	stderr *bytes.Buffer
	cancel context.CancelFunc
}

func startInteropPinexServer(t *testing.T, binary, address string) *interopPeerProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	cmd := exec.CommandContext(ctx, binary, "server", address)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("Pinex server stdout: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start Pinex server: %v", err)
	}

	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "READY" {
		cancel()
		_ = cmd.Wait()
		t.Fatalf("Pinex server readiness = %q, err=%v, stderr=%s", ready, err, stderr.String())
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	process := &interopPeerProcess{done: done, stderr: stderr, cancel: cancel}
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	return process
}

func waitInteropPinexServer(t *testing.T, process *interopPeerProcess) {
	t.Helper()
	select {
	case err := <-process.done:
		if err != nil {
			t.Fatalf("Pinex C++ server failed: %v; stderr=%s", err, process.stderr.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("Pinex C++ server did not exit; stderr=%s", process.stderr.String())
	}
}

func runInteropPinexClient(t *testing.T, binary, address string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "client", address)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start Pinex C++ client: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Pinex C++ client failed: %v; stdout=%s; stderr=%s", err, stdout.String(), stderr.String())
		}
	case <-ctx.Done():
		t.Fatalf("Pinex C++ client timed out; stdout=%s; stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS go:from-cpp") {
		t.Fatalf("Pinex C++ client did not confirm the Go response: stdout=%s; stderr=%s", stdout.String(), stderr.String())
	}
}

func interopFreeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func TestPinexCPPServerWithGoClient(t *testing.T) {
	binary := interopPeerBinary(t)
	address := interopFreeAddress(t)
	peer := startInteropPinexServer(t, binary, address)

	client, err := NewClient(
		WithClientAddress(address),
		WithClientSystemID("neta-go-client"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Go Client Connect to Pinex C++ Server: %v", err)
	}
	session, ok := client.CurrentSession()
	if !ok {
		t.Fatal("Go Client did not publish its Bound Session")
	}
	if session.SystemID() != "pinex-cpp-server" {
		t.Fatalf("Pinex server SystemID = %q", session.SystemID())
	}

	type completion struct {
		response Response
		err      error
	}
	completed := make(chan completion, 1)
	if _, err := session.SendRequest([]byte("from-go"), func(response Response, err error) {
		completed <- completion{response: response, err: err}
	}); err != nil {
		t.Fatalf("Go Client SendRequest: %v", err)
	}

	select {
	case result := <-completed:
		if result.err != nil {
			t.Fatalf("Go Client Completion: %v", result.err)
		}
		if result.response.Status != 0 || string(result.response.Payload) != "cpp:from-go" {
			t.Fatalf("Pinex response = status %d payload %q", result.response.Status, result.response.Payload)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for Pinex C++ Server response")
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Go Client graceful Stop: %v", err)
	}
	waitInteropPinexServer(t, peer)
}

func TestGoServerWithPinexCPPClient(t *testing.T) {
	binary := interopPeerBinary(t)
	requests := make(chan error, 1)
	server, err := NewServer(
		WithServerListenAddress("127.0.0.1:0"),
		WithServerOnRequestHandler(func(session *Session, request Request) {
			payload := append([]byte("go:"), request.Payload...)
			requests <- session.SendResponse(request.Sequence, payload, 0)
		}),
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Go Server Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = server.Stop(stopCtx)
	}()

	runInteropPinexClient(t, binary, server.Addr().String())
	select {
	case err := <-requests:
		if err != nil {
			t.Fatalf("Go Server SendResponse: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Go Server did not receive the Pinex C++ Request")
	}
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Go Server graceful Stop: %v", err)
	}
}
