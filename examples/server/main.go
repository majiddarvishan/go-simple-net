// Command server demonstrates the public neta Server API for the
// companion examples/client program.
//
// It accepts non-empty Bind System IDs for demonstration only. Production
// services should apply their own identity/authentication policy in the
// BindValidator.
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

	"github.com/majiddarvishan/neta-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	listenAddress := flag.String("listen", "127.0.0.1:2775", "TCP listen address")
	flag.Parse()

	processCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	server, err := neta.NewServer(
		neta.WithServerListenAddress(*listenAddress),
		neta.WithBindValidator(func(info neta.BindInfo) error {
			if info.SystemID == "" {
				return fmt.Errorf("Bind System ID must not be empty")
			}
			log.Printf("Bind request: system_id=%q remote=%v", info.SystemID, info.RemoteAddr)
			return nil
		}),
		neta.WithServerOnBoundHandler(func(session *neta.Session) {
			log.Printf("session bound: system_id=%q remote=%v", session.SystemID(), session.RemoteAddr())
		}),
		neta.WithServerOnRequestHandler(func(session *neta.Session, request neta.Request) {
			// The example uses an application-level prefix to distinguish a
			// one-way event from a Request that expects a Response.
			if bytes.HasPrefix(request.Payload, []byte("event:")) {
				log.Printf("one-way event sequence=%d payload=%q", request.Sequence, request.Payload)
				return
			}

			responsePayload := append([]byte("echo: "), request.Payload...)
			if sendErr := session.SendResponse(request.Sequence, responsePayload, 0); sendErr != nil {
				log.Printf("SendResponse sequence=%d: %v", request.Sequence, sendErr)
			}
		}),
		neta.WithServerOnCloseHandler(func(session *neta.Session, cause error) {
			log.Printf("session closed: system_id=%q cause=%v", session.SystemID(), cause)
		}),
		neta.WithServerSessionOptions(
			neta.WithWriteTimeout(15*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("construct Server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("start Server: %w", err)
	}
	address := server.Addr()
	if address == nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
		return fmt.Errorf("Server started without a listener address")
	}
	log.Printf("listening on %s; start examples/client with -addr %q", address, address.String())

	<-processCtx.Done()

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()
	if err := server.Stop(stopCtx); err != nil {
		return fmt.Errorf("graceful Server Stop: %w", err)
	}
	log.Printf("Server stopped")
	return nil
}
