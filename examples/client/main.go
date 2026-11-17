// Command client demonstrates the public neta Client API against the
// companion examples/server program.
//
// Start the server first, then run this command. A successful SendRequest or
// SendNoWait return confirms local admission only; it is not a peer-delivery
// acknowledgement.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/majiddarvishan/neta-go"
)

type completionResult struct {
	response neta.Response
	err      error
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	address := flag.String("addr", "127.0.0.1:2775", "Server TCP address")
	systemID := flag.String("system-id", "neta-example-client", "Client Bind System ID")
	flag.Parse()

	processCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	client, err := neta.NewClient(
		neta.WithClientAddress(*address),
		neta.WithClientSystemID(*systemID),
		neta.WithClientOnBoundHandler(func(session *neta.Session) {
			log.Printf("bound: system_id=%q remote=%v", session.SystemID(), session.RemoteAddr())
		}),
		neta.WithClientOnCloseHandler(func(_ *neta.Session, cause error) {
			log.Printf("session closed: %v", cause)
		}),
		neta.WithClientSessionOptions(
			neta.WithRequestTimeout(15*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("construct Client: %w", err)
	}

	// Stop is terminal and idempotent. It gracefully Unbinds the active Session
	// and waits for internal teardown, but it does not join arbitrary callbacks.
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if stopErr := client.Stop(stopCtx); stopErr != nil {
			log.Printf("Client Stop: %v", stopErr)
		}
	}()

	connectCtx, cancelConnect := context.WithTimeout(processCtx, 10*time.Second)
	err = client.Connect(connectCtx)
	cancelConnect()
	if err != nil {
		return fmt.Errorf("connect and Bind: %w", err)
	}

	session, ok := client.CurrentSession()
	if !ok {
		return fmt.Errorf("Connect returned successfully but no Bound Session is available")
	}

	completed := make(chan completionResult, 1)
	sequence, err := session.SendRequest([]byte("hello from neta-go"), func(response neta.Response, completionErr error) {
		completed <- completionResult{response: response, err: completionErr}
	})
	if err != nil {
		return fmt.Errorf("SendRequest: %w", err)
	}
	log.Printf("Request sequence=%d admitted locally", sequence)

	oneWaySequence, err := session.SendNoWait([]byte("event: client-started"))
	if err != nil {
		return fmt.Errorf("SendNoWait: %w", err)
	}
	log.Printf("one-way sequence=%d admitted locally; no Completion is expected", oneWaySequence)

	select {
	case result := <-completed:
		if result.err != nil {
			return fmt.Errorf("Request sequence %d completed with error: %w", sequence, result.err)
		}
		if result.response.Status != 0 {
			return fmt.Errorf("Request sequence %d returned status %d", sequence, result.response.Status)
		}
		log.Printf("Completion sequence=%d payload=%q", result.response.Sequence, result.response.Payload)
	case <-processCtx.Done():
		return nil
	}

	log.Printf("client is running; press Ctrl-C to gracefully Unbind and stop")
	<-processCtx.Done()
	return nil
}
