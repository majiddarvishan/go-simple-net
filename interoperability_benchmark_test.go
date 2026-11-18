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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

type interopBenchmarkPeerProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func startInteropBenchmarkServer(b *testing.B, binary, address string) *interopBenchmarkPeerProcess {
	b.Helper()
	cmd := exec.Command(binary, "server-echo", address)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.Fatalf("neta-cpp peer stdout: %v", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		b.Fatalf("start neta-cpp Server: %v", err)
	}
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "READY" {
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		b.Fatalf("neta-cpp Server readiness = %q, read error=%v, process error=%v", ready, err, waitErr)
	}
	peer := &interopBenchmarkPeerProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		peer.err = cmd.Wait()
		close(peer.done)
	}()
	return peer
}

func (p *interopBenchmarkPeerProcess) wait(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.err
	case <-timer.C:
		return errors.New("neta-cpp Server process did not exit after graceful Unbind")
	}
}

func (p *interopBenchmarkPeerProcess) killAndWait() {
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

func benchmarkFreeTCPAddress(b *testing.B) string {
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

func benchmarkClientWorkersRetired(client *Client) bool {
	return client.callbacks.application.snapshot().Workers == 0 &&
		client.callbacks.completion.snapshot().Workers == 0 &&
		client.callbacks.onClose.snapshot().Workers == 0
}

func BenchmarkInteropCppServer(b *testing.B) {
	binary := os.Getenv("NETA_CPP_PEER_BIN")
	if binary == "" {
		b.Skip("set NETA_CPP_PEER_BIN to benchmark against the pinned neta-cpp Server")
	}
	if _, err := os.Stat(binary); err != nil {
		b.Fatalf("neta-cpp peer binary %q: %v", binary, err)
	}

	for _, size := range []int{64, 1024, 4096, 64 << 10, wire.MaxBodySize} {
		producers := []int{1, 4, 16}
		if size == wire.MaxBodySize {
			producers = []int{1}
		}
		for _, producerCount := range producers {
			name := fmt.Sprintf("payload=%s/producers=%d", benchmarkPayloadName(size), producerCount)
			b.Run(name, func(b *testing.B) {
				address := benchmarkFreeTCPAddress(b)
				peer := startInteropBenchmarkServer(b, binary, address)

				var client *Client
				b.Cleanup(func() {
					if client != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						_ = client.Stop(ctx)
						cancel()
						deadline := time.Now().Add(2 * time.Second)
						for !benchmarkClientWorkersRetired(client) && time.Now().Before(deadline) {
							time.Sleep(time.Millisecond)
						}
					}
					peer.killAndWait()
				})

				var err error
				client, err = NewClient(
					WithClientAddress(address),
					WithClientSystemID("benchmark-neta-cpp-client"),
					WithClientSessionOptions(WithProtocolVersion(ProtocolVersionV1)),
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
					b.Fatalf("Go Client Connect to neta-cpp Server: %v", err)
				}
				session, ok := client.CurrentSession()
				if !ok {
					b.Fatal("Go Client has no Bound Session")
				}

				fakeServer := &Server{callbacks: newOwnerCallbackDispatcher(defaultOwnerConfig())}
				env := &benchmarkEnv{
					server:   fakeServer,
					clients:  []*Client{client},
					sessions: []*Session{session},
					errors:   make(chan error, 1),
				}
				benchmarkRunRequestResponse(b, env, benchmarkPayload(size), producerCount)

				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := client.Stop(stopCtx); err != nil {
					stopCancel()
					b.Fatalf("Go Client graceful Stop: %v", err)
				}
				stopCancel()
				if err := peer.wait(5 * time.Second); err != nil {
					peer.killAndWait()
					b.Fatalf("neta-cpp Server graceful exit: %v", err)
				}
				deadline := time.Now().Add(2 * time.Second)
				for !benchmarkClientWorkersRetired(client) && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if !benchmarkClientWorkersRetired(client) {
					b.Errorf("Go Client callback workers did not retire after idle grace")
				}
			})
		}
	}
}

func BenchmarkInteropCppClient(b *testing.B) {
	binary := os.Getenv("NETA_CPP_PEER_BIN")
	if binary == "" {
		b.Skip("set NETA_CPP_PEER_BIN to benchmark the neta-cpp Client against the Go Server")
	}
	if _, err := os.Stat(binary); err != nil {
		b.Fatalf("neta-cpp peer binary %q: %v", binary, err)
	}

	for _, size := range []int{64, 1024, 4096, 64 << 10, wire.MaxBodySize} {
		name := fmt.Sprintf("payload=%s", benchmarkPayloadName(size))
		b.Run(name, func(b *testing.B) {
			serverErrors := make(chan error, 1)
			server, err := NewServer(
				WithServerListenAddress("127.0.0.1:0"),
				WithServerApplicationWorkerIdleGrace(10*time.Millisecond),
				WithServerCompletionWorkerIdleGrace(10*time.Millisecond),
				WithServerOnCloseWorkerIdleGrace(10*time.Millisecond),
				WithServerOnRequestHandler(func(session *Session, request Request) {
					if err := session.SendResponse(request.Sequence, request.Payload, 0); err != nil {
						select {
						case serverErrors <- err:
						default:
						}
					}
				}),
			)
			if err != nil {
				b.Fatalf("NewServer: %v", err)
			}
			if err := server.Start(); err != nil {
				b.Fatalf("Server.Start: %v", err)
			}

			var cmd *exec.Cmd
			var stdin io.WriteCloser
			var stdout *bufio.Reader
			b.Cleanup(func() {
				if cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = server.Stop(ctx)
				cancel()
			})

			cmd = exec.Command(binary, "client-bench", server.Addr().String(),
				strconv.Itoa(b.N), strconv.Itoa(size))
			cmd.Stderr = io.Discard
			stdin, err = cmd.StdinPipe()
			if err != nil {
				b.Fatalf("neta-cpp Client stdin: %v", err)
			}
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				b.Fatalf("neta-cpp Client stdout: %v", err)
			}
			stdout = bufio.NewReader(stdoutPipe)
			if err := cmd.Start(); err != nil {
				b.Fatalf("start neta-cpp Client: %v", err)
			}

			type lineResult struct {
				line string
				err  error
			}
			readyCh := make(chan lineResult, 1)
			go func() {
				line, err := stdout.ReadString('\n')
				readyCh <- lineResult{line: line, err: err}
			}()
			select {
			case ready := <-readyCh:
				if ready.err != nil || strings.TrimSpace(ready.line) != "READY" {
					b.Fatalf("neta-cpp Client readiness = %q, error=%v", ready.line, ready.err)
				}
			case <-time.After(10 * time.Second):
				b.Fatal("neta-cpp Client did not Bind within 10 seconds")
			}

			var before runtime.MemStats
			runtime.ReadMemStats(&before)
			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			if _, err := fmt.Fprintln(stdin, "GO"); err != nil {
				b.Fatalf("release neta-cpp Client benchmark gate: %v", err)
			}

			resultCh := make(chan lineResult, 1)
			go func() {
				line, err := stdout.ReadString('\n')
				resultCh <- lineResult{line: line, err: err}
			}()
			var result lineResult
			select {
			case result = <-resultCh:
			case <-time.After(30 * time.Second):
				b.StopTimer()
				b.Fatal("neta-cpp Client benchmark did not finish within 30 seconds")
			}
			b.StopTimer()
			elapsed := b.Elapsed()
			if result.err != nil {
				b.Fatalf("read neta-cpp Client benchmark result: %v", result.err)
			}

			fields := strings.Fields(result.line)
			if len(fields) != 6 || fields[0] != "PASS" {
				b.Fatalf("unexpected neta-cpp Client benchmark result: %q", result.line)
			}
			count, err1 := strconv.Atoi(fields[1])
			totalNS, err2 := strconv.ParseInt(fields[2], 10, 64)
			p50, err3 := strconv.ParseInt(fields[3], 10, 64)
			p95, err4 := strconv.ParseInt(fields[4], 10, 64)
			p99, err5 := strconv.ParseInt(fields[5], 10, 64)
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
				b.Fatalf("parse neta-cpp Client benchmark result: %q", result.line)
			}
			if count != b.N || totalNS <= 0 {
				b.Fatalf("neta-cpp Client completed %d requests, expected %d; elapsed=%d", count, b.N, totalNS)
			}
			b.ReportMetric(float64(count)*1e9/float64(totalNS), "peer-ops/s")
			b.ReportMetric(float64(totalNS)/float64(count), "peer-ns/op")
			b.ReportMetric(float64(p50), "p50-ns")
			b.ReportMetric(float64(p95), "p95-ns")
			b.ReportMetric(float64(p99), "p99-ns")

			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			appSnapshot := server.callbacks.application.snapshot()
			goroutines := runtime.NumGoroutine()
			benchmarkReportRuntime(b, before, after, elapsed, goroutines, appSnapshot, 0)

			select {
			case serverErr := <-serverErrors:
				b.Fatalf("Go Server callback: %v", serverErr)
			default:
			}
			waitCh := make(chan error, 1)
			go func() { waitCh <- cmd.Wait() }()
			select {
			case err := <-waitCh:
				if err != nil {
					b.Fatalf("neta-cpp Client process: %v", err)
				}
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				b.Fatal("neta-cpp Client process did not exit after Unbind")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Stop(ctx); err != nil {
				cancel()
				b.Fatalf("Go Server Stop: %v", err)
			}
			cancel()
		})
	}
}
