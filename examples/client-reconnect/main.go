// Command client-reconnect observes successive Client Session generations.
// Restart the paired Server after an abrupt transport loss to see reconnect.
// No Request is automatically sent, replayed or deduplicated by this demo.
package main

import (
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
	fs := flag.NewFlagSet("client-reconnect", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:2775", "server address")
	id := fs.String("system-id", "neta-reconnect", "Bind SystemID")
	window := fs.Duration("window", 45*time.Second, "bounded demonstration window (1s..10m)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional args: %v", fs.Args())
	}
	if *window < time.Second || *window > 10*time.Minute {
		return fmt.Errorf("-window must be between 1s and 10m")
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(rootCtx, *window)
	defer cancel()
	var observed atomic.Uint64
	client, err := neta.NewClient(
		neta.WithClientAddress(*addr),
		neta.WithClientSystemID(*id),
		neta.WithAutoReconnect(true),
		neta.WithClientOnBoundHandler(func(session *neta.Session) {
			n := observed.Add(1)
			log.Printf("OnBound generation-observation=%d session=%p system_id=%q", n, session, session.SystemID())
		}),
		neta.WithClientOnCloseHandler(func(session *neta.Session, cause error) {
			log.Printf("OnClose session=%p terminal_cause=%v", session, cause)
		}),
	)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if stopErr := client.Stop(stopCtx); stopErr != nil {
			log.Printf("Client.Stop: %v", stopErr)
		}
	}()

	log.Print("reconnect is opt-in. For a retryable transport loss, abruptly terminate and restart the Server on the same address. A graceful Unbind is not retryable.")
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	initialErr := client.Connect(connectCtx)
	connectCancel()
	if initialErr != nil {
		// A dial failure may activate the reconnect supervisor. A rejected
		// Bind is terminal and will not be retried as a compatibility probe.
		log.Printf("initial Connect failed: %v; observing bounded window", initialErr)
	} else {
		log.Print("initial Connect is Bound; no messages are being sent")
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var last *neta.Session
	for {
		select {
		case <-ctx.Done():
			if rootCtx.Err() != nil {
				return nil
			}
			log.Printf("window ended; observed OnBound callbacks=%d; no messages replayed", observed.Load())
			return nil
		case <-ticker.C:
			current, bound := client.CurrentSession()
			if !bound {
				if last != nil {
					log.Printf("CurrentSession unavailable after prior Bound generation %p", last)
					last = nil
				}
				continue
			}
			if current != last {
				last = current
				log.Printf("CurrentSession now Bound: session=%p protocol=%d", current, current.ProtocolVersion())
			}
		}
	}
}
