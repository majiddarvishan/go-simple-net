// Command server-smpp-multi-me demonstrates a config-driven neta-go
// listener with bounded concurrency and routing to multiple ME Sessions.
// This is internal TCP + opaque application payload, NOT the SMPP protocol.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func buildServer(cfg demoConfig, registry *meRegistry) (*neta.Server, error) {
	preBind, _ := time.ParseDuration(cfg.PreBindTimeout)
	frameTimeout, _ := time.ParseDuration(cfg.FrameTimeout)
	writeTimeout, _ := time.ParseDuration(cfg.WriteTimeout)
	reqTimeout, _ := time.ParseDuration(cfg.RequestTimeout)
	version := neta.ProtocolVersionV2
	if cfg.ProtocolVersion == "v1" {
		version = neta.ProtocolVersionV1
	}
	options := []neta.ServerOption{
		neta.WithServerListenAddress(cfg.ListenAddress),
		neta.WithServerPreBindHandshakeTimeout(preBind),
		neta.WithServerMaxBoundSessions(cfg.MaxBoundSessions),
		neta.WithServerMaxPendingPreBindSessions(cfg.MaxPendingPreBind),
		neta.WithServerApplicationWorkers(cfg.ApplicationWorkers),
		neta.WithServerCompletionWorkers(cfg.CompletionWorkers),
		neta.WithServerOnCloseWorkers(cfg.OnCloseWorkers),
		neta.WithServerOnCloseEventCredits(cfg.OnCloseCredits),
		neta.WithServerMaxAggregateQueuedApplicationBytes(cfg.QueueBytes),
		neta.WithServerMaxAggregateQueuedApplicationFrames(cfg.QueueFrames),
		neta.WithServerSessionOptions(
			neta.WithProtocolVersion(version),
			neta.WithFrameAssemblyTimeout(frameTimeout),
			neta.WithWriteTimeout(writeTimeout),
			neta.WithRequestTimeout(reqTimeout),
			neta.WithApplicationCallbackConcurrency(cfg.AppConcurrency),
			neta.WithMaxIncomingApplicationFrames(cfg.MaxIncomingFrames),
		),
		neta.WithBindValidator(registry.validateBind),
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			registry.onBound(s)
			log.Printf("ME Bound: bind_id=%q protocol=%d", s.SystemID(), s.ProtocolVersion())
		}),
		neta.WithServerOnCloseHandler(func(s *neta.Session, cause error) {
			registry.onClose(s)
			log.Printf("ME Closed: bind_id=%q cause=%v", s.SystemID(), cause)
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, r neta.Request) {
			// An echo keeps the existing example Client usable for a demo.
			// Real SMSC handlers must use the approved application Protobuf,
			// durable outcomes, bounded handoff and business correlation IDs.
			if bytes.HasPrefix(r.Payload, []byte("event:")) {
				return
			}
			if err := s.SendResponse(r.Sequence, append([]byte("echo: "), r.Payload...), 0); err != nil {
				log.Printf("demo response failed: %v", err)
			}
		}),
	}
	return neta.NewServer(options...)
}

func consoleCommands(ctx context.Context, registry *meRegistry) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 64<<10)
	log.Printf("demo commands: list | send <route-key> <text> | help; Ctrl-C to stop")
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "help" {
			log.Printf("list | send <route-key> <text> | help")
			continue
		}
		if line == "list" {
			for _, me := range registry.peers() {
				log.Printf("ME id=%s bind=%s online=%t", me.MEID, me.SystemID, me.Online)
			}
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 || parts[0] != "send" || parts[1] == "" || parts[2] == "" {
			if line != "" {
				log.Printf("invalid command (use list or send <route-key> <text>)")
			}
			continue
		}
		meID, seq, err := registry.dispatch(parts[1], []byte(parts[2]), func(resp neta.Response, completionErr error) {
			log.Printf("outbound completion: response_seq=%d status=%d err=%v (not a business ACK)",
				resp.Sequence, resp.Status, completionErr)
		})
		if err != nil {
			log.Printf("route=%q me=%q not admitted: %v", parts[1], meID, err)
		} else {
			log.Printf("route=%q me=%q sequence=%d locally admitted (not delivered/processed)",
				parts[1], meID, seq)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("console disabled: %v", err)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	registry := newMERegistry(cfg)
	srv, err := buildServer(cfg, registry)
	if err != nil {
		return fmt.Errorf("NewServer: %w", err)
	}
	if err := srv.Start(); err != nil {
		return fmt.Errorf("Server.Start: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Stop(stopCtx); err != nil {
			log.Printf("Server.Stop: %v", err)
		}
	}()
	log.Printf("internal neta listener=%s configured_MEs=%d routes=%d", srv.Addr(), len(cfg.MessageEngines), len(cfg.Routes))
	go consoleCommands(ctx, registry)
	select {
	case <-ctx.Done():
		return nil
	case <-srv.AcceptDone():
		if err := srv.AcceptErr(); err != nil {
			return fmt.Errorf("Accept: %w", err)
		}
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
