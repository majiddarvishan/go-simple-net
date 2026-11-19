// Command client-diagnostics demonstrates Completion errors and typed Session
// diagnostics. Set -mode=panic explicitly to panic in its own Completion;
// the library recovers and closes that Session without process termination.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

type completionResult struct {
	response neta.Response
	err      error
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("client-diagnostics", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:2775", "paired server-diagnostics TCP address")
	id := fs.String("system-id", "neta-diagnostics", "Bind SystemID")
	mode := fs.String("mode", "ok", "ok, timeout, or panic (panic is opt-in)")
	timeout := fs.Duration("timeout", 2*time.Second, "RequestTimeout, must be positive")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional args: %v", fs.Args())
	}
	if *timeout <= 0 || *timeout > time.Minute {
		return fmt.Errorf("-timeout must be between 1ns and 1m")
	}
	switch *mode {
	case "ok", "timeout", "panic":
	default:
		return fmt.Errorf("-mode must be ok, timeout or panic")
	}
	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()
	closed := make(chan error, 1)
	client, err := neta.NewClient(
		neta.WithClientAddress(*addr),
		neta.WithClientSystemID(*id),
		neta.WithClientSessionOptions(
			neta.WithRequestTimeout(*timeout),
			neta.WithCallbackPanicRingCapacity(4),
			neta.WithCallbackPanicByteBudget(64<<10),
		),
		neta.WithClientOnCloseHandler(func(s *neta.Session, cause error) {
			log.Printf("OnClose: session=%p cause=%v", s, cause)
			select { case closed <- cause: default: }
		}),
	)
	if err != nil { return fmt.Errorf("NewClient: %w", err) }
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if stopErr := client.Stop(stopCtx); stopErr != nil { log.Printf("Client.Stop: %v", stopErr) }
	}()
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	err = client.Connect(connectCtx)
	connectCancel()
	if err != nil { return fmt.Errorf("Connect: %w", err) }
	session, ok := client.CurrentSession()
	if !ok { return fmt.Errorf("no Bound Session") }
	payload := []byte("diagnostics:echo")
	if *mode == "timeout" { payload = []byte("no-response:demo") }
	resultCh := make(chan completionResult, 1)
	seq, err := session.SendRequest(payload, func(r neta.Response, completionErr error) {
		if *mode == "panic" {
			panic("intentional demo Completion panic")
		}
		resultCh <- completionResult{response: r, err: completionErr}
	})
	if err != nil { return fmt.Errorf("SendRequest admission: %w", err) }
	log.Printf("sequence=%d locally admitted; waiting for Completion (not proof of delivery)", seq)

	if *mode == "panic" {
		select {
		case <-session.Done():
		case <-ctx.Done():
			return fmt.Errorf("waiting for panic-induced Session.Done: %w", ctx.Err())
		}
		// Done is not a join of arbitrary callbacks, and subsequent
		// Diagnostics snapshots may still gain late records.
		for {
			snapshot := session.Diagnostics()
			if first := snapshot.CallbackPanics.First; first != nil {
				log.Printf("recovered Completion panic: callback=%s type=%s text=%q truncated=%v recent=%d dropped=%d", first.Callback, first.ValueType, first.ValueText, first.ValueTruncated, len(snapshot.CallbackPanics.Recent), snapshot.CallbackPanics.Dropped)
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("panic diagnostics unavailable before demo deadline: %w", ctx.Err())
			case <-time.After(20 * time.Millisecond):
			}
		}
		select {
		case cause := <-closed:
			if !errors.Is(cause, neta.ErrCallbackPanic) {
				return fmt.Errorf("unexpected OnClose cause after panic: %v", cause)
			}
		case <-time.After(3*time.Second):
			log.Print("OnClose callback has not returned; Session.Done does not join callbacks")
		}
		return nil
	}
	select {
	case result := <-resultCh:
		if result.err != nil {
			log.Printf("Completion sequence=%d error=%v", seq, result.err)
			if *mode != "timeout" || !errors.Is(result.err, neta.ErrRequestTimeout) {
				return fmt.Errorf("unexpected Completion error: %w", result.err)
			}
			log.Printf("expected RequestTimeout occurred; Diagnostics=%+v", session.Diagnostics().CallbackPanics)
			return nil
		}
		if *mode == "timeout" {
			return fmt.Errorf("timeout mode received Response; paired server must use -drop-timeout-demo=true")
		}
		if result.response.Sequence != seq || result.response.Status != 0 {
			return fmt.Errorf("Completion: mismatched sequence/status %d/%d", result.response.Sequence, result.response.Status)
		}
		log.Printf("Completion sequence=%d status=%d payload=%q", result.response.Sequence, result.response.Status, result.response.Payload)
		snapshot := session.Diagnostics()
		log.Printf("diagnostics: recent_panics=%d dropped=%d", len(snapshot.CallbackPanics.Recent), snapshot.CallbackPanics.Dropped)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
