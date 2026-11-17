package neta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

type phase8BenchEnv struct {
	closeOnce sync.Once
	closeErr  error
	server   *Server
	clients  []*Client
	sessions []*Session
	received chan struct{}
	errors   chan error
}

type phase8CompletionResult struct {
	response Response
	err      error
}

func newPhase8BenchEnv(b testing.TB, sessionCount int, applicationConcurrency, completionConcurrency uint32, respond bool, raiseWorkerCeilings bool) *phase8BenchEnv {
	b.Helper()
	env := &phase8BenchEnv{
		clients:  make([]*Client, 0, sessionCount),
		sessions: make([]*Session, 0, sessionCount),
		received: make(chan struct{}, 4096),
		errors:   make(chan error, 1),
	}
	serverOptions := []ServerOption{
		WithServerListenAddress("127.0.0.1:0"),
		WithServerApplicationWorkerIdleGrace(10 * time.Millisecond),
		WithServerCompletionWorkerIdleGrace(10 * time.Millisecond),
		WithServerOnCloseWorkerIdleGrace(10 * time.Millisecond),
		WithServerOnRequestHandler(func(session *Session, request Request) {
			if respond {
				if err := session.SendResponse(request.Sequence, request.Payload, 0); err != nil {
					env.recordError(err)
				}
				return
			}
			select {
			case env.received <- struct{}{}:
			default:
				env.recordError(errors.New("phase8 benchmark receive signal buffer exhausted"))
			}
		}),
	}
	if applicationConcurrency != 0 {
		serverOptions = append(serverOptions, WithServerSessionOptions(
			WithApplicationCallbackConcurrency(applicationConcurrency),
		))
	}
	if raiseWorkerCeilings {
		serverOptions = append(serverOptions, WithServerApplicationWorkers(32))
	}
	server, err := NewServer(serverOptions...)
	if err != nil {
		b.Fatalf("NewServer: %v", err)
	}
	env.server = server
	if err := server.Start(); err != nil {
		b.Fatalf("Server.Start: %v", err)
	}
	b.Cleanup(func() { env.close(b) })

	for i := 0; i < sessionCount; i++ {
		options := []ClientOption{
			WithClientAddress(server.Addr().String()),
			WithClientSystemID("phase8-" + strconv.Itoa(i)),
			WithClientApplicationWorkerIdleGrace(10 * time.Millisecond),
			WithClientCompletionWorkerIdleGrace(10 * time.Millisecond),
			WithClientOnCloseWorkerIdleGrace(10 * time.Millisecond),
		}
		if completionConcurrency != 0 {
			options = append(options, WithClientSessionOptions(
				WithCompletionCallbackConcurrency(completionConcurrency),
			))
		}
		if raiseWorkerCeilings {
			options = append(options, WithClientCompletionWorkers(32))
		}
		client, err := NewClient(options...)
		if err != nil {
			b.Fatalf("NewClient %d: %v", i, err)
		}
		env.clients = append(env.clients, client)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = client.Connect(ctx)
		cancel()
		if err != nil {
			b.Fatalf("Client %d Connect: %v", i, err)
		}
		session, ok := client.CurrentSession()
		if !ok {
			b.Fatalf("Client %d has no Bound Session", i)
		}
		env.sessions = append(env.sessions, session)
	}
	return env
}


func (e *phase8BenchEnv) close(t testing.TB) {
	t.Helper()
	e.closeOnce.Do(func() {
		for _, client := range e.clients {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := client.Stop(ctx); err != nil && e.closeErr == nil {
				e.closeErr = fmt.Errorf("Client.Stop: %w", err)
			}
			cancel()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := e.server.Stop(ctx); err != nil && e.closeErr == nil {
			e.closeErr = fmt.Errorf("Server.Stop: %w", err)
		}
		cancel()

		retired := func() bool {
			if e.server.callbacks.application.snapshot().Workers != 0 ||
				e.server.callbacks.completion.snapshot().Workers != 0 ||
				e.server.callbacks.onClose.snapshot().Workers != 0 {
				return false
			}
			for _, client := range e.clients {
				if client.callbacks.application.snapshot().Workers != 0 ||
					client.callbacks.completion.snapshot().Workers != 0 ||
					client.callbacks.onClose.snapshot().Workers != 0 {
					return false
				}
			}
			return true
		}
		deadline := time.Now().Add(2 * time.Second)
		for !retired() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !retired() && e.closeErr == nil {
			e.closeErr = errors.New("callback workers remained alive after idle grace")
		}
	})
	if e.closeErr != nil {
		t.Errorf("Phase 8 benchmark environment cleanup: %v", e.closeErr)
	}
}

func (e *phase8BenchEnv) recordError(err error) {
	if err == nil {
		return
	}
	select {
	case e.errors <- err:
	default:
	}
}

func (e *phase8BenchEnv) checkError(b testing.TB) {
	b.Helper()
	select {
	case err := <-e.errors:
		b.Fatalf("benchmark peer: %v", err)
	default:
	}
}

func phase8Payload(size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte((i * 31) + 7)
	}
	return payload
}

