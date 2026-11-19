// Command client-concurrent demonstrates bounded concurrent Request/Response.
// SendRequest success is local admission only, never a delivery guarantee.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

type completion struct {
	response neta.Response
	err      error
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("client-concurrent", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:2775", "server address")
	systemID := fs.String("system-id", "neta-concurrent", "unique Bind SystemID per client")
	count := fs.Int("count", 16, "number of Requests, 1..1000")
	inflight := fs.Int("inflight", 4, "parallel request workers / Session Request credits, 1..32")
	timeout := fs.Duration("timeout", 8*time.Second, "per-request completion timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if *count < 1 || *count > 1000 || *inflight < 1 || *inflight > 32 || *timeout <= 0 {
		return fmt.Errorf("expected count 1..1000, inflight 1..32 and a positive timeout")
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	// An outer deadline guarantees this demo terminates without a user signal.
	workCtx, cancelWork := context.WithTimeout(ctx, time.Duration(*count+1)*(*timeout)+15*time.Second)
	defer cancelWork()
	client, err := neta.NewClient(
		neta.WithClientAddress(*addr),
		neta.WithClientSystemID(*systemID),
		neta.WithClientSessionOptions(
			neta.WithMaxOutstandingRequestsPerSession(uint32(*inflight)),
			neta.WithRequestTimeout(*timeout),
		),
	)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if stopErr := client.Stop(stopCtx); stopErr != nil {
			log.Printf("Client.Stop: %v", stopErr)
		}
	}()
	connectCtx, cancelConnect := context.WithTimeout(workCtx, 10*time.Second)
	err = client.Connect(connectCtx)
	cancelConnect()
	if err != nil {
		return fmt.Errorf("Connect: %w", err)
	}
	session, ok := client.CurrentSession()
	if !ok {
		return fmt.Errorf("missing Bound Session")
	}

	// Exactly inflight worker goroutines. Jobs are unbuffered and each
	// Completion is delivered to a one-slot channel, never blocking a
	// callback after this worker exits or its context expires.
	jobs := make(chan int)
	var wg sync.WaitGroup
	var admitted, successful, failed atomic.Int64
	for worker := 0; worker < *inflight; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				payload := []byte(fmt.Sprintf("request-%04d", id))
				resultCh := make(chan completion, 1)
				sequence, sendErr := session.SendRequest(payload, func(r neta.Response, callbackErr error) {
					resultCh <- completion{response: r, err: callbackErr}
				})
				if sendErr != nil {
					failed.Add(1)
					log.Printf("id=%d local admission failed: %v", id, sendErr)
					continue
				}
				admitted.Add(1)
				log.Printf("id=%d sequence=%d locally admitted; peer delivery not confirmed", id, sequence)
				waitCtx, cancel := context.WithTimeout(workCtx, *timeout+time.Second)
				select {
				case result := <-resultCh:
					if result.err != nil || result.response.Status != 0 ||
						result.response.Sequence != sequence ||
						!bytes.Equal(result.response.Payload, append([]byte("echo: "), payload...)) {
						failed.Add(1)
						log.Printf("id=%d sequence=%d Completion status=%d err=%v payload=%q", id, sequence, result.response.Status, result.err, result.response.Payload)
					} else {
						successful.Add(1)
						log.Printf("id=%d sequence=%d Completion OK", id, sequence)
					}
				case <-waitCtx.Done():
					failed.Add(1)
					log.Printf("id=%d sequence=%d Completion wait ended: %v (request may still be in flight)", id, sequence, waitCtx.Err())
				}
				cancel()
			}
		}()
	}
dispatch:
	for id := 1; id <= *count; id++ {
		select {
		case jobs <- id:
		case <-workCtx.Done():
			break dispatch
		}
	}
	close(jobs)
	wg.Wait()
	log.Printf("finished: requested=%d locally_admitted=%d completions_ok=%d failures=%d", *count, admitted.Load(), successful.Load(), failed.Load())
	if err := workCtx.Err(); err != nil {
		return fmt.Errorf("work window ended: %w", err)
	}
	if failed.Load() != 0 || successful.Load() != int64(*count) {
		return fmt.Errorf("some Requests failed or did not complete: successful=%d failed=%d", successful.Load(), failed.Load())
	}
	return nil
}
