// Command server-config demonstrates strict optional JSON and explicit CLI
// overrides for the public Server and Session Functional Options.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

type serverConfig struct {
	ListenAddress       *string `json:"listen_address"`
	ProtocolVersion     *string `json:"protocol_version"`
	PreBindTimeout      *string `json:"pre_bind_handshake_timeout"`
	FrameTimeout        *string `json:"frame_assembly_timeout"`
	WriteTimeout        *string `json:"write_timeout"`
	MaxBoundSessions    *uint32 `json:"max_bound_sessions"`
	MaxPendingPreBind   *uint32 `json:"max_pending_pre_bind_sessions"`
	ApplicationWorkers  *uint32 `json:"application_workers"`
	CompletionWorkers   *uint32 `json:"completion_workers"`
	OnCloseWorkers      *uint32 `json:"on_close_workers"`
	OnCloseCredits      *uint32 `json:"on_close_event_credits"`
	AggregateQueueBytes *uint64 `json:"aggregate_queued_application_bytes"`
	AggregateQueueFrames *uint32 `json:"aggregate_queued_application_frames"`
	AppConcurrency      *uint32 `json:"application_callback_concurrency"`
	MaxIncomingFrames   *uint32 `json:"max_incoming_application_frames"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil { log.Fatal(err) }
}

func readServerJSON(path string) (serverConfig, error) {
	file, err := os.Open(path)
	if err != nil { return serverConfig{}, fmt.Errorf("open -config: %w", err) }
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config serverConfig
	if err := decoder.Decode(&config); err != nil { return serverConfig{}, fmt.Errorf("decode -config: %w", err) }
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil { return serverConfig{}, fmt.Errorf("-config must contain exactly one JSON object") }
		return serverConfig{}, fmt.Errorf("trailing -config data: %w", err)
	}
	return config, nil
}

func parseServerArgs(args []string) (serverConfig, error) {
	fs := flag.NewFlagSet("server-config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configFile := fs.String("config", "", "optional strict JSON config file")
	listen := fs.String("listen", "", "Server TCP address (use 127.0.0.1:0 for an ephemeral port)")
	protocol := fs.String("protocol", "", "v1 or v2 (default v2, accepts both)")
	preBind := fs.String("pre-bind-timeout", "", "e.g. 10s")
	frame := fs.String("frame-timeout", "", "e.g. 10s")
	write := fs.String("write-timeout", "", "e.g. 10s")
	maxBound := fs.Uint("max-bound", 0, "maximum Bound sessions")
	maxPending := fs.Uint("max-pending-prebind", 0, "maximum pending Bind sessions")
	appWorkers := fs.Uint("app-workers", 0, "Server-wide Application workers")
	completionWorkers := fs.Uint("completion-workers", 0, "Server-wide Completion workers")
	onCloseWorkers := fs.Uint("onclose-workers", 0, "Server-wide OnClose workers")
	onCloseCredits := fs.Uint("onclose-credits", 0, "Server-wide pre-admission OnClose credits")
	queueBytes := fs.Uint64("queue-bytes", 0, "Server-wide queued Application budget in bytes")
	queueFrames := fs.Uint("queue-frames", 0, "Server-wide queued Application frame count")
	appConcurrency := fs.Uint("app-concurrency", 0, "per-Session Application callback concurrency")
	maxIncoming := fs.Uint("max-incoming", 0, "per-Session incoming Application frames")
	if err := fs.Parse(args); err != nil { return serverConfig{}, err }
	if fs.NArg() > 0 { return serverConfig{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args()) }
	var cfg serverConfig
	var err error
	if *configFile != "" {
		cfg, err = readServerJSON(*configFile)
		if err != nil { return serverConfig{}, err }
	}
	var overflow error
	uint32Value := func(name string, value uint) *uint32 {
		if value > uint(^uint32(0)) { overflow = fmt.Errorf("%s overflows uint32", name); return nil }
		v := uint32(value)
		return &v
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen": cfg.ListenAddress = listen
		case "protocol": cfg.ProtocolVersion = protocol
		case "pre-bind-timeout": cfg.PreBindTimeout = preBind
		case "frame-timeout": cfg.FrameTimeout = frame
		case "write-timeout": cfg.WriteTimeout = write
		case "max-bound": cfg.MaxBoundSessions = uint32Value(f.Name, *maxBound)
		case "max-pending-prebind": cfg.MaxPendingPreBind = uint32Value(f.Name, *maxPending)
		case "app-workers": cfg.ApplicationWorkers = uint32Value(f.Name, *appWorkers)
		case "completion-workers": cfg.CompletionWorkers = uint32Value(f.Name, *completionWorkers)
		case "onclose-workers": cfg.OnCloseWorkers = uint32Value(f.Name, *onCloseWorkers)
		case "onclose-credits": cfg.OnCloseCredits = uint32Value(f.Name, *onCloseCredits)
		case "queue-bytes": cfg.AggregateQueueBytes = queueBytes
		case "queue-frames": cfg.AggregateQueueFrames = uint32Value(f.Name, *queueFrames)
		case "app-concurrency": cfg.AppConcurrency = uint32Value(f.Name, *appConcurrency)
		case "max-incoming": cfg.MaxIncomingFrames = uint32Value(f.Name, *maxIncoming)
		}
	})
	if overflow != nil { return serverConfig{}, overflow }
	return cfg, nil
}

func serverOptions(cfg serverConfig) ([]neta.ServerOption, error) {
	listen := "127.0.0.1:2775"
	if cfg.ListenAddress != nil { listen = *cfg.ListenAddress }
	options := []neta.ServerOption{neta.WithServerListenAddress(listen)}
	duration := func(name string, raw *string) (time.Duration, error) {
		d, err := time.ParseDuration(*raw)
		if err != nil { return 0, fmt.Errorf("%s %q: %w", name, *raw, err) }
		return d, nil
	}
	sessionOptions := []neta.SessionOption{}
	if cfg.ProtocolVersion != nil {
		switch strings.ToLower(*cfg.ProtocolVersion) {
		case "v1": sessionOptions = append(sessionOptions, neta.WithProtocolVersion(neta.ProtocolVersionV1))
		case "v2": sessionOptions = append(sessionOptions, neta.WithProtocolVersion(neta.ProtocolVersionV2))
		default: return nil, fmt.Errorf("protocol_version: expected v1/v2, got %q", *cfg.ProtocolVersion)
		}
	}
	if cfg.PreBindTimeout != nil {
		d, err := duration("pre_bind_handshake_timeout", cfg.PreBindTimeout)
		if err != nil { return nil, err }
		options = append(options, neta.WithServerPreBindHandshakeTimeout(d))
	}
	if cfg.FrameTimeout != nil {
		d, err := duration("frame_assembly_timeout", cfg.FrameTimeout)
		if err != nil { return nil, err }
		sessionOptions = append(sessionOptions, neta.WithFrameAssemblyTimeout(d))
	}
	if cfg.WriteTimeout != nil {
		d, err := duration("write_timeout", cfg.WriteTimeout)
		if err != nil { return nil, err }
		sessionOptions = append(sessionOptions, neta.WithWriteTimeout(d))
	}
	if cfg.AppConcurrency != nil { sessionOptions = append(sessionOptions, neta.WithApplicationCallbackConcurrency(*cfg.AppConcurrency)) }
	if cfg.MaxIncomingFrames != nil { sessionOptions = append(sessionOptions, neta.WithMaxIncomingApplicationFrames(*cfg.MaxIncomingFrames)) }
	if len(sessionOptions) > 0 { options = append(options, neta.WithServerSessionOptions(sessionOptions...)) }

	if cfg.MaxBoundSessions != nil { options = append(options, neta.WithServerMaxBoundSessions(*cfg.MaxBoundSessions)) }
	if cfg.MaxPendingPreBind != nil { options = append(options, neta.WithServerMaxPendingPreBindSessions(*cfg.MaxPendingPreBind)) }
	if cfg.ApplicationWorkers != nil { options = append(options, neta.WithServerApplicationWorkers(*cfg.ApplicationWorkers)) }
	if cfg.CompletionWorkers != nil { options = append(options, neta.WithServerCompletionWorkers(*cfg.CompletionWorkers)) }
	if cfg.OnCloseWorkers != nil { options = append(options, neta.WithServerOnCloseWorkers(*cfg.OnCloseWorkers)) }
	if cfg.OnCloseCredits != nil { options = append(options, neta.WithServerOnCloseEventCredits(*cfg.OnCloseCredits)) }
	if cfg.AggregateQueueBytes != nil { options = append(options, neta.WithServerMaxAggregateQueuedApplicationBytes(*cfg.AggregateQueueBytes)) }
	if cfg.AggregateQueueFrames != nil { options = append(options, neta.WithServerMaxAggregateQueuedApplicationFrames(*cfg.AggregateQueueFrames)) }
	return options, nil
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseServerArgs(args)
	if err != nil { return err }
	opts, err := serverOptions(cfg)
	if err != nil { return err }
	opts = append(opts,
		neta.WithServerOnBoundHandler(func(s *neta.Session) {
			log.Printf("session bound: system_id=%q protocol_version=%d", s.SystemID(), s.ProtocolVersion())
		}),
		neta.WithServerOnRequestHandler(func(s *neta.Session, r neta.Request) {
			// A demo-only application convention; not a wire-protocol feature.
			if bytes.HasPrefix(r.Payload, []byte("event:")) {
				log.Printf("one-way event sequence=%d", r.Sequence)
				return
			}
			payload := append([]byte("echo: "), r.Payload...)
			if err := s.SendResponse(r.Sequence, payload, 0); err != nil {
				log.Printf("SendResponse sequence=%d: %v", r.Sequence, err)
			}
		}),
		neta.WithServerOnCloseHandler(func(s *neta.Session, cause error) {
			log.Printf("session closed: system_id=%q cause=%v", s.SystemID(), cause)
		}),
	)
	server, err := neta.NewServer(opts...)
	if err != nil { return fmt.Errorf("validate server config: %w", err) }
	if err := server.Start(); err != nil { return fmt.Errorf("Server.Start: %w", err) }
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Stop(stopCtx); err != nil { log.Printf("Server.Stop: %v", err) }
	}()
	if server.Addr() == nil { return fmt.Errorf("server listening address is nil") }
	log.Printf("listening on %s; pair with examples/client-config -addr %q", server.Addr(), server.Addr().String())
	select {
	case <-ctx.Done():
		return nil
	case <-server.AcceptDone():
		if err := server.AcceptErr(); err != nil { return fmt.Errorf("Server.Accept: %w", err) }
		return nil
	}
}