func phase8PayloadName(size int) string {
	switch size {
	case 64:
		return "64B"
	case 256:
		return "256B"
	case 1024:
		return "1KiB"
	case 4096:
		return "4KiB"
	case wire.MaxBodySize:
		return "near-1MiB"
	default:
		return strconv.Itoa(size) + "B"
	}
}

func phase8ResponseMatches(response Response, payload []byte) bool {
	if response.Status != 0 || len(response.Payload) != len(payload) {
		return false
	}
	if len(payload) <= 4096 {
		return bytes.Equal(response.Payload, payload)
	}
	if len(payload) == 0 {
		return true
	}
	middle := len(payload) / 2
	return response.Payload[0] == payload[0] &&
		response.Payload[middle] == payload[middle] &&
		response.Payload[len(payload)-1] == payload[len(payload)-1]
}

func phase8Percentile(samples []int64, percentile float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	index := int(float64(len(samples))*percentile+0.999999) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(samples) {
		index = len(samples) - 1
	}
	return float64(samples[index])
}

func phase8ReportSamples(b *testing.B, samples []int64, prefix string) {
	if len(samples) == 0 {
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	b.ReportMetric(phase8Percentile(samples, 0.50), prefix+"p50-ns")
	b.ReportMetric(phase8Percentile(samples, 0.95), prefix+"p95-ns")
	b.ReportMetric(phase8Percentile(samples, 0.99), prefix+"p99-ns")
}

func phase8ReportRuntime(b *testing.B, before, after runtime.MemStats, elapsed time.Duration, goroutines int, appWorkers callbackWorkerPoolSnapshot, completionWorkers uint32) {
	if b.N > 0 {
		b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/float64(b.N), "heap-alloc-B/op")
	}
	b.ReportMetric(float64(after.HeapAlloc), "heap-live-B")
	b.ReportMetric(float64(after.NumGC-before.NumGC), "gc-cycles")
	b.ReportMetric(float64(goroutines), "goroutines")
	b.ReportMetric(float64(appWorkers.Workers), "server-app-workers")
	b.ReportMetric(float64(appWorkers.ActiveCallbacks), "server-app-active")
	b.ReportMetric(float64(completionWorkers), "client-completion-workers")
	if elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed.Seconds(), "ops/s")
	}
}

