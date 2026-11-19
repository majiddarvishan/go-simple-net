// Command server-bind-validator demonstrates a demo-only SystemID allowlist.
// An allowlist is NOT authentication: Bind has no cryptographic proof.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func parseAllowlist(text string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{})
	for _, token := range strings.Split(text, ",") {
		id := strings.TrimSpace(token)
		if id == "" || len(id) > 20 || strings.ContainsRune(id, 0) {
			return nil, fmt.Errorf("allowlist identity must be 1..20 bytes, nonempty and NUL-free")
		}
		if _, duplicate := allowed[id]; duplicate {
			return nil, fmt.Errorf("duplicate configured SystemID in allowlist")
		}
		allowed[id] = struct{}{}
	}
	return allowed, nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("server-bind-validator", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:2775", "TCP listening address")
	allow := fs.String("allow", "neta-client-config,neta-reconnect,neta-concurrent", "comma-separated demonstration System IDs")
	version := fs.String("protocol", "v2", "v1 only, or v2 (accepts v1 and v2 binds)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected positional args: %v", fs.Args())
	}
	allowed, err := parseAllowlist(*allow)
	if err != nil {
		return err
	}
	var selected neta.ProtocolVersion
	switch strings.ToLower(*version) {
	case "v1":
		selected = neta.ProtocolVersionV1
		for id := range allowed {
			if len(id) > 19 {
				return fmt.Errorf("allowlisted SystemID %q is too long for V1 (19 bytes)", id)
			}
		}
	case "v2":
		selected = neta.ProtocolVersionV2
	default:
		return fmt.Errorf("unknown protocol %q: choose v1 or v2", *version)
	}
	server, err := neta.NewServer(
		neta.WithServerListenAddress(*listen),
		neta.WithServerSessionOptions(neta.WithProtocolVersion(selected)),
		neta.WithBindValidator(func(info neta.BindInfo) error {
			if _, ok := allowed[info.SystemID]; !ok {
				// Avoid remote address/opaque credentials in rejection logs.
				log.Printf("Bind rejected by demo allowlist (ID not authorized)")
				return fmt.Errorf("demo allowlist rejects this SystemID")
			}
			log.Printf("Bind accepted by demo allowlist: identity=%q", info.SystemID)
			// neta-go additionally rejects duplicate active SystemIDs;
			// this callback does not override its admission ledger.
			return nil
		}),
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			log.Printf("Bound: system_id=%q protocol=%d", s.SystemID(), s.ProtocolVersion())
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, req neta.Request) {
			if bytes.HasPrefix(req.Payload, []byte("event:")) {
				return
			}
			reply := append([]byte("echo: "), req.Payload...)
			if err := s.SendResponse(req.Sequence, reply, 0); err != nil {
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
	log.Printf("listening on %s; allowlist is demonstrative, not secure authentication", server.Addr())
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
