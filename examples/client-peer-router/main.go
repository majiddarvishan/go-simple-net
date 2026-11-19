// Command client-peer-router is a bidirectional Client companion for the
// server-peer-router example. Both peers can originate Stream Requests.
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

type clientArgs struct {
	address  string
	systemID string
	window   time.Duration
	greeting string
}

func parseClientArgs(args []string) (clientArgs, error) {
	fs := flag.NewFlagSet("client-peer-router", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:19090", "server-peer-router listener address")
	id := fs.String("system-id", "peer-east", "short Bind label registered on Server")
	window := fs.Duration("window", 60*time.Second, "maximum demonstration duration")
	greeting := fs.String("greeting", "hello-from-peer", "initial Client-to-Server Request (empty disables)")
	if err := fs.Parse(args); err != nil {
		return clientArgs{}, err
	}
	if fs.NArg() > 0 {
		return clientArgs{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if *window <= 0 {
		return clientArgs{}, fmt.Errorf("-window must be positive")
	}
	return clientArgs{address: *addr, systemID: *id, window: *window, greeting: *greeting}, nil
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseClientArgs(args)
	if err != nil {
		return err
	}
	client, err := neta.NewClient(
		neta.WithClientAddress(cfg.address),
		neta.WithClientSystemID(cfg.systemID),
		neta.WithClientBindType(neta.BindTypeBiDirectional),
		neta.WithClientSessionOptions(neta.WithRequestTimeout(5*time.Second)),
		neta.WithClientOnBoundHandler(func(s *neta.Session) {
			log.Printf("peer bound: label=%q version=%d", cfg.systemID, s.ProtocolVersion())
		}),
		neta.WithClientOnRequestHandler(func(s *neta.Session, request neta.Request) {
			log.Printf("peer received Request: sequence=%d payload=%q", request.Sequence, request.Payload)
			reply := append([]byte("ack:"), request.Payload...)
			if err := s.SendResponse(request.Sequence, reply, 0); err != nil {
				log.Printf("peer SendResponse sequence=%d: %v", request.Sequence, err)
			}
		}),
		neta.WithClientOnCloseHandler(func(_ *neta.Session, cause error) {
			log.Printf("peer closed: %v", cause)
		}),
	)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Stop(stopCtx); err != nil && !errors.Is(err, neta.ErrClosed) {
			log.Printf("Client.Stop: %v", err)
		}
	}()
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = client.Connect(connectCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("Client.Connect: %w", err)
	}
	session, ok := client.CurrentSession()
	if !ok {
		return fmt.Errorf("Connect returned without a Bound Session")
	}
	if cfg.greeting != "" {
		seq, err := session.SendRequest([]byte(cfg.greeting), func(response neta.Response, completionErr error) {
			log.Printf("peer initial Completion: sequence=%d status=%d payload=%q err=%v",
				response.Sequence, response.Status, response.Payload, completionErr)
		})
		if err != nil {
			return fmt.Errorf("initial SendRequest: %w", err)
		}
		log.Printf("initial Client Request locally admitted: sequence=%d (not a business receipt)", seq)
	}
	log.Printf("connected to %s; responding to Server Requests for up to %s", cfg.address, cfg.window)
	timer := time.NewTimer(cfg.window)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil
	case <-session.Done():
		return fmt.Errorf("peer Session closed: %v", session.Err())
	case <-timer.C:
		return nil
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