func phase8ReportSessionFairness(b *testing.B, perSession [][]int64) {
	var minP95, maxP95 float64
	active := 0
	for _, samples := range perSession {
		if len(samples) == 0 {
			continue
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		p95 := phase8Percentile(samples, 0.95)
		if active == 0 || p95 < minP95 {
			minP95 = p95
		}
		if p95 > maxP95 {
			maxP95 = p95
		}
		active++
	}
	if active > 1 {
		b.ReportMetric(minP95, "session-p95-min-ns")
		b.ReportMetric(maxP95, "session-p95-max-ns")
		if minP95 > 0 {
			b.ReportMetric(maxP95/minP95, "session-p95-ratio")
		}
	}
}

func phase8RunRequestResponse(b *testing.B, env *phase8BenchEnv, payload []byte, producers int) {
	b.Helper()
	if producers < 1 {
		b.Fatal("producer count must be positive")
	}
	counts := make([]int, producers)
	samples := make([][]int64, producers)
	submitSamples := make([][]int64, producers)
	completions := make([]chan phase8CompletionResult, producers)
	handlers := make([]CompletionHandler, producers)
	for worker := 0; worker < producers; worker++ {
		counts[worker] = b.N / producers
		if worker < b.N%producers {
			counts[worker]++
		}
		samples[worker] = make([]int64, counts[worker])
		submitSamples[worker] = make([]int64, counts[worker])
		completions[worker] = make(chan phase8CompletionResult, 1)
		done := completions[worker]
		handlers[worker] = func(response Response, err error) {
			done <- phase8CompletionResult{response: response, err: err}
		}
	}

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var stop atomic.Bool
	var errorOnce sync.Once
	var firstError error
	recordError := func(err error) {
		errorOnce.Do(func() {
			firstError = err
			stop.Store(true)
		})
	}
	for worker := 0; worker < producers; worker++ {
		session := env.sessions[worker%len(env.sessions)]
		done := completions[worker]
		handler := handlers[worker]
		workerSamples := samples[worker]
		workerSubmitSamples := submitSamples[worker]
		workerCount := counts[worker]
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for operation := 0; operation < workerCount && !stop.Load(); operation++ {
				began := time.Now()
				if _, err := session.SendRequest(payload, handler); err != nil {
					recordError(fmt.Errorf("SendRequest: %w", err))
					return
				}
				workerSubmitSamples[operation] = time.Since(began).Nanoseconds()
				result := <-done
				if result.err != nil {
					recordError(fmt.Errorf("Completion: %w", result.err))
					return
				}
				if !phase8ResponseMatches(result.response, payload) {
					recordError(fmt.Errorf("response mismatch: status=%d payload_length=%d expected=%d",
						result.response.Status, len(result.response.Payload), len(payload)))
					return
				}
				workerSamples[operation] = time.Since(began).Nanoseconds()
			}
		}()
	}

	b.ReportAllocs()
	b.SetBytes(int64(2 * len(payload)))
	b.ResetTimer()
	close(start)
	wg.Wait()
	b.StopTimer()
	elapsed := b.Elapsed()
	appSnapshot := env.server.callbacks.application.snapshot()
	var completionWorkerCount uint32
	for _, client := range env.clients {
		completionWorkerCount += client.callbacks.completion.snapshot().Workers
	}
	goroutineCount := runtime.NumGoroutine()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if firstError != nil {
		b.Fatalf("request/response benchmark failed: %v", firstError)
	}
	env.checkError(b)

	latencies := make([]int64, 0, b.N)
	submitLatencies := make([]int64, 0, b.N)
	perSession := make([][]int64, len(env.sessions))
	for worker, workerSamples := range samples {
		latencies = append(latencies, workerSamples...)
		submitLatencies = append(submitLatencies, submitSamples[worker]...)
		sessionIndex := worker % len(env.sessions)
		perSession[sessionIndex] = append(perSession[sessionIndex], workerSamples...)
	}
	phase8ReportSamples(b, submitLatencies, "submit-")
	phase8ReportSamples(b, latencies, "")
	phase8ReportSessionFairness(b, perSession)
	phase8ReportRuntime(b, before, after, elapsed, goroutineCount, appSnapshot, completionWorkerCount)
}

