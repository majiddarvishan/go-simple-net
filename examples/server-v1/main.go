// Command server-v1 accepts ONLY legacy V1 Bind framing. The default Go V2
// client cannot Bind: configure WithProtocolVersion(ProtocolVersionV1) on it.
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
	fs := flag.NewFlagSet("server-v1", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:2775", "V1-only TCP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	server, err := neta.NewServer(
		neta.WithServerListenAddress(*listen),
		neta.WithServerSessionOptions(neta.WithProtocolVersion(neta.ProtocolVersionV1)),
		neta.WithBindValidator(func(info neta.BindInfo) error {
			if len(info.SystemID) == 0 || len(info.SystemID) > 19 {
				return fmt.Errorf("V1 SystemID exceeds 19-byte boundary")
			}
			log.Printf("valid V1 Bind identity=%q", info.SystemID)
			return nil
		}),
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			log.Printf("V1 Bound system_id=%q protocol=%d", s.SystemID(), s.ProtocolVersion())
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, r neta.Request) {
			if bytes.HasPrefix(r.Payload, []byte("event:")) {
				return
			}
			data := append([]byte("echo: "), r.Payload...)
			if err := s.SendResponse(r.Sequence, data, 0); err != nil {
				log.Printf("SendResponse: %v", err)
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
	}()
	log.Printf("listening on %s; V1 ONLY (V2 Bind rejected), pair with examples/client-v1", server.Addr())
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
