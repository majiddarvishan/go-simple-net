// Command server-concurrency accepts multiple bound clients with explicitly
// bounded callback workers and queue admission. This is a demo, not a benchmark.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("server-concurrency", flag.ContinueOnError)
	addr := fs.String("listen", "127.0.0.1:2775", "TCP listen address (use port 0 for ephemeral port)")
	maxBound := fs.Uint("max-bound", 16, "maximum simultaneous Bound sessions")
	appWorkers := fs.Uint("app-workers", 4, "global Application callback workers")
	appConcurrency := fs.Uint("app-concurrency", 2, "per-Session concurrent Application callbacks")
	queueFrames := fs.Uint("queue-frames", 128, "aggregate queued Application frames")
	queueBytes := fs.Uint64("queue-bytes", 1<<20, "aggregate queued Application bytes")
	onCloseCredits := fs.Uint("onclose-credits", 32, "pre-admission OnClose credits")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if *maxBound == 0 || *maxBound > 4096 || *appWorkers == 0 || *appWorkers > 512 ||
		*appConcurrency == 0 || *appConcurrency > 32 || *queueFrames == 0 || *queueFrames > 262144 ||
		*queueBytes == 0 || *onCloseCredits == 0 || *onCloseCredits > 4096 {
		return fmt.Errorf("capacity flags out of demonstration bounds")
	}
	var boundCount, requestCount, closeCount atomic.Uint64
	server, err := neta.NewServer(
		neta.WithServerListenAddress(*addr),
		neta.WithServerMaxBoundSessions(uint32(*maxBound)),
		neta.WithServerApplicationWorkers(uint32(*appWorkers)),
		neta.WithServerMaxAggregateQueuedApplicationFrames(uint32(*queueFrames)),
		neta.WithServerMaxAggregateQueuedApplicationBytes(*queueBytes),
		neta.WithServerOnCloseEventCredits(uint32(*onCloseCredits)),
		neta.WithServerSessionOptions(
			neta.WithApplicationCallbackConcurrency(uint32(*appConcurrency)),
			neta.WithMaxIncomingApplicationFrames(32),
		),
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			boundCount.Add(1)
			log.Printf("bound: system_id=%q active total=%d", s.SystemID(), boundCount.Load())
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, r neta.Request) {
			requestCount.Add(1)
			if bytes.HasPrefix(r.Payload, []byte("event:")) {
				log.Printf("one-way event sequence=%d", r.Sequence)
				return
			}
			response := append([]byte("echo: "), r.Payload...)
			if err := s.SendResponse(r.Sequence, response, 0); err != nil {
				log.Printf("SendResponse sequence=%d: %v", r.Sequence, err)
			}
		}),
		neta.WithServerOnCloseHandler(func(s *neta.Session, cause error) {
			closeCount.Add(1)
			log.Printf("closed: system_id=%q cause=%v", s.SystemID(), cause)
		}),
	)
	if err != nil {
		return fmt.Errorf("NewServer: %w", err)
	}
	if err := server.Start(); err != nil {
		return fmt.Errorf("Server.Start: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if stopErr := server.Stop(stopCtx); stopErr != nil {
			log.Printf("Server.Stop: %v", stopErr)
		}
		log.Printf("summary: bound_total=%d requests_received=%d closed_callbacks=%d", boundCount.Load(), requestCount.Load(), closeCount.Load())
	}()
	log.Printf("listening on %s; configure multiple client-concurrent processes with different -system-id", server.Addr())
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	select {
	case <-ctx.Done():
		return nil
	case <-server.AcceptDone():
		if err := server.AcceptErr(); err != nil {
			return fmt.Errorf("Accept: %w", err)
		}
		return nil
	}
}