func phase8RunSendNoWait(b *testing.B, env *phase8BenchEnv, payload []byte, producers int) {
	b.Helper()
	counts := make([]int, producers)
	submitSamples := make([][]int64, producers)
	deliverySamples := make([][]int64, producers)
	for worker := 0; worker < producers; worker++ {
		counts[worker] = b.N / producers
		if worker < b.N%producers {
			counts[worker]++
		}
		submitSamples[worker] = make([]int64, counts[worker])
		deliverySamples[worker] = make([]int64, counts[worker])
	}

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var stop atomic.Bool
	var errorOnce sync.Once
	var firstError error
	recordError := func(err error) {
		errorOnce.Do(func() {
			firstError = err
			stop.Store(true)
		})
	}
	for worker := 0; worker < producers; worker++ {
		session := env.sessions[worker%len(env.sessions)]
		submit := submitSamples[worker]
		delivery := deliverySamples[worker]
		workerCount := counts[worker]
		wg.Add(1)
		go func() {
			defer wg.Done()
			timer := time.NewTimer(10 * time.Second)
			if !timer.Stop() {
				<-timer.C
			}
			defer timer.Stop()
			<-start
			for operation := 0; operation < workerCount && !stop.Load(); operation++ {
				began := time.Now()
				if _, err := session.SendNoWait(payload); err != nil {
					recordError(fmt.Errorf("SendNoWait: %w", err))
					return
				}
				submit[operation] = time.Since(began).Nanoseconds()
				timer.Reset(10 * time.Second)
				select {
				case <-env.received:
					delivery[operation] = time.Since(began).Nanoseconds()
				case <-timer.C:
					recordError(errors.New("timed out waiting for SendNoWait delivery"))
					return
				}
			}
		}()
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	close(start)
	wg.Wait()
	b.StopTimer()
	elapsed := b.Elapsed()
	appSnapshot := env.server.callbacks.application.snapshot()
	var completionWorkerCount uint32
	for _, client := range env.clients {
		completionWorkerCount += client.callbacks.completion.snapshot().Workers
	}
	goroutineCount := runtime.NumGoroutine()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if firstError != nil {
		b.Fatalf("SendNoWait benchmark failed: %v", firstError)
	}
	env.checkError(b)

	submitLatencies := make([]int64, 0, b.N)
	deliveryLatencies := make([]int64, 0, b.N)
	perSession := make([][]int64, len(env.sessions))
	for worker := 0; worker < producers; worker++ {
		submitLatencies = append(submitLatencies, submitSamples[worker]...)
		deliveryLatencies = append(deliveryLatencies, deliverySamples[worker]...)
		sessionIndex := worker % len(env.sessions)
		perSession[sessionIndex] = append(perSession[sessionIndex], deliverySamples[worker]...)
	}
	phase8ReportSamples(b, submitLatencies, "submit-")
	phase8ReportSamples(b, deliveryLatencies, "delivery-")
	phase8ReportSessionFairness(b, perSession)
	phase8ReportRuntime(b, before, after, elapsed, goroutineCount, appSnapshot, completionWorkerCount)
}

func TestPhase8RuntimeMetadata(t *testing.T) {
	t.Logf("go=%s goos=%s goarch=%s numcpu=%d gomaxprocs=%d",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.GOMAXPROCS(0))
	t.Logf("GOMAXPROCS env=%q", os.Getenv("GOMAXPROCS"))
	if info, ok := debug.ReadBuildInfo(); ok {
		t.Logf("main module=%s go-version=%s", info.Main.Path, info.GoVersion)
	}
}

var phase8WireSink []byte

func BenchmarkPhase8WireCodec(b *testing.B) {
	for _, size := range []int{64, 256, 1024, 4096, wire.MaxBodySize} {
		payload := phase8Payload(size)
		frame, err := wire.EncodeStreamRequest(1, payload)
		if err != nil {
			b.Fatalf("fixture encode: %v", err)
		}
		name := phase8PayloadName(size)
		b.Run("Encode/payload="+name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				encoded, err := wire.EncodeStreamRequest(uint32(i+1), payload)
				if err != nil {
					b.Fatal(err)
				}
				phase8WireSink = encoded
			}
		})
		b.Run("DecodeFrame/payload="+name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				decoded, err := wire.DecodeFrame(frame)
				if err != nil {
					b.Fatal(err)
				}
				phase8WireSink = decoded.Body
			}
		})
		b.Run("DecodeStreamRequest/payload="+name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				decoded, err := wire.DecodeFrame(frame)
				if err != nil {
					b.Fatal(err)
				}
				body, err := wire.DecodeStreamRequest(decoded)
				if err != nil {
					b.Fatal(err)
				}
				phase8WireSink = body
			}
		})
	}
}

func BenchmarkPhase8RequestResponse(b *testing.B) {
	for _, size := range []int{64, 256, 1024, 4096, wire.MaxBodySize} {
		sessions := []int{1, 8, 32}
		if size == wire.MaxBodySize {
			sessions = []int{1}
		}
		for _, sessionCount := range sessions {
			var producers []int
			switch sessionCount {
			case 1:
				producers = []int{1, 4, 16, 64}
			case 8:
				producers = []int{8, 32, 64}
			case 32:
				producers = []int{32, 64}
			}
			if size == wire.MaxBodySize {
				producers = []int{1, 4}
			}
			for _, producerCount := range producers {
				name := fmt.Sprintf("payload=%s/sessions=%d/producers=%d",
					phase8PayloadName(size), sessionCount, producerCount)
				b.Run(name, func(b *testing.B) {
					env := newPhase8BenchEnv(b, sessionCount, 1, 1, true, false)
					phase8RunRequestResponse(b, env, phase8Payload(size), producerCount)
				})
			}
		}
	}
}

