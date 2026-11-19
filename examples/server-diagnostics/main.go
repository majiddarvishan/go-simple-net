// Command server-diagnostics shows bounded callback panic diagnostics,
// on-close lifecycle, and explicit opt-in demonstration failures.
// There is no authentication or application delivery guarantee in this demo.
package main

import (
	"bytes"
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
	fs := flag.NewFlagSet("server-diagnostics", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:2775", "TCP listening address")
	dropTimeout := fs.Bool("drop-timeout-demo", false, "opt in to withholding Responses for no-response: Requests")
	allowPanic := fs.Bool("panic-on-request", false, "opt in to intentional OnRequest panic for panic: Requests")
	ring := fs.Uint("panic-ring", 4, "recent panic entries (1..32)")
	budget := fs.Uint64("panic-byte-budget", 64<<10, "recent panic text byte budget (65536..2097152)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if *ring == 0 || *ring > 32 || *budget < 64<<10 || *budget > 2<<20 {
		return fmt.Errorf("invalid panic ring or recent diagnostic byte budget")
	}

	server, err := neta.NewServer(
		neta.WithServerListenAddress(*listen),
		neta.WithServerSessionOptions(
			neta.WithCallbackPanicRingCapacity(uint32(*ring)),
			neta.WithCallbackPanicByteBudget(*budget),
			neta.WithRequestTimeout(5*time.Second),
		),
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			log.Printf("OnBound: SystemID=%q Session=%p", s.SystemID(), s)
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, r neta.Request) {
			log.Printf("OnRequest: session=%p sequence=%d bytes=%d", s, r.Sequence, len(r.Payload))
			if *allowPanic && bytes.HasPrefix(r.Payload, []byte("panic:")) {
				panic("intentional server diagnostics demo panic")
			}
			if *dropTimeout && bytes.HasPrefix(r.Payload, []byte("no-response:")) {
				log.Printf("withholding response sequence=%d to demonstrate Client RequestTimeout", r.Sequence)
				return
			}
			payload := append([]byte("echo: "), r.Payload...)
			if err := s.SendResponse(r.Sequence, payload, 0); err != nil {
				log.Printf("SendResponse sequence=%d failed: %v", r.Sequence, err)
			}
		}),
		neta.WithServerOnCloseHandler(func(s *neta.Session, cause error) {
			snap := s.Diagnostics()
			panicDiag := snap.CallbackPanics
			log.Printf("OnClose: session=%p cause=%v first_panic=%t recent=%d dropped=%d", s, cause, panicDiag.First != nil, len(panicDiag.Recent), panicDiag.Dropped)
			if panicDiag.First != nil {
				log.Printf("first recovered callback=%q type=%q value=%q", panicDiag.First.Callback, panicDiag.First.ValueType, panicDiag.First.ValueText)
			}
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
		// Server.Stop waits for transport teardown, not arbitrary callbacks.
		log.Printf("Server.Stop returned; any already-started user callback may still be running")
	}()
	log.Printf("listening on %s; panic-on-request=%v drop-timeout-demo=%v", server.Addr(), *allowPanic, *dropTimeout)
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
