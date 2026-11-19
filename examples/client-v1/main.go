// Command client-v1 demonstrates explicit Pinex-compatible V1 Bind framing.
// V1 SystemID is max 19 bytes plus NUL; V2 default is 20 bytes without NUL.
// There is no automatic V2-to-V1 downgrade or capability negotiation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("client-v1", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:2775", "V1-only server address")
	systemID := fs.String("system-id", "neta-client-v1", "V1 Bind identity (at most 19 bytes)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if len(*systemID) == 0 || len(*systemID) > 19 {
		return fmt.Errorf("V1 SystemID length must be 1..19 bytes; got %d", len(*systemID))
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	client, err := neta.NewClient(
		neta.WithClientAddress(*addr),
		neta.WithClientSystemID(*systemID),
		neta.WithClientSessionOptions(
			neta.WithProtocolVersion(neta.ProtocolVersionV1),
			neta.WithRequestTimeout(10*time.Second),
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
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = client.Connect(connectCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("V1 Bind failed: %w (no automatic downgrade attempted)", err)
	}
	session, ok := client.CurrentSession()
	if !ok || session.ProtocolVersion() != neta.ProtocolVersionV1 {
		return fmt.Errorf("V1 Client not Bound as V1")
	}
	type result struct { response neta.Response; err error }
	done := make(chan result, 1)
	sequence, err := session.SendRequest([]byte("hello over V1"), func(r neta.Response, e error) {
		done <- result{response: r, err: e}
	})
	if err != nil {
		return fmt.Errorf("SendRequest: %w", err)
	}
	log.Printf("V1 sequence=%d locally admitted (not delivered)", sequence)
	select {
	case r := <-done:
		if r.err != nil {
			return fmt.Errorf("V1 Completion: %w", r.err)
		}
		if r.response.Sequence != sequence || r.response.Status != 0 {
			return fmt.Errorf("V1 Completion invalid: sequence=%d status=%d", r.response.Sequence, r.response.Status)
		}
		log.Printf("V1 Completion sequence=%d payload=%q", r.response.Sequence, r.response.Payload)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15*time.Second):
		return fmt.Errorf("V1 Completion timed out")
	}
}