func BenchmarkPhase8SendNoWait(b *testing.B) {
	for _, size := range []int{64, 1024, 4096} {
		for _, sessionCount := range []int{1, 8, 32} {
			producers := []int{1, 16}
			if sessionCount == 8 {
				producers = []int{8, 32}
			} else if sessionCount == 32 {
				producers = []int{32, 64}
			}
			for _, producerCount := range producers {
				name := fmt.Sprintf("payload=%s/sessions=%d/producers=%d",
					phase8PayloadName(size), sessionCount, producerCount)
				b.Run(name, func(b *testing.B) {
					env := newPhase8BenchEnv(b, sessionCount, 1, 1, false, false)
					phase8RunSendNoWait(b, env, phase8Payload(size), producerCount)
				})
			}
		}
	}
}

func BenchmarkPhase8CallbackConcurrency(b *testing.B) {
	for _, concurrency := range []uint32{1, 2, 4, 8, 16} {
		b.Run("Application/concurrency="+strconv.Itoa(int(concurrency)), func(b *testing.B) {
			env := newPhase8BenchEnv(b, 1, concurrency, 1, true, true)
			phase8RunRequestResponse(b, env, phase8Payload(256), 16)
		})
		b.Run("Completion/concurrency="+strconv.Itoa(int(concurrency)), func(b *testing.B) {
			env := newPhase8BenchEnv(b, 1, 1, concurrency, true, true)
			phase8RunRequestResponse(b, env, phase8Payload(256), 16)
		})
	}
}

func BenchmarkPhase8ProfileBaseline(b *testing.B) {
	env := newPhase8BenchEnv(b, 1, 1, 1, true, false)
	phase8RunRequestResponse(b, env, phase8Payload(256), 4)
}

func BenchmarkPhase8ProfileLargeBaseline(b *testing.B) {
	env := newPhase8BenchEnv(b, 1, 1, 1, true, false)
	phase8RunRequestResponse(b, env, phase8Payload(wire.MaxBodySize), 1)
}

func phase8BlockedWriterSession(b *testing.B) (*Session, func()) {
	b.Helper()
	local, peer := net.Pipe()
	config := defaultSessionConfig()
	config.maxQueuedOutgoingBytes = 8 << 20
	owner := defaultOwnerConfig()
	session := newBoundSessionWithResources(
		local,
		config,
		sessionIdentity{role: sessionRoleClient, bindType: wire.BindBiDirectional},
		newOwnerCallbackDispatcher(owner),
		sessionHandlers{},
		newClientResourceLedger(owner),
	)
	cleanup := func() {
		session.Close()
		_ = peer.Close()
		<-session.Done()
	}
	return session, cleanup
}

func BenchmarkPhase8QueueFullAdmission(b *testing.B) {
	session, cleanup := phase8BlockedWriterSession(b)
	b.Cleanup(cleanup)
	payload := phase8Payload(64 << 10)
	saturated := false
	for i := 0; i < 10000; i++ {
		_, err := session.SendNoWait(payload)
		if errors.Is(err, ErrQueueFull) {
			saturated = true
			break
		}
		if err != nil {
			b.Fatalf("fill outgoing queue: %v", err)
		}
	}
	if !saturated {
		b.Skip("loopback peer accepted the full queue into the kernel; Queue Full state was not reached")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := session.SendNoWait(payload); !errors.Is(err, ErrQueueFull) {
			b.Fatalf("saturated SendNoWait error = %v, want ErrQueueFull", err)
		}
	}
	b.StopTimer()
	if elapsed := b.Elapsed(); elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed.Seconds(), "rejections/s")
	}
}

func BenchmarkScalePhase8Sessions(b *testing.B) {
	for _, size := range []int{64, 256} {
		for _, sessionCount := range []int{1, 8, 32, 128, 512} {
			name := fmt.Sprintf("payload=%s/sessions=%d/producers=%d",
				phase8PayloadName(size), sessionCount, sessionCount)
			b.Run(name, func(b *testing.B) {
				env := newPhase8BenchEnv(b, sessionCount, 1, 1, true, false)
				phase8RunRequestResponse(b, env, phase8Payload(size), sessionCount)
			})
		}
	}
}

func TestPhase8MemoryStabilitySoak(t *testing.T) {
	rawDuration := os.Getenv("NETA_PHASE8_SOAK_SECONDS")
	if rawDuration == "" {
		t.Skip("set NETA_PHASE8_SOAK_SECONDS to run the five-minute mixed-payload memory soak")
	}
	seconds, err := strconv.Atoi(rawDuration)
	if err != nil || seconds < 300 || seconds > 600 {
		t.Fatalf("NETA_PHASE8_SOAK_SECONDS must be between 300 and 600, got %q", rawDuration)
	}

	env := newPhase8BenchEnv(t, 8, 1, 1, true, false)
	payloads := [][]byte{
		phase8Payload(64),
		phase8Payload(256),
		phase8Payload(1024),
		phase8Payload(4096),
		phase8Payload(64 << 10),
	}
	const producers = 4
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+30)*time.Second)
	defer cancel()

	errCh := make(chan error, producers)
	var wg sync.WaitGroup
	for worker := 0; worker < producers; worker++ {
		session := env.sessions[worker]
		done := make(chan phase8CompletionResult, 1)
		handler := func(response Response, err error) {
			done <- phase8CompletionResult{response: response, err: err}
		}
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for operation := 0; ; operation++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				payload := payloads[(worker+operation)%len(payloads)]
				if _, err := session.SendRequest(payload, handler); err != nil {
					select {
					case errCh <- fmt.Errorf("producer %d SendRequest: %w", worker, err):
					default:
					}
					return
				}
				select {
				case result := <-done:
					if result.err != nil {
						select {
						case errCh <- fmt.Errorf("producer %d Completion: %w", worker, result.err):
						default:
						}
						return
					}
					if !phase8ResponseMatches(result.response, payload) {
						select {
						case errCh <- fmt.Errorf("producer %d response payload mismatch", worker):
						default:
						}
						return
					}
				case <-ctx.Done():
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(worker)
	}

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	started := time.Now()
	deadline := time.NewTimer(time.Duration(seconds) * time.Second)
	ticker := time.NewTicker(10 * time.Second)
	defer deadline.Stop()
	defer ticker.Stop()
	var maxHeapInuse uint64
	for {
		select {
		case err := <-errCh:
			cancel()
			wg.Wait()
			env.close(t)
			t.Fatalf("memory soak failed: %v", err)
		case <-ticker.C:
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			if stats.HeapInuse > maxHeapInuse {
				maxHeapInuse = stats.HeapInuse
			}
			serverUsage := env.server.resources.snapshot()
			t.Logf("elapsed=%s heap_alloc=%d heap_inuse=%d total_alloc=%d num_gc=%d goroutines=%d server_retained_app=%d server_queued_app=%d",
				time.Since(started).Round(time.Second), stats.HeapAlloc, stats.HeapInuse,
				stats.TotalAlloc, stats.NumGC, runtime.NumGoroutine(),
				serverUsage.retainedApplicationBytes, serverUsage.queuedApplicationBytes)
		case <-deadline.C:
			goto soakComplete
		case <-ctx.Done():
			t.Fatalf("memory soak context ended early: %v", ctx.Err())
		}
	}

soakComplete:
	cancel()
	wg.Wait()
	env.close(t)
	env.checkError(t)

	for i, client := range env.clients {
		usage := client.resources.snapshot()
		if usage.outstandingRequests != 0 || usage.retainedApplicationBytes != 0 ||
			usage.retainedCompletionBytes != 0 || usage.onCloseEventCredits != 0 {
			t.Errorf("Client %d resource leak after soak: %+v", i, usage)
		}
	}
	serverUsage := env.server.resources.snapshot()
	if serverUsage.outstandingRequests != 0 || serverUsage.retainedApplicationBytes != 0 ||
		serverUsage.retainedCompletionBytes != 0 || serverUsage.onCloseEventCredits != 0 ||
		serverUsage.queuedApplicationBytes != 0 || serverUsage.queuedApplicationFrames != 0 {
		t.Errorf("Server resource leak after soak: %+v", serverUsage)
	}

	runtime.GC()
	var final runtime.MemStats
	runtime.ReadMemStats(&final)
	t.Logf("soak complete: duration=%s baseline_heap=%d final_heap=%d max_heap_inuse=%d total_alloc=%d gc_cycles=%d goroutines=%d",
		time.Since(started).Round(time.Second), baseline.HeapAlloc, final.HeapAlloc,
		maxHeapInuse, final.TotalAlloc-baseline.TotalAlloc, final.NumGC-baseline.NumGC,
		runtime.NumGoroutine())
}

